package secrets

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"
)

type SecretMetadata struct {
	Name        string
	Version     int
	CreatedAt   time.Time
	RotatedAt   time.Time
	AccessedAt  time.Time
	AccessCount int64
}

type Secret struct {
	Value    []byte
	Metadata SecretMetadata
}

type Provider interface {
	Load(ctx context.Context, name string) ([]byte, error)
	Store(ctx context.Context, name string, value []byte) error
	Exists(ctx context.Context, name string) (bool, error)
}

type AuditLogger interface {
	Log(ctx context.Context, name string, action string, success bool)
}

type SecretManager struct {
	provider    Provider
	cache       map[string]cachedSecret
	audit       AuditLogger
	mu          sync.RWMutex
	encryptKey  []byte
}

type cachedSecret struct {
	value     []byte
	expiresAt time.Time
}

type ManagerOption func(*SecretManager)

func WithAuditLogger(audit AuditLogger) ManagerOption {
	return func(m *SecretManager) { m.audit = audit }
}

func NewSecretManager(provider Provider, encryptionKey string, opts ...ManagerOption) *SecretManager {
	hash := sha256.Sum256([]byte(encryptionKey))
	m := &SecretManager{
		provider:   provider,
		cache:      make(map[string]cachedSecret),
		encryptKey: hash[:],
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

func (m *SecretManager) Get(ctx context.Context, name string) (string, error) {
	m.mu.RLock()
	if cached, ok := m.cache[name]; ok && time.Now().Before(cached.expiresAt) {
		m.mu.RUnlock()
		m.auditLog(ctx, name, "get", true)
		return string(cached.value), nil
	}
	m.mu.RUnlock()

	encrypted, err := m.provider.Load(ctx, name)
	if err != nil {
		m.auditLog(ctx, name, "get", false)
		return "", fmt.Errorf("secret manager get %s: %w", name, err)
	}

	plaintext, err := m.decrypt(encrypted)
	if err != nil {
		m.auditLog(ctx, name, "get", false)
		return "", fmt.Errorf("secret manager decrypt %s: %w", name, err)
	}

	m.mu.Lock()
	m.cache[name] = cachedSecret{value: plaintext, expiresAt: time.Now().Add(5 * time.Minute)}
	m.mu.Unlock()

	m.auditLog(ctx, name, "get", true)
	return string(plaintext), nil
}

func (m *SecretManager) Set(ctx context.Context, name, value string) error {
	if value == "" {
		return fmt.Errorf("secret value is empty for %s", name)
	}

	encrypted, err := m.encrypt([]byte(value))
	if err != nil {
		m.auditLog(ctx, name, "set", false)
		return fmt.Errorf("secret manager encrypt %s: %w", name, err)
	}

	if err := m.provider.Store(ctx, name, encrypted); err != nil {
		m.auditLog(ctx, name, "set", false)
		return fmt.Errorf("secret manager store %s: %w", name, err)
	}

	m.mu.Lock()
	m.cache[name] = cachedSecret{value: []byte(value), expiresAt: time.Now().Add(5 * time.Minute)}
	m.mu.Unlock()

	m.auditLog(ctx, name, "set", true)
	return nil
}

func (m *SecretManager) Delete(ctx context.Context, name string) error {
	m.mu.Lock()
	delete(m.cache, name)
	m.mu.Unlock()

	m.auditLog(ctx, name, "delete", true)
	return fmt.Errorf("provider does not support deletion")
}

func (m *SecretManager) Rotate(ctx context.Context, name, newValue string) error {
	oldValue, err := m.Get(ctx, name)
	if err != nil {
		return fmt.Errorf("secret manager rotate %s: %w", name, err)
	}

	if err := m.Set(ctx, name+".prev", oldValue); err != nil {
		m.auditLog(ctx, name, "rotate", false)
		return fmt.Errorf("secret manager rotate backup %s: %w", name, err)
	}

	if err := m.Set(ctx, name, newValue); err != nil {
		m.auditLog(ctx, name, "rotate", false)
		return fmt.Errorf("secret manager rotate update %s: %w", name, err)
	}

	m.auditLog(ctx, name, "rotate", true)
	return nil
}

func (m *SecretManager) Exists(ctx context.Context, name string) (bool, error) {
	exists, err := m.provider.Exists(ctx, name)
	if err != nil {
		return false, fmt.Errorf("secret manager exists %s: %w", name, err)
	}
	return exists, nil
}

func (m *SecretManager) InvalidateCache(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.cache, name)
}

func (m *SecretManager) encrypt(plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(m.encryptKey)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	encoded := make([]byte, base64.StdEncoding.EncodedLen(len(ciphertext)))
	base64.StdEncoding.Encode(encoded, ciphertext)
	return encoded, nil
}

func (m *SecretManager) decrypt(encrypted []byte) ([]byte, error) {
	decoded := make([]byte, base64.StdEncoding.DecodedLen(len(encrypted)))
	n, err := base64.StdEncoding.Decode(decoded, encrypted)
	if err != nil {
		return nil, err
	}
	ciphertext := decoded[:n]

	block, err := aes.NewCipher(m.encryptKey)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce, ct := ciphertext[:nonceSize], ciphertext[nonceSize:]
	return gcm.Open(nil, nonce, ct, nil)
}

func (m *SecretManager) auditLog(ctx context.Context, name, action string, success bool) {
	if m.audit != nil {
		m.audit.Log(ctx, name, action, success)
	}
}

type MultiProvider struct {
	providers []Provider
}

func NewMultiProvider(providers ...Provider) *MultiProvider {
	return &MultiProvider{providers: providers}
}

func (m *MultiProvider) Load(ctx context.Context, name string) ([]byte, error) {
	var errs []string
	for _, p := range m.providers {
		if val, err := p.Load(ctx, name); err == nil {
			return val, nil
		} else {
			errs = append(errs, fmt.Sprintf("%T: %v", p, err))
		}
	}
	return nil, fmt.Errorf("all providers failed for %s: %s", name, strings.Join(errs, "; "))
}

func (m *MultiProvider) Store(ctx context.Context, name string, value []byte) error {
	for _, p := range m.providers {
		if err := p.Store(ctx, name, value); err != nil {
			return err
		}
	}
	return nil
}

func (m *MultiProvider) Exists(ctx context.Context, name string) (bool, error) {
	for _, p := range m.providers {
		if exists, err := p.Exists(ctx, name); err == nil && exists {
			return true, nil
		}
	}
	return false, nil
}

type MemoryProvider struct {
	mu    sync.RWMutex
	store map[string][]byte
}

func NewMemoryProvider() *MemoryProvider {
	return &MemoryProvider{store: make(map[string][]byte)}
}

func (p *MemoryProvider) Load(ctx context.Context, name string) ([]byte, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	v, ok := p.store[name]
	if !ok {
		return nil, fmt.Errorf("secret not found: %s", name)
	}
	return v, nil
}

func (p *MemoryProvider) Store(ctx context.Context, name string, value []byte) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.store[name] = value
	return nil
}

func (p *MemoryProvider) Exists(ctx context.Context, name string) (bool, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	_, ok := p.store[name]
	return ok, nil
}
