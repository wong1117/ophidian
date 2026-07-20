package plugins

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type testPlugin struct {
	name    string
	version string
	typ     string
	initErr error
	startErr error
	stopErr  error
	injected bool
	execFn   func(params map[string]interface{}) (map[string]interface{}, error)
	mu       sync.Mutex
	events   []string
	config   json.RawMessage
	deps     map[string]interface{}
}

func newTestPlugin(name, version, typ string) *testPlugin {
	return &testPlugin{
		name:    name,
		version: version,
		typ:     typ,
	}
}

func (p *testPlugin) Name() string     { return p.name }
func (p *testPlugin) Version() string  { return p.version }
func (p *testPlugin) Type() string     { return p.typ }

func (p *testPlugin) Execute(params map[string]interface{}) (map[string]interface{}, error) {
	if p.execFn != nil {
		return p.execFn(params)
	}
	return map[string]interface{}{"result": "ok"}, nil
}

func (p *testPlugin) Initialize(ctx context.Context, config json.RawMessage) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.events = append(p.events, "initialize")
	p.config = config
	return p.initErr
}

func (p *testPlugin) Start(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.events = append(p.events, "start")
	return p.startErr
}

func (p *testPlugin) Stop(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.events = append(p.events, "stop")
	return p.stopErr
}

func (p *testPlugin) Inject(deps DependencyContainer) error {
	p.injected = true
	p.deps = deps.List()
	return nil
}

type testLogger struct {
	entries []string
	mu      sync.Mutex
}

func (l *testLogger) Debug(msg string, kv ...interface{}) {}
func (l *testLogger) Info(msg string, kv ...interface{})  { l.record(msg) }
func (l *testLogger) Warn(msg string, kv ...interface{})  { l.record(msg) }
func (l *testLogger) Error(msg string, kv ...interface{}) { l.record(msg) }
func (l *testLogger) record(msg string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries = append(l.entries, msg)
}

func TestPluginManager_Register(t *testing.T) {
	m := NewPluginManager()
	plug := newTestPlugin("test", "1.0.0", "scanner")

	err := m.Register(plug, PluginMetadata{
		Name:    plug.Name(),
		Version: plug.Version(),
		Type:    plug.Type(),
	}, nil)

	assert.NoError(t, err)
	assert.Len(t, m.plugins, 1)
}

func TestPluginManager_Register_Duplicate(t *testing.T) {
	m := NewPluginManager()
	plug := newTestPlugin("test", "1.0.0", "scanner")

	_ = m.Register(plug, PluginMetadata{Name: "test", Version: "1.0.0", Type: "scanner"}, nil)
	err := m.Register(plug, PluginMetadata{Name: "test", Version: "1.0.0", Type: "scanner"}, nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already registered")
}

func TestPluginManager_Register_EmptyName(t *testing.T) {
	m := NewPluginManager()
	plug := newTestPlugin("", "1.0.0", "scanner")

	err := m.Register(plug, PluginMetadata{}, nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "name is required")
}

func TestPluginManager_Register_EmitsEvent(t *testing.T) {
	m := NewPluginManager()
	plug := newTestPlugin("test", "1.0.0", "scanner")

	m.Register(plug, PluginMetadata{Name: "test", Version: "1.0.0", Type: "scanner"}, nil)

	select {
	case evt := <-m.Events():
		assert.Equal(t, EventPluginLoaded, evt.EventType)
		assert.Equal(t, "test", evt.PluginName)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("expected event not received")
	}
}

func TestPluginManager_Unregister(t *testing.T) {
	m := NewPluginManager()
	plug := newTestPlugin("test", "1.0.0", "scanner")

	m.Register(plug, PluginMetadata{Name: "test", Version: "1.0.0", Type: "scanner"}, nil)
	err := m.Unregister("test")

	assert.NoError(t, err)
	assert.Empty(t, m.plugins)

	events := drainEvents(m)
	assertHasEvent(t, events, EventPluginUnloaded)
}

func TestPluginManager_Unregister_NotFound(t *testing.T) {
	m := NewPluginManager()
	err := m.Unregister("nonexistent")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestPluginManager_Lifecycle(t *testing.T) {
	m := NewPluginManager()
	plug := newTestPlugin("test", "1.0.0", "scanner")

	m.Register(plug, PluginMetadata{Name: "test", Version: "1.0.0", Type: "scanner"}, nil)

	ctx := context.Background()

	err := m.InitializeAll(ctx)
	assert.NoError(t, err)
	assert.Contains(t, plug.events, "initialize")

	err = m.StartAll(ctx)
	assert.NoError(t, err)
	assert.Contains(t, plug.events, "start")

	err = m.StopAll(ctx)
	assert.NoError(t, err)
	assert.Contains(t, plug.events, "stop")
}

func TestPluginManager_Lifecycle_MultiplePlugins(t *testing.T) {
	m := NewPluginManager()

	p1 := newTestPlugin("p1", "1.0.0", "type-a")
	p2 := newTestPlugin("p2", "2.0.0", "type-b")
	p3 := newTestPlugin("p3", "1.5.0", "type-c")

	m.Register(p1, PluginMetadata{Name: "p1", Version: "1.0.0", Type: "type-a"}, nil)
	m.Register(p2, PluginMetadata{Name: "p2", Version: "2.0.0", Type: "type-b"}, nil)
	m.Register(p3, PluginMetadata{Name: "p3", Version: "1.5.0", Type: "type-c"}, nil)

	ctx := context.Background()

	assert.NoError(t, m.InitializeAll(ctx))
	assert.NoError(t, m.StartAll(ctx))
	assert.NoError(t, m.StopAll(ctx))

	assert.Len(t, p1.events, 3)
	assert.Len(t, p2.events, 3)
	assert.Len(t, p3.events, 3)
}

func TestPluginManager_DependencyInjection(t *testing.T) {
	m := NewPluginManager(
		WithDependency("db", "postgres-conn"),
		WithDependency("cache", "redis-conn"),
	)

	plug := newTestPlugin("test", "1.0.0", "scanner")

	m.Register(plug, PluginMetadata{Name: "test", Version: "1.0.0", Type: "scanner"}, nil)
	ctx := context.Background()

	m.InitializeAll(ctx)

	assert.True(t, plug.injected)
	assert.Equal(t, "postgres-conn", plug.deps["db"])
	assert.Equal(t, "redis-conn", plug.deps["cache"])
}

func TestPluginManager_InitializeError(t *testing.T) {
	m := NewPluginManager()
	plug := newTestPlugin("test", "1.0.0", "scanner")
	plug.initErr = assert.AnError

	m.Register(plug, PluginMetadata{Name: "test", Version: "1.0.0", Type: "scanner"}, nil)

	ctx := context.Background()
	err := m.InitializeAll(ctx)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "initialize plugin test")
}

func TestPluginManager_StartError(t *testing.T) {
	m := NewPluginManager()
	plug := newTestPlugin("test", "1.0.0", "scanner")
	plug.startErr = assert.AnError

	m.Register(plug, PluginMetadata{Name: "test", Version: "1.0.0", Type: "scanner"}, nil)

	ctx := context.Background()
	err := m.StartAll(ctx)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "start plugin test")
}

func TestPluginManager_WithLogger(t *testing.T) {
	logger := &testLogger{}
	m := NewPluginManager(WithLogger(logger))

	plug := newTestPlugin("test", "1.0.0", "scanner")
	m.Register(plug, PluginMetadata{Name: "test", Version: "1.0.0", Type: "scanner"}, nil)

	assert.NotEmpty(t, logger.entries)
	assert.Contains(t, logger.entries[0], "plugin registered")
}

func TestPluginManager_List(t *testing.T) {
	m := NewPluginManager()

	p1 := newTestPlugin("a", "1.0.0", "t1")
	p2 := newTestPlugin("b", "2.0.0", "t2")

	m.Register(p1, PluginMetadata{Name: "a", Version: "1.0.0", Type: "t1"}, nil)
	m.Register(p2, PluginMetadata{Name: "b", Version: "2.0.0", Type: "t2"}, nil)

	list := m.List()

	assert.Len(t, list, 2)
}

func TestPluginManager_Get(t *testing.T) {
	m := NewPluginManager()
	plug := newTestPlugin("test", "1.0.0", "scanner")

	m.Register(plug, PluginMetadata{Name: "test", Version: "1.0.0", Type: "scanner"}, nil)

	got, ok := m.Get("test")
	assert.True(t, ok)
	assert.Equal(t, plug, got)

	_, ok = m.Get("nonexistent")
	assert.False(t, ok)
}

func TestPluginManager_GetMetadata(t *testing.T) {
	m := NewPluginManager()
	meta := PluginMetadata{
		Name:    "test",
		Version: "1.0.0",
		Type:    "scanner",
		Tags:    []string{"network", "recon"},
	}

	m.Register(newTestPlugin("test", "1.0.0", "scanner"), meta, nil)

	got, ok := m.GetMetadata("test")
	assert.True(t, ok)
	assert.Equal(t, "test", got.Name)
	assert.Equal(t, []string{"network", "recon"}, got.Tags)
}

func TestPluginManager_RegisterService(t *testing.T) {
	m := NewPluginManager()
	m.RegisterService("http-client", "some-client")

	assert.Len(t, m.services, 1)
}

func TestPluginManager_RegisterHandler(t *testing.T) {
	m := NewPluginManager()
	m.RegisterHandler("GET", "/api/scan", func() {})

	assert.Len(t, m.handlers, 1)
}

func TestValidateManifest_Valid(t *testing.T) {
	meta := PluginMetadata{
		Name:    "my-plugin",
		Version: "1.0.0",
		Type:    "scanner",
		Dependencies: []string{"dep-a", "dep-b"},
	}

	err := ValidateManifest(meta)
	assert.NoError(t, err)
}

func TestValidateManifest_EmptyName(t *testing.T) {
	err := ValidateManifest(PluginMetadata{Version: "1.0.0", Type: "scanner"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "name is required")
}

func TestValidateManifest_InvalidChars(t *testing.T) {
	err := ValidateManifest(PluginMetadata{Name: "bad/name", Version: "1.0.0", Type: "scanner"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid characters")
}

func TestValidateManifest_EmptyVersion(t *testing.T) {
	err := ValidateManifest(PluginMetadata{Name: "test", Type: "scanner"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "version is required")
}

func TestValidateManifest_EmptyType(t *testing.T) {
	err := ValidateManifest(PluginMetadata{Name: "test", Version: "1.0.0"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "type is required")
}

func TestValidateManifest_EmptyDependency(t *testing.T) {
	err := ValidateManifest(PluginMetadata{
		Name: "test", Version: "1.0.0", Type: "scanner",
		Dependencies: []string{"valid", ""},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "dependency name is empty")
}

func TestDependencyStore(t *testing.T) {
	store := newDependencyStore()

	store.Set("key", "value")
	v, ok := store.Get("key")
	assert.True(t, ok)
	assert.Equal(t, "value", v)

	_, ok = store.Get("missing")
	assert.False(t, ok)

	list := store.List()
	assert.Len(t, list, 1)
}

func TestLoader_BackwardCompatible(t *testing.T) {
	l := NewLoader()
	assert.NotNil(t, l)
	assert.Equal(t, 0, l.Count())
}

func TestToMetadata(t *testing.T) {
	m := PluginManifest{
		Name:        "test",
		Version:     "1.0.0",
		Type:        "scanner",
		Description: "desc",
		Author:      "author",
		License:     "MIT",
		Dependencies: []string{"dep"},
		Config:      json.RawMessage(`{"key":"value"}`),
	}

	meta := toMetadata(m)

	assert.Equal(t, "test", meta.Name)
	assert.Equal(t, "1.0.0", meta.Version)
	assert.Equal(t, "scanner", meta.Type)
	assert.Equal(t, "desc", meta.Description)
	assert.Equal(t, "MIT", meta.License)
	assert.Equal(t, json.RawMessage(`{"key":"value"}`), meta.Config)
}

func drainEvents(m *PluginManager) []PluginEvent {
	var events []PluginEvent
	for {
		select {
		case evt := <-m.Events():
			events = append(events, evt)
		default:
			return events
		}
	}
}

func assertHasEvent(t *testing.T, events []PluginEvent, eventType PluginEventType) {
	t.Helper()
	for _, ev := range events {
		if ev.EventType == eventType {
			return
		}
	}
	t.Errorf("expected event %s", eventType)
}
