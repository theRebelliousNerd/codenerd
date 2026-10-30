package perception

import (
	"strings"
	"testing"

	"codenerd/internal/types"
)

// A side turn's perception reads what the running work has done, so its reply
// about the work is grounded instead of a promise to check on it.
func TestBuildPrompt_CarriesTheRunningWork(t *testing.T) {
	progress := "Running: coder: fix README.md, for 22m0s.\n- coder-7, running 22m0s; 3 model call(s); 9 tool result(s); last tool edit_file"
	got := (&LLMTransducer{}).BuildPrompt("how is progress?", nil, nil, &types.SessionContext{RunningWork: progress}, "")
	if !strings.Contains(got, "## Running Work") || !strings.Contains(got, progress) {
		t.Fatalf("the prompt does not carry the running work:\n%s", got)
	}
	if strings.Index(got, "## Running Work") > strings.Index(got, "## Current Request") {
		t.Fatal("the running work comes after the request it grounds")
	}
	if plain := (&LLMTransducer{}).BuildPrompt("hi", nil, nil, &types.SessionContext{}, ""); strings.Contains(plain, "## Running Work") {
		t.Fatal("a turn with nothing running got a Running Work section")
	}
}
