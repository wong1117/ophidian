package opsec

import (
	"strings"
)

type OSType string

const (
	OSWindows OSType = "windows"
	OSLinux   OSType = "linux"
	OSMacOS   OSType = "macos"
)

type Utility struct {
	Name        string
	Path        string
	Args        string
	Capabilities []string
	OS           OSType
	DetectionRisk int // 1-10, lower is better
}

type LotLDatabase struct {
	utilities []Utility
}

func NewLotLDatabase() *LotLDatabase {
	db := &LotLDatabase{}
	db.initWindows()
	db.initLinux()
	return db
}

func (db *LotLDatabase) initWindows() {
	db.utilities = append(db.utilities,
		Utility{Name: "certutil", Path: "certutil.exe", Capabilities: []string{"download", "encode", "decode", "hash"}, OS: OSWindows, DetectionRisk: 3},
		Utility{Name: "wmic", Path: "wmic.exe", Capabilities: []string{"process", "service", "query", "command"}, OS: OSWindows, DetectionRisk: 3},
		Utility{Name: "powershell", Path: "powershell.exe", Capabilities: []string{"script", "download", "execute", "bypass"}, OS: OSWindows, DetectionRisk: 2},
		Utility{Name: "cscript", Path: "cscript.exe", Capabilities: []string{"run_script", "download"}, OS: OSWindows, DetectionRisk: 4},
		Utility{Name: "mshta", Path: "mshta.exe", Capabilities: []string{"execute_hta", "download"}, OS: OSWindows, DetectionRisk: 5},
		Utility{Name: "rundll32", Path: "rundll32.exe", Capabilities: []string{"execute_dll", "download"}, OS: OSWindows, DetectionRisk: 5},
		Utility{Name: "regsvr32", Path: "regsvr32.exe", Capabilities: []string{"register_dll", "execute_code"}, OS: OSWindows, DetectionRisk: 4},
		Utility{Name: "msiexec", Path: "msiexec.exe", Capabilities: []string{"install_msi", "execute_code"}, OS: OSWindows, DetectionRisk: 5},
		Utility{Name: "wscript", Path: "wscript.exe", Capabilities: []string{"run_script", "execute_vba"}, OS: OSWindows, DetectionRisk: 4},
		Utility{Name: "bitsadmin", Path: "bitsadmin.exe", Capabilities: []string{"download", "upload"}, OS: OSWindows, DetectionRisk: 4},
		Utility{Name: "certreq", Path: "certreq.exe", Capabilities: []string{"download", "upload"}, OS: OSWindows, DetectionRisk: 6},
		Utility{Name: "cmstp", Path: "cmstp.exe", Capabilities: []string{"execute_script", "bypass"}, OS: OSWindows, DetectionRisk: 5},
		Utility{Name: "iexplore", Path: "iexplore.exe", Capabilities: []string{"download", "execute_code"}, OS: OSWindows, DetectionRisk: 6},
		Utility{Name: "msbuild", Path: "msbuild.exe", Capabilities: []string{"compile_execute"}, OS: OSWindows, DetectionRisk: 3},
		Utility{Name: "installutil", Path: "installutil.exe", Capabilities: []string{"execute_code"}, OS: OSWindows, DetectionRisk: 4},
		Utility{Name: "msdt", Path: "msdt.exe", Capabilities: []string{"execute_diagpkg"}, OS: OSWindows, DetectionRisk: 5},
		Utility{Name: "pcalua", Path: "pcalua.exe", Capabilities: []string{"execute_com", "execute_script"}, OS: OSWindows, DetectionRisk: 6},
		Utility{Name: "regsvcs", Path: "regsvcs.exe", Capabilities: []string{"execute_code"}, OS: OSWindows, DetectionRisk: 5},
		Utility{Name: "regasm", Path: "regasm.exe", Capabilities: []string{"execute_code"}, OS: OSWindows, DetectionRisk: 5},
		Utility{Name: "msxsl", Path: "msxsl.exe", Capabilities: []string{"execute_script"}, OS: OSWindows, DetectionRisk: 6},
		Utility{Name: "wmips", Path: "wmiprvse.exe", Capabilities: []string{"wmi_query", "execute_code"}, OS: OSWindows, DetectionRisk: 4},
	)
}

func (db *LotLDatabase) initLinux() {
	db.utilities = append(db.utilities,
		Utility{Name: "bash", Path: "/bin/bash", Capabilities: []string{"script", "execute", "pipe"}, OS: OSLinux, DetectionRisk: 2},
		Utility{Name: "curl", Path: "/usr/bin/curl", Capabilities: []string{"download", "upload", "api"}, OS: OSLinux, DetectionRisk: 2},
		Utility{Name: "wget", Path: "/usr/bin/wget", Capabilities: []string{"download", "upload"}, OS: OSLinux, DetectionRisk: 2},
		Utility{Name: "python", Path: "/usr/bin/python3", Capabilities: []string{"script", "download", "execute"}, OS: OSLinux, DetectionRisk: 3},
		Utility{Name: "perl", Path: "/usr/bin/perl", Capabilities: []string{"script", "execute"}, OS: OSLinux, DetectionRisk: 3},
		Utility{Name: "ruby", Path: "/usr/bin/ruby", Capabilities: []string{"script", "reverse_shell"}, OS: OSLinux, DetectionRisk: 4},
		Utility{Name: "php", Path: "/usr/bin/php", Capabilities: []string{"script", "execute"}, OS: OSLinux, DetectionRisk: 4},
		Utility{Name: "lua", Path: "/usr/bin/lua", Capabilities: []string{"script"}, OS: OSLinux, DetectionRisk: 5},
		Utility{Name: "openssl", Path: "/usr/bin/openssl", Capabilities: []string{"encrypt", "decrypt", "ssl"}, OS: OSLinux, DetectionRisk: 2},
		Utility{Name: "nc", Path: "/usr/bin/nc", Capabilities: []string{"connect", "listen", "download"}, OS: OSLinux, DetectionRisk: 4},
		Utility{Name: "ncat", Path: "/usr/bin/ncat", Capabilities: []string{"connect", "listen", "ssl"}, OS: OSLinux, DetectionRisk: 4},
		Utility{Name: "socat", Path: "/usr/bin/socat", Capabilities: []string{"connect", "listen", "forward"}, OS: OSLinux, DetectionRisk: 4},
		Utility{Name: "gawk", Path: "/usr/bin/gawk", Capabilities: []string{"network", "execute"}, OS: OSLinux, DetectionRisk: 5},
		Utility{Name: "telnet", Path: "/usr/bin/telnet", Capabilities: []string{"connect"}, OS: OSLinux, DetectionRisk: 3},
		Utility{Name: "ssh", Path: "/usr/bin/ssh", Capabilities: []string{"connect", "tunnel", "execute"}, OS: OSLinux, DetectionRisk: 2},
		Utility{Name: "scp", Path: "/usr/bin/scp", Capabilities: []string{"download", "upload"}, OS: OSLinux, DetectionRisk: 3},
		Utility{Name: "rsync", Path: "/usr/bin/rsync", Capabilities: []string{"download", "upload"}, OS: OSLinux, DetectionRisk: 4},
		Utility{Name: "git", Path: "/usr/bin/git", Capabilities: []string{"download", "upload"}, OS: OSLinux, DetectionRisk: 2},
		Utility{Name: "svn", Path: "/usr/bin/svn", Capabilities: []string{"download"}, OS: OSLinux, DetectionRisk: 4},
		Utility{Name: "pip", Path: "/usr/bin/pip3", Capabilities: []string{"download"}, OS: OSLinux, DetectionRisk: 4},
	)
}

func (db *LotLDatabase) FindUtilities(os OSType, capability string) []Utility {
	var results []Utility
	for _, u := range db.utilities {
		if u.OS != os {
			continue
		}
		for _, cap := range u.Capabilities {
			if strings.EqualFold(cap, capability) {
				results = append(results, u)
				break
			}
		}
	}
	return results
}

func (db *LotLDatabase) FindBestUtility(os OSType, capability string) *Utility {
	utilities := db.FindUtilities(os, capability)
	if len(utilities) == 0 {
		return nil
	}
	best := &utilities[0]
	for _, u := range utilities[1:] {
		if u.DetectionRisk < best.DetectionRisk {
			best = &u
		}
	}
	return best
}
