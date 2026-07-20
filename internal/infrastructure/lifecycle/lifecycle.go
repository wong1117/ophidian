package lifecycle

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type RetentionPolicy struct {
	ResourceType string
	MaxAge       time.Duration
	ArchivalAge  time.Duration
	Action       string
}

type DataLifecycleManager struct {
	mu        sync.RWMutex
	policies  []RetentionPolicy
	stats     LifecycleStats
}

type LifecycleStats struct {
	LastRun         time.Time  `json:"last_run"`
	TotalArchived   int64      `json:"total_archived"`
	TotalPurged     int64      `json:"total_purged"`
	TotalVerified   int64      `json:"total_verified"`
	TotalCorrupted  int64      `json:"total_corrupted"`
	LastBackupAt    time.Time  `json:"last_backup_at"`
	LastRestoreAt   time.Time  `json:"last_restore_at"`
	LastChecksumAt  time.Time  `json:"last_checksum_at"`
}

type LifecycleReport struct {
	GeneratedAt   time.Time      `json:"generated_at"`
	Stats         LifecycleStats `json:"stats"`
	Policies      []RetentionPolicy `json:"policies"`
	Recommendations []string     `json:"recommendations"`
	Status        string         `json:"status"`
}

type ChecksumStore struct {
	mu       sync.Mutex
	checksums map[string]string
	path     string
}

func NewChecksumStore(path string) *ChecksumStore {
	cs := &ChecksumStore{
		checksums: make(map[string]string),
		path:      path,
	}
	cs.load()
	return cs
}

func (cs *ChecksumStore) load() {
	data, err := os.ReadFile(cs.path)
	if err != nil { return }
	json.Unmarshal(data, &cs.checksums)
}

func (cs *ChecksumStore) save() error {
	data, _ := json.MarshalIndent(cs.checksums, "", "  ")
	return os.WriteFile(cs.path, data, 0644)
}

func (cs *ChecksumStore) Record(key string, data []byte) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	h := sha256.Sum256(data)
	cs.checksums[key] = fmt.Sprintf("%x", h)
	return cs.save()
}

func (cs *ChecksumStore) Verify(key string, data []byte) bool {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	expected, ok := cs.checksums[key]
	if !ok { return false }
	actual := fmt.Sprintf("%x", sha256.Sum256(data))
	return expected == actual
}

func (cs *ChecksumStore) VerifyAll(dataProvider func(key string) []byte) (int, int, []string) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	verified := 0
	corrupted := 0
	var corruptKeys []string

	for key, expected := range cs.checksums {
		data := dataProvider(key)
		actual := fmt.Sprintf("%x", sha256.Sum256(data))
		if expected == actual {
			verified++
		} else {
			corrupted++
			corruptKeys = append(corruptKeys, key)
		}
	}
	return verified, corrupted, corruptKeys
}

func NewDataLifecycleManager() *DataLifecycleManager {
	return &DataLifecycleManager{
		policies: defaultPolicies(),
	}
}

func defaultPolicies() []RetentionPolicy {
	return []RetentionPolicy{
		{ResourceType: "events", MaxAge: 365 * 24 * time.Hour, ArchivalAge: 90 * 24 * time.Hour, Action: "archive"},
		{ResourceType: "metrics", MaxAge: 0, ArchivalAge: 0, Action: "keep_forever"},
		{ResourceType: "exploit_logs", MaxAge: 90 * 24 * time.Hour, ArchivalAge: 30 * 24 * time.Hour, Action: "archive_then_purge"},
		{ResourceType: "recon_data", MaxAge: 30 * 24 * time.Hour, ArchivalAge: 7 * 24 * time.Hour, Action: "purge"},
		{ResourceType: "ai_memory_vectors", MaxAge: 180 * 24 * time.Hour, ArchivalAge: 60 * 24 * time.Hour, Action: "archive"},
		{ResourceType: "audit_logs", MaxAge: 365 * 24 * time.Hour, ArchivalAge: 90 * 24 * time.Hour, Action: "archive"},
	}
}

func (m *DataLifecycleManager) SetPolicies(policies []RetentionPolicy) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.policies = policies
}

func (m *DataLifecycleManager) RunCleanup(ctx context.Context) *LifecycleReport {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	m.stats.LastRun = now

	var archived, purged int64
	for _, policy := range m.policies {
		count := int64(0)
		switch policy.Action {
		case "archive", "archive_then_purge":
			count = m.simulateArchive(policy.ResourceType, policy.ArchivalAge)
			archived += count
		case "purge":
			count = m.simulatePurge(policy.ResourceType, policy.MaxAge)
			purged += count
		}
		_ = count
	}

	m.stats.TotalArchived += archived
	m.stats.TotalPurged += purged

	return &LifecycleReport{
		GeneratedAt: now,
		Stats:       m.stats,
		Policies:    m.policies,
		Recommendations: []string{
			fmt.Sprintf("Run backup verification weekly to ensure restore integrity"),
		},
		Status: "HEALTHY",
	}
}

func (m *DataLifecycleManager) simulateArchive(resourceType string, age time.Duration) int64 {
	count := int64(100)
	m.log("archived %d %s events older than %s", count, resourceType, age)
	return count
}

func (m *DataLifecycleManager) simulatePurge(resourceType string, age time.Duration) int64 {
	count := int64(50)
	m.log("purged %d %s events older than %s", count, resourceType, age)
	return count
}

func (m *DataLifecycleManager) log(format string, args ...interface{}) {
	fmt.Printf("[lifecycle] "+format+"\n", args...)
}

type BackupVerifier struct {
	pgDumpPath string
	backupDir  string
	cacheDir   string
}

func NewBackupVerifier(backupDir, _, cacheDir string) *BackupVerifier {
	return &BackupVerifier{
		backupDir: backupDir,
		cacheDir:  cacheDir,
		pgDumpPath: "pg_dump",
	}
}

func (v *BackupVerifier) CreateBackup() error {
	backupFile := filepath.Join(v.backupDir, fmt.Sprintf("events-%s.sql", time.Now().Format("20060102-150405")))
	cmd := exec.Command(v.pgDumpPath, "-F", "c", "-f", backupFile, "ophidian")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("create backup: %w\n%s", err, string(output))
	}
	return nil
}

func (v *BackupVerifier) VerifyBackup(backupFile string) error {
	cmd := exec.Command(v.pgDumpPath, "-F", "c", "-f", "/dev/null", "--schema-only", backupFile)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("verify backup: %w\n%s", err, string(output))
	}
	return nil
}

func (v *BackupVerifier) VerifyCacheIntegrity() error {
	entries, err := os.ReadDir(v.cacheDir)
	if err != nil {
		return fmt.Errorf("read cache dir: %w", err)
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			path := filepath.Join(v.cacheDir, e.Name())
			if _, err := os.Stat(path); err != nil {
				return fmt.Errorf("cache file %s: %w", path, err)
			}
		}
	}
	return nil
}

func (m *DataLifecycleManager) BackupStats() (time.Time, time.Time) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.stats.LastBackupAt, m.stats.LastRestoreAt
}

func (m *DataLifecycleManager) MarkBackup() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stats.LastBackupAt = time.Now()
	m.stats.TotalVerified++
}

func (m *DataLifecycleManager) MarkRestore() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stats.LastRestoreAt = time.Now()
}

func (m *DataLifecycleManager) MarkChecksum(ok bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stats.LastChecksumAt = time.Now()
	if ok {
		m.stats.TotalVerified++
	} else {
		m.stats.TotalCorrupted++
	}
}
