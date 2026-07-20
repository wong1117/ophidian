package ollama

import (
	"bytes"
	"fmt"
	"text/template"
)

type PromptManager struct {
	templates map[string]*template.Template
}

func NewPromptManager() *PromptManager {
	return &PromptManager{
		templates: make(map[string]*template.Template),
	}
}

func (pm *PromptManager) Register(name, tmpl string) error {
	t, err := template.New(name).Parse(tmpl)
	if err != nil {
		return err
	}
	pm.templates[name] = t
	return nil
}

func (pm *PromptManager) Render(name string, data interface{}) (string, error) {
	t, ok := pm.templates[name]
	if !ok {
		return "", fmt.Errorf("template %s not found", name)
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}
