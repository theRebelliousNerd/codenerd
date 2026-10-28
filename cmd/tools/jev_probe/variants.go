package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"time"

	"codenerd/internal/turnjudge"
	"codenerd/internal/typesafe"
)

// variantSet is several wordings of one turn judgment. Each case is asked once,
// with every wording as its own question over the same state, so the wordings
// differ in nothing else.
type variantSet struct {
	Judgment string `json:"judgment"`
	Variants map[string]struct {
		Instructions any    `json:"instructions"`
		True         string `json:"true"`
		False        string `json:"false"`
	} `json:"variants"`
}

func runVariants(client *typesafe.Client, path string, cases []probeCase, yes, no float64) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var set variantSet
	if err := json.Unmarshal(data, &set); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	names := make([]string, 0, len(set.Variants))
	for n := range set.Variants {
		names = append(names, n)
	}
	sort.Strings(names)
	tallies := map[string]*tally{}
	var lines []string
	for _, c := range cases {
		want, ok := c.Expect[set.Judgment]
		if !ok {
			continue
		}
		state, _, _ := turnjudge.Request(turnjudge.Input{Message: c.Message, Recent: c.Recent, Brief: c.Brief, HeldWork: c.HeldWork, RunningWork: c.RunningWork})
		qs := map[string]typesafe.Question{}
		for _, n := range names {
			v := set.Variants[n]
			q := typesafe.Noul{Instructions: v.Instructions}
			if v.True != "" || v.False != "" {
				q.Criteria = &typesafe.NoulCriteria{}
				if v.True != "" {
					q.Criteria.True = v.True
				}
				if v.False != "" {
					q.Criteria.False = v.False
				}
			}
			qs[n] = q
		}
		ctx, cancel := context.WithTimeout(withLabel(context.Background(), "variants-"+set.Judgment+"-"+c.ID), time.Minute)
		resp, err := client.Ask(ctx, state, qs)
		cancel()
		if err != nil {
			return fmt.Errorf("case %s: %w", c.ID, err)
		}
		row := fmt.Sprintf("  %-22s want=%-5v", c.ID, want)
		for _, n := range names {
			p, err := resp.Noul(n)
			if err != nil {
				return err
			}
			t := tallies[n]
			if t == nil {
				t = &tally{}
				tallies[n] = t
			}
			switch {
			case p >= yes && want, p <= no && !want:
				t.right++
			case p > no && p < yes:
				t.unsettled++
			default:
				t.wrong++
			}
			row += fmt.Sprintf("  %s=%.2f", n, p)
		}
		lines = append(lines, row)
	}
	fmt.Printf("\nvariants of %s over %d case(s)\n", set.Judgment, len(lines))
	for _, l := range lines {
		fmt.Println(l)
	}
	for _, n := range names {
		t := tallies[n]
		fmt.Printf("  %-12s right %2d  wrong %2d  unsettled %2d\n", n, t.right, t.wrong, t.unsettled)
	}
	return nil
}
