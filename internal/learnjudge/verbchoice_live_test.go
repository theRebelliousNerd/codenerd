//go:build jevlive

package learnjudge

import (
	"context"
	"fmt"
	"testing"

	"codenerd/internal/perception"
)

// TestLiveVerbChoice probes the verb choice over the full built-in corpus
// with requests that have a clear verb, and two that have none.
func TestLiveVerbChoice(t *testing.T) {
	c, _ := liveClient(t, "verb_choice")
	var options []VerbOption
	for _, v := range perception.DefaultTaxonomyData {
		options = append(options, VerbOption{Verb: v.Verb, Means: v.Synonyms})
	}
	cases := []struct{ name, request, want string }{
		{"fix", "the prompt atoms for your shard agents are way too big, fix whatever system produced them", "/fix"},
		{"refactor", "split agents.go into smaller files", "/refactor"},
		{"review", "look over the auth module for problems", "/review"},
		{"explain", "what does the brief do", "/explain"},
		{"tool", "make a tool that counts TODO comments", "/generate_tool"},
		{"stats", "how many go files are in the repo", "/stats"},
		{"self", "debug yourself, your logs show perception misrouting", "/self_diagnose"},
		{"none-yes", "yes", ""},
		{"none-mode", "what work mode are you in, and is debug-self on?", ""},
	}
	var lines []string
	for _, tc := range cases {
		got, err := ChooseVerb(probeLabel(context.Background(), tc.name), c, tc.request, options)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		learned := got.Fits >= 0.7 && got.Confidence >= 0.7
		ok := (tc.want == "" && !learned) || (tc.want != "" && learned && got.Verb == tc.want)
		mark := " "
		if !ok {
			mark = "x"
		}
		lines = append(lines, fmt.Sprintf("%s %-9s %-14s conf=%.2f fits=%.2f want=%q", mark, tc.name, got.Verb, got.Confidence, got.Fits, tc.want))
		t.Log(lines[len(lines)-1])
	}
	probeSummary(t, "verb_choice", append([]string{"verb choice probe: learned when fits>=0.70 and confidence>=0.70"}, lines...))
}
