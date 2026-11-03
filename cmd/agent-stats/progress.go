package main

import (
	"io"
	"os"
	"time"

	cobrahelptree "github.com/alexgorbatchev/cobra-help-tree/v4"
	"github.com/schollz/progressbar/v3"
	"golang.org/x/term"
)

// isTerminal reports whether w is a terminal; tests replace it.
var isTerminal = func(w io.Writer) bool {
	f, ok := w.(*os.File)
	return ok && term.IsTerminal(int(f.Fd()))
}

// scanIndicator shows people that a scan is running: a note while sessions are found,
// then a count of sessions read. It writes to stderr and clears itself when done, so the
// results printed afterwards are unaffected.
type scanIndicator struct {
	bar *progressbar.ProgressBar
}

// newScanIndicator returns an indicator writing to w, or nil when none should be shown:
// in agent mode, for JSON output, or when w is not a terminal.
func newScanIndicator(w io.Writer, jsonOutput bool) *scanIndicator {
	if cobrahelptree.IsAgentMode() || jsonOutput || !isTerminal(w) {
		return nil
	}
	bar := progressbar.NewOptions(-1,
		progressbar.OptionSetWriter(w),
		progressbar.OptionSetDescription("Finding sessions"),
		progressbar.OptionSpinnerType(14),
		progressbar.OptionShowCount(),
		progressbar.OptionShowTotalBytes(true), // shows the count as read/total

		progressbar.OptionSetElapsedTime(false),
		progressbar.OptionSetPredictTime(false),
		progressbar.OptionThrottle(100*time.Millisecond),
		progressbar.OptionClearOnFinish(),
	)
	_ = bar.RenderBlank()
	return &scanIndicator{bar: bar}
}

func (s *scanIndicator) Discovered(total int) {
	s.bar.Describe("Reading sessions")
	s.bar.ChangeMax(total)
	_ = s.bar.RenderBlank()
}

func (s *scanIndicator) Read() {
	_ = s.bar.Add(1)
}

// done clears the indicator before results are printed.
func (s *scanIndicator) done() {
	if s != nil {
		_ = s.bar.Finish()
	}
}
