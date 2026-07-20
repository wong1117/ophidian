package opsec

import (
	"context"
	"encoding/base64"
	"github.com/ophidian/ophidian/internal/domain/common"
)

type InjectionMethod string

const (
	InjectProcessHollow InjectionMethod = "process_hollowing"
	InjectReflectiveDLL InjectionMethod = "reflective_dll"
	InjectShellcode     InjectionMethod = "shellcode_injection"
	InjectThreadless    InjectionMethod = "threadless_inject"
	InjectExecMemFD     InjectionMethod = "memfd_linux"
)

type MemoryPayload struct {
	ID       common.ID
	Data     []byte
	Method   InjectionMethod
	Arch     string
	OS       string
	EntryPoint string
}

type MemoryExecutor interface {
	ExecuteInMemory(ctx context.Context, payload *MemoryPayload) (string, error)
	InjectIntoProcess(ctx context.Context, pid int, payload *MemoryPayload) error
	ExecuteShellcode(ctx context.Context, shellcode []byte, method InjectionMethod) (string, error)
}

type MemoryExecutionEngine struct {
	executor MemoryExecutor
}

func NewMemoryExecutionEngine(executor MemoryExecutor) *MemoryExecutionEngine {
	return &MemoryExecutionEngine{executor: executor}
}

func (e *MemoryExecutionEngine) ExecutePayload(ctx context.Context, payload *MemoryPayload) (string, error) {
	return e.executor.ExecuteInMemory(ctx, payload)
}

func (e *MemoryExecutionEngine) ExecuteShellcode(ctx context.Context, shellcode []byte, arch string) (string, error) {
	method := InjectShellcode
	if arch == "x64" {
		method = InjectShellcode
	}
	return e.executor.ExecuteShellcode(ctx, shellcode, method)
}

func (e *MemoryExecutionEngine) BuildReflectivePayload(dllData []byte) *MemoryPayload {
	return &MemoryPayload{
		ID:     common.NewID(),
		Data:   dllData,
		Method: InjectReflectiveDLL,
	}
}

func (e *MemoryExecutionEngine) EncodePayload(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

func (e *MemoryExecutionEngine) DecodePayload(encoded string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(encoded)
}
