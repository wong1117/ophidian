package config

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server   ServerConfig   `yaml:"server" json:"server"`
	Database DatabaseConfig `yaml:"database" json:"database"`
	Redis    RedisConfig    `yaml:"redis" json:"redis"`
	AI       AIConfig       `yaml:"ai" json:"ai"`
	Logging  LoggingConfig  `yaml:"logging" json:"logging"`
	Auth     AuthConfig     `yaml:"auth" json:"auth"`
}

type ServerConfig struct {
	Host         string `yaml:"host" json:"host"`
	Port         int    `yaml:"port" json:"port"`
	ReadTimeout  int    `yaml:"read_timeout" json:"read_timeout"`
	WriteTimeout int    `yaml:"write_timeout" json:"write_timeout"`
}

type DatabaseConfig struct {
	Host     string `yaml:"host" json:"host"`
	Port     int    `yaml:"port" json:"port"`
	User     string `yaml:"user" json:"user"`
	Password string `yaml:"password" json:"password"`
	Database string `yaml:"database" json:"database"`
	SSLMode  string `yaml:"ssl_mode" json:"ssl_mode"`
	MaxConns int    `yaml:"max_conns" json:"max_conns"`
}

type RedisConfig struct {
	Host     string `yaml:"host" json:"host"`
	Port     int    `yaml:"port" json:"port"`
	Password string `yaml:"password" json:"password"`
	DB       int    `yaml:"db" json:"db"`
}

type AIConfig struct {
	Provider    string  `yaml:"provider" json:"provider"`
	Model       string  `yaml:"model" json:"model"`
	APIKey      string  `yaml:"api_key" json:"api_key"`
	MaxTokens   int     `yaml:"max_tokens" json:"max_tokens"`
	Temperature float64 `yaml:"temperature" json:"temperature"`
}

type LoggingConfig struct {
	Level  string `yaml:"level" json:"level"`
	Format string `yaml:"format" json:"format"`
	Output string `yaml:"output" json:"output"`
}

type AuthConfig struct {
	JWTSecret    string `yaml:"jwt_secret" json:"jwt_secret"`
	TokenExpiry  int    `yaml:"token_expiry" json:"token_expiry"`
}

type Repository interface {
	Save(ctx context.Context, version int, cfg *Config) error
	Load(ctx context.Context, version int) (*Config, error)
	LatestVersion(ctx context.Context) (int, error)
}

func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Host:         "0.0.0.0",
			Port:         8080,
			ReadTimeout:  30,
			WriteTimeout: 30,
		},
		Database: DatabaseConfig{
			Host:     "localhost",
			Port:     5432,
			SSLMode:  "disable",
			MaxConns: 20,
		},
		Redis: RedisConfig{
			Host: "localhost",
			Port: 6379,
		},
		AI: AIConfig{
			MaxTokens:   4096,
			Temperature: 0.7,
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "json",
			Output: "stdout",
		},
		Auth: AuthConfig{
			TokenExpiry: 3600,
		},
	}
}

type Loader struct {
	path string
}

func NewLoader(path string) *Loader {
	return &Loader{path: path}
}

func (l *Loader) Load() (*Config, error) {
	cfg := DefaultConfig()

	if l.path != "" {
		data, err := os.ReadFile(l.path)
		if err != nil {
			if !os.IsNotExist(err) {
				return nil, fmt.Errorf("load config file: %w", err)
			}
		} else {
			if err := yaml.Unmarshal(data, cfg); err != nil {
				return nil, fmt.Errorf("parse config file: %w", err)
			}
		}
	}

	applyEnvOverrides(cfg)
	resolveSecrets(cfg)

	return cfg, nil
}

func applyEnvOverrides(cfg *Config) {
	v := reflect.ValueOf(cfg).Elem()
	applyEnvOverride("", v)
}

func applyEnvOverride(prefix string, v reflect.Value) {
	t := v.Type()
	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		fieldType := t.Field(i)

		yamlKey := fieldType.Tag.Get("yaml")
		if yamlKey == "" {
			yamlKey = fieldType.Name
		}

		var envKey string
		if prefix != "" {
			envKey = prefix + "_" + strings.ToUpper(yamlKey)
		} else {
			envKey = "APP_" + strings.ToUpper(yamlKey)
		}

		if field.Kind() == reflect.Struct {
			applyEnvOverride(envKey, field)
			continue
		}

		if envVal := os.Getenv(envKey); envVal != "" {
			switch field.Kind() {
			case reflect.String:
				field.SetString(envVal)
			case reflect.Int, reflect.Int64:
				var val int
				fmt.Sscanf(envVal, "%d", &val)
				field.SetInt(int64(val))
			case reflect.Float64:
				var val float64
				fmt.Sscanf(envVal, "%f", &val)
				field.SetFloat(val)
			}
		}
	}
}

var secretPattern = regexp.MustCompile(`\${([^}]+)}`)

func resolveSecrets(cfg *Config) {
	v := reflect.ValueOf(cfg).Elem()
	resolveSecretsInStruct(v)
}

func resolveSecretsInStruct(v reflect.Value) {
	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		if field.Kind() == reflect.Struct {
			resolveSecretsInStruct(field)
			continue
		}
		if field.Kind() == reflect.String && field.CanSet() {
			resolved := secretPattern.ReplaceAllStringFunc(field.String(), func(match string) string {
				key := strings.TrimSuffix(strings.TrimPrefix(match, "${"), "}")
				if val := os.Getenv(key); val != "" {
					return val
				}
				return match
			})
			field.SetString(resolved)
		}
	}
}

type Service struct {
	mu       sync.RWMutex
	cfg      *Config
	loader   *Loader
	repo     Repository
	version  int
	hash     string
	watchers []func(*Config)
}

func NewService(loader *Loader, repo Repository) *Service {
	return &Service{
		loader:   loader,
		repo:     repo,
		watchers: make([]func(*Config), 0),
	}
}

func (s *Service) Load(ctx context.Context) error {
	cfg, err := s.loader.Load()
	if err != nil {
		return fmt.Errorf("config service load: %w", err)
	}

	if err := Validate(cfg); err != nil {
		return fmt.Errorf("config service validate: %w", err)
	}

	s.mu.Lock()
	s.cfg = cfg
	s.version = 1
	s.hash = computeHash(cfg)
	s.mu.Unlock()

	if s.repo != nil {
		if err := s.repo.Save(ctx, 1, cfg); err != nil {
			return fmt.Errorf("config service persist: %w", err)
		}
	}

	return nil
}

func (s *Service) Reload(ctx context.Context) error {
	cfg, err := s.loader.Load()
	if err != nil {
		return fmt.Errorf("config reload: %w", err)
	}

	if err := Validate(cfg); err != nil {
		return fmt.Errorf("config reload validate: %w", err)
	}

	newHash := computeHash(cfg)

	s.mu.RLock()
	same := newHash == s.hash
	s.mu.RUnlock()

	if same {
		return nil
	}

	s.mu.Lock()
	oldVersion := s.version
	newVersion := oldVersion + 1
	s.cfg = cfg
	s.version = newVersion
	s.hash = newHash

	watchers := make([]func(*Config), len(s.watchers))
	copy(watchers, s.watchers)
	s.mu.Unlock()

	if s.repo != nil {
		if err := s.repo.Save(ctx, newVersion, cfg); err != nil {
			return fmt.Errorf("config reload persist: %w", err)
		}
	}

	for _, w := range watchers {
		w(cfg)
	}

	return nil
}

func (s *Service) Get() *Config {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cp := *s.cfg
	return &cp
}

func (s *Service) Version() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.version
}

func (s *Service) Watch(fn func(*Config)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.watchers = append(s.watchers, fn)
}

func (s *Service) StartWatcher(ctx context.Context, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				_ = s.Reload(ctx)
			}
		}
	}()
}

func computeHash(cfg *Config) string {
	data, _ := json.Marshal(cfg)
	h := sha256.Sum256(data)
	return fmt.Sprintf("%x", h[:8])
}

var validLevels = map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
var validFormats = map[string]bool{"json": true, "text": true}

func Validate(cfg *Config) error {
	if cfg == nil {
		return fmt.Errorf("config is nil")
	}
	if cfg.Server.Port < 1 || cfg.Server.Port > 65535 {
		return fmt.Errorf("server port must be between 1 and 65535")
	}
	if cfg.Server.ReadTimeout < 1 {
		return fmt.Errorf("server read_timeout must be positive")
	}
	if cfg.Database.MaxConns < 1 {
		return fmt.Errorf("database max_conns must be positive")
	}
	if !validLevels[cfg.Logging.Level] {
		return fmt.Errorf("logging level must be one of: debug, info, warn, error")
	}
	if !validFormats[cfg.Logging.Format] {
		return fmt.Errorf("logging format must be json or text")
	}
	if cfg.Auth.TokenExpiry < 1 {
		return fmt.Errorf("auth token_expiry must be positive")
	}
	return nil
}
