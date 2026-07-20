package dephealth

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

type DependencyInfo struct {
	Name           string          `json:"name"`
	Version        string          `json:"version"`
	Outdated       bool            `json:"outdated"`
	Vulnerabilities []Vulnerability `json:"vulnerabilities"`
	Direct         bool            `json:"direct"`
}

type Vulnerability struct {
	CVE      string `json:"cve"`
	Package  string `json:"package"`
	Severity string `json:"severity"`
	URL      string `json:"url"`
}

type HealthReport struct {
	GeneratedAt     time.Time        `json:"generated_at"`
	Total           int              `json:"total"`
	Direct          int              `json:"direct"`
	Indirect        int              `json:"indirect"`
	Vulnerable      int              `json:"vulnerable"`
	Dependencies    []DependencyInfo `json:"dependencies"`
	Vulnerabilities []Vulnerability  `json:"vulnerabilities"`
	Status          string           `json:"status"`
}

type Checker struct{}

func NewChecker() *Checker { return &Checker{} }

func (c *Checker) Check() (*HealthReport, error) {
	report := &HealthReport{GeneratedAt: time.Now()}

	deps, err := c.parseGoMod("go.mod")
	if err != nil {
		return nil, fmt.Errorf("parse go.mod: %w", err)
	}

	report.Dependencies = deps
	report.Total = len(deps)
	for _, d := range deps {
		if d.Direct { report.Direct++ } else { report.Indirect++ }
	}

	vulns, err := c.checkVulnsFast()
	if err == nil && len(vulns) > 0 {
		report.Vulnerabilities = vulns
		for _, v := range vulns {
			for i, d := range deps {
				if strings.HasPrefix(d.Name, v.Package) {
					report.Dependencies[i].Vulnerabilities = append(report.Dependencies[i].Vulnerabilities, v)
					report.Vulnerable++
				}
			}
		}
	}

	if report.Vulnerable > 0 {
		report.Status = "CRITICAL"
	} else {
		report.Status = "HEALTHY"
	}

	return report, nil
}

func (c *Checker) parseGoMod(path string) ([]DependencyInfo, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var deps []DependencyInfo
	var inRequire bool
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if line == "require (" {
			inRequire = true
			continue
		}
		if inRequire && line == ")" {
			break
		}
		if strings.HasPrefix(line, "// indirect") {
			continue
		}

		if inRequire {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				isIndirect := strings.Contains(line, "indirect")
				deps = append(deps, DependencyInfo{
					Name:     parts[0],
					Version:  parts[1],
					Direct:   !isIndirect,
				})
			}
		}
	}

	return deps, nil
}

func (c *Checker) checkVulnsFast() ([]Vulnerability, error) {
	cmd := exec.Command("go", "run", "golang.org/x/vuln/cmd/govulncheck@latest", "-json", "./...")
	cmd.Env = append(os.Environ(), "GOVULNCHECK_DB=https://vuln.go.dev")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var vulns []Vulnerability
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, "\"id\":") && strings.Contains(line, "GO-") {
			parts := strings.SplitN(line, "\"", 4)
			if len(parts) >= 4 {
				cve := parts[3]
				vulns = append(vulns, Vulnerability{
					CVE: cve,
					URL: fmt.Sprintf("https://pkg.go.dev/vuln/%s", cve),
				})
			}
		}
		if strings.Contains(line, "\"path\":") {
			parts := strings.SplitN(line, "\"", 4)
			if len(parts) >= 4 && len(vulns) > 0 {
				vulns[len(vulns)-1].Package = strings.Trim(parts[3], "\",")
			}
		}
	}

	return vulns, nil
}

func (r *HealthReport) Format() string {
	var sb strings.Builder
	sb.WriteString("=== Dependency Health Report ===\n\n")
	sb.WriteString(fmt.Sprintf("Status: %s | Total: %d (Direct: %d, Indirect: %d)\n",
		r.Status, r.Total, r.Direct, r.Indirect))

	if len(r.Vulnerabilities) > 0 {
		sb.WriteString(fmt.Sprintf("\nVULNERABILITIES: %d\n", len(r.Vulnerabilities)))
		for _, v := range r.Vulnerabilities {
			sb.WriteString(fmt.Sprintf("  [%s] %s\n", v.CVE, v.URL))
		}
	}

	return sb.String()
}
