package main

import (
	"log"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "ophidian",
	Short: "OPHIDIAN - Offensive AI Security Platform",
	Long:  `OPHIDIAN is an offensive AI security platform with three-plane architecture.`,
}

var missionCmd = &cobra.Command{
	Use:   "mission",
	Short: "Manage missions",
}

var createMissionCmd = &cobra.Command{
	Use:   "create [name]",
	Short: "Create a new mission",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		// TODO: implement
	},
}

var listMissionCmd = &cobra.Command{
	Use:   "list",
	Short: "List all missions",
	Run: func(cmd *cobra.Command, args []string) {
		// TODO: implement
	},
}

func main() {
	rootCmd.AddCommand(missionCmd)
	missionCmd.AddCommand(createMissionCmd)
	missionCmd.AddCommand(listMissionCmd)

	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
