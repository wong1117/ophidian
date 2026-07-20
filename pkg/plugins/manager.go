package plugins

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	goplugin "plugin"
)

type PluginManager struct {
	mu        sync.RWMutex
	plugins   map[string]*pluginEntry
	services  map[string]interface{}
	handlers  map[string]interface{}
	logger    PluginLogger
	events    chan PluginEvent
	depStore  *dependencyStore
	dirs      []string
}

type pluginEntry struct {
	instance interface{}
	metadata PluginMetadata
	config   json.RawMessage
	status   pluginStatus
	loadTime time.Time
	deps     []string
}

type pluginStatus int

const (
	statusRegistered pluginStatus = iota
	statusInitialized
	statusStarted
	statusStopped
	statusError
)

type dependencyStore struct {
	mu   sync.RWMutex
	deps map[string]interface{}
}

func newDependencyStore() *dependencyStore {
	return &dependencyStore{deps: make(map[string]interface{})}
}

func (d *dependencyStore) Get(name string) (interface{}, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	v, ok := d.deps[name]
	return v, ok
}

func (d *dependencyStore) Set(name string, dep interface{}) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.deps[name] = dep
}

func (d *dependencyStore) List() map[string]interface{} {
	d.mu.RLock()
	defer d.mu.RUnlock()
	result := make(map[string]interface{}, len(d.deps))
	for k, v := range d.deps {
		result[k] = v
	}
	return result
}

type ManagerOption func(*PluginManager)

func WithLogger(logger PluginLogger) ManagerOption {
	return func(m *PluginManager) { m.logger = logger }
}

func WithDirectories(dirs ...string) ManagerOption {
	return func(m *PluginManager) { m.dirs = dirs }
}

func WithDependency(name string, dep interface{}) ManagerOption {
	return func(m *PluginManager) { m.depStore.Set(name, dep) }
}

func NewPluginManager(opts ...ManagerOption) *PluginManager {
	m := &PluginManager{
		plugins:  make(map[string]*pluginEntry),
		services: make(map[string]interface{}),
		handlers: make(map[string]interface{}),
		events:   make(chan PluginEvent, 100),
		depStore: newDependencyStore(),
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

func (m *PluginManager) Events() <-chan PluginEvent {
	return m.events
}

func (m *PluginManager) Register(instance interface{}, meta PluginMetadata, config json.RawMessage) error {
	name := meta.Name
	if name == "" {
		if p, ok := instance.(Plugin); ok {
			name = p.Name()
		}
	}
	if name == "" {
		return fmt.Errorf("register plugin: name is required")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.plugins[name]; exists {
		return fmt.Errorf("register plugin: %s already registered", name)
	}

	m.plugins[name] = &pluginEntry{
		instance: instance,
		metadata: meta,
		config:   config,
		status:   statusRegistered,
		loadTime: time.Now(),
		deps:     meta.Dependencies,
	}

	if lp, ok := instance.(LifecyclePlugin); ok {
		meta := lp
		_ = meta
	}

	m.emit(PluginEvent{
		PluginName: name,
		EventType:  EventPluginLoaded,
		Metadata:   meta,
		Timestamp:  time.Now(),
	})
	m.logInfo("plugin registered", "name", name, "version", meta.Version, "type", meta.Type)

	return nil
}

func (m *PluginManager) Unregister(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	entry, ok := m.plugins[name]
	if !ok {
		return fmt.Errorf("unregister plugin: %s not found", name)
	}

	if entry.status == statusStarted {
		if lp, ok := entry.instance.(LifecyclePlugin); ok {
			if err := lp.Stop(context.Background()); err != nil {
				m.emit(PluginEvent{
					PluginName: name, EventType: EventPluginError,
					Metadata: entry.metadata, Timestamp: time.Now(), Error: err.Error(),
				})
			}
		}
	}

	delete(m.plugins, name)

	m.emit(PluginEvent{
		PluginName: name,
		EventType:  EventPluginUnloaded,
		Metadata:   entry.metadata,
		Timestamp:  time.Now(),
	})
	m.logInfo("plugin unregistered", "name", name)

	return nil
}

func (m *PluginManager) InitializeAll(ctx context.Context) error {
	m.mu.RLock()
	entries := make([]*pluginEntry, 0, len(m.plugins))
	for _, e := range m.plugins {
		entries = append(entries, e)
	}
	m.mu.RUnlock()

	for _, entry := range entries {
		if err := m.initializePlugin(ctx, entry); err != nil {
			return err
		}
	}
	return nil
}

func (m *PluginManager) StartAll(ctx context.Context) error {
	m.mu.RLock()
	entries := make([]*pluginEntry, 0, len(m.plugins))
	for _, e := range m.plugins {
		entries = append(entries, e)
	}
	m.mu.RUnlock()

	for _, entry := range entries {
		if err := m.startPlugin(ctx, entry); err != nil {
			return err
		}
	}
	return nil
}

func (m *PluginManager) StopAll(ctx context.Context) error {
	m.mu.RLock()
	entries := make([]*pluginEntry, 0, len(m.plugins))
	for _, e := range m.plugins {
		entries = append(entries, e)
	}
	m.mu.RUnlock()

	for i := len(entries) - 1; i >= 0; i-- {
		if err := m.stopPlugin(ctx, entries[i]); err != nil {
			return err
		}
	}
	return nil
}

func (m *PluginManager) initializePlugin(ctx context.Context, entry *pluginEntry) error {
	if lp, ok := entry.instance.(LifecyclePlugin); ok {
		if err := lp.Initialize(ctx, entry.config); err != nil {
			entry.status = statusError
			return fmt.Errorf("initialize plugin %s: %w", entry.metadata.Name, err)
		}
	}
	entry.status = statusInitialized

	if inj, ok := entry.instance.(Injectable); ok {
		if err := inj.Inject(m.depStore); err != nil {
			return fmt.Errorf("inject dependencies for %s: %w", entry.metadata.Name, err)
		}
	}

	return nil
}

func (m *PluginManager) startPlugin(ctx context.Context, entry *pluginEntry) error {
	if entry.status != statusInitialized {
		if err := m.initializePlugin(ctx, entry); err != nil {
			return err
		}
	}

	if lp, ok := entry.instance.(LifecyclePlugin); ok {
		if err := lp.Start(ctx); err != nil {
			entry.status = statusError
			return fmt.Errorf("start plugin %s: %w", entry.metadata.Name, err)
		}
	}
	entry.status = statusStarted

	m.emit(PluginEvent{
		PluginName: entry.metadata.Name,
		EventType:  EventPluginStarted,
		Metadata:   entry.metadata,
		Timestamp:  time.Now(),
	})
	m.logInfo("plugin started", "name", entry.metadata.Name)

	return nil
}

func (m *PluginManager) stopPlugin(ctx context.Context, entry *pluginEntry) error {
	if entry.status != statusStarted && entry.status != statusInitialized {
		return nil
	}

	if lp, ok := entry.instance.(LifecyclePlugin); ok {
		if err := lp.Stop(ctx); err != nil {
			return fmt.Errorf("stop plugin %s: %w", entry.metadata.Name, err)
		}
	}
	entry.status = statusStopped

	m.emit(PluginEvent{
		PluginName: entry.metadata.Name,
		EventType:  EventPluginStopped,
		Metadata:   entry.metadata,
		Timestamp:  time.Now(),
	})
	m.logInfo("plugin stopped", "name", entry.metadata.Name)

	return nil
}

func (m *PluginManager) Discover(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("discover plugins: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !strings.HasSuffix(entry.Name(), ".so") {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		manifestPath := strings.TrimSuffix(path, ".so") + ".json"
		if _, statErr := os.Stat(manifestPath); statErr != nil {
			manifestPath = ""
		}

		if err := m.loadPluginFile(path, manifestPath); err != nil {
			m.logWarn("plugin discovery failed", "path", path, "error", err.Error())
			continue
		}
	}
	return nil
}

func (m *PluginManager) loadPluginFile(path, manifestPath string) error {
	p, err := goplugin.Open(path)
	if err != nil {
		return fmt.Errorf("open plugin %s: %w", path, err)
	}

	sym, err := p.Lookup("NewPlugin")
	if err != nil {
		return fmt.Errorf("lookup NewPlugin in %s: %w", path, err)
	}

	constructor, ok := sym.(func() interface{})
	if !ok {
		return fmt.Errorf("NewPlugin in %s is not func() interface{}", path)
	}

	instance := constructor()

	var meta PluginMetadata
	if manifestPath != "" {
		data, err := os.ReadFile(manifestPath)
		if err != nil {
			return fmt.Errorf("read manifest %s: %w", manifestPath, err)
		}
		var manifest PluginManifest
		if err := json.Unmarshal(data, &manifest); err != nil {
			return fmt.Errorf("parse manifest %s: %w", manifestPath, err)
		}
		meta = toMetadata(manifest)
	} else {
		if p, ok := instance.(Plugin); ok {
			meta = PluginMetadata{
				Name:    p.Name(),
				Version: p.Version(),
				Type:    p.Type(),
			}
		}
	}

	if err := ValidateManifest(meta); err != nil {
		return fmt.Errorf("validate manifest for %s: %w", path, err)
	}

	return m.Register(instance, meta, nil)
}

func (m *PluginManager) List() []PluginMetadata {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]PluginMetadata, 0, len(m.plugins))
	for _, e := range m.plugins {
		result = append(result, e.metadata)
	}
	return result
}

func (m *PluginManager) Get(name string) (interface{}, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	e, ok := m.plugins[name]
	if !ok {
		return nil, false
	}
	return e.instance, true
}

func (m *PluginManager) GetMetadata(name string) (PluginMetadata, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	e, ok := m.plugins[name]
	if !ok {
		return PluginMetadata{}, false
	}
	return e.metadata, true
}

func (m *PluginManager) RegisterService(name string, handler interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.services[name] = handler
}

func (m *PluginManager) RegisterHandler(method, path string, handler interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := fmt.Sprintf("%s:%s", method, path)
	m.handlers[key] = handler
}

func (m *PluginManager) emit(event PluginEvent) {
	select {
	case m.events <- event:
	default:
	}
}

func (m *PluginManager) logInfo(msg string, kv ...interface{}) {
	if m.logger != nil {
		m.logger.Info(msg, kv...)
	}
}

func (m *PluginManager) logWarn(msg string, kv ...interface{}) {
	if m.logger != nil {
		m.logger.Warn(msg, kv...)
	}
}

func toMetadata(m PluginManifest) PluginMetadata {
	return PluginMetadata{
		Name:         m.Name,
		Version:      m.Version,
		Type:         m.Type,
		Description:  m.Description,
		Author:       m.Author,
		License:      m.License,
		Homepage:     m.Homepage,
		Dependencies: m.Dependencies,
		MinVersion:   m.MinVersion,
		MaxVersion:   m.MaxVersion,
		Tags:         m.Tags,
		Config:       m.Config,
	}
}
