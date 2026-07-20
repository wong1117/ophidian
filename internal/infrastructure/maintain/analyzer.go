package maintain

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type MaintainabilityConfig struct {
	MaxFileCountPerPkg  int     `json:"max_file_count_per_pkg"`
	MaxLinesPerFile     int     `json:"max_lines_per_file"`
	MaxInterfaceMethods int     `json:"max_interface_methods"`
	MaxFunctionLines    int     `json:"max_function_lines"`
	MinDuplicationLines int     `json:"min_duplication_lines"`
	MinGoDocCoverage    float64 `json:"min_godoc_coverage"`
	ExcludePaths        []string `json:"exclude_paths"`
}

func DefaultConfig() MaintainabilityConfig {
	return MaintainabilityConfig{
		MaxFileCountPerPkg:  15,
		MaxLinesPerFile:     500,
		MaxInterfaceMethods: 7,
		MaxFunctionLines:    80,
		MinDuplicationLines: 6,
		MinGoDocCoverage:    80.0,
		ExcludePaths:        []string{"vendor", ".git", "test"},
	}
}

type PackageStats struct {
	Path       string `json:"path"`
	FileCount  int    `json:"file_count"`
	TotalLines int    `json:"total_lines"`
	Status     string `json:"status"`
}

type Duplication struct {
	Lines []string `json:"lines"`
	Files []string `json:"files"`
	Count int      `json:"count"`
}

type InterfaceViolation struct {
	Package    string `json:"package"`
	Name       string `json:"name"`
	MethodCount int   `json:"method_count"`
}

type FunctionViolation struct {
	File     string `json:"file"`
	Function string `json:"function"`
	Lines    int    `json:"lines"`
}

type GoDocCoverage struct {
	TotalExported      int     `json:"total_exported"`
	TotalDocumented    int     `json:"total_documented"`
	CoveragePct        float64 `json:"coverage_pct"`
	UndocumentedItems  []string `json:"undocumented_items"`
}

type MaintainabilityReport struct {
	GeneratedAt         time.Time            `json:"generated_at"`
	Config              MaintainabilityConfig `json:"config"`
	TotalFiles          int                  `json:"total_files"`
	TotalLines          int                  `json:"total_lines"`
	TotalPackages       int                  `json:"total_packages"`
	OversizedPackages   []PackageStats       `json:"oversized_packages"`
	Duplications        []Duplication        `json:"duplications"`
	OversizedInterfaces []InterfaceViolation `json:"oversized_interfaces"`
	LongFunctions       []FunctionViolation  `json:"long_functions"`
	GoDoc               GoDocCoverage        `json:"godoc"`
	Score               float64              `json:"maintainability_score"`
	Grade               string               `json:"grade"`
}

type Analyzer struct {
	config MaintainabilityConfig
	root   string
}

func NewAnalyzer(root string) *Analyzer {
	return &Analyzer{
		config: DefaultConfig(),
		root:   root,
	}
}

func (a *Analyzer) Run() *MaintainabilityReport {
	report := &MaintainabilityReport{
		GeneratedAt: time.Now(),
		Config:      a.config,
	}

	var allFiles []string
	filepath.Walk(a.root, func(path string, info os.FileInfo, err error) error {
		if err != nil || !strings.HasSuffix(path, ".go") {
			return nil
		}
		for _, excl := range a.config.ExcludePaths {
			if strings.Contains(path, excl) {
				return nil
			}
		}
		allFiles = append(allFiles, path)
		return nil
	})

	report.TotalFiles = len(allFiles)

	pkgFiles := make(map[string][]string)
	for _, f := range allFiles {
		pkg := filepath.Dir(f)
		pkgFiles[pkg] = append(pkgFiles[pkg], f)
	}
	report.TotalPackages = len(pkgFiles)

	var totalLines int
	for _, files := range pkgFiles {
		lines := 0
		for _, f := range files {
			data, _ := os.ReadFile(f)
			lines += strings.Count(string(data), "\n")
		}
		totalLines += lines

		pkgPath := filepath.Base(files[0])
		if len(files) > a.config.MaxFileCountPerPkg {
			report.OversizedPackages = append(report.OversizedPackages, PackageStats{
				Path: pkgPath, FileCount: len(files), TotalLines: lines, Status: "OVERSIZED",
			})
		}
	}
	report.TotalLines = totalLines

	report.Duplications = a.detectDuplications(pkgFiles)
	report.OversizedInterfaces = a.detectOversizedInterfaces(allFiles)
	report.LongFunctions = a.detectLongFunctions(allFiles)
	report.GoDoc = a.calculateGoDocCoverage(allFiles)

	report.Score = a.calculateScore(report)
	report.Grade = gradeScore(report.Score)

	return report
}

func (a *Analyzer) detectDuplications(pkgFiles map[string][]string) []Duplication {
	var dups []Duplication
	seen := make(map[string][]string)

	for pkg, files := range pkgFiles {
		for _, file := range files {
			data, err := os.ReadFile(file)
			if err != nil { continue }
			lines := strings.Split(string(data), "\n")
			for i := 0; i < len(lines)-a.config.MinDuplicationLines; i++ {
				chunk := strings.Join(lines[i:i+a.config.MinDuplicationLines], "\n")
				key := strings.TrimSpace(chunk)
				if len(key) < 30 { continue }
				seen[key] = append(seen[key], fmt.Sprintf("%s:%d", file, i+1))
			}
			_ = pkg
		}
	}

	for chunk, locations := range seen {
		if len(locations) >= 2 {
			dups = append(dups, Duplication{
				Lines: strings.Split(chunk, "\n"),
				Files: locations,
				Count: len(locations),
			})
		}
	}

	sort.Slice(dups, func(i, j int) bool { return dups[i].Count > dups[j].Count })
	if len(dups) > 10 {
		dups = dups[:10]
	}

	return dups
}

func (a *Analyzer) detectOversizedInterfaces(files []string) []InterfaceViolation {
	var violations []InterfaceViolation

	for _, file := range files {
		fset := token.NewFileSet()
		node, err := parser.ParseFile(fset, file, nil, parser.ParseComments)
		if err != nil { continue }

		pkgName := node.Name.Name
		for _, decl := range node.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.TYPE { continue }
			for _, spec := range gen.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok { continue }
				iface, ok := ts.Type.(*ast.InterfaceType)
				if !ok { continue }

				methods := iface.Methods.NumFields()
				if methods > a.config.MaxInterfaceMethods {
					violations = append(violations, InterfaceViolation{
						Package: pkgName, Name: ts.Name.Name, MethodCount: methods,
					})
				}
			}
		}
	}

	return violations
}

func (a *Analyzer) detectLongFunctions(files []string) []FunctionViolation {
	var violations []FunctionViolation

	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil { continue }
		lines := strings.Split(string(data), "\n")

		fset := token.NewFileSet()
		node, err := parser.ParseFile(fset, file, nil, 0)
		if err != nil { continue }

		for _, decl := range node.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok { continue }

			start := fset.Position(fn.Pos()).Line
			end := fset.Position(fn.End()).Line
			lineCount := end - start + 1

			if lineCount > a.config.MaxFunctionLines {
				name := fn.Name.Name
				if fn.Recv != nil && len(fn.Recv.List) > 0 {
					if ident, ok := fn.Recv.List[0].Type.(*ast.Ident); ok {
						name = fmt.Sprintf("%s.%s", ident.Name, fn.Name.Name)
					} else if se, ok := fn.Recv.List[0].Type.(*ast.StarExpr); ok {
						if ident, ok := se.X.(*ast.Ident); ok {
							name = fmt.Sprintf("*%s.%s", ident.Name, fn.Name.Name)
						}
					}
				}
				violations = append(violations, FunctionViolation{
					File: file, Function: name, Lines: lineCount,
				})
			}
			_ = lines
		}
	}

	return violations
}

func (a *Analyzer) calculateGoDocCoverage(files []string) GoDocCoverage {
	cov := GoDocCoverage{}

	for _, file := range files {
		fset := token.NewFileSet()
		node, err := parser.ParseFile(fset, file, nil, parser.ParseComments)
		if err != nil { continue }

		for _, decl := range node.Decls {
			switch d := decl.(type) {
			case *ast.GenDecl:
				for _, spec := range d.Specs {
					switch spec.(type) {
					case *ast.TypeSpec, *ast.ValueSpec:
						cov.TotalExported++
						if d.Doc != nil && d.Doc.Text() != "" {
							cov.TotalDocumented++
						} else {
							cov.UndocumentedItems = append(cov.UndocumentedItems,
								fmt.Sprintf("%s:%d", file, fset.Position(d.Pos()).Line))
						}
					}
				}
			case *ast.FuncDecl:
				if d.Name.IsExported() {
					cov.TotalExported++
					if d.Doc != nil && d.Doc.Text() != "" {
						cov.TotalDocumented++
					} else {
						cov.UndocumentedItems = append(cov.UndocumentedItems,
							fmt.Sprintf("%s:%d:%s", file, fset.Position(d.Pos()).Line, d.Name.Name))
					}
				}
			}
		}
	}

	if cov.TotalExported > 0 {
		cov.CoveragePct = float64(cov.TotalDocumented) / float64(cov.TotalExported) * 100
	}
	return cov
}

func (a *Analyzer) calculateScore(report *MaintainabilityReport) float64 {
	score := 100.0

	if len(report.OversizedPackages) > 0 {
		score -= float64(len(report.OversizedPackages)) * 5
	}
	if len(report.Duplications) > 0 {
		score -= float64(len(report.Duplications)) * 3
	}
	if len(report.OversizedInterfaces) > 0 {
		score -= float64(len(report.OversizedInterfaces)) * 2
	}
	if len(report.LongFunctions) > 0 {
		score -= float64(len(report.LongFunctions)) * 1
	}

	goDocPenalty := (a.config.MinGoDocCoverage - report.GoDoc.CoveragePct) * 0.5
	if goDocPenalty > 0 {
		score -= goDocPenalty
	}

	if score < 0 { score = 0 }
	if score > 100 { score = 100 }
	return float64(int(score*10)) / 10
}

func gradeScore(score float64) string {
	switch {
	case score >= 90: return "A"
	case score >= 80: return "B"
	case score >= 70: return "C"
	case score >= 60: return "D"
	default: return "F"
	}
}

func (r *MaintainabilityReport) Format() string {
	var sb strings.Builder
	sb.WriteString("=== Maintainability Report ===\n\n")
	sb.WriteString(fmt.Sprintf("Files: %d | Lines: %d | Packages: %d\n", r.TotalFiles, r.TotalLines, r.TotalPackages))
	sb.WriteString(fmt.Sprintf("Score: %.1f/100 (%s)\n\n", r.Score, r.Grade))

	if len(r.OversizedPackages) > 0 {
		sb.WriteString(fmt.Sprintf("Oversized Packages: %d\n", len(r.OversizedPackages)))
		for _, p := range r.OversizedPackages {
			sb.WriteString(fmt.Sprintf("  %s: %d files, %d lines\n", p.Path, p.FileCount, p.TotalLines))
		}
		sb.WriteString("\n")
	}

	if len(r.Duplications) > 0 {
		sb.WriteString(fmt.Sprintf("Duplications: %d\n", len(r.Duplications)))
		for _, d := range r.Duplications {
			sb.WriteString(fmt.Sprintf("  %d occurrences across %v\n", d.Count, d.Files))
		}
		sb.WriteString("\n")
	}

	if len(r.OversizedInterfaces) > 0 {
		sb.WriteString(fmt.Sprintf("Oversized Interfaces: %d\n", len(r.OversizedInterfaces)))
		for _, i := range r.OversizedInterfaces {
			sb.WriteString(fmt.Sprintf("  %s.%s: %d methods\n", i.Package, i.Name, i.MethodCount))
		}
		sb.WriteString("\n")
	}

	if len(r.LongFunctions) > 0 {
		sb.WriteString(fmt.Sprintf("Long Functions: %d\n", len(r.LongFunctions)))
		count := len(r.LongFunctions)
		if count > 5 { count = 5 }
		for _, f := range r.LongFunctions[:count] {
			sb.WriteString(fmt.Sprintf("  %s: %s (%d lines)\n", f.File, f.Function, f.Lines))
		}
		if len(r.LongFunctions) > 5 {
			sb.WriteString(fmt.Sprintf("  ... and %d more\n", len(r.LongFunctions)-5))
		}
		sb.WriteString("\n")
	}

	sb.WriteString(fmt.Sprintf("GoDoc Coverage: %.1f%% (%d/%d documented)\n",
		r.GoDoc.CoveragePct, r.GoDoc.TotalDocumented, r.GoDoc.TotalExported))
	if r.GoDoc.CoveragePct < r.Config.MinGoDocCoverage {
		sb.WriteString(fmt.Sprintf("  Below target (%.0f%%)\n", r.Config.MinGoDocCoverage))
	}

	return sb.String()
}

func (r *MaintainabilityReport) ToJSON() []byte {
	data, _ := json.MarshalIndent(r, "", "  ")
	return data
}
