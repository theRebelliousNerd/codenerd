package chat

import (
	"context"
	"strings"
	"testing"

	"codenerd/internal/campaign"
)

var recurseTestKnown = map[string]bool{"session": true, "cli": true, "wiring": true, "review": true, "bench": true}

func TestParseRecurseArgs(t *testing.T) {
	cfg, err := parseRecurseArgs(nil, recurseTestKnown)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MaxWaves != 1 || len(cfg.Subsystems) != 0 {
		t.Fatalf("defaults wrong: %+v", cfg)
	}

	cfg, err = parseRecurseArgs([]string{"--waves", "3", "--subsystem", "session"}, recurseTestKnown)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MaxWaves != 3 || len(cfg.Subsystems) != 1 || cfg.Subsystems[0] != "session" {
		t.Fatalf("parsed wrong: %+v", cfg)
	}

	// Bare tokens name subsystems.
	cfg, err = parseRecurseArgs([]string{"session", "cli"}, recurseTestKnown)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Subsystems) != 2 {
		t.Fatalf("bare subsystems wrong: %+v", cfg.Subsystems)
	}

	for _, args := range [][]string{
		{"--waves", "x"},
		{"--waves", "-1"},
		{"--angles", "harden"},
		{"--nope"},
		{"sauron"},
		{"--subsystem", "sauron"},
	} {
		if _, err := parseRecurseArgs(args, recurseTestKnown); err == nil {
			t.Errorf("%v must fail", args)
		}
	}
}

func runningRecurse(t *testing.T) (Model, *bool) {
	t.Helper()
	m := NewTestModel()
	m.workspace = t.TempDir()
	cancelled := false
	m.recurse = &recurseState{cancel: func() { cancelled = true }, lines: make(chan string, 4)}
	return m, &cancelled
}

func lastContent(m Model) string {
	if len(m.history) == 0 {
		return ""
	}
	return m.history[len(m.history)-1].Content
}

// /recurse stop and /campaign pause both cancel the loop; the loop reverts an
// attempt in flight and reports back through recurseFinishedMsg.
func TestRecurse_StopAndPauseCancelTheLoop(t *testing.T) {
	for _, cmd := range [][]string{{"/campaign", "recurse", "stop"}, {"/campaign", "pause"}} {
		m, cancelled := runningRecurse(t)
		updated, _ := m.handleCampaignCommand(strings.Join(cmd, " "), cmd)
		result := updated.(Model)
		if !*cancelled {
			t.Fatalf("%v must cancel the loop", cmd)
		}
		if !strings.Contains(lastContent(result), "reverted") {
			t.Fatalf("%v must say what stopping does: %q", cmd, lastContent(result))
		}
	}

	m := NewTestModel()
	updated, _ := m.handleCampaignCommand("/campaign recurse stop", []string{"/campaign", "recurse", "stop"})
	if got := lastContent(updated.(Model)); !strings.Contains(got, "No recurse loop") {
		t.Fatalf("stop with nothing running: %q", got)
	}
}

// Progress lines stream into the conversation until the loop finishes; the
// finish clears the loop and reports the result, and a stop is not an error.
func TestRecurse_MessagesStreamAndFinish(t *testing.T) {
	m, _ := runningRecurse(t)
	lines := m.recurse.lines
	next, cmd, ok := m.handleRecurseMsg(recurseLineMsg{line: "recurse cycle 1: kept", lines: lines})
	if !ok || cmd == nil || lastContent(next) != "recurse cycle 1: kept" {
		t.Fatalf("a line is shown and the listener re-armed: ok=%v cmd=%v last=%q", ok, cmd != nil, lastContent(next))
	}

	res := &campaign.RecurseCycleResult{Passes: 1, Cycles: 2, Kept: 1, Reverted: 1}
	done, _, _ := next.handleRecurseMsg(recurseFinishedMsg{result: res, err: context.Canceled})
	if done.recurse != nil {
		t.Fatal("a finished loop is cleared")
	}
	if got := lastContent(done); !strings.Contains(got, "1 kept, 1 reverted") || strings.Contains(got, "error") {
		t.Fatalf("finish summary = %q", got)
	}
}

func TestRecurseLineWriter_SplitsAndFlushes(t *testing.T) {
	ch := make(chan string, 8)
	w := &recurseLineWriter{lines: ch}
	for _, chunk := range []string{"first line\nsec", "ond line\n\n", "tail"} {
		if _, err := w.Write([]byte(chunk)); err != nil {
			t.Fatal(err)
		}
	}
	w.flush()
	close(ch)
	var got []string
	for l := range ch {
		got = append(got, l)
	}
	if strings.Join(got, "|") != "first line|second line|tail" {
		t.Fatalf("lines = %q", got)
	}
}
