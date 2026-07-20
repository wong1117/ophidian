package persistence

import (
	"context"
	"github.com/ophidian/ophidian/internal/domain/common"
)

type MethodType string

const (
	MethodWMISubscription MethodType = "wmi_event_subscription"
	MethodCOMHijack      MethodType = "com_hijacking"
	MethodScheduledTask   MethodType = "scheduled_task"
	MethodRegistryRun     MethodType = "registry_run"
	MethodServiceInstall  MethodType = "service_install"
	MethodStartupFolder   MethodType = "startup_folder"
	MethodDLLHijack       MethodType = "dll_hijacking"
	MethodImageFileOpt    MethodType = "image_file_execution_options"
	MethodBootkit         MethodType = "bootkit"
	MethodLDPreload       MethodType = "ld_preload"
	MethodCronJob         MethodType = "cron_job"
	MethodSSHKey          MethodType = "ssh_authorized_key"
	MethodProfileDotD     MethodType = "profile_d_dropin"
	MethodSystemdService  MethodType = "systemd_service"
)

type PersistenceModule struct {
	ID          common.ID
	Method      MethodType
	Name        string
	Description string
	OS          string
	Stealth     int
	RiskLevel   string
	Config      map[string]interface{}
	CleanupCmd  string
}

type ModuleExecutor interface {
	Install(ctx context.Context, module *PersistenceModule) error
	Remove(ctx context.Context, module *PersistenceModule) error
	Check(ctx context.Context, module *PersistenceModule) (bool, error)
}

type ModuleManager struct {
	executor ModuleExecutor
	modules  map[MethodType]*PersistenceModule
}

func NewModuleManager(executor ModuleExecutor) *ModuleManager {
	mm := &ModuleManager{
		executor: executor,
		modules:  make(map[MethodType]*PersistenceModule),
	}
	mm.initModules()
	return mm
}

func (mm *ModuleManager) initModules() {
	mm.modules[MethodWMISubscription] = &PersistenceModule{
		Method:      MethodWMISubscription,
		Name:        "WMI Event Subscription",
		Description: "Persist via WMI event filter and consumer",
		OS:          "windows",
		Stealth:     9,
		RiskLevel:   "LOW",
	}
	mm.modules[MethodCOMHijack] = &PersistenceModule{
		Method:      MethodCOMHijack,
		Name:        "COM Hijacking",
		Description: "Hijack COM object registration for persistence",
		OS:          "windows",
		Stealth:     8,
		RiskLevel:   "LOW",
	}
	mm.modules[MethodScheduledTask] = &PersistenceModule{
		Method:      MethodScheduledTask,
		Name:        "Scheduled Task",
		Description: "Create scheduled task for persistence",
		OS:          "windows",
		Stealth:     5,
		RiskLevel:   "MEDIUM",
	}
	mm.modules[MethodRegistryRun] = &PersistenceModule{
		Method:      MethodRegistryRun,
		Name:        "Registry Run Key",
		Description: "Add entry to HKCU Run registry key",
		OS:          "windows",
		Stealth:     3,
		RiskLevel:   "HIGH",
	}
	mm.modules[MethodServiceInstall] = &PersistenceModule{
		Method:      MethodServiceInstall,
		Name:        "Windows Service Installation",
		Description: "Install as a Windows service",
		OS:          "windows",
		Stealth:     4,
		RiskLevel:   "MEDIUM",
	}
	mm.modules[MethodStartupFolder] = &PersistenceModule{
		Method:      MethodStartupFolder,
		Name:        "Startup Folder",
		Description: "Place binary in startup folder",
		OS:          "windows",
		Stealth:     2,
		RiskLevel:   "HIGH",
	}
	mm.modules[MethodDLLHijack] = &PersistenceModule{
		Method:      MethodDLLHijack,
		Name:        "DLL Hijacking",
		Description: "Replace missing DLL with malicious one",
		OS:          "windows",
		Stealth:     7,
		RiskLevel:   "MEDIUM",
	}
	mm.modules[MethodImageFileOpt] = &PersistenceModule{
		Method:      MethodImageFileOpt,
		Name:        "Image File Execution Options",
		Description: "Set debugger for system binaries",
		OS:          "windows",
		Stealth:     6,
		RiskLevel:   "MEDIUM",
	}
	mm.modules[MethodLDPreload] = &PersistenceModule{
		Method:      MethodLDPreload,
		Name:        "LD_PRELOAD",
		Description: "Set LD_PRELOAD environment variable",
		OS:          "linux",
		Stealth:     7,
		RiskLevel:   "MEDIUM",
	}
	mm.modules[MethodCronJob] = &PersistenceModule{
		Method:      MethodCronJob,
		Name:        "Cron Job",
		Description: "Add entry to crontab",
		OS:          "linux",
		Stealth:     4,
		RiskLevel:   "MEDIUM",
	}
	mm.modules[MethodSSHKey] = &PersistenceModule{
		Method:      MethodSSHKey,
		Name:        "SSH Authorized Key",
		Description: "Add SSH public key to authorized_keys",
		OS:          "linux",
		Stealth:     8,
		RiskLevel:   "LOW",
	}
	mm.modules[MethodSystemdService] = &PersistenceModule{
		Method:      MethodSystemdService,
		Name:        "Systemd Service",
		Description: "Create systemd service unit",
		OS:          "linux",
		Stealth:     5,
		RiskLevel:   "MEDIUM",
	}
	mm.modules[MethodProfileDotD] = &PersistenceModule{
		Method:      MethodProfileDotD,
		Name:        "Profile.d Drop-in",
		Description: "Add script to /etc/profile.d/",
		OS:          "linux",
		Stealth:     6,
		RiskLevel:   "MEDIUM",
	}
}

func (mm *ModuleManager) GetModule(method MethodType) *PersistenceModule {
	return mm.modules[method]
}

func (mm *ModuleManager) ListModules(os string) []*PersistenceModule {
	var result []*PersistenceModule
	for _, m := range mm.modules {
		if m.OS == os {
			result = append(result, m)
		}
	}
	return result
}

func (mm *ModuleManager) SelectBestModules(os string, maxStealth int) []*PersistenceModule {
	all := mm.ListModules(os)
	var filtered []*PersistenceModule
	for _, m := range all {
		if m.Stealth <= maxStealth {
			filtered = append(filtered, m)
		}
	}

	for i := 0; i < len(filtered); i++ {
		for j := i + 1; j < len(filtered); j++ {
			if filtered[j].Stealth > filtered[i].Stealth {
				filtered[i], filtered[j] = filtered[j], filtered[i]
			}
		}
	}

	return filtered
}

func (mm *ModuleManager) Install(ctx context.Context, method MethodType) error {
	module, ok := mm.modules[method]
	if !ok {
		return common.ErrInvalidTarget
	}
	return mm.executor.Install(ctx, module)
}

func (mm *ModuleManager) Remove(ctx context.Context, method MethodType) error {
	module, ok := mm.modules[method]
	if !ok {
		return common.ErrInvalidTarget
	}
	return mm.executor.Remove(ctx, module)
}
