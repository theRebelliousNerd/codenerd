package chat

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"codenerd/internal/logging"
	"codenerd/internal/problemlog"
	"codenerd/internal/session"
)

// A self-repair fix turn leaves the tree as it found it unless it ends
// installed.
//
// Observed live 2026-11-12: the first autonomous fix turn edited
// internal/core/kernel_facts.go and wrote a test the repair loop could not
// make pass; the turn failed, and both files stayed in the tree -- the red
// test included -- for the next self-repair, the user's own work and the next
// rebuild to inherit.
//
// So the fix step runs under a write journal (session.WriteJournal): every
// turn of the fix chain -- the step, its verified delegation's attempts, the
// continuation steps after it -- records what it wrote and what each path held
// before. When the chain ends (settleFixTurn, on the Update thread, before a
// queued message of the user's starts), a fix that was not built, self-checked
// and installed is undone: each path goes back as the chain found it, except
// one another writer changed since, which is left as found and named. The
// undone edits are saved under .nerd/self_repair/attempts/<job>.diff with the
// tests that failed, the ledger note names the file, and a retry is told both
// (writeSelfRepairHistory).
//
// Ruling: only the self-repair fix chain is undone. A turn the user asked for
// that fails verification keeps its edits: the turn is theirs to inspect, and
// its report names what failed. codeNERD's own unasked attempt is not the
// user's work, and a broken tree is the one thing it must not leave behind.

// selfRepairAttemptsDir is where undone fix attempts are kept, per job.
func selfRepairAttemptsDir(workspace string) string {
	return filepath.Join(workspace, ".nerd", "self_repair", "attempts")
}

// beginFixTurn opens the fix chain of step's job: its write journal and its
// fix window (problems logged inside it are the attempt's, self_repair.mg).
//
// Only in yolo, where the fix runs unattended. In any other mode the step is
// proposed or held for the user's go: a fix the user approved is theirs, its
// edits stay for them to inspect, and the turns of conversation around the
// held step are theirs too, so no window spans them.
func (m Model) beginFixTurn(step chainStep) {
	if m.selfRepair == nil || step.Job == "" || !m.yoloEnabled() {
		return
	}
	m.selfRepair.mu.Lock()
	m.selfRepair.journal = session.NewWriteJournal()
	m.selfRepair.journalJob = step.Job
	m.selfRepair.mu.Unlock()
	ledger, err := m.selfRepair.ledgerFor(m.workspace)
	if err != nil {
		return
	}
	now := m.selfRepair.clock()
	if err := ledger.OpenFixWindow(step.Job, now); err != nil {
		logging.Get(logging.CategoryKernel).Error("%s self-repair ledger: %v", selfDebugMarker, err)
	}
	if m.kernel != nil {
		if err := problemlog.ReplaceRepair(m.kernel, ledger, now); err != nil {
			logging.Get(logging.CategoryKernel).Warn("%s self-repair facts: %v", selfDebugMarker, err)
		}
	}
}

// withFixJournal puts the open fix chain's journal on ctx, so the turns that
// run under it record their writes.
func (m Model) withFixJournal(ctx context.Context) context.Context {
	if m.selfRepair == nil {
		return ctx
	}
	m.selfRepair.mu.Lock()
	j := m.selfRepair.journal
	m.selfRepair.mu.Unlock()
	return session.WithWriteJournal(ctx, j)
}

// fixChainRunning reports whether the fix chain may still do work: a turn or
// a continuation step running, a next step offered or held, an approval open.
func (m Model) fixChainRunning() bool {
	return m.isLoading || m.turn.current() != 0 || m.offeredSubtask != nil ||
		m.pendingWork != nil || m.viewMode == ApprovalView
}

// settleFixTurn settles the fix chain once it has stopped running: its
// attempts end stopped (the user stopped it) or failed (anything short of an
// installed fix), its edits are undone unless the fix was installed, and its
// window closes. A chain still running, or none open, is left alone.
func (m Model) settleFixTurn() Model {
	if m.selfRepair == nil || m.fixChainRunning() {
		return m
	}
	m.selfRepair.mu.Lock()
	fixTurn, journal, job := m.selfRepair.fixTurn, m.selfRepair.journal, m.selfRepair.journalJob
	m.selfRepair.fixTurn, m.selfRepair.journal, m.selfRepair.journalJob = 0, nil, ""
	m.selfRepair.mu.Unlock()
	if fixTurn == 0 && journal == nil {
		return m
	}
	log := logging.Get(logging.CategoryKernel)
	ledger, err := m.selfRepair.ledgerFor(m.workspace)
	if err != nil {
		log.Error("%s self-repair ledger unreadable; the fix turn's edits stay in the tree: %v", selfDebugMarker, err)
		return m
	}
	now := m.selfRepair.clock()
	stopped := m.turn.endedStopped(fixTurn)
	installed := jobInstalled(ledger, job)

	var failures []string
	var undoNote, diffPath string
	if journal != nil {
		failures = journal.TestFailures()
		if !installed {
			undo := journal.Undo()
			diffPath = saveFixAttempt(m.workspace, job, undo.Patch, failures)
			undoNote = describeFixUndo(undo, diffPath)
			if undo.Err != nil {
				log.Error("%s undoing the failed fix turn's edits: %v", selfDebugMarker, undo.Err)
			}
		}
	}
	if job != "" {
		if err := ledger.CloseFixWindow(job, now); err != nil {
			log.Error("%s self-repair ledger: %v", selfDebugMarker, err)
		}
	}

	status, note := problemlog.RepairFailed, "the fix turn ended without a verified, rebuilt and self-checked binary"
	if stopped {
		status, note = problemlog.RepairStopped, "its fix turn was stopped by the user"
	}
	var n int
	if job != "" {
		n, err = ledger.EndJob(job, status, note, now)
	} else {
		n, err = ledger.EndFixing(status, note, now)
	}
	if err != nil {
		log.Error("%s self-repair ledger: %v", selfDebugMarker, err)
	}
	if !installed && job != "" && (undoNote != "" || len(failures) > 0) {
		if _, err := ledger.Annotate(job, undoNote, diffPath, failures); err != nil {
			log.Error("%s self-repair ledger: %v", selfDebugMarker, err)
		}
	}
	switch {
	case n > 0 && stopped:
		m = m.pushAssistantMsg(fmt.Sprintf("_Self-repair: you stopped the fix turn for %d target(s); they are recorded as stopped, not failed.%s_", n, sentence(undoNote)))
	case n > 0:
		m = m.pushAssistantMsg(fmt.Sprintf("_Self-repair: the fix turn for %d target(s) ended without installing a fix; they are recorded as failed.%s_", n, sentence(undoNote)))
	case undoNote != "":
		m = m.pushAssistantMsg(fmt.Sprintf("_Self-repair: the fix was not installed.%s_", sentence(undoNote)))
	}
	m.syncSelfRepairFixing(ledger)
	if m.kernel != nil {
		if err := problemlog.ReplaceRepair(m.kernel, ledger, now); err != nil {
			log.Warn("%s self-repair facts: %v", selfDebugMarker, err)
		}
	}
	return m
}

func sentence(s string) string {
	if s == "" {
		return ""
	}
	return " " + strings.ToUpper(s[:1]) + s[1:] + "."
}

// jobInstalled reports whether job's fix was built, self-checked and
// installed: any of its targets waits for the restart or is past it.
func jobInstalled(ledger *problemlog.RepairLedger, job string) bool {
	if job == "" {
		return false
	}
	for _, a := range ledger.Attempts() {
		if a.Job != job {
			continue
		}
		switch a.Status {
		case problemlog.RepairPendingRestart, problemlog.RepairVerifying, problemlog.RepairVerified, problemlog.RepairRegressed:
			return true
		}
	}
	return false
}

// saveFixAttempt writes the undone attempt -- its diff, then the tests that
// were failing when it ended -- under .nerd/self_repair/attempts and returns
// the workspace-relative path, or "" when there was nothing to keep or it
// could not be written (logged: the undo has already happened, and the note
// must not name a file that is not there).
func saveFixAttempt(workspace, job, patch string, failures []string) string {
	if strings.TrimSpace(patch) == "" && len(failures) == 0 {
		return ""
	}
	if job == "" {
		job = "unknown-job"
	}
	var b strings.Builder
	b.WriteString(patch)
	if len(failures) > 0 {
		b.WriteString("\n\nTests failing when the attempt ended:\n")
		for _, f := range failures {
			b.WriteString("- " + f + "\n")
		}
	}
	dir := selfRepairAttemptsDir(workspace)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		logging.Get(logging.CategoryKernel).Error("%s save the undone fix attempt: %v", selfDebugMarker, err)
		return ""
	}
	path := filepath.Join(dir, job+".diff")
	if err := os.WriteFile(path, []byte(b.String()), 0o600); err != nil {
		logging.Get(logging.CategoryKernel).Error("%s save the undone fix attempt: %v", selfDebugMarker, err)
		return ""
	}
	rel, err := filepath.Rel(workspace, path)
	if err != nil {
		return path
	}
	return filepath.ToSlash(rel)
}

// describeFixUndo is the sentence the ledger note and the user get.
func describeFixUndo(undo session.JournalUndo, diffPath string) string {
	var parts []string
	if len(undo.Restored) > 0 {
		parts = append(parts, "its edits were undone ("+strings.Join(undo.Restored, ", ")+" restored as the turn found them)")
	}
	if len(undo.Left) > 0 {
		parts = append(parts, "left as found: "+strings.Join(undo.Left, ", "))
	}
	if undo.Err != nil {
		parts = append(parts, "some could not be restored: "+oneLine(undo.Err.Error()))
	}
	if diffPath != "" {
		parts = append(parts, "the attempt is saved at "+diffPath)
	}
	return strings.Join(parts, "; ")
}
