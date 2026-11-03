package main

import (
	"fmt"
	"os"

	cobrahelptree "github.com/alexgorbatchev/cobra-help-tree/v4"
	"github.com/spf13/cobra"
)

const (
	skillRepository = "alexgorbatchev/agent-stats-cli"
	skillBranch     = "main"
)

var techCatalog = cobrahelptree.TechCatalog{
	"agent-stats": {
		Env: []cobrahelptree.EnvSpec{
			{Name: "AGENT", Description: "1, true, or yes switches to compact agent output"},
			{Name: "CLAUDE_CONFIG_DIR", Description: "Claude Code configuration directory, searched before ~/.claude"},
			{Name: "CODEX_HOME", Description: "Codex home directory; sessions and archived_sessions live under it"},
			{Name: "CODEX_SESSIONS_DIR", Description: "Codex sessions directory, overriding CODEX_HOME/sessions"},
			{Name: "PI_SESSIONS_DIR", Description: "Pi sessions directory"},
			{Name: "PI_CODING_AGENT_DIR", Description: "Pi agent directory; sessions live under it when PI_SESSIONS_DIR is unset"},
			{Name: "OPENCODE_DB_PATH", Description: "OpenCode database file, overriding every other OpenCode location"},
			{Name: "OPENCODE_DB", Description: "OpenCode's own database setting: absolute, or relative to the OpenCode data directory"},
			{Name: "XDG_DATA_HOME", Description: "Data directory holding opencode/, defaulting to ~/.local/share"},
		},
	},
}

// helpWriteError keeps a failed help write for run, because cobra's help function
// cannot return an error.
var helpWriteError error

func setupHelp(cmd *cobra.Command) {
	err := cobrahelptree.SetupWithOptions(cmd, cobrahelptree.HelpOptions{
		Catalog: techCatalog,
		Skill:   cobrahelptree.SkillSource{Repository: skillRepository, Branch: skillBranch},
		Tree:    cobrahelptree.TreeOptions{HideGeneratedCommands: true},
		OnHelpWriteError: func(_ *cobra.Command, err error) {
			helpWriteError = err
		},
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
