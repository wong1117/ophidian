package secrets

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

type testAudit struct {
	mu      sync.Mutex
	entries []auditEntry
}

type auditEntry struct {
	name    string
	action  string
	success bool
}

func (a *testAudit) Log(ctx context.Context, name, action string, success bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.entries = append(a.entries, auditEntry{name: name, action: action, success: success})
}

func TestSecretManager_SetGet(t *testing.T) {
	prov := NewMemoryProvider()
	mgr := NewSecretManager(prov, "super-secret-encryption-key-32bytes!!")
	ctx := context.Background()

	err := mgr.Set(ctx, "db-password", "s3cr3t-p@ss!")
	assert.NoError(t, err)

	val, err := mgr.Get(ctx, "db-password")
	assert.NoError(t, err)
	assert.Equal(t, "s3cr3t-p@ss!", val)
}

func TestSecretManager_Get_NotFound(t *testing.T) {
	prov := NewMemoryProvider()
	mgr := NewSecretManager(prov, "super-secret-encryption-key-32bytes!!")
	ctx := context.Background()

	_, err := mgr.Get(ctx, "nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "secret manager get")
}

func TestSecretManager_Rotate(t *testing.T) {
	prov := NewMemoryProvider()
	audit := &testAudit{}
	mgr := NewSecretManager(prov, "super-secret-encryption-key-32bytes!!", WithAuditLogger(audit))
	ctx := context.Background()

	mgr.Set(ctx, "api-key", "old-key")
	err := mgr.Rotate(ctx, "api-key", "new-key")
	assert.NoError(t, err)

	val, _ := mgr.Get(ctx, "api-key")
	assert.Equal(t, "new-key", val)

	prev, _ := mgr.Get(ctx, "api-key.prev")
	assert.Equal(t, "old-key", prev)

	assert.GreaterOrEqual(t, len(audit.entries), 3)
}

func TestSecretManager_Cache(t *testing.T) {
	prov := NewMemoryProvider()
	mgr := NewSecretManager(prov, "super-secret-encryption-key-32bytes!!")
	ctx := context.Background()

	mgr.Set(ctx, "cached-secret", "cached-value")

	mgr.Get(ctx, "cached-secret")
	mgr.Get(ctx, "cached-secret")

	exists, _ := prov.Exists(ctx, "cached-secret")
	assert.True(t, exists)
}

func TestSecretManager_InvalidateCache(t *testing.T) {
	prov := NewMemoryProvider()
	mgr := NewSecretManager(prov, "super-secret-encryption-key-32bytes!!")
	ctx := context.Background()

	mgr.Set(ctx, "cache-test", "value1")
	mgr.Get(ctx, "cache-test")
	mgr.InvalidateCache("cache-test")

	mgr.mu.RLock()
	_, cached := mgr.cache["cache-test"]
	mgr.mu.RUnlock()
	assert.False(t, cached)
}

func TestSecretManager_Set_EmptyValue(t *testing.T) {
	prov := NewMemoryProvider()
	mgr := NewSecretManager(prov, "super-secret-encryption-key-32bytes!!")
	ctx := context.Background()

	err := mgr.Set(ctx, "empty", "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "secret value is empty")
}

func TestSecretManager_EncryptDecryptRoundtrip(t *testing.T) {
	mgr := NewSecretManager(nil, "super-secret-encryption-key-32bytes!!")

	plaintext := []byte("my-secret-data-for-testing")
	encrypted, err := mgr.encrypt(plaintext)
	assert.NoError(t, err)
	assert.NotEqual(t, plaintext, encrypted)

	decrypted, err := mgr.decrypt(encrypted)
	assert.NoError(t, err)
	assert.Equal(t, plaintext, decrypted)
}

func TestSecretManager_Audit(t *testing.T) {
	prov := NewMemoryProvider()
	audit := &testAudit{}
	mgr := NewSecretManager(prov, "super-secret-encryption-key-32bytes!!", WithAuditLogger(audit))
	ctx := context.Background()

	mgr.Set(ctx, "audited", "val")
	mgr.Get(ctx, "audited")
	mgr.Get(ctx, "nonexistent")

	assert.Len(t, audit.entries, 3)
	assert.Equal(t, "set", audit.entries[0].action)
	assert.True(t, audit.entries[0].success)
	assert.Equal(t, "get", audit.entries[1].action)
	assert.True(t, audit.entries[1].success)
	assert.Equal(t, "get", audit.entries[2].action)
	assert.False(t, audit.entries[2].success)
}

func TestMultiProvider(t *testing.T) {
	p1 := NewMemoryProvider()
	p2 := NewMemoryProvider()
	mp := NewMultiProvider(p1, p2)
	ctx := context.Background()

	p1.Store(ctx, "only-in-p1", []byte("p1-value"))

	val, err := mp.Load(ctx, "only-in-p1")
	assert.NoError(t, err)
	assert.Equal(t, []byte("p1-value"), val)

	_, err = mp.Load(ctx, "nowhere")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "all providers failed")

	err = mp.Store(ctx, "shared", []byte("shared-value"))
	assert.NoError(t, err)

	v1, _ := p1.Load(ctx, "shared")
	v2, _ := p2.Load(ctx, "shared")
	assert.Equal(t, []byte("shared-value"), v1)
	assert.Equal(t, []byte("shared-value"), v2)
}

func TestMultiProvider_Exists(t *testing.T) {
	p1 := NewMemoryProvider()
	p2 := NewMemoryProvider()
	mp := NewMultiProvider(p1, p2)
	ctx := context.Background()

	p2.Store(ctx, "secret", []byte("val"))

	exists, err := mp.Exists(ctx, "secret")
	assert.NoError(t, err)
	assert.True(t, exists)

	exists, err = mp.Exists(ctx, "missing")
	assert.NoError(t, err)
	assert.False(t, exists)
}

func TestEncryption_UniqueCiphertexts(t *testing.T) {
	mgr := NewSecretManager(nil, "super-secret-encryption-key-32bytes!!")

	e1, _ := mgr.encrypt([]byte("hello"))
	e2, _ := mgr.encrypt([]byte("hello"))

	assert.NotEqual(t, e1, e2, "same plaintext should produce different ciphertexts (nonce)")
}

func TestConcurrency(t *testing.T) {
	prov := NewMemoryProvider()
	mgr := NewSecretManager(prov, "super-secret-encryption-key-32bytes!!")
	ctx := context.Background()

	k := 50
	var wg sync.WaitGroup
	for i := 0; i < k; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			name := fmt.Sprintf("conc-%d", idx)
			mgr.Set(ctx, name, fmt.Sprintf("val-%d", idx))
			v, _ := mgr.Get(ctx, name)
			assert.Equal(t, fmt.Sprintf("val-%d", idx), v)
		}(i)
	}
	wg.Wait()
}
