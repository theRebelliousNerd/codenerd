package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"codenerd/internal/config"
	"codenerd/internal/problemlog"
	"codenerd/internal/retrieval"
	"codenerd/internal/turnjudge"
	"codenerd/internal/typesafe"
)

// usesCases are labeled cases for Jev's other three uses. A nil expectation is
// a hard case: its probability is reported, not scored.
type usesCases struct {
	Relevance []struct {
		ID      string `json:"id"`
		Request string `json:"request"`
		Files   []struct {
			Path     string `json:"path"`
			Relevant bool   `json:"relevant"`
		} `json:"files"`
	} `json:"relevance"`
	Findings []struct {
		ID      string `json:"id"`
		File    string `json:"file"`
		Line    int    `json:"line"`
		Message string `json:"message"`
		Source  string `json:"source"`
		Expect  bool   `json:"expect"`
	} `json:"findings"`
	Triage []struct {
		ID       string `json:"id"`
		Level    string `json:"level"`
		Category string `json:"category"`
		Message  string `json:"message"`
		Defect   *bool  `json:"defect"`
	} `json:"triage"`
}

// reading classifies a probability against the thresholds.
func reading(p, yes, no float64) string {
	switch {
	case p >= yes:
		return "yes"
	case p <= no:
		return "no"
	}
	return "unsettled"
}

func verdict(p float64, want *bool, yes, no float64) string {
	r := reading(p, yes, no)
	switch {
	case want == nil:
		return "hard"
	case r == "unsettled":
		return "~"
	case (r == "yes") == *want:
		return "ok"
	}
	return "WRONG"
}

func runUses(client *typesafe.Client, sec *config.TypeSafeConfig, path string, yes, no float64) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var cases usesCases
	if err := json.Unmarshal(data, &cases); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	uses := sec.Uses()
	fmt.Printf("\nretrieval relevance (repo files, head %d bytes)\n", uses.RelevanceHeadBytes)
	judge := turnjudge.RelevanceJudge{Asker: client, BatchSize: uses.BatchSize}
	for _, c := range cases.Relevance {
		var files []retrieval.RelevanceCandidate
		for _, f := range c.Files {
			raw, err := os.ReadFile(f.Path)
			if err != nil {
				return err
			}
			head := raw[:min(len(raw), uses.RelevanceHeadBytes)]
			files = append(files, retrieval.RelevanceCandidate{Path: f.Path, Excerpt: string(head), ExcerptOf: len(raw)})
		}
		ctx, cancel := context.WithTimeout(withLabel(context.Background(), "relevance-"+c.ID), time.Minute)
		got, err := judge.Relevance(ctx, c.Request, files)
		cancel()
		if err != nil {
			return err
		}
		fmt.Printf(" %s: %q\n", c.ID, c.Request)
		for _, f := range c.Files {
			want := f.Relevant
			fmt.Printf("   %-6s p=%.2f want=%-5v %s\n", verdict(got[f.Path], &want, yes, no), got[f.Path], want, filepath.Base(f.Path))
		}
	}

	fmt.Printf("\nreview finding checks\n")
	dir, err := os.MkdirTemp("", "jev_probe")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	for _, c := range cases.Findings {
		sub := filepath.Join(dir, c.ID)
		if err := os.MkdirAll(sub, 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(sub, c.File), []byte(c.Source), 0o644); err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(withLabel(context.Background(), "finding-"+c.ID), time.Minute)
		v, err := turnjudge.CheckFindings(ctx, client, sub, []turnjudge.Finding{{File: c.File, Line: c.Line, Message: c.Message}},
			turnjudge.FindingOptions{BatchSize: uses.BatchSize, ContextLines: uses.FindingContextLines})
		cancel()
		if err != nil {
			return err
		}
		if len(v) != 1 {
			return fmt.Errorf("finding %s was not judged", c.ID)
		}
		want := c.Expect
		fmt.Printf("   %-6s p=%.2f want=%-5v %s\n", verdict(v[0].Probability, &want, yes, no), v[0].Probability, want, c.ID)
	}

	fmt.Printf("\nself-diagnosis triage (defect = a bug in codeNERD's code)\n")
	var problems []problemlog.Problem
	for _, c := range cases.Triage {
		problems = append(problems, problemlog.Problem{Signature: c.ID, Level: c.Level, Category: c.Category, Sample: c.Message, Count: 1})
	}
	ctx, cancel := context.WithTimeout(withLabel(context.Background(), "triage"), time.Minute)
	probs, err := turnjudge.TriageProblems(ctx, client, problems, uses.BatchSize)
	cancel()
	if err != nil {
		return err
	}
	for i := range cases.Triage {
		c := cases.Triage[i]
		fmt.Printf("   %-6s p=%.2f want=%-5v %s\n", verdict(probs[i], c.Defect, yes, no), probs[i], wantText(c.Defect), c.ID)
	}
	return nil
}

func wantText(b *bool) string {
	if b == nil {
		return "?"
	}
	return fmt.Sprint(*b)
}
