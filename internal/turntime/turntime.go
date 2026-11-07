// Package turntime is a turn's time ledger: where its wall time went.
//
// A coder turn runs ten to forty-six minutes and the session log priced it in
// tokens only, so whether the time went to the model, to tools, to the
// build/test gates or to repair rounds was guesswork. The executive installs a
// Ledger on the turn's context (session.Executor.ProcessWithIntent); the
// broker (every model call), the tool dispatcher, runVerificationCommand
// (every build, vet, test, coverage and pinning subprocess) and the repair
// loop open spans on it. The turn's end reads one Split from it.
//
// # Accounting rule
//
// Every instant of the turn's wall time is charged to exactly one phase, so
// the phases sum to the total and nothing is counted twice, whatever overlaps:
//
//   - A verification subprocess is verification (by kind: build, vet, tests,
//     importer, coverage, pin, final) wherever it runs, including inside a
//     repair round. Verification outranks model and tool time.
//   - Otherwise an open model call is model time, then an open tool call is tool
//     time. Concurrent calls of one phase overlap in wall time and are charged
//     once: the ledger measures the turn's clock, not the sum of its workers.
//   - Everything else (prompt compilation, memory, kernel work, gaps) is other.
//
// A repair round is a count and a lens, not a fifth phase. Its own time is the
// model and tool time that fell inside it; verification inside a round stays
// verification, and the recheck a round triggers is verification too. Repair
// time is therefore a subset of model plus tool time, reported per kind.
//
// A nil *Ledger and a context without one are valid and record nothing.
package turntime

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// Ledger accumulates one turn's phase times. It is safe for concurrent use.
type Ledger struct {
	mu  sync.Mutex
	now func() time.Time

	start  time.Time
	last   time.Time
	closed bool

	model, tool, other time.Duration
	modelCalls         int
	toolCalls          int

	activeModel  int
	activeTool   int
	activeVerify map[string]int
	activeRepair map[string]int

	verify map[string]*kindTime
	repair map[string]*kindTime
}

type kindTime struct {
	count int
	d     time.Duration
}

// New starts a ledger on the wall clock.
func New() *Ledger { return NewWithClock(time.Now) }

// NewWithClock starts a ledger on the given clock. Tests use it to give each
// phase a known duration.
func NewWithClock(now func() time.Time) *Ledger {
	if now == nil {
		now = time.Now
	}
	t := now()
	return &Ledger{
		now: now, start: t, last: t,
		activeVerify: map[string]int{}, activeRepair: map[string]int{},
		verify: map[string]*kindTime{}, repair: map[string]*kindTime{},
	}
}

// advance charges the time since the last event to the phase that was open
// during it. Callers hold l.mu.
func (l *Ledger) advance() {
	if l.closed {
		return
	}
	t := l.now()
	dt := t.Sub(l.last)
	if dt <= 0 {
		return
	}
	l.last = t
	if kind := lowest(l.activeVerify); kind != "" {
		l.verify[kind].d += dt
		return
	}
	switch {
	case l.activeModel > 0:
		l.model += dt
	case l.activeTool > 0:
		l.tool += dt
	default:
		l.other += dt
		return
	}
	if kind := lowest(l.activeRepair); kind != "" {
		l.repair[kind].d += dt
	}
}

// lowest is the alphabetically first kind with an open span, so overlapping
// spans of different kinds are charged to one kind deterministically.
func lowest(active map[string]int) string {
	best := ""
	for kind, n := range active {
		if n > 0 && (best == "" || kind < best) {
			best = kind
		}
	}
	return best
}

// span runs begin under the lock after charging the time so far, and returns
// the once-only call that charges the time so far and runs finish.
// Spans opened after Close record nothing.
func (l *Ledger) span(begin, finish func()) func() {
	if l == nil {
		return func() {}
	}
	l.mu.Lock()
	l.advance()
	if l.closed {
		l.mu.Unlock()
		return func() {}
	}
	begin()
	l.mu.Unlock()
	var once sync.Once
	return func() {
		once.Do(func() {
			l.mu.Lock()
			l.advance()
			finish()
			l.mu.Unlock()
		})
	}
}

// Model opens a model call and returns the call that ends it.
func (l *Ledger) Model() func() {
	return l.span(func() { l.activeModel++; l.modelCalls++ }, func() { l.activeModel-- })
}

// Tool opens a tool execution and returns the call that ends it.
func (l *Ledger) Tool() func() {
	return l.span(func() { l.activeTool++; l.toolCalls++ }, func() { l.activeTool-- })
}

// Verify opens one verification run of the given kind.
func (l *Ledger) Verify(kind string) func() {
	if kind == "" {
		kind = KindOther
	}
	return l.span(func() {
		l.activeVerify[kind]++
		if l.verify[kind] == nil {
			l.verify[kind] = &kindTime{}
		}
		l.verify[kind].count++
	}, func() { l.activeVerify[kind]-- })
}

// Repair opens one repair round of the given kind. See the package comment:
// the round claims the model and tool time inside it, not its verification.
func (l *Ledger) Repair(kind string) func() {
	if kind == "" {
		kind = KindOther
	}
	return l.span(func() {
		l.activeRepair[kind]++
		if l.repair[kind] == nil {
			l.repair[kind] = &kindTime{}
		}
		l.repair[kind].count++
	}, func() { l.activeRepair[kind]-- })
}

// Close freezes the ledger at the current time; later spans record nothing.
func (l *Ledger) Close() {
	if l == nil {
		return
	}
	l.mu.Lock()
	l.advance()
	l.closed = true
	l.mu.Unlock()
}

// Snapshot is the split so far (final once the ledger is closed).
func (l *Ledger) Snapshot() Split {
	if l == nil {
		return Split{}
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.advance()
	s := Split{
		Total: l.last.Sub(l.start), Model: l.model, Tool: l.tool, Other: l.other,
		ModelCalls: l.modelCalls, ToolCalls: l.toolCalls,
	}
	for kind, kt := range l.verify {
		s.Verify = append(s.Verify, Kind{Kind: kind, Count: kt.count, Time: kt.d})
	}
	for kind, kt := range l.repair {
		s.Repair = append(s.Repair, Kind{Kind: kind, Count: kt.count, Time: kt.d})
	}
	sort.Slice(s.Verify, func(i, j int) bool { return s.Verify[i].Kind < s.Verify[j].Kind })
	sort.Slice(s.Repair, func(i, j int) bool { return s.Repair[i].Kind < s.Repair[j].Kind })
	return s
}

// Verification and repair kinds. A caller names a kind on the context
// (WithVerifyKind) when the command alone does not say what the run is for.
const (
	KindBuild    = "build"
	KindVet      = "vet"
	KindTests    = "tests"
	KindImporter = "importer"
	KindCoverage = "coverage"
	KindPin      = "pinning"
	KindFinal    = "final"
	KindOther    = "other"
)

// Kind is one verification kind's runs (Count) and time, or one repair kind's
// rounds (Count) and its model and tool time.
type Kind struct {
	Kind  string        `json:"kind"`
	Count int           `json:"count"`
	Time  time.Duration `json:"time"`
}

// Split is a turn's time ledger at a moment. Total is Model + Tool + the
// verification times + Other; repair times are inside Model and Tool.
type Split struct {
	Total      time.Duration `json:"total"`
	Model      time.Duration `json:"model"`
	Tool       time.Duration `json:"tool"`
	Other      time.Duration `json:"other"`
	ModelCalls int           `json:"model_calls"`
	ToolCalls  int           `json:"tool_calls"`
	Verify     []Kind        `json:"verify,omitempty"`
	Repair     []Kind        `json:"repair,omitempty"`
}

// VerifyTime is the time all verification runs took.
func (s Split) VerifyTime() time.Duration {
	var t time.Duration
	for _, k := range s.Verify {
		t += k.Time
	}
	return t
}

// VerifyRuns is the number of verification runs.
func (s Split) VerifyRuns() int {
	n := 0
	for _, k := range s.Verify {
		n += k.Count
	}
	return n
}

// RepairRounds is the number of repair rounds, all kinds.
func (s Split) RepairRounds() int {
	n := 0
	for _, k := range s.Repair {
		n += k.Count
	}
	return n
}

// RepairTime is the model and tool time spent inside repair rounds.
func (s Split) RepairTime() time.Duration {
	var t time.Duration
	for _, k := range s.Repair {
		t += k.Time
	}
	return t
}

// Recorded reports whether the split measured anything.
func (s Split) Recorded() bool { return s.Total > 0 }

func secs(d time.Duration) string { return fmt.Sprintf("%.1fs", d.Seconds()) }

// LogFields renders the split as key=value pairs in seconds for the turn_cost
// log line.
func (s Split) LogFields() string {
	var b strings.Builder
	fmt.Fprintf(&b, "secs_total=%s secs_model=%s secs_tool=%s secs_verify=%s secs_other=%s model_calls=%d verify_runs=%d repair_rounds=%d secs_repair=%s",
		secs(s.Total), secs(s.Model), secs(s.Tool), secs(s.VerifyTime()), secs(s.Other),
		s.ModelCalls, s.VerifyRuns(), s.RepairRounds(), secs(s.RepairTime()))
	for _, k := range s.Verify {
		fmt.Fprintf(&b, " verify_%s=%s/%d", k.Kind, secs(k.Time), k.Count)
	}
	for _, k := range s.Repair {
		fmt.Fprintf(&b, " repair_%s=%s/%d", k.Kind, secs(k.Time), k.Count)
	}
	return b.String()
}

// Line is the compact one-line form the chat shows at the end of a turn.
func (s Split) Line() string {
	if !s.Recorded() {
		return ""
	}
	parts := []string{
		fmt.Sprintf("model %s (%d call%s)", short(s.Model), s.ModelCalls, plural(s.ModelCalls)),
		fmt.Sprintf("tools %s (%d)", short(s.Tool), s.ToolCalls),
	}
	if s.VerifyRuns() > 0 {
		var kinds []string
		for _, k := range s.Verify {
			kinds = append(kinds, k.Kind+" "+short(k.Time))
		}
		parts = append(parts, fmt.Sprintf("verify %s (%s)", short(s.VerifyTime()), strings.Join(kinds, ", ")))
	}
	if n := s.RepairRounds(); n > 0 {
		var kinds []string
		for _, k := range s.Repair {
			kinds = append(kinds, fmt.Sprintf("%s %d", k.Kind, k.Count))
		}
		parts = append(parts, fmt.Sprintf("repair %d round%s (%s; %s)", n, plural(n), strings.Join(kinds, ", "), short(s.RepairTime())))
	}
	parts = append(parts, "other "+short(s.Other))
	return "Time " + short(s.Total) + ": " + strings.Join(parts, ", ")
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

// short is a duration at the resolution a person reads: tenths under ten
// seconds, whole seconds above.
func short(d time.Duration) string {
	if d < 10*time.Second {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	return d.Round(time.Second).String()
}

type ledgerKey struct{}
type kindKey struct{}

// WithLedger returns ctx carrying l.
func WithLedger(ctx context.Context, l *Ledger) context.Context {
	return context.WithValue(ctx, ledgerKey{}, l)
}

// From returns the ledger on ctx, or nil (whose spans record nothing).
func From(ctx context.Context) *Ledger {
	if ctx == nil {
		return nil
	}
	l, _ := ctx.Value(ledgerKey{}).(*Ledger)
	return l
}

// WithVerifyKind names what verification subprocesses started under ctx are
// for ("importer", "coverage", "pin", "final").
func WithVerifyKind(ctx context.Context, kind string) context.Context {
	return context.WithValue(ctx, kindKey{}, kind)
}

// VerifyKindFrom returns the kind WithVerifyKind set, or "".
func VerifyKindFrom(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	k, _ := ctx.Value(kindKey{}).(string)
	return k
}
