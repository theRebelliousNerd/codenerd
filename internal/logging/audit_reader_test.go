package logging

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The defect these guard (F-GLASS-1, observed live): `nerd glassbox` and
// `nerd transparency` queried route_to, shard_routing, tool_invocation and
// file_state. Grep proves the only occurrences of those four strings in the
// whole repo are the queries themselves — no Go producer, no rule in the live
// .mg corpus. Both commands printed zeros on every run since they were
// written, which reads as "the system did nothing" rather than "this was never
// wired".
//
// Pointing them at the predicates that DO record decisions would not have
// fixed it: user_intent / next_action / permitted are session-scoped and die
// with the process, so a fresh CLI invocation sees an empty kernel by
// construction. Verified live: `nerd query next_action` returns nothing while
// `nerd query tool_registered` returns the tool Ouroboros registered minutes
// earlier.
//
// The durable record already existed — the audit log, 40 MB on the day this
// was written. Nothing read it.

func writeAuditLog(t *testing.T, lines ...string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "2026-08-08_audit.log")
	body := "# Audit log started\n" + strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write audit log: %v", err)
	}
	return path
}

const (
	spawnLine = `{"ts":1786181585414,"event":"shard_spawn","shard":"coder-1","target":"coder","success":true}`
	execLine  = `{"ts":1786181585484,"event":"shard_execute","shard":"coder-1","target":"build","success":true}`
	doneLine  = `{"ts":1786181585594,"event":"shard_complete","shard":"coder-1","target":"build","success":true,"dur_ms":110}`
	queryLine = `{"ts":1786181585600,"event":"kernel_query","target":"permitted","success":true}`
	perfLine  = `{"ts":1786181585601,"event":"perf_metric","cat":"kernel","action":"Query","success":true}`
)

func TestReadRecentAuditEvents_FiltersByType(t *testing.T) {
	path := writeAuditLog(t, spawnLine, queryLine, perfLine, execLine)

	events, err := ReadRecentAuditEvents(path, []AuditEventType{AuditShardSpawn, AuditShardExecute}, 10)
	if err != nil {
		t.Fatalf("ReadRecentAuditEvents: %v", err)
	}

	if len(events) != 2 {
		t.Fatalf("got %d events, want the 2 shard events out of 4 lines: %+v", len(events), events)
	}
	for _, e := range events {
		if e.EventType != AuditShardSpawn && e.EventType != AuditShardExecute {
			t.Errorf("unrequested event type leaked through: %s", e.EventType)
		}
	}
}

// The log is tens of megabytes a day and dominated by perf_metric and
// kernel_query. Only the newest matches are kept, so memory stays flat.
func TestReadRecentAuditEvents_KeepsNewestWithinLimit(t *testing.T) {
	lines := make([]string, 0, 20)
	for i := range 20 {
		lines = append(lines, `{"ts":`+itoa(1786181585000+i)+`,"event":"shard_spawn","shard":"s`+itoa(i)+`","success":true}`)
	}
	path := writeAuditLog(t, lines...)

	events, err := ReadRecentAuditEvents(path, []AuditEventType{AuditShardSpawn}, 3)
	if err != nil {
		t.Fatalf("ReadRecentAuditEvents: %v", err)
	}

	if len(events) != 3 {
		t.Fatalf("got %d events, want 3", len(events))
	}
	// Oldest first within the retained window: s17, s18, s19.
	if events[0].ShardID != "s17" || events[2].ShardID != "s19" {
		t.Errorf("kept the wrong window: first=%s last=%s, want s17..s19", events[0].ShardID, events[2].ShardID)
	}
}

// A live writer can leave a torn final line. Dropping the whole read because
// of it would make the command fail exactly when the system is busiest.
func TestReadRecentAuditEvents_SkipsMalformedLines(t *testing.T) {
	path := writeAuditLog(t, spawnLine, `{"ts":1786181585500,"event":"shard_ex`, doneLine)

	events, err := ReadRecentAuditEvents(path, nil, 10)
	if err != nil {
		t.Fatalf("a torn line failed the whole read: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("got %d events, want the 2 intact ones", len(events))
	}
}

// Empty type list means "everything", which is what the count-free callers want.
func TestReadRecentAuditEvents_NoTypesMatchesAll(t *testing.T) {
	path := writeAuditLog(t, spawnLine, queryLine, perfLine)

	events, err := ReadRecentAuditEvents(path, nil, 10)
	if err != nil {
		t.Fatalf("ReadRecentAuditEvents: %v", err)
	}
	if len(events) != 3 {
		t.Errorf("got %d events, want all 3", len(events))
	}
}

// A missing log must be distinguishable from an empty one, or the caller
// cannot tell "debug mode is off" from "nothing happened" — which is the exact
// confusion this whole fix exists to remove.
func TestReadRecentAuditEvents_MissingFileIsDistinguishable(t *testing.T) {
	_, err := ReadRecentAuditEvents(filepath.Join(t.TempDir(), "absent.log"), nil, 5)
	if err == nil {
		t.Fatal("reading an absent log returned no error")
	}
	if err != ErrNoAuditLog {
		t.Errorf("got %v, want ErrNoAuditLog so the caller can explain the cause", err)
	}
}

// CountAuditEventTypes is what lets the command say "not instrumented" instead
// of "none recorded" for families with no production call site.
func TestCountAuditEventTypes_TalliesEveryFamily(t *testing.T) {
	path := writeAuditLog(t, spawnLine, execLine, doneLine, queryLine, perfLine, spawnLine)

	counts, err := CountAuditEventTypes(path)
	if err != nil {
		t.Fatalf("CountAuditEventTypes: %v", err)
	}

	if counts[AuditShardSpawn] != 2 {
		t.Errorf("shard_spawn = %d, want 2", counts[AuditShardSpawn])
	}
	if counts[AuditKernelQuery] != 1 {
		t.Errorf("kernel_query = %d, want 1", counts[AuditKernelQuery])
	}
	// The families with no production call site must be absent, not zero-valued
	// by accident of some default.
	if _, present := counts[AuditToolInvoke]; present {
		t.Error("tool_invoke counted despite never appearing in the log")
	}
}

func TestLatestAuditLogPath_PicksTheNewestByName(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"2026-08-06_audit.log", "2026-08-08_audit.log", "2026-08-07_audit.log"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("#\n"), 0o600); err != nil {
			t.Fatalf("seed %s: %v", name, err)
		}
	}

	configMu.Lock()
	saved := logsDir
	logsDir = dir
	configMu.Unlock()
	t.Cleanup(func() {
		configMu.Lock()
		logsDir = saved
		configMu.Unlock()
	})

	path, err := LatestAuditLogPath()
	if err != nil {
		t.Fatalf("LatestAuditLogPath: %v", err)
	}
	if filepath.Base(path) != "2026-08-08_audit.log" {
		t.Errorf("got %s, want the newest log", filepath.Base(path))
	}
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b []byte
	for i > 0 {
		b = append([]byte{byte('0' + i%10)}, b...)
		i /= 10
	}
	return string(b)
}
