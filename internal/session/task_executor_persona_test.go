package session

import (
	"strings"
	"testing"
)

// Sweep finding F6: a persona became a verb in three Go tables that
// disagreed -- nemesis ran /attack from chat and /review through this
// executor. The kernel's persona_verb table is the one table, and intentFor
// is its one reader.

// personaExecutor is a JIT executor whose session executor holds a real
// kernel loaded with the policy corpus.
func personaExecutor(t *testing.T) *JITExecutor {
	t.Helper()
	return &JITExecutor{executor: &Executor{kernel: realKernel(t)}}
}

func TestIntentFor_ThePersonaTableIsTheKernels(t *testing.T) {
	j := personaExecutor(t)
	for _, tc := range []struct{ in, want string }{
		{"coder", "/fix"},
		{"/coder", "/fix"},
		{"/Coder", "/fix"},
		{"tester", "/test"},
		{"reviewer", "/review"},
		{"researcher", "/research"},
		{"nemesis", "/attack"},
		{"/nemesis", "/attack"},
		{"generalist", "/implement"},
		{"specialist", "/research"},
		{"tool_generator", "/generate_tool"},
		{"/fix", "/fix"},
		{"/generate_tool", "/generate_tool"},
		{"/consult/rustexpert", "/consult/rustexpert"},
		{"GoExpert", "/goexpert"},
		{"/", "/"},
	} {
		got, err := j.intentFor(tc.in)
		if err != nil {
			t.Fatalf("intentFor(%q): %v", tc.in, err)
		}
		if got != tc.want {
			t.Errorf("intentFor(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
	for _, bad := range []string{"", "  ", "not a verb"} {
		if got, err := j.intentFor(bad); err == nil {
			t.Errorf("intentFor(%q) = %q, want an error", bad, got)
		}
	}
}

// Image generation fails closed, bare or slashed: mapped to /create it would
// run on the worker client.
func TestIntentFor_ImageGenerationFailsClosed(t *testing.T) {
	j := personaExecutor(t)
	for _, verb := range []string{"image_generator", "imagen", "nano_banana", "image", "/image"} {
		if got, err := j.intentFor(verb); err == nil || got != "" {
			t.Errorf("intentFor(%q) = %q, %v; want a fail-closed error", verb, got, err)
		}
	}
}

// A kernel without the table cannot map a name, and says so rather than
// running the persona as a user agent of the same name.
func TestIntentFor_NoPersonaTableIsAnError(t *testing.T) {
	j := &JITExecutor{executor: &Executor{kernel: &MockKernel{}}}
	if got, err := j.intentFor("coder"); err == nil || !strings.Contains(err.Error(), "persona_verb") {
		t.Fatalf("intentFor(coder) on a kernel with no table = %q, %v; want an error naming persona_verb", got, err)
	}
	// A verb the taxonomy knows needs no table.
	if got, err := j.intentFor("/fix"); err != nil || got != "/fix" {
		t.Fatalf("intentFor(/fix) = %q, %v", got, err)
	}
}

// Whether a verb runs as an isolated subagent is the kernel's verb_isolated.
func TestRunsIsolated_IsTheKernels(t *testing.T) {
	j := personaExecutor(t)
	for verb, want := range map[string]bool{"/research": true, "/refactor": true, "/fix": false, "/review": false} {
		got, err := j.runsIsolated(verb)
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Errorf("runsIsolated(%s) = %v, want %v", verb, got, want)
		}
	}
}

// The subagent's label is the taxonomy's shard for the verb, the agent's name
// for /consult/<name>, and "executor" for anything else, whatever the input.
func TestAgentName_FollowsTheTaxonomy(t *testing.T) {
	for in, want := range map[string]string{
		"/fix":                "coder",
		"/test":               "tester",
		"/review":             "reviewer",
		"/research":           "researcher",
		"/consult/rustexpert": "rustexpert",
		"":                    "executor",
		"/":                   "executor",
		"fix":                 "executor",
		"\x00":                "executor",
		"修复":                  "executor",
		"/implement_this_extremely_long_intent_verb_that_nobody_would_ever_use": "executor",
	} {
		if got := agentName(in); got != want {
			t.Errorf("agentName(%q) = %q, want %q", in, got, want)
		}
	}
}
