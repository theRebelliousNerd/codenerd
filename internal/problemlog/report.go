package problemlog

import (
	"fmt"
	"os"
	"sort"
	"time"
)

// DefaultMaxProblems bounds how many distinct problems a Report keeps. A run
// that logs thousands of distinct warnings must not become thousands of
// kernel facts; the rest is counted in the Report, never dropped silently.
const DefaultMaxProblems = 64

// Problem is one distinct problem: every entry sharing a level, a category and
// a signature. Sample is the first occurrence's whole message.
type Problem struct {
	Signature string
	Level     string
	Category  string
	Sample    string
	Count     int
	First     time.Time
	Last      time.Time
}

// Report is a problems log grouped into distinct problems, most urgent first
// (errors before warnings, then by count, then by recency).
type Report struct {
	Path     string
	Problems []Problem
	// Entries is the WARN and ERROR entries read, kept or not.
	Entries int
	// OmittedProblems distinct problems fell below the MaxProblems cut, and
	// OmittedEntries is how many log entries they account for.
	OmittedProblems int
	OmittedEntries  int
}

// Options bound and filter a read.
type Options struct {
	// MaxProblems caps the distinct problems kept; 0 means DefaultMaxProblems.
	MaxProblems int
	// Skip drops an entry before it is counted.
	Skip func(Entry) bool
}

// Omitted reports whether the cut left problems out.
func (r *Report) Omitted() bool { return r != nil && r.OmittedProblems > 0 }

// Summary names what the report holds and what it left out.
func (r *Report) Summary() string {
	if r == nil {
		return "no problems log"
	}
	s := fmt.Sprintf("%d problem(s) from %d log entr(ies)", len(r.Problems), r.Entries)
	if r.Omitted() {
		s += fmt.Sprintf("; %d lower-ranked problem(s) covering %d entr(ies) left out", r.OmittedProblems, r.OmittedEntries)
	}
	return s
}

// ReadFile reads and groups the problems log at path.
func ReadFile(path string, opts Options) (*Report, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	rep := &Report{Path: path}
	groups := map[string]*Problem{}
	err = Scan(f, opts.Skip, func(e Entry) {
		rep.Entries++
		sig := Signature(e.Message)
		key := e.Level + "|" + e.Category + "|" + sig
		if p, ok := groups[key]; ok {
			p.Count++
			if e.Time.Before(p.First) {
				p.First = e.Time
			}
			if e.Time.After(p.Last) {
				p.Last = e.Time
			}
			return
		}
		groups[key] = &Problem{
			Signature: sig, Level: e.Level, Category: e.Category, Sample: e.Message,
			Count: 1, First: e.Time, Last: e.Time,
		}
	})
	if err != nil {
		return nil, fmt.Errorf("read problems log %s: %w", path, err)
	}
	all := make([]Problem, 0, len(groups))
	for _, p := range groups {
		all = append(all, *p)
	}
	sort.Slice(all, func(i, j int) bool {
		a, b := all[i], all[j]
		if a.Level != b.Level {
			return a.Level == LevelError
		}
		if a.Count != b.Count {
			return a.Count > b.Count
		}
		if !a.Last.Equal(b.Last) {
			return a.Last.After(b.Last)
		}
		if a.Category != b.Category {
			return a.Category < b.Category
		}
		return a.Signature < b.Signature
	})
	limit := opts.MaxProblems
	if limit <= 0 {
		limit = DefaultMaxProblems
	}
	if len(all) > limit {
		for _, p := range all[limit:] {
			rep.OmittedEntries += p.Count
		}
		rep.OmittedProblems = len(all) - limit
		all = all[:limit]
	}
	rep.Problems = all
	return rep, nil
}
