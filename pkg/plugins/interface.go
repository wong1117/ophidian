package plugins

import (
	"context"
	"encoding/json"
	"time"
)

// Plugin is the base interface — preserved for backward compatibility.
// Existing plugins implementing this will continue to work.
type Plugin interface {
	Name() string
	Version() string
	Type() string
	Execute(params map[string]interface{}) (map[string]interface{}, error)
}

// LifecyclePlugin extends Plugin with initialization and shutdown hooks.
type LifecyclePlugin interface {
	Plugin
	Initialize(ctx context.Context, config json.RawMessage) error
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}

// ServiceRegistrar allows plugins to register services and HTTP handlers.
type ServiceRegistrar interface {
	RegisterService(name string, handler interface{})
	RegisterHandler(method, path string, handler interface{})
}

// Injectable interface allows plugins to receive dependencies from the manager.
type Injectable interface {
	Inject(deps DependencyContainer) error
}

// DependencyContainer provides named dependencies to plugins.
type DependencyContainer interface {
	Get(name string) (interface{}, bool)
	Set(name string, dep interface{})
	List() map[string]interface{}
}

// PluginMetadata describes a plugin's identity and dependencies.
type PluginMetadata struct {
	Name         string            `json:"name"`
	Version      string            `json:"version"`
	Type         string            `json:"type"`
	Description  string            `json:"description,omitempty"`
	Author       string            `json:"author,omitempty"`
	License      string            `json:"license,omitempty"`
	Homepage     string            `json:"homepage,omitempty"`
	Dependencies []string          `json:"dependencies,omitempty"`
	MinVersion   string            `json:"min_version,omitempty"`
	MaxVersion   string            `json:"max_version,omitempty"`
	Tags         []string          `json:"tags,omitempty"`
	Config       json.RawMessage   `json:"config,omitempty"`
}

// PluginManifest is the structure expected in a plugin's manifest file (plugin.json/plugin.yaml).
type PluginManifest struct {
	Name         string          `json:"name"`
	Version      string          `json:"version"`
	Type         string          `json:"type"`
	Description  string          `json:"description,omitempty"`
	Author       string          `json:"author,omitempty"`
	License      string          `json:"license,omitempty"`
	Homepage     string          `json:"homepage,omitempty"`
	Dependencies []string        `json:"dependencies,omitempty"`
	MinVersion   string          `json:"min_version,omitempty"`
	MaxVersion   string          `json:"max_version,omitempty"`
	Tags         []string        `json:"tags,omitempty"`
	Config       json.RawMessage `json:"config,omitempty"`
	EntryPoint   string          `json:"entry_point,omitempty"`
}

// PluginEvent represents lifecycle events emitted by the PluginManager.
type PluginEvent struct {
	PluginName string
	EventType  PluginEventType
	Metadata   PluginMetadata
	Timestamp  time.Time
	Error      string
}

// PluginEventType identifies the type of lifecycle event.
type PluginEventType string

const (
	EventPluginLoaded   PluginEventType = "PLUGIN_LOADED"
	EventPluginUnloaded PluginEventType = "PLUGIN_UNLOADED"
	EventPluginStarted  PluginEventType = "PLUGIN_STARTED"
	EventPluginStopped  PluginEventType = "PLUGIN_STOPPED"
	EventPluginError    PluginEventType = "PLUGIN_ERROR"
)

// PluginLogger provides structured logging for plugins.
type PluginLogger interface {
	Debug(msg string, kv ...interface{})
	Info(msg string, kv ...interface{})
	Warn(msg string, kv ...interface{})
	Error(msg string, kv ...interface{})
}
