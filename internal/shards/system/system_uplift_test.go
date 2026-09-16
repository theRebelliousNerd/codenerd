package system

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"codenerd/internal/campaign"
	"codenerd/internal/core"
	"codenerd/internal/types"
)

// The proposal prompt is hashed by FeedbackLoop.CanRetryPrompt for per-prompt
// retry accounting: map-ordered context and pattern lists would mint a
// fresh-looking prompt on every call and defeat the retry budget.
func TestBuildPolicyProposalPrompt_Stable(t *testing.T) {
	e := NewExecutivePolicyShard()
	e.patternSuccess["zebra"] = 5
	e.patternSuccess["alpha"] = 5
	e.patternFailure["delta"] = 3
	e.patternFailure["beta"] = 3
	cases := []UnhandledCase{{
		Query:   "next_action(/x)",
		Context: map[string]string{"zeta": "1", "aardvark": "2"},
	}}
	first := e.buildPolicyProposalPrompt(cases)
	for i := 0; i < 10; i++ {
		if got := e.buildPolicyProposalPrompt(cases); got != first {
			t.Fatalf("prompt differs between builds:\n%s\n---\n%s", first, got)
		}
	}
	if !strings.Contains(first, "alpha\n- zebra") {
		t.Errorf("success patterns not in sorted order:\n%s", first)
	}
	if !strings.Contains(first, "aardvark: 2\n   zeta: 1") {
		t.Errorf("case context not in sorted order:\n%s", first)
	}
}

// Proposed-rule confidence documents 0.0-1.0 and gates HotLoad: 1.5 must
// not auto-apply, nor -0.5 linger.
func TestParseProposedRule_ConfidenceClamped(t *testing.T) {
	e := NewExecutivePolicyShard()
	if got := e.parseProposedRule("RULE: x :- y.\nCONFIDENCE: 1.5\n", nil).Confidence; got != 1 {
		t.Errorf("1.5 -> %v, want 1", got)
	}
	if got := e.parseProposedRule("RULE: x :- y.\nCONFIDENCE: -0.5\n", nil).Confidence; got != 0 {
		t.Errorf("-0.5 -> %v, want 0", got)
	}
	if got := e.parseProposedRule("RULE: x :- y.\nCONFIDENCE: 0.7\n", nil).Confidence; got != 0.7 {
		t.Errorf("0.7 -> %v, want 0.7", got)
	}
}

// slowFirstSpawner answers the first target last, so completion order is the
// reverse of requested order. The batch contract is requested order: both
// consumers render responses in slice order into reports.
type slowFirstSpawner struct{}

func (slowFirstSpawner) SpawnConsultation(ctx context.Context, specialistName, task string) (string, error) {
	if specialistName == "slow" {
		select {
		case <-time.After(150 * time.Millisecond):
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
	return "ADVICE: from " + specialistName + "\nCONFIDENCE: 80", nil
}

func TestBatchConsultation_RequestedOrder(t *testing.T) {
	m := newCampaignRunnerConsultationManager(slowFirstSpawner{})
	responses, err := m.RequestBatchConsultation(context.Background(), campaign.BatchConsultRequest{
		Question:   "q",
		TargetSpec: []string{"slow", "fast-a", "fast-b"},
	})
	if err != nil {
		t.Fatalf("batch failed: %v", err)
	}
	if len(responses) != 3 {
		t.Fatalf("got %d responses, want 3", len(responses))
	}
	for i, want := range []string{"slow", "fast-a", "fast-b"} {
		if responses[i].FromSpec != want {
			t.Fatalf("position %d = %s, want %s (full: %v)", i, responses[i].FromSpec, want, responses)
		}
	}
}

// Learned-pattern lists read map registries: they must come out sorted, not
// in iteration order, for every consumer that renders them.
func TestGetLearnedPatterns_Sorted(t *testing.T) {
	e := NewExecutivePolicyShard()
	for _, p := range []string{"zebra", "alpha", "mike"} {
		e.patternSuccess[p] = 5
		e.patternFailure[p] = 3
	}
	for i := 0; i < 5; i++ {
		got := e.GetLearnedPatterns()
		want := []string{"alpha", "mike", "zebra"}
		for j, w := range want {
			if got["successful"][j] != w || got["failed"][j] != w {
				t.Fatalf("run %d: got %v / %v, want sorted %v", i, got["successful"], got["failed"], want)
			}
		}
	}

	p := NewPerceptionFirewallShard()
	for _, q := range []string{"zebra", "alpha", "mike"} {
		p.BaseSystemShard.patternSuccess[q] = 5
		p.BaseSystemShard.corrections[q] = 3
	}
	got := p.GetLearnedPatterns()
	for j, w := range []string{"alpha", "mike", "zebra"} {
		if got["successful"][j] != w || got["corrections"][j] != w {
			t.Fatalf("perception: got %v / %v, want sorted", got["successful"], got["corrections"])
		}
	}
}

// Directory excludes match whole segments: vendor/ goes, but codevendor/
// stays. The old substring check dropped every path containing the word.
func TestExcludedByPatterns_SegmentMatch(t *testing.T) {
	excludes := []string{"vendor/*", "node_modules/*", ".git/*", "*.exe"}
	cases := []struct {
		path string
		want bool
	}{
		{"root/vendor/x.go", true},
		{"root/codevendor/x.go", false},
		{"root/node_modules/x.go", true},
		{"root/my-node_modules/x.go", false},
		{"root/.git/config", true},
		{"root/app.exe", true},
		{"root/app.go", false},
	}
	for _, c := range cases {
		if got := excludedByPatterns(c.path, excludes); got != c.want {
			t.Errorf("excludedByPatterns(%q) = %v, want %v", c.path, got, c.want)
		}
	}
}

// Full and incremental scans must agree on what the world contains: the
// incremental path used to skip the include gate, so steady state ingested
// logs and binaries a restart would never include.
func TestWorldModelScans_IncludeParity(t *testing.T) {
	dir := t.TempDir()
	write := func(rel, body string) {
		t.Helper()
		p := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("vendor/skip.go", "package vendor\n")
	write("codevendor/keep.go", "package keep\n")
	write("note.log", "noise\n")

	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatal(err)
	}
	cfg := DefaultWorldModelConfig()
	cfg.RootPath = dir
	w := NewWorldModelIngestorShardWithConfig(cfg)
	w.Kernel = kernel
	ctx := context.Background()

	if err := w.performFullScan(ctx); err != nil {
		t.Fatalf("full scan: %v", err)
	}
	topology := func() map[string]bool {
		facts, err := kernel.Query("file_topology")
		if err != nil {
			t.Fatalf("query: %v", err)
		}
		seen := map[string]bool{}
		for _, f := range facts {
			seen[types.ExtractString(f.Args[0])] = true
		}
		return seen
	}
	seen := topology()
	if seen["vendor/skip.go"] {
		t.Error("full scan ingested vendor/skip.go")
	}
	if !seen["codevendor/keep.go"] {
		t.Error("full scan dropped codevendor/keep.go")
	}
	if seen["note.log"] {
		t.Error("full scan ingested note.log")
	}

	write("app.log", "more noise\n")
	write("new.go", "package newpkg\n")
	if err := w.performIncrementalScan(ctx); err != nil {
		t.Fatalf("incremental scan: %v", err)
	}
	seen = topology()
	if !seen["new.go"] {
		t.Error("incremental scan missed new.go")
	}
	if seen["app.log"] {
		t.Error("incremental scan ingested app.log, which the full scan would never include")
	}
}

// The domain allowlist matches hosts, not substrings: evil-github.com and
// github.com.evil.com merely contain an allowed name and must be refused,
// while genuine subdomains and ports stay admitted.
func TestIsAllowedDomain_HostMatch(t *testing.T) {
	shard := NewConstitutionGateShard()
	cases := []struct {
		target string
		want   bool
	}{
		{"https://github.com/user/repo", true},
		{"https://api.github.com:8443/v1", true},
		{"github.com", true},
		{"https://evil-github.com/x", false},
		{"https://github.com.evil.com/x", false},
		{"https://githubXcom/", false},
		{"https://evil.example.com", false},
		{"", false},
	}
	for _, c := range cases {
		if got := shard.isAllowedDomain(c.target); got != c.want {
			t.Errorf("isAllowedDomain(%q) = %v, want %v", c.target, got, c.want)
		}
	}
}

// The rule-proposal prompt is hashed by CanRetryPrompt for per-prompt retry
// accounting: map-ordered context would mint a fresh-looking prompt on every
// call and defeat the retry budget.
func TestBuildRuleProposalPrompt_Stable(t *testing.T) {
	c := NewConstitutionGateShard()
	cases := []UnhandledCase{{
		Query:   "permitted(/x)",
		Context: map[string]string{"zeta": "1", "aardvark": "2", "mid": "3"},
	}}
	first := c.buildRuleProposalPrompt(cases)
	for i := 0; i < 10; i++ {
		if got := c.buildRuleProposalPrompt(cases); got != first {
			t.Fatalf("prompt differs between builds:\n%s\n---\n%s", first, got)
		}
	}
}

// classifyVerbMapping's unknown-verb branch is unreachable through Perceive
// (the transducer only emits corpus verbs), but it is the documented guard:
// a corpus-unknown verb must classify unmapped rather than slip through.
func TestClassifyVerbMapping_UnknownVerb(t *testing.T) {
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatal(err)
	}
	// A real kernel: with no kernel the mapping check degrades to mapped
	// (kernelless Perceive is already degraded mode everywhere else), so
	// the unknown-verb branch needs mappings to consult.
	p := NewPerceptionFirewallShard()
	p.SetParentKernel(kernel)
	mapped, reason := p.classifyVerbMapping("/teleport")
	if mapped || reason != "/unknown_verb" {
		t.Errorf("classify(/teleport) = (%v, %q), want (false, /unknown_verb)", mapped, reason)
	}
	if mapped, _ := p.classifyVerbMapping(""); mapped {
		t.Error("classify(\"\") must not map")
	}
}

// AddTask is exported and the kernel arrives later via SetParentKernel: the
// agenda write must land with or without a kernel attached.
func TestPlannerAddTask_WithoutKernel(t *testing.T) {
	p := NewSessionPlannerShard()
	id := p.AddTask("kernel-less task", 1)
	if id == "" {
		t.Fatal("AddTask returned no ID")
	}
	if agenda := p.GetAgenda(); len(agenda) != 1 || agenda[0].Description != "kernel-less task" {
		t.Fatalf("agenda = %v, want the one task", agenda)
	}
}
