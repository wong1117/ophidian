package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var (
	version   = "0.1.0"
	commit    = "dev"
	buildTime = "unknown"
)

var rootCmd = &cobra.Command{
	Use:   "ophidian",
	Short: "OPHIDIAN - Offensive AI Security Platform",
	Long: `OPHIDIAN is an offensive AI security platform with a three-plane architecture:
  - Control Plane: Mission lifecycle, task scheduling, RoE enforcement
  - AI Plane: Reasoning, planning, strategy adaptation
  - Execution Plane: Recon, exploit, post-exploit, reporting`,
	Version: fmt.Sprintf("%s (commit: %s, built: %s)", version, commit, buildTime),
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Ophidian %s\n", version)
		fmt.Printf("  Commit:    %s\n", commit)
		fmt.Printf("  Built:     %s\n", buildTime)
		fmt.Printf("  Go:        go1.22+\n")
	},
}

var scaffoldCmd = &cobra.Command{
	Use:   "scaffold [name]",
	Short: "Scaffold a new Ophidian service or plugin",
	Long: `Generate a new service or plugin from templates.

Available templates:
  service    - Create a new application service with domain + infrastructure layers
  plugin     - Create a new plugin with Plugin interface implementation
  repository - Create a new PostgreSQL repository`,
	Args: cobra.ExactArgs(1),
	Run:  runScaffold,
}

var templateFlag string

func init() {
	scaffoldCmd.Flags().StringVarP(&templateFlag, "template", "t", "service", "Template type: service, plugin, repository")
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(scaffoldCmd)
	rootCmd.AddCommand(docsCmd())
	rootCmd.AddCommand(devCmd())
}

func docsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "docs",
		Short: "Developer documentation commands",
	}
}

func devCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dev",
		Short: "Local development commands",
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "setup",
		Short: "Set up local development environment",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("Setting up Ophidian development environment...")
			fmt.Println("  1. Installing Go dependencies...")
			runCmd("go", "mod", "download")
			fmt.Println("  2. Installing golangci-lint...")
			runCmd("go", "install", "github.com/golangci/golangci-lint/cmd/golangci-lint@latest")
			fmt.Println("  3. Generating API docs...")
			runCmd("go", "install", "github.com/swaggo/swag/cmd/swag@latest")
			fmt.Println("  4. Creating local config...")
			createLocalConfig()
			fmt.Println("  Done! Run 'make run-server' to start.")
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "gen-docs",
		Short: "Generate API documentation",
		Run: func(cmd *cobra.Command, args []string) {
			genAPIDocs()
		},
	})
	return cmd
}

func runScaffold(cmd *cobra.Command, args []string) {
	name := args[0]
	base := filepath.Join(".", "internal", "application", name)

	switch templateFlag {
	case "service":
		os.MkdirAll(base, 0755)
		writeFile(filepath.Join(base, name+"_service.go"), serviceTemplate(name))
		writeFile(filepath.Join(base, name+"_service_test.go"), serviceTestTemplate(name))
		fmt.Printf("Service '%s' created at %s\n", name, base)
	case "plugin":
		pluginDir := filepath.Join("pkg", "plugins", name)
		os.MkdirAll(pluginDir, 0755)
		writeFile(filepath.Join(pluginDir, name+".go"), pluginTemplate(name))
		fmt.Printf("Plugin '%s' created at %s\n", name, pluginDir)
	case "repository":
		repoDir := filepath.Join(".", "internal", "infrastructure", "persistence", "postgres")
		writeFile(filepath.Join(repoDir, name+"_repo.go"), repoTemplate(name))
		fmt.Printf("Repository '%s' created at %s\n", name, repoDir)
	default:
		fmt.Fprintf(os.Stderr, "Unknown template: %s\n", templateFlag)
		os.Exit(1)
	}
}

func serviceTemplate(name string) string {
	return fmt.Sprintf(`package %s

import (
	"context"
	"fmt"

	"github.com/ophidian/ophidian/internal/domain/common"
)

type %sService struct {
	// Add dependencies here
}

func New%sService() *%sService {
	return &%sService{}
}

func (s *%sService) Execute(ctx context.Context, input string) (string, error) {
	if input == "" {
		return "", fmt.Errorf("%%w: input is required", common.ErrInvalidID)
	}
	return fmt.Sprintf("processed: %%s", input), nil
}
`, name, title(name), title(name), title(name), title(name))
}

func serviceTestTemplate(name string) string {
	return fmt.Sprintf(`package %s

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test%sService_Execute(t *testing.T) {
	svc := New%sService()
	result, err := svc.Execute(context.Background(), "test-input")
	assert.NoError(t, err)
	assert.Equal(t, "processed: test-input", result)
}

func Test%sService_Execute_EmptyInput(t *testing.T) {
	svc := New%sService()
	_, err := svc.Execute(context.Background(), "")
	assert.Error(t, err)
}
`, name, title(name), title(name), title(name))
}

func pluginTemplate(name string) string {
	return fmt.Sprintf(`package %s

import "context"

type Plugin struct {
	name    string
	version string
}

func NewPlugin() *Plugin {
	return &Plugin{name: "%s", version: "0.1.0"}
}

func (p *Plugin) Name() string    { return p.name }
func (p *Plugin) Version() string { return p.version }
func (p *Plugin) Type() string    { return "%s" }

func (p *Plugin) Initialize(ctx context.Context, config json.RawMessage) error { return nil }
func (p *Plugin) Start(ctx context.Context) error { return nil }
func (p *Plugin) Stop(ctx context.Context) error  { return nil }
func (p *Plugin) Execute(params map[string]interface{}) (map[string]interface{}, error) {
	return map[string]interface{}{"status": "ok"}, nil
}
`, name, name, name)
}

func repoTemplate(name string) string {
	return fmt.Sprintf(`package postgres

import (
	"context"
	"fmt"

	"github.com/ophidian/ophidian/internal/domain/%s"
	"github.com/jackc/pgx/v5/pgxpool"
)

type %sRepo struct {
	deps RepoDeps
}

func New%sRepo(pool *pgxpool.Pool) *%sRepo {
	return &%sRepo{deps: RepoDepsFromPool(pool)}
}

func (r *%sRepo) Save(ctx context.Context, entity *%s.Entity) error {
	_, err := r.deps.Exec(ctx,
		"INSERT INTO %ss (id, data, created_at) VALUES ($1, $2, $3)",
		entity.ID, marshalJSON(entity.Data), entity.CreatedAt,
	)
	return fmt.Errorf("save %s: %%w", err)
}

func (r *%sRepo) FindByID(ctx context.Context, id string) (*%s.Entity, error) {
	return nil, fmt.Errorf("not implemented")
}
`, name, title(name), title(name), title(name), title(name), title(name), name, name, name, title(name), name)
}

func genAPIDocs() {
	fmt.Println("Generating OpenAPI/Swagger documentation...")
	runCmd("swag", "init", "-g", "cmd/ophidian-server/main.go", "-o", "docs/api", "--parseDependency", "--parseInternal")
	fmt.Println("API documentation generated at docs/api/")
}

func createLocalConfig() {
	configDir := filepath.Join(".", "configs")
	os.MkdirAll(configDir, 0755)
	config := `server:
  host: "0.0.0.0"
  port: 8080
  read_timeout: 30
  write_timeout: 30

database:
  host: "localhost"
  port: 5432
  user: "ophidian"
  password: "ophidian-dev"
  database: "ophidian"
  ssl_mode: "disable"
  max_conns: 10

redis:
  host: "localhost"
  port: 6379
  db: 0

logging:
  level: "debug"
  format: "text"
  output: "stdout"

auth:
  jwt_secret: "dev-secret-do-not-use-in-production"
  token_expiry: 86400
`
	writeFile(filepath.Join(configDir, "config.local.yaml"), config)
	fmt.Println("  Created configs/config.local.yaml")
}

func writeFile(path, content string) {
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing %s: %v\n", path, err)
		os.Exit(1)
	}
	fmt.Printf("  Created %s\n", path)
}

func runCmd(name string, args ...string) {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "  Warning: %s failed: %v\n", name, err)
	}
}

func title(s string) string {
	if len(s) == 0 {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
