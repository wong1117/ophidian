package plugins

import (
	"github.com/ophidian/ophidian/internal/domain/execution"
)

type ToolRegistry struct {
	tools []execution.ExternalTool
}

func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{tools: make([]execution.ExternalTool, 0)}
}

func (r *ToolRegistry) Register(tool execution.ExternalTool) {
	if tool == nil {
		return
	}
	r.tools = append(r.tools, tool)
}

func (r *ToolRegistry) GetAll() []execution.ExternalTool {
	return r.tools
}
