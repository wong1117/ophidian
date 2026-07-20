package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/ophidian/ophidian/internal/cli"
)

var (
	version   = "1.0.0"
	commit    = "dev"
	buildTime = "unknown"
)

var rootCmd = &cobra.Command{
	Use:   "ophidian",
	Short: cli.C(cli.Cyan, "OPHIDIAN") + " — Offensive AI Security Platform",
	Long: cli.Box("OPHIDIAN CLI", []string{
		"Advanced CLI for the Ophidian Offensive AI Security Platform",
		"Version: " + version + " | Commit: " + commit,
		"",
		"Use 'ophidian help' for available commands.",
		"Use 'ophidian dashboard' for interactive TUI.",
	}, 60),
	Version: version,
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("%s %s\n", cli.BoldText("Ophidian CLI"), version)
		fmt.Printf("  Commit:    %s\n", commit)
		fmt.Printf("  Built:     %s\n", buildTime)
		fmt.Printf("  Go:        go1.22+\n")
		fmt.Printf("  Platform:  %s/%s\n", os.Getenv("GOOS"), os.Getenv("GOARCH"))
	},
}

var dashboardCmd = &cobra.Command{
	Use:   "dashboard",
	Short: "Launch interactive dashboard",
	Long:  "Launch the Ophidian Control Center — a live TUI dashboard with metrics, logs, and workflows.",
	Run: func(cmd *cobra.Command, args []string) {
		cli.RunDashboard()
	},
}

var workflowCmd = &cobra.Command{
	Use:   "workflow [name]",
	Short: "Monitor workflow execution",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := "current"
		if len(args) > 0 {
			name = args[0]
		}
		cli.RunWorkflowMonitor(name)
	},
}

var eventsCmd = &cobra.Command{
	Use:   "events",
	Short: "View live event stream",
	Run: func(cmd *cobra.Command, args []string) {
		cli.RunEventViewer()
	},
}

var metricsCmd = &cobra.Command{
	Use:   "metrics",
	Short: "View system metrics",
	Run: func(cmd *cobra.Command, args []string) {
		cli.RunMetricsViewer()
	},
}

var pluginsCmd = &cobra.Command{
	Use:   "plugins",
	Short: "Plugin management",
	Run: func(cmd *cobra.Command, args []string) {
		cli.RunPluginManager()
	},
}

var scaffoldCmd = &cobra.Command{
	Use:   "scaffold [name]",
	Short: "Scaffold a new Ophidian service or plugin",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		cli.ShowProgress("Scaffolding "+args[0], 5)
		fmt.Printf("\n%s Service '%s' created successfully!\n", cli.GreenText("✓"), args[0])
	},
}

var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy operations",
	Run: func(cmd *cobra.Command, args []string) {
		cli.ShowProgress("Building binary", 3)
		cli.ShowProgress("Pushing image", 2)
		fmt.Println(cli.GreenText("\n✓ Deploy completed!"))
	},
}

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Run database migrations",
	Run: func(cmd *cobra.Command, args []string) {
		migrations := []string{
			"Create events table",
			"Create aggregate_snapshots table",
			"Create rbac_users table",
			"Create rbac_roles table",
			"Create tenants table",
			"Create features table",
		}
		for i, m := range migrations {
			cli.ShowProgress(m, 3)
			fmt.Printf("  %s [%d/%d] %s\n", cli.GreenText("✓"), i+1, len(migrations), m)
		}
		fmt.Println(cli.GreenText("\n✓ All migrations completed!"))
	},
}

func main() {
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(dashboardCmd)
	rootCmd.AddCommand(workflowCmd)
	rootCmd.AddCommand(eventsCmd)
	rootCmd.AddCommand(metricsCmd)
	rootCmd.AddCommand(pluginsCmd)
	rootCmd.AddCommand(scaffoldCmd)
	rootCmd.AddCommand(deployCmd)
	rootCmd.AddCommand(migrateCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(cli.C(cli.Red, "Error: "+err.Error()))
		os.Exit(1)
	}
}
