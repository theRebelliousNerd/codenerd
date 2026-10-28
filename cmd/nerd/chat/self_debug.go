package chat

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"codenerd/internal/config"
	"codenerd/internal/core"
	"codenerd/internal/logging"
	"codenerd/internal/problemlog"
	"codenerd/internal/types"
)

// /debug-self turns codenerd on its own logs (stage 1: reporting only).
// Persisted as logging.debug_self in .nerd/config.json. While on, a session
// implies debug logging (no log files exist without it) and session-level
// yolo via yoloFlag — work_mode yolo is never persisted because of debug_self.
//
// Signal scope: new ERROR lines are the actionable set; new WARN lines are a
// deduped digest (one entry per normalized signature, with counts). Stage 2
// (a background fixer shard) must consume the same batch via
// pendingSelfProblemsForTurn; nothing here spawns shards or edits code.
//
// Main-context ruling (user steering): what enters the MAIN chat model's
// context is strictly delta-only and deduped by signature under a hard
// small cap — counts, signatures, first/last seen, and a problems-log
// path + byte-offset reference. Full raw lines never enter the main
// context; they stay in PendingProblems/the file range for stage 2.
// Signatures already reported in earlier turns collapse to one tally line.
//
// Evidence in the kernel: separately from that digest, the whole problems log
// is projected into the kernel as self_problem facts (internal/problemlog,
// policy/self_problems.mg) and debug_self_active() is asserted while debug-self
// is on, so the main agent and shards can query their own failures and hot
// problems reach their context as injectable_context lines.

// selfDebugMarker prefixes every user-visible debug-self note. The watcher
// drops problem lines carrying it, so renderSystemWarnings logging our own
// note at WARN cannot feed back as a "new" warning next turn.
const selfDebugMarker = "debug-self:"

const (
	maxSelfDrainBytes = 512 * 1024
	maxSelfPendingRaw = 500
	// Hard caps for the MAIN-context digest: total bytes and per-signature.
	maxSelfRenderChars    = 600
	maxSelfSignatureChars = 80
)

// SelfProblem is one parsed `[LEVEL] [category] msg` problems-log line.
type SelfProblem struct {
	Level    string // "ERROR" or "WARN"
	Category string
	Message  string
	SeenAt   string // log-line timestamp prefix, "" when absent
}

// SelfProblemKind is one deduped signature with counts and first/last seen.
// Example holds a raw line for stage 2; it is never rendered to main context.
type SelfProblemKind struct {
	Signature string
	Category  string
	Example   string
	Count     int
	FirstSeen string
	LastSeen  string
	// Repeat marks a signature already reported in an earlier turn.
	Repeat bool
}

// SelfWarnKind is the warning flavour of SelfProblemKind.
type SelfWarnKind = SelfProblemKind

// PendingProblems is the new-problem set for one turn: the stage-2 seam.
// Errors/WarnKinds carry raw lines for a fixer; the main chat model only
// ever sees Render()'s capped digest. [StartOffset, EndOffset) is this
// batch's byte range in Path, so stage 2 can read exactly its batch.
type PendingProblems struct {
	Path          string
	StartOffset   int64
	EndOffset     int64
	Errors        []SelfProblem
	ErrorKinds    []SelfProblemKind
	WarnKinds     []SelfWarnKind
	TotalErrors   int
	TotalWarnings int
	// Dropped counts parsed lines beyond the pending cap; MoreInFile means
	// the per-turn byte cap left unread bytes for the next turn.
	Dropped    int
	MoreInFile bool
}

// SelfProblemStatus names a batch's lifecycle for stage 2. Stage 1 only
// produces "reported"; a fixer shard would advance a batch through the
// rest. Status lives outside PendingProblems until stage 2 needs it.
type SelfProblemStatus string

const (
	SelfProblemReported                  SelfProblemStatus = "reported"
	SelfProblemBeingFixed                SelfProblemStatus = "being-fixed"
	SelfProblemFixedOnDiskPendingRestart SelfProblemStatus = "fixed-on-disk-pending-restart"
)

// Empty reports whether there is nothing new to inject or announce.
func (p PendingProblems) Empty() bool {
	return p.TotalErrors == 0 && p.TotalWarnings == 0 && p.Dropped == 0
}

// SummaryLine is the one visible note per turn, via renderSystemWarnings.
func (p PendingProblems) SummaryLine() string {
	return fmt.Sprintf("%s %d new error(s), %d warning kind(s) since last turn — in context",
		selfDebugMarker, p.TotalErrors, len(p.WarnKinds))
}

// Render is the MAIN-context articulation section: strictly delta-only,
// deduped by signature, hard-capped at maxSelfRenderChars. No raw lines —
// the batch is referenced by problems-log path + byte-offset range, and
// signatures reported in earlier turns collapse to a single tally line.
// Empty when there is nothing new.
func (p PendingProblems) Render() string {
	if p.Empty() {
		return ""
	}
	var lines []string
	lines = append(lines, "## Debug-Self: New codeNERD Log Problems")
	lines = append(lines, "codeNERD's own new log problems since your last turn (debug-self). ERROR signatures are actionable; raw lines are only in the ref range — read them there if needed.")
	if p.Path != "" {
		lines = append(lines, fmt.Sprintf("ref: `%s` bytes %d..%d", p.Path, p.StartOffset, p.EndOffset))
	}
	lines = append(lines, fmt.Sprintf("new: %d error line(s)/%d signature(s), %d warning line(s)/%d signature(s)",
		p.TotalErrors, len(p.ErrorKinds), p.TotalWarnings, len(p.WarnKinds)))

	type entry struct {
		level string
		kind  SelfProblemKind
	}
	var fresh []entry
	repeatKinds, repeatLines := 0, 0
	for _, k := range p.ErrorKinds {
		if k.Repeat {
			repeatKinds++
			repeatLines += k.Count
		} else {
			fresh = append(fresh, entry{"E", k})
		}
	}
	for _, k := range p.WarnKinds {
		if k.Repeat {
			repeatKinds++
			repeatLines += k.Count
		} else {
			fresh = append(fresh, entry{"W", k})
		}
	}

	out := strings.Join(lines, "\n") + "\n"
	shown := 0
	for _, e := range fresh {
		line := fmt.Sprintf("- %s [%s] %s x%d %s..%s\n", e.level, e.kind.Category,
			truncateSelfSignature(e.kind.Signature), e.kind.Count,
			shortSelfSeen(e.kind.FirstSeen), shortSelfSeen(e.kind.LastSeen))
		// Reserve room for the repeat/overflow tallies.
		if len(out)+len(line) > maxSelfRenderChars-60 {
			break
		}
		out += line
		shown++
	}
	if rest := len(fresh) - shown; rest > 0 {
		out += fmt.Sprintf("+ %d more new signature(s) in ref range\n", rest)
	}
	if repeatKinds > 0 {
		out += fmt.Sprintf("repeats of already-reported: %d signature(s), %d line(s)\n", repeatKinds, repeatLines)
	}
	if p.Dropped > 0 || p.MoreInFile {
		out += fmt.Sprintf("[truncated: dropped=%d moreInFile=%v]\n", p.Dropped, p.MoreInFile)
	}
	// The digest is bounded by design (the raw lines stay in the problems
	// log at the recorded offset); the cut is rune-safe.
	return types.Ellipsize(out, maxSelfRenderChars)
}

func truncateSelfSignature(s string) string {
	return types.Ellipsize(strings.TrimSpace(s), maxSelfSignatureChars)
}

// shortSelfSeen reduces a "2006/01/02 15:04:05" prefix to its clock part.
func shortSelfSeen(s string) string {
	if len(s) >= 19 {
		return s[11:19]
	}
	if s == "" {
		return "?"
	}
	return s
}

// =============================================================================
// WATCHER
// =============================================================================

// selfDebugWatcher tracks a byte offset into the session problems log.
// It is held by pointer on Model so the offset survives Model value copies.
// collect() drains new complete lines into pending without consuming the
// batch; consume() clears it once a turn actually delivered it, so a turn
// that exits before articulation never loses problems.
type selfDebugWatcher struct {
	mu         sync.Mutex
	path       string
	offset     int64
	pending    []SelfProblem
	dropped    int
	moreInFile bool
	// [batchStart, batchEnd) is the pending batch's byte range in path.
	batchStart int64
	batchEnd   int64
	// reported holds signatures delivered in earlier turns (level|cat|sig
	// -> lines delivered), so recurrences collapse to a tally in Render.
	reported map[string]int
	// projectedPath/projectedOffset mark the log position the kernel's
	// self_problem facts were last built from; projected is false until the
	// first projection after prime.
	projected       bool
	projectedPath   string
	projectedOffset int64
}

func newSelfDebugWatcher() *selfDebugWatcher {
	return &selfDebugWatcher{reported: make(map[string]int)}
}

func selfKindKey(level, category, signature string) string {
	return level + "|" + category + "|" + signature
}

// prime positions the watcher at the current end of path ("" = no file yet),
// so only problems logged after activation are reported.
func (w *selfDebugWatcher) prime(path string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.path = path
	w.offset = 0
	w.pending = nil
	w.dropped = 0
	w.moreInFile = false
	w.reported = make(map[string]int)
	w.projected = false
	if path != "" {
		if fi, err := os.Stat(path); err == nil {
			w.offset = fi.Size()
		}
	}
	w.batchStart = w.offset
	w.batchEnd = w.offset
}

// consume marks the pending batch delivered: its signatures become
// already-reported (later recurrences collapse to a tally) and the next
// batch's byte range starts at the current offset.
func (w *selfDebugWatcher) consume() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.reported == nil {
		w.reported = make(map[string]int)
	}
	for _, p := range w.pending {
		w.reported[selfKindKey(p.Level, p.Category, problemlog.Signature(p.Message))]++
	}
	w.pending = nil
	w.dropped = 0
	w.batchStart = w.offset
	w.batchEnd = w.offset
}

func (w *selfDebugWatcher) collect(path string) PendingProblems {
	w.mu.Lock()
	defer w.mu.Unlock()

	if path != "" && path != w.path {
		// First sight, rotation, or a new run's file: read it from 0.
		w.path = path
		w.offset = 0
		if len(w.pending) == 0 {
			w.batchStart = 0
			w.batchEnd = 0
		}
	}
	if w.path == "" {
		return w.buildLocked()
	}
	f, err := os.Open(w.path)
	if err != nil {
		return w.buildLocked()
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return w.buildLocked()
	}
	size := fi.Size()
	if size < w.offset {
		w.offset = 0 // truncated/rotated in place
		if len(w.pending) == 0 {
			w.batchStart = 0
		}
	}
	if len(w.pending) == 0 && w.dropped == 0 {
		w.batchStart = w.offset
	}
	if size == w.offset {
		w.moreInFile = false
		return w.buildLocked()
	}
	toRead := size - w.offset
	capped := false
	if toRead > maxSelfDrainBytes {
		toRead = maxSelfDrainBytes
		capped = true
	}
	buf := make([]byte, toRead)
	n, err := f.ReadAt(buf, w.offset)
	if err != nil && n == 0 {
		return w.buildLocked()
	}
	buf = buf[:n]
	// Consume complete lines only, unless the byte cap forces a cut.
	consumed := len(buf)
	if last := strings.LastIndexByte(string(buf), '\n'); last >= 0 {
		consumed = last + 1
	} else if !capped {
		return w.buildLocked() // one partial line: wait for its newline
	}
	w.offset += int64(consumed)
	w.batchEnd = w.offset
	w.moreInFile = capped && w.offset < size
	// A chunk read from the middle of a multi-line message starts with
	// continuation lines; Scan ignores them.
	_ = problemlog.Scan(strings.NewReader(string(buf[:consumed])), isSelfNote, func(e problemlog.Entry) {
		if len(w.pending) >= maxSelfPendingRaw {
			w.dropped++
			return
		}
		w.pending = append(w.pending, SelfProblem{
			Level: e.Level, Category: e.Category, Message: e.Message,
			SeenAt: e.Time.Format(selfSeenLayout),
		})
	})
	return w.buildLocked()
}

func (w *selfDebugWatcher) buildLocked() PendingProblems {
	out := PendingProblems{
		Path: w.path, StartOffset: w.batchStart, EndOffset: w.batchEnd,
		Dropped: w.dropped, MoreInFile: w.moreInFile,
	}
	errIndex := make(map[string]int)
	warnIndex := make(map[string]int)
	addKind := func(kinds *[]SelfProblemKind, index map[string]int, p SelfProblem) {
		sig := problemlog.Signature(p.Message)
		key := selfKindKey(p.Level, p.Category, sig)
		if i, seen := index[key]; seen {
			(*kinds)[i].Count++
			(*kinds)[i].LastSeen = p.SeenAt
			return
		}
		index[key] = len(*kinds)
		_, repeat := w.reported[key]
		*kinds = append(*kinds, SelfProblemKind{
			Signature: sig, Category: p.Category, Example: p.Message,
			Count: 1, FirstSeen: p.SeenAt, LastSeen: p.SeenAt, Repeat: repeat,
		})
	}
	for _, p := range w.pending {
		switch p.Level {
		case "ERROR":
			out.Errors = append(out.Errors, p)
			out.TotalErrors++
			addKind(&out.ErrorKinds, errIndex, p)
		case "WARN":
			out.TotalWarnings++
			addKind(&out.WarnKinds, warnIndex, p)
		}
	}
	return out
}

// project reads the whole problems log when it has moved since the kernel's
// facts were last built from it. It returns a nil report when it has not, or
// when there is no log yet. The caller replaces the facts wholesale, so the
// report is the entire log, not the pending delta.
func (w *selfDebugWatcher) project() (*problemlog.Report, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.path == "" {
		return nil, nil
	}
	if w.projected && w.projectedPath == w.path && w.projectedOffset == w.offset {
		return nil, nil
	}
	rep, err := problemlog.ReadFile(w.path, problemlog.Options{Skip: isSelfNote})
	if err != nil {
		return nil, err
	}
	w.projected, w.projectedPath, w.projectedOffset = true, w.path, w.offset
	return rep, nil
}

// forgetProjection makes the next project re-read the log.
func (w *selfDebugWatcher) forgetProjection() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.projected = false
}

// selfSeenLayout is the log-line timestamp prefix SelfProblem.SeenAt keeps.
const selfSeenLayout = "2006/01/02 15:04:05"

// isSelfNote drops the lines the reporting itself generates (our visible note
// is re-logged at WARN by renderSystemWarnings).
func isSelfNote(e problemlog.Entry) bool {
	return strings.Contains(e.Message, selfDebugMarker)
}

// =============================================================================
// TURN SEAM (stage 2 consumes this same batch)
// =============================================================================

func (m Model) debugSelfEnabled() bool {
	return m.Config != nil && m.Config.DebugSelfMode()
}

// pendingSelfProblemsForTurn returns the pending new-problem set for a turn:
// lines added to the session problems log since the last delivered batch.
// It drains the file (advancing the watcher offset) but does not consume
// the batch; call consumeSelfProblems once the batch is delivered.
func pendingSelfProblemsForTurn(m Model) PendingProblems {
	if !m.debugSelfEnabled() || m.selfDebug == nil {
		return PendingProblems{}
	}
	path, _, _ := logging.ProblemsSnapshot()
	pending := m.selfDebug.collect(path)
	m.syncSelfProblemFacts()
	return pending
}

// syncSelfProblemFacts mirrors /debug-self into the kernel. While on, it keeps
// debug_self_active() asserted (the gate on injecting hot problems) and
// replaces the self_problem facts with the whole problems log whenever the
// log has moved since the last projection; off, it retracts both. Reads and
// failures are logged under selfDebugMarker so the watcher does not report
// its own note as a new problem. A nil kernel (pre-boot toggle) is fine: the
// next turn's collect syncs once the kernel exists.
func (m Model) syncSelfProblemFacts() {
	if m.kernel == nil {
		return
	}
	log := logging.Get(logging.CategoryKernel)
	if !m.debugSelfEnabled() {
		if err := m.kernel.Retract(problemlog.DebugSelfActivePredicate); err != nil {
			log.Warn("%s retract %s: %v", selfDebugMarker, problemlog.DebugSelfActivePredicate, err)
		}
		if err := problemlog.Replace(m.kernel, nil); err != nil {
			log.Warn("%s retract self problems: %v", selfDebugMarker, err)
		}
		if m.selfDebug != nil {
			m.selfDebug.forgetProjection()
		}
		return
	}
	if m.selfDebug == nil {
		return
	}
	if err := config.EnsureParams(m.kernel, m.Config.Logging.Params()); err != nil {
		log.Warn("%s assert logging params: %v", selfDebugMarker, err)
	}
	if rows, err := m.kernel.Query(problemlog.DebugSelfActivePredicate); err != nil || len(rows) == 0 {
		if err := m.kernel.Assert(core.Fact{Predicate: problemlog.DebugSelfActivePredicate}); err != nil {
			log.Warn("%s assert %s: %v", selfDebugMarker, problemlog.DebugSelfActivePredicate, err)
		}
	}
	rep, err := m.selfDebug.project()
	if err != nil {
		log.Warn("%s read problems log: %v", selfDebugMarker, err)
		return
	}
	if rep == nil {
		return
	}
	if err := problemlog.Replace(m.kernel, rep); err != nil {
		log.Warn("%s assert self problems: %v", selfDebugMarker, err)
		m.selfDebug.forgetProjection()
		return
	}
	if rep.Omitted() {
		log.Info("%s self problems: %s", selfDebugMarker, rep.Summary())
	}
}

func (m Model) consumeSelfProblems() {
	if m.selfDebug != nil {
		m.selfDebug.consume()
	}
}

// =============================================================================
// /debug-self COMMAND (modelled on handleCmdYolo)
// =============================================================================

func (m Model) handleCmdDebugSelf(input string, parts []string) (tea.Model, tea.Cmd) {
	_ = input
	on := true
	if len(parts) > 1 {
		switch parts[1] {
		case "on":
			on = true
		case "off":
			on = false
		case "status":
			return m.pushYoloNote(debugSelfStatusText(m)), nil
		default:
			return m.pushYoloNote("Usage: `/debug-self [on|off|status]` — bare `/debug-self` toggles."), nil
		}
	} else {
		on = !m.debugSelfEnabled()
	}
	if m.Config == nil {
		return m.pushYoloNote("Debug-self unavailable: no config loaded, so logging.debug_self cannot be persisted."), nil
	}
	var captureNote string
	m, captureNote = m.setDebugSelf(on)
	if on {
		return m.pushYoloNote("Debug-self mode ON. " + captureNote + "\n" + debugSelfStatusText(m)), nil
	}
	return m.pushYoloNote("Debug-self mode OFF: no per-turn log problem reporting. " + captureNote), nil
}

func (m Model) setDebugSelf(on bool) (Model, string) {
	if m.Config == nil {
		return m, ""
	}
	if m.Config.Logging == nil {
		m.Config.Logging = &config.LoggingConfig{}
	}
	m.Config.Logging.DebugSelf = on
	if err := m.Config.Save(m.userConfigPath()); err != nil {
		m = m.addMessage(Message{
			Role:    "assistant",
			Content: fmt.Sprintf("Debug-self mode %s for this session, but saving failed: %v", onOff(on), err),
			Time:    time.Now(),
		})
	}
	if !on {
		m.syncSelfProblemFacts()
		m.consumeSelfProblems()
		m.applyLoggingIfBound()
		return m, "Debug logging now follows logging.debug_mode."
	}
	if m.selfDebug == nil {
		m.selfDebug = newSelfDebugWatcher()
	}
	path, _, _ := logging.ProblemsSnapshot()
	m.selfDebug.prime(path)
	m.syncSelfProblemFacts()
	return m, m.activateSelfDebugCapture()
}

// activateSelfDebugCapture starts log capture mid-session when the logging
// sinks are bound to this workspace: ApplyConfig (debug_self implies debug
// mode) + a same-workspace Initialize creates .nerd/logs and the audit log,
// and loggers are fetched per call site (no-op loggers are not cached), so
// capture starts for subsequent log lines. Logger values already held as
// no-ops stay silent, so a restart remains the complete-capture path.
func (m Model) activateSelfDebugCapture() string {
	if logging.IsDebugMode() {
		m.applyLoggingIfBound()
		return "Log capture is already active (debug logging was on)."
	}
	if m.applyLoggingIfBound() && logging.IsDebugMode() {
		return "Log capture started for this session (debug logging implied by debug_self); restart codenerd for complete capture from boot."
	}
	return "Restart codenerd for log capture to start; reporting applies immediately."
}

// applyLoggingIfBound re-applies the config's logging settings only when
// the logging package is already bound to this model's workspace, so a
// toggle never rebinds another workspace's sinks (or a test's globals).
func (m Model) applyLoggingIfBound() bool {
	bound := logging.BoundWorkspace()
	if bound == "" || m.workspace == "" || m.Config == nil {
		return false
	}
	abs, err := filepath.Abs(m.workspace)
	if err != nil || filepath.Clean(abs) != bound {
		return false
	}
	config.ApplyLoggingConfig(m.Config)
	return logging.Initialize(m.workspace) == nil
}

func debugSelfStatusText(m Model) string {
	if !m.debugSelfEnabled() {
		return "Debug-self mode is OFF. Enable with `/debug-self on` (persists logging.debug_self; implies debug logging)."
	}
	capturePart := "NOT active yet — restart codenerd for log capture; reporting applies immediately"
	if logging.IsDebugMode() {
		capturePart = "active"
	}
	pathPart := "not created yet (no WARN/ERROR logged since capture started)"
	if path, _, _ := logging.ProblemsSnapshot(); path != "" {
		pathPart = "`" + path + "`"
	}
	return fmt.Sprintf("Debug-self mode is ON (logging.debug_self). Debug logging: implied on. Work mode: %s (debug-self does not change it). Log capture: %s. Problems log: %s. New ERROR lines are injected each turn; new WARN lines arrive as a deduped digest.",
		workModeLabel(m.workMode()), capturePart, pathPart)
}
