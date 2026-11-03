package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/alexgorbatchev/agent-stats-cli/internal/displaypath"
	cobrahelptree "github.com/alexgorbatchev/cobra-help-tree/v4"
	"github.com/spf13/cobra"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	code := run(ctx, newRootCommand())
	stop()
	os.Exit(code)
}

// run executes the command tree and returns the process exit code: 0 on success and 1
// when the command fails or a help screen cannot be written.
func run(ctx context.Context, cmd *cobra.Command) int {
	err := cmd.ExecuteContext(ctx)
	if err == nil {
		err = helpWriteError
	}
	if err == nil {
		return 0
	}
	prefix := "[ERROR] "
	if cobrahelptree.IsAgentMode() {
		prefix = "ERR: "
	}
	// The exit code already reports the failure; a stderr that cannot be written leaves
	// nowhere else to report it.
	_, _ = fmt.Fprintln(cmd.ErrOrStderr(), prefix+displaypath.FoldText(err.Error()))
	return 1
}
