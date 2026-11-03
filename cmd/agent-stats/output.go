package main

import (
	"fmt"
	"io"
	"strings"

	"charm.land/lipgloss/v2"
	lgtable "charm.land/lipgloss/v2/table"
	cobrahelptree "github.com/alexgorbatchev/cobra-help-tree/v4"
)

func warn(w io.Writer, format string, args ...any) error {
	prefix := "[WARN] "
	if cobrahelptree.IsAgentMode() {
		prefix = "WARN: "
	}
	_, err := fmt.Fprintf(w, prefix+format+"\n", args...)
	return err
}

// cellStyle pads cells so that wrapped text stays clear of the borders.
var cellStyle = lipgloss.NewStyle().Padding(0, 1)

// renderTable renders rows for people. When the table is wider than the terminal, its
// columns shrink and their text wraps so that every line fits.
func renderTable(w io.Writer, header []string, rows [][]string) error {
	t := lgtable.New().
		Border(lipgloss.NormalBorder()).
		Headers(header...).
		Rows(rows...).
		StyleFunc(func(row, col int) lipgloss.Style { return cellStyle })
	if width := cobrahelptree.GetTerminalWidth(); width > 0 && lipgloss.Width(t.String()) > width {
		t.Width(width)
	}
	_, err := fmt.Fprintln(w, t.String())
	return err
}

func tsv(w io.Writer, rows [][]string) error {
	for _, row := range rows {
		if _, err := fmt.Fprintln(w, strings.Join(row, "\t")); err != nil {
			return err
		}
	}
	return nil
}
