// Package usage counts how agents use skills across the transcripts harnesses store
// on this machine.
package usage

import (
	"bufio"
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"runtime"
	"slices"
	"sync"
	"time"

	parser "github.com/alexgorbatchev/agent-parser"
)

// maxLineBytes bounds a single transcript line. Tool outputs embedded in a line can be
// tens of megabytes.
const maxLineBytes = 256 << 20

// Options selects the transcripts SkillCounts reads.
type Options struct {
	// Harnesses limits the count; empty means every supported harness.
	Harnesses []parser.Harness
	// Since drops invocations recorded before it; the zero time keeps everything.
	Since time.Time
	// Home replaces the user's home directory as the root of default harness locations.
	Home string
	// Workers bounds how many transcripts are read at once; zero means one per CPU.
	Workers int
	// Progress, when set, is told how many sessions were found and when each one has been
	// read.
	Progress Progress
}

// Progress receives scan progress from SkillCounts. Its methods are called from one
// goroutine.
type Progress interface {
	// Discovered reports how many sessions will be read.
	Discovered(total int)
	// Read reports that one more session has been read, successfully or not.
	Read()
}

// Count is how often one skill was invoked through one harness.
type Count struct {
	Skill   string         `json:"skill"`
	Harness parser.Harness `json:"harness,omitempty"`
	Model   int            `json:"model"`
	User    int            `json:"user"`
}

// Total is the number of invocations by the model and the user together.
func (c Count) Total() int { return c.Model + c.User }

// Report holds per-harness skill counts and what could not be read.
type Report struct {
	// Counts has one entry per skill and harness.
	Counts []Count
	// Sessions is the number of sessions read completely. Sessions listed in Problems
	// are not included, even when part of them was counted.
	Sessions int
	// Problems lists sessions or locations that could not be read. Counting continues
	// past them.
	Problems []Problem
}

// Problem is a session or location SkillCounts could not read completely.
type Problem struct {
	// Path is the transcript or database involved, or empty for a discovery failure.
	Path string
	Err  error
}

// SkillCounts reads every discovered session for the selected harnesses and counts the
// skills invoked in them. A fork counts only what happened after it was created, because a
// fork transcript can replay its parent's history first.
func SkillCounts(ctx context.Context, opts Options) (Report, error) {
	sources, err := parser.DiscoverSessions(ctx, parser.DiscoverOptions{Harnesses: opts.Harnesses, Home: opts.Home})
	if ctxErr := ctx.Err(); ctxErr != nil {
		return Report{}, ctxErr
	}
	var report Report
	if err != nil {
		report.Problems = append(report.Problems, Problem{Err: err})
	}
	if opts.Progress != nil {
		opts.Progress.Discovered(len(sources))
	}

	opencode := newOpencodeDBs()
	defer opencode.closeAll()

	workers := opts.Workers
	if workers <= 0 {
		workers = runtime.NumCPU()
	}
	since := int64(0)
	if !opts.Since.IsZero() {
		since = opts.Since.UnixMilli()
	}

	type result struct {
		counts map[countKey]*Count
		err    error
		path   string
	}
	jobs := make(chan parser.SessionSource)
	results := make(chan result)
	var wg sync.WaitGroup
	for range workers {
		wg.Go(func() {
			for source := range jobs {
				counts, err := countSource(ctx, source, since, opencode)
				results <- result{counts: counts, err: err, path: source.Path}
			}
		})
	}
	go func() {
		defer close(jobs)
		for _, source := range sources {
			select {
			case jobs <- source:
			case <-ctx.Done():
				return
			}
		}
	}()
	go func() {
		wg.Wait()
		close(results)
	}()

	totals := map[countKey]*Count{}
	for res := range results {
		if opts.Progress != nil {
			opts.Progress.Read()
		}
		if res.err != nil {
			report.Problems = append(report.Problems, Problem{Path: res.path, Err: res.err})
		} else {
			report.Sessions++
		}
		for key, count := range res.counts {
			total := totals[key]
			if total == nil {
				total = &Count{Skill: key.skill, Harness: key.harness}
				totals[key] = total
			}
			total.Model += count.Model
			total.User += count.User
		}
	}
	if err := ctx.Err(); err != nil {
		return Report{}, err
	}
	for _, count := range totals {
		report.Counts = append(report.Counts, *count)
	}
	SortCounts(report.Counts)
	return report, nil
}

// Combined merges each skill's counts across harnesses, leaving Harness empty.
func (r Report) Combined() []Count {
	bySkill := map[string]*Count{}
	for _, count := range r.Counts {
		merged := bySkill[count.Skill]
		if merged == nil {
			merged = &Count{Skill: count.Skill}
			bySkill[count.Skill] = merged
		}
		merged.Model += count.Model
		merged.User += count.User
	}
	combined := make([]Count, 0, len(bySkill))
	for _, count := range bySkill {
		combined = append(combined, *count)
	}
	SortCounts(combined)
	return combined
}

// SortCounts orders counts by total invocations, most first, then by skill name and
// harness.
func SortCounts(counts []Count) {
	slices.SortFunc(counts, func(a, b Count) int {
		return cmp.Or(
			cmp.Compare(b.Total(), a.Total()),
			cmp.Compare(a.Skill, b.Skill),
			cmp.Compare(a.Harness, b.Harness),
		)
	})
}

type countKey struct {
	skill   string
	harness parser.Harness
}

// addSkills counts the skills in events recorded at or after since and strictly after
// forkedAt; forkedAt is zero outside forks.
func addSkills(counts map[countKey]*Count, harness parser.Harness, events []parser.ParsedEvent, since, forkedAt int64) {
	for _, event := range events {
		if (since > 0 && event.Timestamp < since) || (forkedAt > 0 && event.Timestamp <= forkedAt) {
			continue
		}
		for _, skill := range event.Data.Skills {
			key := countKey{skill: skill.Name, harness: harness}
			count := counts[key]
			if count == nil {
				count = &Count{Skill: skill.Name, Harness: harness}
				counts[key] = count
			}
			if skill.Source == parser.SkillSourceUser {
				count.User++
			} else {
				count.Model++
			}
		}
	}
}

// errUnknownForkTime reports a fork whose creation time discovery could not determine.
// Its transcript cannot be separated from the parent history it replays, so it is skipped.
var errUnknownForkTime = errors.New("fork creation time is unknown, so its replayed parent history cannot be told apart")

func countSource(ctx context.Context, source parser.SessionSource, since int64, opencode *opencodeDBs) (map[countKey]*Count, error) {
	counts := map[countKey]*Count{}
	if source.Fork && source.ForkedAt.IsZero() {
		return counts, errUnknownForkTime
	}
	if source.Harness == parser.HarnessOpencode {
		db, err := opencode.open(source.Path)
		if err != nil {
			return counts, err
		}
		events, err := db.ReconstructSessionEvents(ctx, source.SessionID)
		if err != nil {
			return counts, fmt.Errorf("reading OpenCode session %s: %w", source.SessionID, err)
		}
		addSkills(counts, source.Harness, events, since, 0)
		return counts, nil
	}

	reader, err := parser.OpenTranscript(source.Path)
	if err != nil {
		return counts, err
	}
	defer func() { _ = reader.Close() }()
	return counts, countTranscript(reader, source, since, counts)
}

// countTranscript parses a JSONL transcript line by line. Lines a parser rejects are
// skipped; a read failure stops the transcript and is returned with what was counted.
// A fork's events at or before its fork time are replayed parent history and are skipped.
func countTranscript(reader io.Reader, source parser.SessionSource, since int64, counts map[countKey]*Count) error {
	parse := lineParser(source.Harness)
	forkedAt := int64(0)
	if source.Fork && !source.ForkedAt.IsZero() {
		forkedAt = source.ForkedAt.UnixMilli()
	}
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 1<<20), maxLineBytes)
	for scanner.Scan() {
		events, err := parse(scanner.Bytes())
		if err != nil {
			continue
		}
		addSkills(counts, source.Harness, events, since, forkedAt)
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("reading transcript: %w", err)
	}
	return nil
}

func lineParser(harness parser.Harness) func([]byte) ([]parser.ParsedEvent, error) {
	switch harness {
	case parser.HarnessClaudeCode:
		return parser.NewClaudeCodeParser().ParseLine
	case parser.HarnessCodex:
		return parser.NewCodexParser().ParseLine
	default:
		return func(line []byte) ([]parser.ParsedEvent, error) { return parser.ParsePiLine(line, "") }
	}
}

// opencodeDBs shares one read-only handle per OpenCode database across workers.
type opencodeDBs struct {
	mu  sync.Mutex
	dbs map[string]*parser.OpencodeDB
}

func newOpencodeDBs() *opencodeDBs {
	return &opencodeDBs{dbs: map[string]*parser.OpencodeDB{}}
}

func (o *opencodeDBs) open(path string) (*parser.OpencodeDB, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if db, ok := o.dbs[path]; ok {
		return db, nil
	}
	db, err := parser.OpenOpencodeDB(path)
	if err != nil {
		return nil, err
	}
	o.dbs[path] = db
	return db, nil
}

// closeAll closes every handle. The databases were opened read-only, so a failed close
// loses nothing.
func (o *opencodeDBs) closeAll() {
	o.mu.Lock()
	defer o.mu.Unlock()
	for _, db := range o.dbs {
		_ = db.Close()
	}
}
