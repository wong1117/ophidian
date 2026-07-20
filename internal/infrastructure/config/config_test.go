package config

import (
	"context"
	"encoding/json"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type testRepo struct {
	mu      sync.Mutex
	configs map[int]*Config
}

func newTestRepo() *testRepo {
	return &testRepo{configs: make(map[int]*Config)}
}

func (r *testRepo) Save(ctx context.Context, version int, cfg *Config) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.configs[version] = cfg
	return nil
}

func (r *testRepo) Load(ctx context.Context, version int) (*Config, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.configs[version], nil
}

func (r *testRepo) LatestVersion(ctx context.Context) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	max := 0
	for v := range r.configs {
		if v > max {
			max = v
		}
	}
	return max, nil
}

func TestLoader_LoadDefaults(t *testing.T) {
	l := NewLoader("")
	cfg, err := l.Load()

	assert.NoError(t, err)
	assert.Equal(t, 8080, cfg.Server.Port)
	assert.Equal(t, "localhost", cfg.Database.Host)
	assert.Equal(t, "info", cfg.Logging.Level)
}

func TestLoader_LoadEnvOverride(t *testing.T) {
	os.Setenv("APP_SERVER_PORT", "9090")
	os.Setenv("APP_DATABASE_HOST", "db.example.com")
	os.Setenv("APP_LOGGING_LEVEL", "debug")
	defer func() {
		os.Unsetenv("APP_SERVER_PORT")
		os.Unsetenv("APP_DATABASE_HOST")
		os.Unsetenv("APP_LOGGING_LEVEL")
	}()

	l := NewLoader("")
	cfg, err := l.Load()

	assert.NoError(t, err)
	assert.Equal(t, 9090, cfg.Server.Port)
	assert.Equal(t, "db.example.com", cfg.Database.Host)
	assert.Equal(t, "debug", cfg.Logging.Level)
}

func TestLoader_ResolveSecrets(t *testing.T) {
	os.Setenv("DB_PASSWORD", "s3cr3t")
	defer os.Unsetenv("DB_PASSWORD")

	orig := &Config{}
	orig.Database.Password = "${DB_PASSWORD}"

	resolveSecrets(orig)

	assert.Equal(t, "s3cr3t", orig.Database.Password)
}

func TestLoader_ResolveSecrets_NoEnv(t *testing.T) {
	orig := &Config{}
	orig.Database.Password = "${NONEXISTENT_VAR}"

	resolveSecrets(orig)

	assert.Equal(t, "${NONEXISTENT_VAR}", orig.Database.Password)
}

func TestValidator_Valid(t *testing.T) {
	cfg := DefaultConfig()
	err := Validate(cfg)
	assert.NoError(t, err)
}

func TestValidator_InvalidPort(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Server.Port = 0
	assert.Error(t, Validate(cfg))

	cfg.Server.Port = 99999
	assert.Error(t, Validate(cfg))
}

func TestValidator_InvalidLogLevel(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Logging.Level = "invalid"
	assert.Error(t, Validate(cfg))
}

func TestValidator_InvalidLogFormat(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Logging.Format = "xml"
	assert.Error(t, Validate(cfg))
}

func TestValidator_Nil(t *testing.T) {
	assert.Error(t, Validate(nil))
}

func TestService_Load(t *testing.T) {
	l := NewLoader("")
	repo := newTestRepo()
	svc := NewService(l, repo)

	err := svc.Load(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, 1, svc.Version())
	assert.NotNil(t, svc.Get())

	v, _ := repo.LatestVersion(context.Background())
	assert.Equal(t, 1, v)
}

func TestService_Reload(t *testing.T) {
	l := NewLoader("")
	repo := newTestRepo()
	svc := NewService(l, repo)

	svc.Load(context.Background())
	v1 := svc.Version()

	os.Setenv("APP_SERVER_PORT", "3000")
	defer os.Unsetenv("APP_SERVER_PORT")

	err := svc.Reload(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, v1+1, svc.Version())
}

func TestService_ReloadSameContent(t *testing.T) {
	l := NewLoader("")
	svc := NewService(l, nil)

	svc.Load(context.Background())
	v1 := svc.Version()

	err := svc.Reload(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, v1, svc.Version(), "reload with same content should not increment version")
}

func TestService_Watch(t *testing.T) {
	l := NewLoader("")
	svc := NewService(l, nil)
	svc.Load(context.Background())

	var received *Config
	svc.Watch(func(cfg *Config) {
		received = cfg
	})

	os.Setenv("APP_SERVER_PORT", "4000")
	defer os.Unsetenv("APP_SERVER_PORT")

	svc.Reload(context.Background())

	assert.NotNil(t, received)
	assert.Equal(t, 4000, received.Server.Port)
}

func TestService_StartWatcher(t *testing.T) {
	l := NewLoader("")
	repo := newTestRepo()
	svc := NewService(l, repo)

	svc.Load(context.Background())
	v1 := svc.Version()

	ctx, cancel := context.WithCancel(context.Background())
	os.Setenv("APP_SERVER_PORT", "5000")
	defer os.Unsetenv("APP_SERVER_PORT")

	svc.StartWatcher(ctx, 10*time.Millisecond)
	time.Sleep(50 * time.Millisecond)
	cancel()

	assert.GreaterOrEqual(t, svc.Version(), v1+1)
}

func TestJSONSerialization(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Server.Port = 9090

	data, err := json.Marshal(cfg)
	assert.NoError(t, err)

	var parsed Config
	err = json.Unmarshal(data, &parsed)
	assert.NoError(t, err)
	assert.Equal(t, 9090, parsed.Server.Port)
}

func TestComputeHash(t *testing.T) {
	a := DefaultConfig()
	b := DefaultConfig()
	assert.Equal(t, computeHash(a), computeHash(b))

	a.Server.Port = 9999
	assert.NotEqual(t, computeHash(a), computeHash(b))
}
