package main

import (
	"fmt"

	cobrahelptree "github.com/alexgorbatchev/cobra-help-tree/v2"
	"github.com/spf13/cobra"
	"github.com/alexgorbatchev/agent-stats-cli/internal/agent"
)

var techCatalog = cobrahelptree.TechCatalog{
	"agent-stats": {
		Summary:     "Single-run CLI that reports usage statistics, such as most-used skills, from local AI coding agent transcripts (Claude Code, Codex, Pi, OpenCode)",
		Description: "Single-run CLI that reports usage statistics, such as most-used skills, from local AI coding agent transcripts (Claude Code, Codex, Pi, OpenCode) - command-line interface.",
	},
	"agent-stats status": {
		Summary:     "Display system and environment status",
		Description: "Outputs current runtime readiness and version information.",
	},
}

func setupHelp(cmd *cobra.Command) {
	_ = cobrahelptree.SetupWithOptions(cmd, cobrahelptree.HelpOptions{
		Catalog: techCatalog,
	})
	help := cmd.HelpFunc()
	cmd.SetHelpFunc(func(c *cobra.Command, args []string) {
		if agent.IsAgentMode() {
			if _, err := fmt.Fprintln(c.OutOrStdout(), "ALERT: Agents must read `AGENT=1 agent-stats skill` before using this tool."); err != nil {
				c.PrintErrln(err)
				return
			}
		}
		help(c, args)
	})
}
