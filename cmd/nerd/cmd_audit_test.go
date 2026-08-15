package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/logging"
	"codenerd/internal/mangle"
)

func TestAuditFactsCmd_WhenGivenAuditLog_ShouldWriteLoadableFactsFile(t *testing.T) {
	dir := t.TempDir()
	auditLog := filepath.Join(dir, "run_audit.log")
	lines := strings.Join([]string{
		`# Audit log started`,
		`{"event":"safety_allow","mangle":"safety_check(1, /safety_allow, \"/edit main.go\", /true)."}`,
		`{"event":"safety_block","mangle":"safety_check(2, /safety_block, \"/shell rm\", /false)."}`,
		`{"event":"safety_block","mangle":"safety_check(2, /safety_block, \"/shell rm\", /false)."}`,
	}, "\n")
	if err := os.WriteFile(auditLog, []byte(lines+"\n"), 0o600); err != nil {
		t.Fatalf("write audit log: %v", err)
	}

	out := filepath.Join(dir, "facts.mg")
	// Drive the leaf command directly rather than auditCmd.Execute().
	//
	// Cobra's Execute() runs c.Root(), and auditCmd is registered on rootCmd —
	// so auditCmd.Execute() actually ran the ROOT command with args
	// ["facts", ...], which matches no root subcommand and falls through to
	// rootCmd's RunE: the interactive chat, which then fails with "could not
	// open a new TTY". The test only passed while auditCmd was unregistered.
	auditFactsLog = auditLog
	auditFactsOut = out
	auditFactsEvents = nil
	defer func() {
		auditFactsOut, auditFactsLog, auditFactsEvents = "", "", nil
	}()
	if err := auditFactsCmd.RunE(auditFactsCmd, nil); err != nil {
		t.Fatalf("audit facts: %v", err)
	}

	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read facts file: %v", err)
	}
	got := string(data)
	if !strings.Contains(got, "Decl safety_check(Arg1, Arg2, Arg3, Arg4).") {
		t.Errorf("missing Decl in export:\n%s", got)
	}
	if n := strings.Count(got, "/safety_block"); n != 1 {
		t.Errorf("duplicate fact was not collapsed (%d occurrences):\n%s", n, got)
	}

	// The point of the export is that the file LOADS. Parse it with the same
	// parser the kernel uses — a facts file that only looks like Mangle is
	// worth nothing to the forensic query it exists for. (This check lives in
	// cmd/nerd because internal/mangle imports internal/logging, so the
	// producing package cannot import the parser.)
	if _, err := mangle.ParseUnit(strings.NewReader(got)); err != nil {
		t.Errorf("exported facts file does not parse as Mangle: %v\n%s", err, got)
	}
}

func TestAuditPlaybookCmd_WhenRun_ShouldPointAtTheLoggingCorpus(t *testing.T) {
	if !strings.Contains(loggingPlaybook, "Docs/architecture/logging/") {
		t.Error("the playbook must point operators at the corpus")
	}
	for _, key := range []string{"debug_mode", "trace_llm_io", "trace_llm_io_raw", "max_log_file_mb", "problems.log"} {
		if !strings.Contains(loggingPlaybook, key) {
			t.Errorf("playbook does not mention %q", key)
		}
	}
}

// TestAuditFacts_WhenEveryEventFamilyRecorded_ShouldParseAsMangle exercises
// each generateMangleFact branch through the public audit API and parses the
// export with the real Mangle parser. Before this, the fact strings were
// write-only: nothing loaded them back, so `%v` booleans (bare `true`, which
// Mangle has no literal for) and unescaped targets survived indefinitely.
func TestAuditFacts_WhenEveryEventFamilyRecorded_ShouldParseAsMangle(t *testing.T) {
	ws := t.TempDir()
	if err := os.MkdirAll(filepath.Join(ws, ".nerd"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	cfg := `{"logging":{"debug_mode":true,"level":"debug"}}`
	if err := os.WriteFile(filepath.Join(ws, ".nerd", "config.json"), []byte(cfg), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	if err := logging.Initialize(ws); err != nil {
		t.Fatalf("logging.Initialize: %v", err)
	}
	t.Cleanup(func() {
		logging.CloseAll()
		// Leave the process-global logger inert for the rest of this binary:
		// the temp workspace is about to disappear.
		logging.ApplyConfig(logging.Config{DebugMode: false})
	})

	a := logging.Audit()
	a.ShardSpawn("shard-1", "coder")
	a.ShardExecute("shard-1", "write \"main.go\"")
	a.ShardComplete("shard-1", "task", 12, false, "boom")
	a.ActionRoute("/edit", "main.go")
	a.ActionComplete("/edit", "main.go", 5, true, "")
	a.KernelAssert("user_intent", 3)
	a.KernelQuery("permitted", 2, 7)
	a.LLMCall("claude-opus-4", 500, 1500, true, "")
	a.FileOp(logging.AuditFileWrite, "a\"b.go", 1024, true, "")
	a.IntentParsed("mutation", "refactor", "auth.go", 0.95)
	a.SafetyCheck("/edit main.go", true, "permitted")
	a.SafetyCheck("/shell rm -rf /", false, "denied")
	a.PerfMetric("kernel.rebuild", 5000, 100)
	a.Error("kernel", errors.New("panic\nin evaluation"), true)
	a.SessionStart("sess-1")
	a.TurnStart("sess-1", 1, 42)
	a.TurnEnd("sess-1", 1, 99, true)
	a.SessionEnd("sess-1", 1, 120)
	a.ToolExec("grep", "search", 3, false, "no match")
	a.CampaignEvent(logging.AuditCampaignStart, "camp-1", "scaffold", true)
	a.LearningEvent(logging.AuditToolGenerated, "shard-2", "tool_x", true)
	a.Log(logging.AuditEvent{EventType: "custom_unmapped", Category: "kernel", Message: "fallback \"quoted\" msg"})
	logging.CloseAll()

	path, err := logging.LatestAuditLogPath()
	if err != nil {
		t.Fatalf("LatestAuditLogPath: %v", err)
	}
	var buf bytes.Buffer
	stats, err := logging.ExportAuditFacts(path, &buf, nil)
	if err != nil {
		t.Fatalf("ExportAuditFacts: %v", err)
	}
	if stats.Facts < 20 {
		t.Fatalf("expected every family to export, got %d facts", stats.Facts)
	}
	if _, err := mangle.ParseUnit(strings.NewReader(buf.String())); err != nil {
		t.Errorf("audit facts do not parse as Mangle: %v\n%s", err, buf.String())
	}
}
