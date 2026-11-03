package main

import (
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	parser "github.com/alexgorbatchev/agent-parser"
	"github.com/alexgorbatchev/agent-stats-cli/internal/displaypath"
	"github.com/alexgorbatchev/agent-stats-cli/internal/usage"
	cobrahelptree "github.com/alexgorbatchev/cobra-help-tree/v4"
	"github.com/spf13/cobra"
)

type skillsFlags struct {
	harnesses []string
	byHarness bool
	since     string
	limit     int
	json      bool
}

// now is the clock --since is measured from; tests replace it.
var now = time.Now

func newUsageSkillsCommand() *cobra.Command {
	var flags skillsFlags
	cmd := &cobra.Command{
		Use:   "skills",
		Short: "Show which skills agents use most",
		Long: "Counts every skill invocation in the sessions Claude Code, Codex, Pi, and OpenCode keep on this machine, " +
			"whether the agent loaded the skill on its own or you asked for it, and lists skills from most to least used.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			harnesses, err := parseHarnesses(flags.harnesses)
			if err != nil {
				return err
			}
			since, err := parseSince(flags.since, now())
			if err != nil {
				return err
			}
			if flags.limit < 0 {
				return fmt.Errorf("--limit must be zero or more, got %d", flags.limit)
			}
			report, err := usage.SkillCounts(cmd.Context(), usage.Options{Harnesses: harnesses, Since: since})
			if err != nil {
				return err
			}
			if err := printProblems(cmd.ErrOrStderr(), report.Problems); err != nil {
				return err
			}
			counts := report.Combined()
			if flags.byHarness {
				counts = report.Counts
			}
			summary := summary{sessions: report.Sessions, skills: len(report.Combined())}
			for _, count := range counts {
				summary.invocations += count.Total()
			}
			if flags.limit > 0 && len(counts) > flags.limit {
				counts = counts[:flags.limit]
			}
			return printSkillCounts(cmd.OutOrStdout(), counts, summary, flags)
		},
	}
	f := cmd.Flags()
	f.StringSliceVar(&flags.harnesses, "harness", nil, "Only count these harnesses: claude-code, codex, pi, opencode (repeatable or comma-separated)")
	f.BoolVar(&flags.byHarness, "by-harness", false, "Show a separate row for each skill and harness")
	f.StringVar(&flags.since, "since", "", "Only count invocations since a time: a duration such as 30d or 12h, or a date such as 2026-10-01")
	f.IntVar(&flags.limit, "limit", 0, "Show at most this many rows; 0 shows all")
	f.BoolVar(&flags.json, "json", false, "Print the rows as a JSON array")
	return cmd
}

func parseHarnesses(values []string) ([]parser.Harness, error) {
	supported := parser.SupportedHarnesses()
	var harnesses []parser.Harness
	for _, value := range values {
		harness := parser.Harness(strings.TrimSpace(strings.ToLower(value)))
		if !slices.Contains(supported, harness) {
			names := make([]string, len(supported))
			for i, h := range supported {
				names[i] = string(h)
			}
			return nil, fmt.Errorf("unknown harness %q: use one of %s", value, strings.Join(names, ", "))
		}
		if !slices.Contains(harnesses, harness) {
			harnesses = append(harnesses, harness)
		}
	}
	return harnesses, nil
}

var dayDuration = regexp.MustCompile(`^(\d+)d$`)

// parseSince turns --since into the earliest time to count: a number of days such as
// 30d, a Go duration such as 12h, a date such as 2026-10-01 (local midnight), or an
// RFC 3339 time. An empty value counts everything.
func parseSince(value string, now time.Time) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, nil
	}
	if match := dayDuration.FindStringSubmatch(value); match != nil {
		days, err := strconv.Atoi(match[1])
		if err != nil {
			return time.Time{}, fmt.Errorf("invalid --since %q: %w", value, err)
		}
		return now.AddDate(0, 0, -days), nil
	}
	if duration, err := time.ParseDuration(value); err == nil {
		if duration < 0 {
			return time.Time{}, fmt.Errorf("invalid --since %q: the duration must not be negative", value)
		}
		return now.Add(-duration), nil
	}
	if date, err := time.ParseInLocation(time.DateOnly, value, now.Location()); err == nil {
		return date, nil
	}
	if at, err := time.Parse(time.RFC3339, value); err == nil {
		return at, nil
	}
	return time.Time{}, fmt.Errorf("invalid --since %q: use a duration such as 30d or 12h, or a date such as 2026-10-01", value)
}

func printProblems(w io.Writer, problems []usage.Problem) error {
	for _, problem := range problems {
		message := displaypath.FoldText(problem.Err.Error())
		if problem.Path != "" {
			message = displaypath.Fold(problem.Path) + ": " + message
		}
		if err := warn(w, "skipped unreadable data: %s", message); err != nil {
			return err
		}
	}
	return nil
}

type countRow struct {
	Skill   string `json:"skill"`
	Harness string `json:"harness,omitempty"`
	Model   int    `json:"model"`
	User    int    `json:"user"`
	Total   int    `json:"total"`
}

// summary describes everything counted, before --limit trims the rows shown.
type summary struct {
	invocations int
	skills      int
	sessions    int
}

func printSkillCounts(w io.Writer, counts []usage.Count, total summary, flags skillsFlags) error {
	rows := make([]countRow, len(counts))
	for i, count := range counts {
		rows[i] = countRow{Skill: count.Skill, Harness: string(count.Harness), Model: count.Model, User: count.User, Total: count.Total()}
	}
	if flags.json {
		encoder := json.NewEncoder(w)
		if !cobrahelptree.IsAgentMode() {
			encoder.SetIndent("", "  ")
		}
		return encoder.Encode(rows)
	}

	if cobrahelptree.IsAgentMode() {
		records := make([][]string, len(rows))
		for i, row := range rows {
			record := []string{row.Skill}
			if flags.byHarness {
				record = append(record, row.Harness)
			}
			records[i] = append(record, strconv.Itoa(row.Model), strconv.Itoa(row.User), strconv.Itoa(row.Total))
		}
		return tsv(w, records)
	}

	if len(rows) == 0 {
		_, err := fmt.Fprintf(w, "[INFO] No skill invocations found in %s.\n", plural(total.sessions, "session"))
		return err
	}
	header := []string{"Skill", "Model", "User", "Total"}
	if flags.byHarness {
		header = []string{"Skill", "Harness", "Model", "User", "Total"}
	}
	table := make([][]string, len(rows))
	for i, row := range rows {
		record := []string{row.Skill}
		if flags.byHarness {
			record = append(record, row.Harness)
		}
		table[i] = append(record, strconv.Itoa(row.Model), strconv.Itoa(row.User), strconv.Itoa(row.Total))
	}
	if err := renderTable(w, header, table); err != nil {
		return err
	}
	_, err := fmt.Fprintf(w, "%s of %s across %s. Model: the agent loaded the skill itself. User: you invoked it.\n",
		plural(total.invocations, "invocation"), plural(total.skills, "skill"), plural(total.sessions, "session"))
	return err
}

func plural(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return strconv.Itoa(n) + " " + noun + "s"
}
