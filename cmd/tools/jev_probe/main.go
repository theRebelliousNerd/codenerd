// Command jev_probe scores the turn judgments (internal/turnjudge) against
// labeled cases on the live Jev API, so question wording is tuned on evidence
// rather than intuition. It reads the typesafe section of a codeNERD config
// (nothing is defaulted) and prints, per judgment, how many cases read right,
// wrong, or unsettled at the configured thresholds, and every miss with its
// probability.
//
// With -uses it also scores the other three uses (retrieval relevance, review
// finding checks, self-diagnosis triage) against labeled cases. With -record
// DIR every request and reply is kept as one JSON file, so any answer can be
// rerun from its exact payload; the API key is only ever in a header, which is
// not recorded.
//
// -variants FILE compares wordings of one judgment instead (variants.go).
//
// Usage: go run ./cmd/tools/jev_probe -config .nerd/config.json [-cases internal/turnjudge/testdata/cases.json] [-uses internal/turnjudge/testdata/uses_cases.json] [-record DIR] [-runs 1]
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"codenerd/internal/config"
	"codenerd/internal/perception"
	"codenerd/internal/turnjudge"
)

type probeCase struct {
	ID          string              `json:"id"`
	Message     string              `json:"message"`
	Recent      []turnjudge.Turn    `json:"recent,omitempty"`
	Brief       *perception.Brief   `json:"brief,omitempty"`
	HeldWork    *turnjudge.HeldWork `json:"held_work,omitempty"`
	RunningWork []string            `json:"running_work,omitempty"`
	Expect      map[string]bool     `json:"expect"`
}

type tally struct{ right, wrong, unsettled int }

func main() {
	cfgPath := flag.String("config", ".nerd/config.json", "codeNERD config holding the typesafe section")
	casesPath := flag.String("cases", "internal/turnjudge/testdata/cases.json", "labeled cases")
	usesPath := flag.String("uses", "", "labeled cases for relevance, review checks and triage (empty skips them)")
	variantsPath := flag.String("variants", "", "wordings of one judgment to compare side by side on the labeled cases (no other scoring runs)")
	recordDir := flag.String("record", "", "directory to keep every request and reply in")
	runs := flag.Int("runs", 1, "times to ask each case (spread shows answer stability)")
	flag.Parse()
	if err := run(*cfgPath, *casesPath, *usesPath, *variantsPath, *recordDir, *runs); err != nil {
		fmt.Fprintln(os.Stderr, "jev_probe:", err)
		os.Exit(1)
	}
}

func run(cfgPath, casesPath, usesPath, variantsPath, recordDir string, runs int) error {
	cfg, err := config.LoadUserConfig(cfgPath)
	if err != nil {
		return err
	}
	if cfg.TypeSafe == nil {
		return fmt.Errorf("%s has no typesafe section", cfgPath)
	}
	client, err := newClient(cfg.TypeSafe, recordDir)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(casesPath)
	if err != nil {
		return err
	}
	var cases []probeCase
	if err := json.Unmarshal(data, &cases); err != nil {
		return fmt.Errorf("%s: %w", casesPath, err)
	}
	yes, no := thresholds(cfg.TypeSafe)

	if variantsPath != "" {
		return runVariants(client, variantsPath, cases, yes, no)
	}

	tallies := map[string]*tally{}
	var misses []string
	tokens := 0
	start := time.Now()
	for r := 0; r < runs; r++ {
		for _, c := range cases {
			ctx, cancel := context.WithTimeout(withLabel(context.Background(), "turn-"+c.ID), time.Minute)
			res, err := turnjudge.Judge(ctx, client, turnjudge.Input{Message: c.Message, Recent: c.Recent, Brief: c.Brief, HeldWork: c.HeldWork, RunningWork: c.RunningWork})
			cancel()
			if err != nil {
				return fmt.Errorf("case %s: %w", c.ID, err)
			}
			tokens += res.InputTokens
			for name, want := range c.Expect {
				p, ok := res.Judgments[name]
				if !ok {
					return fmt.Errorf("case %s: %s was not asked", c.ID, name)
				}
				t := tallies[name]
				if t == nil {
					t = &tally{}
					tallies[name] = t
				}
				switch {
				case p >= yes && want, p <= no && !want:
					t.right++
				case p > no && p < yes:
					t.unsettled++
					misses = append(misses, fmt.Sprintf("  ~ %-22s %-18s p=%.2f want=%v  %q", c.ID, name, p, want, c.Message))
				default:
					t.wrong++
					misses = append(misses, fmt.Sprintf("  x %-22s %-18s p=%.2f want=%v  %q", c.ID, name, p, want, c.Message))
				}
			}
		}
	}

	names := make([]string, 0, len(tallies))
	for n := range tallies {
		names = append(names, n)
	}
	sort.Strings(names)
	fmt.Printf("jev_probe: %d case(s) x %d run(s), thresholds yes>=%.2f no<=%.2f, %d input tokens, %s\n\n",
		len(cases), runs, yes, no, tokens, time.Since(start).Round(time.Millisecond))
	totalRight, total := 0, 0
	for _, n := range names {
		t := tallies[n]
		sum := t.right + t.wrong + t.unsettled
		totalRight += t.right
		total += sum
		fmt.Printf("%-20s right %3d  wrong %3d  unsettled %3d  (%.0f%%)\n", n, t.right, t.wrong, t.unsettled, 100*float64(t.right)/float64(sum))
	}
	fmt.Printf("%-20s right %3d of %d (%.0f%%)\n\n", "all", totalRight, total, 100*float64(totalRight)/float64(total))
	if len(misses) > 0 {
		sort.Strings(misses)
		fmt.Println("misses (x wrong, ~ unsettled):")
		fmt.Println(strings.Join(misses, "\n"))
	}
	if usesPath != "" {
		return runUses(client, cfg.TypeSafe, usesPath, yes, no)
	}
	return nil
}

func thresholds(c *config.TypeSafeConfig) (yes, no float64) {
	for _, p := range c.Params() {
		switch p.Key {
		case "/jev_yes_percent":
			yes = float64(p.Value) / 100
		case "/jev_no_percent":
			no = float64(p.Value) / 100
		}
	}
	return yes, no
}
