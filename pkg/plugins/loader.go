package plugins

import (
	"fmt"
	"plugin"
)

// Loader is retained for backward compatibility.
// New code should use PluginManager for full lifecycle support.
type Loader struct {
	plugins map[string]Plugin
}

func NewLoader() *Loader {
	return &Loader{
		plugins: make(map[string]Plugin),
	}
}

func (l *Loader) Load(path string) error {
	p, err := plugin.Open(path)
	if err != nil {
		return fmt.Errorf("load plugin %s: %w", path, err)
	}

	sym, err := p.Lookup("Plugin")
	if err != nil {
		return fmt.Errorf("lookup Plugin symbol in %s: %w", path, err)
	}

	plug, ok := sym.(Plugin)
	if !ok {
		return fmt.Errorf("plugin symbol in %s does not implement Plugin interface", path)
	}

	l.Register(plug)
	return nil
}

func (l *Loader) Register(p Plugin) {
	l.plugins[p.Name()] = p
}

func (l *Loader) Get(name string) (Plugin, bool) {
	p, ok := l.plugins[name]
	return p, ok
}

func (l *Loader) List() []Plugin {
	result := make([]Plugin, 0, len(l.plugins))
	for _, p := range l.plugins {
		result = append(result, p)
	}
	return result
}

func (l *Loader) Count() int {
	return len(l.plugins)
}
