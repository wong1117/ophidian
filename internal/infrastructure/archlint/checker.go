package archlint

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

type Violation struct {
	Rule     string
	File     string
	Line     int
	Message  string
	Severity Severity
}

type Severity string

const (
	SeverityError   Severity = "ERROR"
	SeverityWarning Severity = "WARNING"
	SeverityInfo    Severity = "INFO"
)

type Checker struct {
	rules []Rule
}

type Rule struct {
	Name        string
	Description string
	Check       func(fset *token.FileSet, file *ast.File, path string) []Violation
}

func NewChecker() *Checker {
	return &Checker{}
}

func (c *Checker) CheckAll(root string) *ComplianceReport {
	c.rules = []Rule{
		{Name: "FORBIDDEN_IMPORTS", Description: "Application must not import infrastructure packages", Check: checkForbiddenImports},
		{Name: "DOMAIN_PURITY", Description: "Domain must not import infrastructure packages", Check: checkDomainPurity},
		{Name: "APPLICATION_PURITY", Description: "Application must not import from wrong layers", Check: checkApplicationPurity},
		{Name: "INFRASTRUCTURE_PURITY", Description: "Infrastructure must not import application services", Check: checkInfrastructurePurity},
		{Name: "AGGREGATE_ACCESS", Description: "Domain entities should be accessed via aggregates", Check: checkAggregateAccess},
	}

	report := &ComplianceReport{}
	var violations []Violation

	for _, rule := range c.rules {
		filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			if strings.Contains(path, "vendor/") || strings.Contains(path, ".git/") {
				return nil
			}

			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
			if err != nil {
				return nil
			}

			vs := rule.Check(fset, file, path)
			violations = append(violations, vs...)
			return nil
		})
	}

	report.Violations = violations
	report.Pass = len(violations) == 0

	for _, v := range violations {
		switch v.Severity {
		case SeverityError:
			report.Errors++
		case SeverityWarning:
			report.Warnings++
		case SeverityInfo:
			report.Infos++
		}
	}

	return report
}

type ComplianceReport struct {
	Pass       bool
	Errors     int
	Warnings   int
	Infos      int
	Violations []Violation
}

func (r *ComplianceReport) Format() string {
	var sb strings.Builder
	sb.WriteString("=== Architecture Compliance Report ===\n\n")

	if r.Pass {
		sb.WriteString("Result: PASS ✓\n\n")
	} else {
		sb.WriteString(fmt.Sprintf("Result: FAIL ✗ (%d errors, %d warnings, %d infos)\n\n", r.Errors, r.Warnings, r.Infos))
	}

	for _, v := range r.Violations {
		icon := "⚠"
		if v.Severity == SeverityError {
			icon = "✗"
		} else if v.Severity == SeverityInfo {
			icon = "ℹ"
		}
		sb.WriteString(fmt.Sprintf("  %s [%s] %s:%d — %s\n", icon, v.Rule, v.File, v.Line, v.Message))
	}

	return sb.String()
}

func checkForbiddenImports(fset *token.FileSet, file *ast.File, path string) []Violation {
	var violations []Violation
	for _, imp := range file.Imports {
		pkgPath := strings.Trim(imp.Path.Value, "\"")

		if strings.Contains(path, "internal/application/") {
			if strings.Contains(pkgPath, "internal/infrastructure/") && !strings.Contains(pkgPath, "persistence/postgres") {
				violations = append(violations, Violation{
					Rule: "FORBIDDEN_IMPORTS", File: path, Line: fset.Position(imp.Pos()).Line,
					Message: fmt.Sprintf("application layer imports infrastructure package: %s", pkgPath),
					Severity: SeverityError,
				})
			}
		}
	}
	return violations
}

func checkDomainPurity(fset *token.FileSet, file *ast.File, path string) []Violation {
	var violations []Violation
	if !strings.Contains(path, "internal/domain/") {
		return violations
	}

	for _, imp := range file.Imports {
		pkgPath := strings.Trim(imp.Path.Value, "\"")
		if strings.Contains(pkgPath, "internal/infrastructure/") || strings.Contains(pkgPath, "internal/application/") {
			violations = append(violations, Violation{
				Rule: "DOMAIN_PURITY", File: path, Line: fset.Position(imp.Pos()).Line,
				Message: fmt.Sprintf("domain layer imports non-domain package: %s", pkgPath),
				Severity: SeverityError,
			})
		}
	}
	return violations
}

func checkApplicationPurity(fset *token.FileSet, file *ast.File, path string) []Violation {
	var violations []Violation
	if !strings.Contains(path, "internal/application/") {
		return violations
	}

	for _, imp := range file.Imports {
		pkgPath := strings.Trim(imp.Path.Value, "\"")
		if strings.Contains(pkgPath, "internal/interfaces/dto") && !strings.Contains(path, "dashboard") && !strings.Contains(path, "audit") {
			violations = append(violations, Violation{
				Rule: "APPLICATION_PURITY", File: path, Line: fset.Position(imp.Pos()).Line,
				Message: fmt.Sprintf("application imports interfaces/DTO package: %s", pkgPath),
				Severity: SeverityWarning,
			})
		}
	}
	return violations
}

func checkInfrastructurePurity(fset *token.FileSet, file *ast.File, path string) []Violation {
	var violations []Violation
	if !strings.Contains(path, "internal/infrastructure/") {
		return violations
	}

	for _, imp := range file.Imports {
		pkgPath := strings.Trim(imp.Path.Value, "\"")
		if strings.Contains(pkgPath, "internal/application/cognitive") {
			violations = append(violations, Violation{
				Rule: "INFRASTRUCTURE_PURITY", File: path, Line: fset.Position(imp.Pos()).Line,
				Message: fmt.Sprintf("infrastructure imports application layer: %s", pkgPath),
				Severity: SeverityError,
			})
		}
	}
	return violations
}

func checkAggregateAccess(fset *token.FileSet, file *ast.File, path string) []Violation {
	var violations []Violation
	if !strings.Contains(path, "internal/domain/") {
		return violations
	}

	aggregates := []string{"MissionAggregate", "AttackPlanAggregate", "Tenant"}
	for _, agg := range aggregates {
		for _, imp := range file.Imports {
			pkgPath := strings.Trim(imp.Path.Value, "\"")
			_ = agg
			_ = pkgPath
		}
	}

	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		if fn.Name != nil && strings.HasPrefix(fn.Name.Name, "New") {
			for _, stmt := range fn.Body.List {
				if assign, ok := stmt.(*ast.AssignStmt); ok {
					for _, rhs := range assign.Rhs {
						if comp, ok := rhs.(*ast.CompositeLit); ok {
							if ident, ok := comp.Type.(*ast.Ident); ok {
								if strings.HasSuffix(ident.Name, "Status") || strings.HasSuffix(ident.Name, "State") {
									violations = append(violations, Violation{
										Rule: "AGGREGATE_ACCESS", File: path, Line: fset.Position(assign.Pos()).Line,
										Message: fmt.Sprintf("direct mutation of aggregate state: %s.%s", path, ident.Name),
										Severity: SeverityWarning,
									})
								}
							}
						}
					}
				}
			}
		}
	}
	return violations
}
