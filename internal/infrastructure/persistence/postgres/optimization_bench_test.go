package postgres

import (
	"encoding/json"
	"fmt"
	"sync"
	"testing"

	"github.com/ophidian/ophidian/internal/domain/common"
	"github.com/ophidian/ophidian/internal/domain/finding"
	"github.com/ophidian/ophidian/internal/domain/mission"
)

func BenchmarkSerialization_MissionToJSON(b *testing.B) {
	m := &mission.Mission{
		ID:        common.NewID(),
		Name:      "test-mission",
		Status:    mission.MissionActive,
		StartedBy: "operator-1",
		Target: mission.Target{
			ID:   common.NewID(),
			Name: "corp-net",
			IPs:  []string{"10.0.0.1", "10.0.0.2", "10.0.0.3"},
			Domains: []string{"corp.com", "api.corp.com"},
			CIDRs: []string{"10.0.0.0/24"},
		},
		RoE: mission.RoEConstraints{
			MaxSeverity:      common.SeverityHigh,
			AllowDestructive: false,
			AllowExfiltration: true,
		},
		CreatedAt: common.Now(),
		UpdatedAt: common.Now(),
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		data, _ := json.Marshal(m)
		_ = data
	}
}

func BenchmarkSerialization_FindingToJSON(b *testing.B) {
	f := &finding.Finding{
		ID:          common.NewID(),
		MissionID:   common.NewID(),
		TargetID:    common.NewID(),
		Title:       "SQL Injection in login form",
		Description: "Found parameterized SQL injection vulnerability allowing authentication bypass through crafted payload",
		Severity:    common.SeverityCritical,
		CVSS:        9.8,
		CVE:         "CVE-2024-0001",
		CWE:         "CWE-89",
		Confidence:  finding.ConfidenceConfirmed,
		Status:      finding.FindingStatusConfirmed,
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		data, _ := json.Marshal(f)
		_ = data
	}
}

func BenchmarkSerialization_MultipleObjects(b *testing.B) {
	missions := make([]*mission.Mission, 100)
	for i := 0; i < 100; i++ {
		missions[i] = &mission.Mission{
			ID:        common.NewID(),
			Name:      fmt.Sprintf("mission-%d", i),
			Status:    mission.MissionActive,
			Target:    mission.Target{Name: fmt.Sprintf("target-%d", i), IPs: []string{"10.0.0.1"}},
			CreatedAt: common.Now(),
		}
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, m := range missions {
			data, _ := json.Marshal(m)
			_ = data
		}
	}
}

func BenchmarkGoroutine_Channel(b *testing.B) {
	ch := make(chan struct{}, 1000)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ch <- struct{}{}
		<-ch
	}
}

func BenchmarkGoroutine_Mutex(b *testing.B) {
	var mu sync.Mutex
	var counter int
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mu.Lock()
		counter++
		mu.Unlock()
	}
}

func BenchmarkGoroutine_RWMutex(b *testing.B) {
	var mu sync.RWMutex
	var counter int
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mu.Lock()
		counter++
		mu.Unlock()
	}
}
