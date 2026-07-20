package lifecycle

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDataLifecycleManager_RunCleanup(t *testing.T) {
	m := NewDataLifecycleManager()
	ctx := context.Background()

	report := m.RunCleanup(ctx)

	assert.NotNil(t, report)
	assert.Equal(t, "HEALTHY", report.Status)
	assert.Greater(t, report.Stats.TotalArchived, int64(0))
	assert.Greater(t, report.Stats.TotalPurged, int64(0))
	assert.NotEmpty(t, report.Policies)
	assert.NotEmpty(t, report.Recommendations)
	assert.False(t, report.GeneratedAt.IsZero())
}

func TestDataLifecycleManager_SetPolicies(t *testing.T) {
	m := NewDataLifecycleManager()
	custom := []RetentionPolicy{
		{ResourceType: "custom_logs", MaxAge: 7 * 24 * time.Hour, ArchivalAge: 24 * time.Hour, Action: "purge"},
	}
	m.SetPolicies(custom)

	report := m.RunCleanup(context.Background())
	assert.Len(t, report.Policies, 1)
	assert.Equal(t, "custom_logs", report.Policies[0].ResourceType)
}

func TestChecksumStore_RecordAndVerify(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "checksums.json")
	cs := NewChecksumStore(path)

	key := "backup-001"
	data := []byte("event payload data")
	err := cs.Record(key, data)
	assert.NoError(t, err)

	ok := cs.Verify(key, data)
	assert.True(t, ok)

	ok = cs.Verify(key, []byte("tampered data"))
	assert.False(t, ok)

	ok = cs.Verify("missing-key", data)
	assert.False(t, ok)
}

func TestChecksumStore_Persistence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "checksums-store.json")
	cs1 := NewChecksumStore(path)
	cs1.Record("e1", []byte("data1"))
	cs1.Record("e2", []byte("data2"))

	cs2 := NewChecksumStore(path)
	assert.True(t, cs2.Verify("e1", []byte("data1")))
	assert.True(t, cs2.Verify("e2", []byte("data2")))
}

func TestBackupVerifier_VerifyCache(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "cache-a.json"), []byte("{}"), 0644)
	os.WriteFile(filepath.Join(dir, "cache-b.json"), []byte("{}"), 0644)

	v := NewBackupVerifier(dir, "", dir)
	err := v.VerifyCacheIntegrity()
	assert.NoError(t, err)
}

func TestLifecycleReport_JSON(t *testing.T) {
	m := NewDataLifecycleManager()
	report := m.RunCleanup(context.Background())

	data, err := json.Marshal(report)
	assert.NoError(t, err)

	var parsed LifecycleReport
	err = json.Unmarshal(data, &parsed)
	assert.NoError(t, err)
	assert.Equal(t, "HEALTHY", parsed.Status)
	assert.Len(t, parsed.Policies, 6)
}

func TestDefaultPolicies(t *testing.T) {
	policies := defaultPolicies()
	assert.Len(t, policies, 6)

	expected := map[string]bool{
		"events": true, "metrics": true, "exploit_logs": true,
		"recon_data": true, "ai_memory_vectors": true, "audit_logs": true,
	}
	for _, p := range policies {
		assert.True(t, expected[p.ResourceType], "unexpected resource type: %s", p.ResourceType)
		if p.ResourceType == "metrics" {
			assert.Equal(t, "keep_forever", p.Action)
			assert.Equal(t, time.Duration(0), p.MaxAge)
		}
	}
}

func TestMarkBackupRestore(t *testing.T) {
	m := NewDataLifecycleManager()
	before := time.Now()

	m.MarkBackup()
	m.MarkRestore()

	lastBackup, lastRestore := m.BackupStats()
	assert.True(t, lastBackup.After(before) || lastBackup.Equal(before))
	assert.True(t, lastRestore.After(before) || lastRestore.Equal(before))
}

func TestMarkChecksum(t *testing.T) {
	m := NewDataLifecycleManager()
	m.MarkChecksum(true)
	assert.Equal(t, int64(1), m.stats.TotalVerified)

	m.MarkChecksum(false)
	assert.Equal(t, int64(1), m.stats.TotalCorrupted)
}

func TestChecksumStore_VerifyAll(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "verifyall.json")
	cs := NewChecksumStore(path)
	cs.Record("good", []byte("good-data"))
	cs.Record("bad", []byte("bad-data"))

	store := map[string][]byte{"good": []byte("good-data"), "bad": []byte("wrong-data")}
	verified, corrupted, keys := cs.VerifyAll(func(key string) []byte {
		return store[key]
	})

	assert.Equal(t, 1, verified)
	assert.Equal(t, 1, corrupted)
	assert.Equal(t, []string{"bad"}, keys)
}
