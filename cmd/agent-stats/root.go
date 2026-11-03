package main

import (
	"github.com/spf13/cobra"
)

var version = "0.1.0"

func newRootCommand() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:           "agent-stats",
		Short:         "Report how AI coding agents are used on this machine",
		Long:          "agent-stats reads the session history that Claude Code, Codex, Pi, and OpenCode keep on this machine and reports usage statistics, such as which skills are used most.",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	rootCmd.SetVersionTemplate("{{.Version}}\n")
	rootCmd.AddCommand(newUsageCommand())
	setupHelp(rootCmd)
	return rootCmd
}

func newUsageCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "usage",
		Short: "Report what agents used across stored sessions",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(newUsageSkillsCommand())
	return cmd
}
