package plugins

import (
	"fmt"
	"strings"
)

func ValidateManifest(meta PluginMetadata) error {
	if meta.Name == "" {
		return fmt.Errorf("plugin name is required")
	}
	if strings.ContainsAny(meta.Name, " /\\*?\"<>|") {
		return fmt.Errorf("plugin name contains invalid characters: %s", meta.Name)
	}
	if meta.Version == "" {
		return fmt.Errorf("plugin version is required")
	}
	if meta.Type == "" {
		return fmt.Errorf("plugin type is required")
	}
	for _, dep := range meta.Dependencies {
		if dep == "" {
			return fmt.Errorf("dependency name is empty for plugin %s", meta.Name)
		}
	}
	return nil
}
