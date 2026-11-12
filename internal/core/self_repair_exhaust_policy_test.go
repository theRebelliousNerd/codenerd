package core

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"codenerd/internal/problemlog"
	"codenerd/internal/types"
)

// A failed fix turn's own log lines -- a provider timeout under the coder, a
// line about the attempt's edits -- are the record of that attempt, not new
// defects: a problem logged only inside fix windows (however many) is
// attributed to the attempt and is not due. A problem also logged at any
// second outside every window is due as before. Observed live 2026-11-12:
// the four "hot problems" picked right after a failed fix turn were that
// turn's own lines.
func TestSelfRepair_ProblemsLoggedInsideAFixTurnAreTheAttemptsNotDue(t *testing.T) {
	dir := t.TempDir()
	log := filepath.Join(dir, "20261112_100000_problems.log")
	lines := "" +
		"2026/11/12 09:00:00.000 [ERROR] [store] before any fix: the index is corrupt\n" +
		"2026/11/12 10:00:05.000 [ERROR] [api] [meta] CompleteWithSystem failed after 120s: context deadline exceeded\n" +
		"2026/11/12 10:00:20.000 [ERROR] [session] the attempt and later: nil session in dispatch\n" +
		"2026/11/12 10:00:30.000 [ERROR] [api] [meta] CompleteWithSystem failed after 120s: context deadline exceeded\n" +
		"2026/11/12 10:05:00.000 [ERROR] [session] the attempt and later: nil session in dispatch\n" +
		"2026/11/12 10:10:10.000 [ERROR] [api] [meta] CompleteWithSystem failed after 120s: context deadline exceeded\n"
	if err := os.WriteFile(log, []byte(lines), 0o600); err != nil {
		t.Fatal(err)
	}
	rep, err := problemlog.ReadFile(log, problemlog.Options{})
	if err != nil {
		t.Fatal(err)
	}
	seconds := map[string][]int64{}
	for _, p := range rep.Problems {
		seconds[p.Signature] = p.Seconds
	}
	var timeout, nilSession, before string
	for sig, secs := range seconds {
		switch len(secs) {
		case 3:
			timeout = sig
		case 2:
			nilSession = sig
		case 1:
			before = sig
		}
	}
	if timeout == "" || nilSession == "" || before == "" {
		t.Fatalf("fixture grouped as %v", seconds)
	}
	// Two fix turns: one from 10:00 to 10:01, one from 10:10 to 10:11.
	first := seconds[timeout][0] - 5
	ledger := emptyLedger(t)
	for _, w := range []struct {
		job   string
		start int64
	}{{"self-repair-1", first}, {"self-repair-2", first + 600}} {
		if err := ledger.OpenFixWindow(w.job, time.Unix(w.start, 0)); err != nil {
			t.Fatal(err)
		}
		if err := ledger.CloseFixWindow(w.job, time.Unix(w.start+60, 0)); err != nil {
			t.Fatal(err)
		}
	}

	k := selfRepairKernel(t, nil, ledger, nil, time.Unix(first+3600, 0))
	if err := problemlog.Replace(k, rep); err != nil {
		t.Fatal(err)
	}
	due := dueByLane(t, k)[problemlog.LaneRepair]
	if slices.Contains(due, timeout) {
		t.Fatalf("the timeout logged only inside fix turns is due: %v", due)
	}
	if !slices.Contains(due, nilSession) || !slices.Contains(due, before) {
		t.Fatalf("due = %v, want the problem that recurred outside a fix turn (%q) and the one from before (%q)", due, nilSession, before)
	}
	rows, err := k.Query("self_problem_from_repair")
	if err != nil {
		t.Fatal(err)
	}
	var jobs []string
	for _, r := range rows {
		if types.ExtractString(r.Args[0]) != timeout {
			t.Fatalf("attributed to an attempt: %v, want only the timeout", r)
		}
		jobs = append(jobs, types.ExtractString(r.Args[1]))
	}
	slices.Sort(jobs)
	if !slices.Equal(jobs, []string{"self-repair-1", "self-repair-2"}) {
		t.Fatalf("the timeout is attributed to %v, want both attempts", jobs)
	}

	// Without the windows nothing is attributed: the same log is all due.
	bare := selfRepairKernel(t, nil, nil, nil, time.Unix(first+3600, 0))
	if err := problemlog.Replace(bare, rep); err != nil {
		t.Fatal(err)
	}
	if got := dueByLane(t, bare)[problemlog.LaneRepair]; !slices.Contains(got, timeout) {
		t.Fatalf("without fix windows the timeout is not due: %v", got)
	}
}

// A window the ledger finds open at load belonged to a process that ended;
// it is closed where it began, so it attributes nothing in this process.
func TestRepairLedger_AnOpenWindowAtLoadAttributesNothing(t *testing.T) {
	l := emptyLedger(t)
	if err := l.OpenFixWindow("self-repair-1", time.Unix(100, 0)); err != nil {
		t.Fatal(err)
	}
	re := reloadLedger(t, l, time.Unix(5000, 0))
	ws := re.FixWindows()
	if len(ws) != 1 || ws[0].EndUnix != 100 {
		t.Fatalf("windows after load = %+v, want the open one closed at its start", ws)
	}
}
