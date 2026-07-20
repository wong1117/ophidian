package observability

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"
)

type MetricsConfig struct {
	Enabled bool
}

type metricEntry struct {
	Name   string
	Type   MetricType
	Value  float64
	Count  int64
	Min    float64
	Max    float64
	Sum    float64
	Tags   []Field
	LastAt time.Time
}

type metricsCollector struct {
	config  MetricsConfig
	mu      sync.RWMutex
	metrics map[string]*metricEntry
}

func NewMetrics(cfg MetricsConfig) Metrics {
	return &metricsCollector{
		config:  cfg,
		metrics: make(map[string]*metricEntry),
	}
}

func (m *metricsCollector) IncrementCounter(ctx context.Context, name string, value int64, tags ...Field) {
	if !m.config.Enabled {
		return
	}
	key := metricKey(name, tags)

	m.mu.Lock()
	defer m.mu.Unlock()

	entry, ok := m.metrics[key]
	if !ok {
		entry = &metricEntry{Name: name, Type: MetricCounter, Tags: tags}
		m.metrics[key] = entry
	}
	entry.Count += value
	entry.Value = float64(entry.Count)
	entry.LastAt = time.Now()
}

func (m *metricsCollector) RecordHistogram(ctx context.Context, name string, value float64, tags ...Field) {
	if !m.config.Enabled {
		return
	}
	key := metricKey(name, tags)

	m.mu.Lock()
	defer m.mu.Unlock()

	entry, ok := m.metrics[key]
	if !ok {
		entry = &metricEntry{
			Name:  name,
			Type:  MetricHistogram,
			Min:   value,
			Max:   value,
			Tags:  tags,
		}
		m.metrics[key] = entry
	}
	entry.Count++
	entry.Sum += value
	entry.Value = entry.Sum / float64(entry.Count)
	if value < entry.Min {
		entry.Min = value
	}
	if value > entry.Max {
		entry.Max = value
	}
	entry.LastAt = time.Now()
}

func (m *metricsCollector) SetGauge(ctx context.Context, name string, value float64, tags ...Field) {
	if !m.config.Enabled {
		return
	}
	key := metricKey(name, tags)

	m.mu.Lock()
	defer m.mu.Unlock()

	entry, ok := m.metrics[key]
	if !ok {
		entry = &metricEntry{Name: name, Type: MetricGauge, Tags: tags}
		m.metrics[key] = entry
	}
	entry.Value = value
	entry.LastAt = time.Now()
}

func (m *metricsCollector) Snapshot() []MetricSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()

	snapshots := make([]MetricSnapshot, 0, len(m.metrics))
	for _, e := range m.metrics {
		snapshots = append(snapshots, MetricSnapshot{
			Name:  e.Name,
			Type:  e.Type,
			Value: e.Value,
			Count: e.Count,
			Min:   e.Min,
			Max:   e.Max,
			Sum:   e.Sum,
			Tags:  e.Tags,
		})
	}
	sort.Slice(snapshots, func(i, j int) bool {
		return snapshots[i].Name < snapshots[j].Name
	})
	return snapshots
}

type MetricSnapshot struct {
	Name  string
	Type  MetricType
	Value float64
	Count int64
	Min   float64
	Max   float64
	Sum   float64
	Tags  []Field
}

func metricKey(name string, tags []Field) string {
	key := name
	if len(tags) > 0 {
		sort.Slice(tags, func(i, j int) bool { return tags[i].Key < tags[j].Key })
		for _, t := range tags {
			key += fmt.Sprintf(";%s=%v", t.Key, t.Value)
		}
	}
	return key
}
