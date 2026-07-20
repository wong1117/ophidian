package plugins

import (
	"fmt"
	"plugin"
)

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
		return err
	}
	return nil
}

func (l *Loader) Register(p Plugin) {
	l.plugins[p.Name()] = p
}

func (l *Loader) Get(name string) (Plugin, bool) {
	p, ok := l.plugins[name]
	return p, ok
}
