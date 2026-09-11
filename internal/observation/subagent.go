package observation

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"codenerd/internal/retain"
	"codenerd/internal/testoutput"
)

// Finding is one issue a subagent reported, with the citation that makes it
// actionable.
//
// File and Line are separate rather than a single "file.go:42" string because
// every consumer already wants them apart: the chat delegation layer ranks
// files by how many findings cite them, and a fixer is dispatched to a path,
// not to a citation.
type Finding struct {
	File     string `json:"file,omitempty"`
	Line     int    `json:"line,omitempty"`
	Severity string `json:"severity,omitempty"`
	Message  string `json:"message"`
}

// Verification is a check that ran, and what it said.
//
// Ran and OK are separate for the reason internal/session's BuildVerification
// states in its own comment: a skipped verification is not a pass. Source
// carries the same distinction one level up — a subagent SAYING its tests
// passed is not the same evidence as the executor having watched them pass —
// and the two must not render identically, because the parent decides whether
// to re-verify on exactly that difference.
type Verification struct {
	// Kind is "build" or "tests".
	Kind string `json:"kind"`
	// Source is SourceObserved when the runtime watched the check run, or
	// SourceReported when this was read out of the subagent's own prose.
	Source string `json:"source"`
	Ran    bool   `json:"ran"`
	OK     bool   `json:"ok"`
	// Passed and Failed are counts where the check produced them; zero
	// otherwise. They are not a substitute for Ran.
	Passed int `json:"passed,omitempty"`
	Failed int `json:"failed,omitempty"`
	// FailedNames are the failures the runner named, in report order. A runner
	// that says "1 failed" without naming it leaves this empty, so its length
	// is not the failure count.
	FailedNames []string `json:"failed_names,omitempty"`
	// Detail is the first line of failure output, when there was one. The rest
	// is in the transcript, behind the handle.
	Detail string `json:"detail,omitempty"`
}

// Verification sources.
const (
	// SourceObserved means the runtime ran the check itself and recorded the
	// verdict. This is evidence.
	SourceObserved = "observed"
	// SourceReported means the verdict was read out of the subagent's own
	// output. This is a claim.
	SourceReported = "reported"
)

// Return is the raw observation a subagent produced: everything the parent was
// handed when the subagent finished.
//
// This is the shape that gets retained, so Output is the whole transcript
// rather than a summary of it. A handle that expands to less than the parent
// was given is a handle that will be discovered to be lossy at the worst
// moment — when the parent redeems it because the projection was not enough.
//
// The structured fields below are the ones the producer ALREADY HAD. They are
// not a request for the producer to go and derive them: internal/session's
// ExecutionResult computes WrittenPaths, BuildCheck, TestCheck and
// UntestedPaths on every turn and then throws all of it away at the subagent
// boundary, where SubAgent.execute returns result.Response and nothing else.
// Filling them is reconnecting a wire that exists on both ends, and it is the
// difference between a projection that KNOWS what changed and one that guessed
// from prose. Leaving them empty is allowed and means exactly "the producer had
// no structure", not "there was none": findings are then read out of the
// output, and Changed and the verifications are simply not reported. See
// collectVerification for why those two do not fall back.
type Return struct {
	// Agent is the subagent's name or intent verb, as the producer knows it.
	Agent string `json:"agent,omitempty"`
	// Task is what it was asked to do.
	Task string `json:"task,omitempty"`
	// Output is the whole return, exactly as the subagent produced it.
	Output string `json:"output"`
	// Failure is the error the subagent returned, when it failed. A failed
	// return still carries whatever Output it produced before failing, which is
	// usually where the reason is.
	Failure  string        `json:"failure,omitempty"`
	Duration time.Duration `json:"duration,omitempty"`

	Findings []Finding     `json:"findings,omitempty"`
	Changed  []string      `json:"changed,omitempty"`
	Build    *Verification `json:"build,omitempty"`
	Tests    *Verification `json:"tests,omitempty"`
	Notes    []string      `json:"notes,omitempty"`
}

// ReturnResult is the projection handed to the parent's reasoning.
type ReturnResult struct {
	Agent    string        `json:"agent,omitempty"`
	Task     string        `json:"task,omitempty"`
	Status   string        `json:"status"`
	Failure  string        `json:"failure,omitempty"`
	Duration time.Duration `json:"duration,omitempty"`

	// Outline is the return's own section headings and where they start. It is
	// what the file-read codec next door does with the part of a file it does
	// not print, and for the same reason: an elision the reader has no handle
	// on is a loss, while a list of what is in there and where lets the next
	// expansion ask for the right lines instead of paging from the top.
	Outline        []Section `json:"outline,omitempty"`
	OutlineOmitted int       `json:"outline_omitted,omitempty"`

	Findings        []Finding      `json:"findings,omitempty"`
	FindingsOmitted int            `json:"findings_omitted,omitempty"`
	Evidence        []string       `json:"evidence,omitempty"`
	EvidenceOmitted int            `json:"evidence_omitted,omitempty"`
	Changed         []string       `json:"changed,omitempty"`
	ChangedOmitted  int            `json:"changed_omitted,omitempty"`
	Verification    []Verification `json:"verification,omitempty"`

	// Uncertainty is what the subagent did not settle: the checks that did not
	// run, the paths it wrote without tests, and its own hedges. It is carried
	// because the parent's next decision turns on it — a finding the subagent
	// is unsure of is not the same input as one it verified, and a projection
	// that flattens the two hands the parent a confidence it was never given.
	Uncertainty        []string `json:"uncertainty,omitempty"`
	UncertaintyOmitted int      `json:"uncertainty_omitted,omitempty"`

	// Verbatim carries the whole return when it was too small for eliding it to
	// pay. See minRetainBytes.
	Verbatim string `json:"verbatim,omitempty"`

	// Bytes and Lines size the transcript that was elided, so the parent can
	// tell a one-line confirmation from a ten-page audit before deciding to
	// redeem the handle.
	Bytes int `json:"bytes,omitempty"`
	Lines int `json:"lines,omitempty"`

	// Handle redeems the retained transcript. Empty means nothing was retained,
	// which is either the empty return or the verbatim case above.
	Handle string `json:"handle,omitempty"`
}

// Section is one heading in a return, and the line it starts on.
//
// Depth is carried so nesting survives without the indentation costing
// anything: a flat list of eighteen headings reads as eighteen peers, which is
// a different document from the one the subagent wrote.
type Section struct {
	Title string `json:"title"`
	Line  int    `json:"line"`
	Depth int    `json:"depth"`
}

// Return statuses. Empty is separate from completed on purpose: a subagent that
// returns nothing has failed at the only thing it was for, and reporting that
// as completion is how a campaign phase advances on an empty deliverable.
const (
	StatusCompleted = "completed"
	StatusFailed    = "failed"
	StatusEmpty     = "empty"
)

// ReturnLimits bound the projection. A subagent that reports two hundred
// findings must cost what one reporting twenty costs, and the counts of what
// was dropped are reported so the omission is visible rather than silent.
type ReturnLimits struct {
	MaxFindings    int
	MaxEvidence    int
	MaxChanged     int
	MaxUncertainty int
	MaxOutline     int
	// MaxMessage caps one finding's message. A subagent that pastes a stack
	// trace into a finding would otherwise walk straight through MaxFindings.
	MaxMessage int
}

// DefaultReturnLimits sizes a projection for a parent deciding what to do next.
//
// 20 findings is above what a reviewer produces for one file and below what it
// produces for a package, which is the boundary where a parent stops reading
// individual findings and starts wanting the worst ones. 12 changed paths is a
// large single-task write set; past that the parent wants the count, not the
// list.
func DefaultReturnLimits() ReturnLimits {
	return ReturnLimits{
		MaxFindings:    20,
		MaxEvidence:    12,
		MaxChanged:     12,
		MaxUncertainty: 6,
		MaxOutline:     18,
		MaxMessage:     200,
	}
}

func (l ReturnLimits) resolved() ReturnLimits {
	def := DefaultReturnLimits()
	if l.MaxFindings <= 0 {
		l.MaxFindings = def.MaxFindings
	}
	if l.MaxEvidence <= 0 {
		l.MaxEvidence = def.MaxEvidence
	}
	if l.MaxChanged <= 0 {
		l.MaxChanged = def.MaxChanged
	}
	if l.MaxUncertainty <= 0 {
		l.MaxUncertainty = def.MaxUncertainty
	}
	if l.MaxOutline <= 0 {
		l.MaxOutline = def.MaxOutline
	}
	if l.MaxMessage <= 0 {
		l.MaxMessage = def.MaxMessage
	}
	return l
}

// minRetainBytes is the transcript size below which nothing is elided.
//
// The codec's header and its handle footer cost roughly 150 bytes together. On
// a "Done — updated internal/x.go" return, eliding thirty bytes behind a
// hundred-and-fifty-byte promise makes the projection five times the size of
// the thing it replaced, and the handle buys the parent nothing it cannot
// already see. Under this ceiling the return is carried whole and no handle is
// minted, which is the same trade the file-read codec makes for a short file.
//
// 512 is set where the overhead stops mattering: at the ceiling the verbatim
// form costs about 12% more than the raw return, and above it every byte of
// growth is a byte the elision saves.
const minRetainBytes = 512

// subagentRetentionKind labels retained transcripts. It is part of the content
// address, so an identical byte sequence retained by another codec gets a
// different id and the two can never be confused for one another.
const subagentRetentionKind = "subagent_return"

// subagentHandlePrefix marks an id as redeemable and as belonging to this codec
// rather than to code search or MCP.
const subagentHandlePrefix = "obs:sa:"

// Subagents encodes subagent returns and retains the raw ones.
//
// It holds a retain.Store and nothing else, and here that matters more than it
// does for the codecs next door. A code search re-run answers from a world that
// moved; a SUBAGENT re-run writes files, spends tokens and can open a pull
// request. "Show me the rest of what you already told me" must never be a
// second delegation. A task executor, a spawner or a workspace root stored on
// this type would each be a way for that to be quietly lost in a later edit;
// with none of them present, re-running is not a thing this type can do, and
// TestSubagents_ShouldHoldNothingItCouldReRunWith pins the field list so it
// stays that way.
type Subagents struct {
	store *retain.Store
}

// NewSubagents creates a codec over its own retention.
func NewSubagents(cfg retain.Config) *Subagents {
	cfg.Prefix = subagentHandlePrefix
	return &Subagents{store: retain.New(cfg)}
}

var (
	sharedSubagentsOnce sync.Once
	sharedSubagents     *Subagents
)

// SharedSubagents returns the process-wide subagent-return codec.
//
// One store, not one per producer. The delegate action mints a handle inside
// the VirtualStore, the campaign orchestrator mints one in its own package, and
// the verb that redeems either lives in a third; separate stores would make
// every published handle unredeemable across that boundary and the agent would
// spend a turn finding out.
func SharedSubagents() *Subagents {
	sharedSubagentsOnce.Do(func() {
		sharedSubagents = NewSubagents(retain.DefaultConfig())
	})
	return sharedSubagents
}

// EncodeReturn projects a subagent's return and retains the raw one.
//
// Retention failing is not an error the caller has to handle: the projection is
// still correct and still useful, it simply cannot be expanded. An empty Handle
// says so, and that is better than refusing to return the structure because the
// transcript did not fit.
func (s *Subagents) EncodeReturn(r Return, limits ReturnLimits) ReturnResult {
	result := ProjectReturn(r, limits)
	// Nothing is retained below the threshold: either the return was carried
	// whole, or it was empty and there is nothing to carry. Minting there would
	// publish a handle that redeems to what the reader is already looking at.
	if s == nil || len(r.Output) < minRetainBytes {
		return result
	}
	payload, err := json.Marshal(r)
	if err != nil {
		// Marshalling this struct cannot fail today. If a future field makes it
		// possible, the projection must still be returned: losing the structure
		// because the expandable copy could not be encoded is strictly worse.
		return result
	}
	result.Handle = s.store.Mint(subagentRetentionKind, payload)
	return result
}

// ReturnWindow bounds a hydration, in lines of the transcript.
//
// Expanding is itself budgeted, because the reason the transcript was withheld
// is that there was too much of it, and an expansion that returns all of it has
// moved the problem one turn later.
type ReturnWindow struct {
	Offset int
	Limit  int
	// Match, when set, keeps only transcript lines containing it. "Show me
	// where it said that" is the question a projection provokes, and answering
	// it by paging through nine hundred lines is the cost this codec exists to
	// avoid.
	Match string
}

// HydratedReturn is the transcript, as returned, bounded to a window.
type HydratedReturn struct {
	Handle  string   `json:"handle"`
	Agent   string   `json:"agent,omitempty"`
	Task    string   `json:"task,omitempty"`
	Failure string   `json:"failure,omitempty"`
	Lines   []string `json:"lines"`
	Total   int      `json:"total"`
	Offset  int      `json:"offset"`
	Match   string   `json:"match,omitempty"`
	// NextOffset is zero once the window reached the end of the retained
	// transcript; otherwise it is the offset that continues the walk.
	NextOffset int `json:"next_offset,omitempty"`
}

// maxReturnHydrateLines caps a single expansion regardless of what was asked
// for, and defaultReturnHydrateLines is what an unbounded request gets.
const (
	maxReturnHydrateLines     = 200
	defaultReturnHydrateLines = 60
)

// HydrateReturn reopens a retained transcript.
//
// It reads the retained bytes and consults nothing else. For this codec that is
// not merely about consistency: re-running the subagent to "expand" its answer
// would run the task again, with every side effect it had the first time, and
// return a different answer from the one the parent's reasoning was built on.
func (s *Subagents) HydrateReturn(handle string, w ReturnWindow) (HydratedReturn, error) {
	if s == nil {
		return HydratedReturn{}, ErrNotFound
	}
	handle = strings.TrimSpace(handle)
	if handle == "" {
		return HydratedReturn{}, fmt.Errorf("%w: empty handle", ErrNotFound)
	}

	kind, payload, err := s.store.Get(handle)
	if err != nil {
		return HydratedReturn{}, err
	}
	if kind != subagentRetentionKind {
		return HydratedReturn{}, fmt.Errorf("%w: %s is not a subagent return", ErrNotFound, handle)
	}

	var r Return
	if err := json.Unmarshal(payload, &r); err != nil {
		return HydratedReturn{}, fmt.Errorf("retained return %s is unreadable: %w", handle, err)
	}

	// Split, not normalise. A hydration is the retained bytes; stripping a
	// carriage return would make this the retained bytes ALMOST, which is the
	// property a handle exists to be free of. Text() writes each line back with
	// a single newline, so a CRLF transcript reconstructs exactly.
	lines := strings.Split(r.Output, "\n")
	if match := strings.TrimSpace(w.Match); match != "" {
		lower := strings.ToLower(match)
		filtered := make([]string, 0, len(lines))
		for _, line := range lines {
			if strings.Contains(strings.ToLower(line), lower) {
				filtered = append(filtered, line)
			}
		}
		lines = filtered
	}

	limit := w.Limit
	if limit <= 0 {
		limit = defaultReturnHydrateLines
	}
	if limit > maxReturnHydrateLines {
		limit = maxReturnHydrateLines
	}
	offset := max(w.Offset, 0)
	offset = min(offset, len(lines))
	end := min(offset+limit, len(lines))

	out := HydratedReturn{
		Handle:  handle,
		Agent:   r.Agent,
		Task:    r.Task,
		Failure: r.Failure,
		Lines:   append([]string(nil), lines[offset:end]...),
		Total:   len(lines),
		Offset:  offset,
		Match:   strings.TrimSpace(w.Match),
	}
	if end < len(lines) {
		out.NextOffset = end
	}
	return out, nil
}

// ProjectReturn turns a subagent's return into what the parent has to decide
// on: findings, the evidence they rest on, what changed, what was verified and
// what is still open.
//
// It is exported separately from EncodeReturn so the projection can be
// exercised, and reasoned about, without retention in the picture: the two
// halves fail differently and a test that could not separate them would be
// testing both at once.
func ProjectReturn(r Return, limits ReturnLimits) ReturnResult {
	limits = limits.resolved()

	// Extraction reads the output as it was returned, not a trimmed copy. The
	// outline reports the line a section starts on and hydration counts lines
	// from the same string; trimming here would shift every one of those
	// numbers by however many blank lines the subagent happened to open with,
	// and an offset that lands one section early is worse than no offset.
	output := r.Output
	result := ReturnResult{
		Agent:    strings.TrimSpace(r.Agent),
		Task:     strings.TrimSpace(r.Task),
		Failure:  strings.TrimSpace(r.Failure),
		Duration: r.Duration,
		Bytes:    len(output),
	}
	switch {
	case result.Failure != "":
		result.Status = StatusFailed
	case strings.TrimSpace(output) == "":
		result.Status = StatusEmpty
	default:
		result.Status = StatusCompleted
	}
	if output != "" {
		result.Lines = strings.Count(output, "\n") + 1
	}

	// What changed and what was verified come from structure only, and stay
	// empty when the producer had none. Reading a write list out of prose is
	// the one guess this codec refuses to make: the parent ACTS on that list —
	// it re-reads those files, it runs the build for them, it reports them as
	// done — and a hallucinated path sends it to edit a file the subagent never
	// touched. "Nothing was reported" is recoverable; a confident wrong answer
	// is not.
	result.Changed, result.ChangedOmitted = capStrings(dedupe(r.Changed), limits.MaxChanged)
	result.Verification = collectVerification(r)

	uncertainty := structuralUncertainty(r)

	// Below the threshold the return is carried whole, and the derived tables
	// are skipped with it. Every finding, citation and hedge in them was read
	// OUT of the text the reader is now looking at, so printing both is the
	// projection paying for the same fact twice — the defect that first made
	// the code-search codec next door larger than the grep output it replaced.
	// The structured halves above survive, because those are facts the
	// transcript does not contain.
	if len(output) < minRetainBytes {
		result.Verbatim = output
		result.Uncertainty, result.UncertaintyOmitted = capStrings(uncertainty, limits.MaxUncertainty)
		return result
	}

	// Structure the producer had beats structure read back out of prose. The
	// chat layer's reviewer findings arrive already parsed with file, line,
	// severity and message; re-deriving them from the rendered text would lose
	// exactly the fields that survived, and would disagree with the copy the
	// same producer hands its own consumers.
	findings := r.Findings
	if len(findings) == 0 {
		findings = extractReturnFindings(output)
	}
	result.Findings, result.FindingsOmitted = capFindings(findings, limits)

	// Evidence excludes the sites the findings already name. A citation printed
	// once as a finding and again as evidence is the same byte cost for no
	// second fact.
	cited := make(map[string]struct{}, len(result.Findings))
	for _, f := range result.Findings {
		if f.File != "" && f.Line > 0 {
			cited[f.File+":"+strconv.Itoa(f.Line)] = struct{}{}
		}
	}
	evidence := make([]string, 0, limits.MaxEvidence)
	for _, ref := range extractEvidenceRefs(output) {
		if _, dup := cited[ref]; dup {
			continue
		}
		evidence = append(evidence, ref)
	}
	result.Evidence, result.EvidenceOmitted = capStrings(evidence, limits.MaxEvidence)

	uncertainty = append(uncertainty, extractHedges(output, limits.MaxMessage)...)
	result.Uncertainty, result.UncertaintyOmitted = capStrings(dedupe(uncertainty), limits.MaxUncertainty)

	// The outline is last because it is the fallback, and it earns its place on
	// exactly the returns the four sections above cannot help with. Measured
	// over the 67 real agent outputs in this repository's .quality_assurance
	// directory, a third of them carry no severity-marked finding and no
	// file:line citation at all — they are long structured prose — and without
	// this they projected to a status line and a handle. That is honest and
	// nearly useless; the headings are the return's own structure and are
	// neither a summary nor the transcript.
	result.Outline, result.OutlineOmitted = extractOutline(output, limits.MaxOutline)
	return result
}

// outlineHeading matches a markdown ATX heading, which is what an agent's long
// return is structured with when it is structured at all.
//
// Only ATX. A looser rule — a short line ending in a colon, a bold-only line —
// picks up ordinary prose and fills the outline with sentences, which costs the
// same bytes as real headings and carries none of the navigation.
var outlineHeading = regexp.MustCompile(`^(#{1,6})\s+(\S.*?)\s*#*$`)

// extractOutline lists the return's headings and where each starts.
func extractOutline(output string, limit int) ([]Section, int) {
	var out []Section
	for i, line := range strings.Split(output, "\n") {
		m := outlineHeading.FindStringSubmatch(strings.TrimSuffix(line, "\r"))
		if m == nil {
			continue
		}
		out = append(out, Section{
			Title: clampMessage(m[2], 120),
			Line:  i + 1,
			Depth: len(m[1]),
		})
	}
	if len(out) <= limit {
		return out, 0
	}
	// The head, not a sample. A document's first headings are its shape; a
	// slice from the middle would describe a document nobody wrote.
	return out[:limit], len(out) - limit
}

// collectVerification reports what ran. It reads the producer's structure and
// NOTHING ELSE — there is no fallback that reads a verdict out of the prose.
//
// That is a deliberate refusal, and it was not the first design. Running the
// repository's test-output parser over a return looks obviously right until you
// run it over a code review: internal/testoutput's generic heuristics match
// "error" and "failed" as whole words, by design, so they can read a non-Go
// runner — and a reviewer writing "the error from Flush is discarded" scored
// three test failures on a shard that never ran a test. A projection that
// reports "tests FAILED (0 passed, 3 failed)" for a code review has not
// summarised the return, it has fabricated a verdict, and the parent will act
// on it.
//
// The knowledge that a given return IS a test log belongs to the producer,
// which knows it spawned a tester, not to a codec looking at text. So a
// producer in that position parses its own output and passes Tests in, marked
// SourceReported; every other producer passes nothing and the projection says
// nothing. The log itself stays one subagent_expand away, so a parent that
// wants the verdict badly enough can read it rather than be handed a guess.
func collectVerification(r Return) []Verification {
	var out []Verification
	if r.Build != nil {
		v := *r.Build
		v.Kind = "build"
		if v.Source == "" {
			v.Source = SourceObserved
		}
		out = append(out, v)
	}
	if r.Tests != nil {
		v := *r.Tests
		v.Kind = "tests"
		if v.Source == "" {
			v.Source = SourceObserved
		}
		out = append(out, v)
	}
	return out
}

// ReportedTests reads a test verdict out of output the CALLER knows to be a
// test runner's, for a producer that can attest to that.
//
// It is exported so a producer holding that knowledge — the chat blackboard,
// which knows the prior shard was the tester — can supply it, and it is not
// called from inside projection for the reason collectVerification gives. The
// verdict comes back marked SourceReported because nothing here watched the run
// happen; the counts are internal/testoutput's, so a fix to how a runner is
// read still reaches this consumer with every other one.
func ReportedTests(output string) *Verification {
	counts := testoutput.Parse(output)
	if !counts.Parsed {
		return nil
	}
	return &Verification{
		Kind:        "tests",
		Source:      SourceReported,
		Ran:         true,
		OK:          counts.Failed == 0,
		Passed:      counts.Passed,
		Failed:      counts.Failed,
		FailedNames: counts.FailedNames,
	}
}

// structuralUncertainty turns what the producer knows into what the parent has
// to weigh.
//
// A verification that did not run is the important one and the easiest to lose:
// it renders as an absence, and an absence reads as "fine". Saying it out loud
// is the whole reason "verification status" and "remaining uncertainty" are two
// separate things in this projection rather than one.
func structuralUncertainty(r Return) []string {
	var out []string
	if r.Build != nil && !r.Build.Ran {
		out = append(out, "build verification did not run for this return")
	}
	if r.Tests != nil && !r.Tests.Ran {
		out = append(out, "test verification did not run for this return")
	}
	out = append(out, r.Notes...)
	return out
}

// returnFindingLine matches the one-line finding format the reviewer atom
// specifies, and the same shape cmd/nerd/chat's extractor reads:
//
//   - [SEVERITY] path/to/file.go:123: message
//
// The line number is optional, because not every finding is about one line, and
// the file is captured non-greedily up to the ":<digits>:" so a Windows path
// with a drive letter does not lose its prefix to the first colon.
var returnFindingLine = regexp.MustCompile(`^[-*\x{2022}]?\s*\[([A-Za-z]+)\]\s*(.+?)(?::(\d+))?:\s*(.+)$`)

// returnSeverityAliases maps every spelling in use onto the reviewer atom's
// four. The extractor and the atom disagreeing on the vocabulary is a defect
// this repository has already had once: the atom says CRITICAL/HIGH/MEDIUM/LOW
// while the reader looked for CRIT/ERR/WARN/INFO, so a finding written exactly
// as instructed matched nothing.
var returnSeverityAliases = map[string]string{
	"crit": "critical", "critical": "critical",
	"err": "high", "error": "high", "high": "high",
	"warn": "medium", "warning": "medium", "medium": "medium",
	"info": "low", "low": "low", "nit": "low",
}

// extractReturnFindings reads findings out of a return that carried no
// structure.
//
// A line with a severity marker that does not match the full shape is still
// kept, with its text and severity. Dropping it would lose a real finding to a
// formatting slip, and every consumer already tolerates a missing file or line.
func extractReturnFindings(output string) []Finding {
	var out []Finding
	for line := range strings.SplitSeq(output, "\n") {
		line = strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		if line == "" {
			continue
		}
		if m := returnFindingLine.FindStringSubmatch(line); m != nil {
			f := Finding{
				Severity: normalizeReturnSeverity(m[1]),
				File:     strings.TrimSpace(m[2]),
				Message:  strings.TrimSpace(m[4]),
			}
			if m[3] != "" {
				if n, err := strconv.Atoi(m[3]); err == nil {
					f.Line = n
				}
			}
			out = append(out, f)
			continue
		}
		if sev, ok := returnSeverityMarkerIn(line); ok {
			out = append(out, Finding{Severity: sev, Message: line})
		}
	}
	return out
}

func normalizeReturnSeverity(marker string) string {
	lower := strings.ToLower(strings.TrimSpace(marker))
	if canonical, ok := returnSeverityAliases[lower]; ok {
		return canonical
	}
	return lower
}

func returnSeverityMarkerIn(line string) (string, bool) {
	upper := strings.ToUpper(line)
	for alias, canonical := range returnSeverityAliases {
		if strings.Contains(upper, "["+strings.ToUpper(alias)+"]") {
			return canonical, true
		}
	}
	return "", false
}

// severityRank orders the reviewer's ladder so findings can be capped by
// worst-first. An unrecognised word sorts below /low rather than above
// /critical, because an unknown severity is missing information, not an
// emergency — the same rule cmd/nerd/chat's findingSeverityRank applies when it
// picks the file a fixer is dispatched to.
func severityRank(severity string) int {
	switch strings.ToLower(strings.TrimSpace(severity)) {
	case "critical":
		return 4
	case "high":
		return 3
	case "medium":
		return 2
	case "low":
		return 1
	default:
		return 0
	}
}

// evidenceRef matches a file:line citation. The extension is required: without
// it "10:30" in a timestamp and "note:5" in prose both read as citations, and a
// projection full of invented references is worse than one with none.
var evidenceRef = regexp.MustCompile(`\b([A-Za-z0-9_][A-Za-z0-9_./\\-]*\.[A-Za-z][A-Za-z0-9]{0,4}):(\d+)\b`)

// extractEvidenceRefs collects the file:line citations a return makes, in first
// appearance order and once each.
//
// First appearance rather than most-cited: the order a subagent presents its
// evidence in is the order it reasoned in, and re-sorting it by frequency
// would put a file it mentioned in passing ahead of the one it opened with.
func extractEvidenceRefs(output string) []string {
	matches := evidenceRef.FindAllStringSubmatch(output, -1)
	seen := make(map[string]struct{}, len(matches))
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		ref := m[1] + ":" + m[2]
		if _, dup := seen[ref]; dup {
			continue
		}
		seen[ref] = struct{}{}
		out = append(out, ref)
	}
	return out
}

// hedgeMarkers are the phrases that mark a sentence as something the subagent
// did NOT settle.
//
// The list is deliberately short and literal. A generous matcher would tag most
// careful prose as uncertain and the section would stop carrying information;
// these are the phrases that appear when a subagent is reporting a gap rather
// than writing carefully. Anything subtler than this belongs in the structured
// Notes the producer supplies, not in a regex over prose.
var hedgeMarkers = []string{
	"could not verify",
	"unable to verify",
	"did not verify",
	"unverified",
	"not verified",
	"could not determine",
	"unable to determine",
	"i am not sure",
	"i'm not sure",
	"unclear whether",
	"assumed that",
	"assuming that",
	"needs manual review",
	"requires manual",
	"blocked by",
	"out of scope",
	"left as-is",
	"todo:",
}

// extractHedges pulls the subagent's own statements of what it did not settle.
func extractHedges(output string, maxMessage int) []string {
	var out []string
	for line := range strings.SplitSeq(output, "\n") {
		line = strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		if line == "" {
			continue
		}
		lower := strings.ToLower(line)
		for _, marker := range hedgeMarkers {
			if strings.Contains(lower, marker) {
				out = append(out, clampMessage(line, maxMessage))
				break
			}
		}
	}
	return out
}

// capFindings orders findings worst-first and reports how many were dropped.
//
// Severity leads, then a cited location, then report order. The middle rule is
// the one worth having on its own merits: given equal severity, a finding that
// names a file and a line is the one the parent can act on without going
// looking, and the one it cannot is the one to drop first.
func capFindings(findings []Finding, limits ReturnLimits) ([]Finding, int) {
	out := make([]Finding, 0, len(findings))
	seen := make(map[string]struct{}, len(findings))
	for _, f := range findings {
		f.Message = clampMessage(strings.TrimSpace(f.Message), limits.MaxMessage)
		f.File = strings.TrimSpace(f.File)
		f.Severity = strings.ToLower(strings.TrimSpace(f.Severity))
		if f.Message == "" && f.File == "" {
			continue
		}
		key := f.Severity + "\x00" + f.File + "\x00" + strconv.Itoa(f.Line) + "\x00" + f.Message
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, f)
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if ra, rb := severityRank(a.Severity), severityRank(b.Severity); ra != rb {
			return ra > rb
		}
		located := func(f Finding) bool { return f.File != "" && f.Line > 0 }
		if located(a) != located(b) {
			return located(a)
		}
		return false
	})
	if len(out) <= limits.MaxFindings {
		return out, 0
	}
	return out[:limits.MaxFindings], len(out) - limits.MaxFindings
}

func capStrings(in []string, limit int) ([]string, int) {
	out := make([]string, 0, len(in))
	for _, s := range in {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	if len(out) == 0 {
		return nil, 0
	}
	if len(out) <= limit {
		return out, 0
	}
	return out[:limit], len(out) - limit
}

func dedupe(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if _, dup := seen[s]; dup {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

// clampMessage bounds one message, cutting on a rune boundary.
func clampMessage(s string, maxLen int) string {
	if maxLen <= 0 || len(s) <= maxLen {
		return s
	}
	return trimToRune(s[:maxLen]) + "..."
}

// Text renders the projection for the parent's reasoning, or an operator.
//
// Text rather than JSON, for the reason the code-search codec next door gives:
// the findings and the verifications are tables, and JSON pays for every column
// name once per row. Naming the columns once in a header is the difference
// between a projection cheaper than the transcript it replaced and one that is
// merely differently shaped.
func (r ReturnResult) Text(expandVerb string) string {
	var sb strings.Builder

	who := r.Agent
	if who == "" {
		who = "subagent"
	}
	fmt.Fprintf(&sb, "%s returned %s", who, r.Status)
	if r.Task != "" {
		fmt.Fprintf(&sb, " for %q", clampMessage(r.Task, 160))
	}
	if r.Duration > 0 {
		fmt.Fprintf(&sb, " in %s", r.Duration.Round(time.Millisecond))
	}
	sb.WriteString("\n")
	if r.Failure != "" {
		fmt.Fprintf(&sb, "failure: %s\n", clampMessage(r.Failure, 400))
	}

	if len(r.Findings) > 0 {
		sb.WriteString("findings (severity file:line message):\n")
		for _, f := range r.Findings {
			sb.WriteString("  ")
			if f.Severity != "" {
				sb.WriteString(f.Severity)
				sb.WriteString(" ")
			}
			if f.File != "" {
				sb.WriteString(f.File)
				if f.Line > 0 {
					fmt.Fprintf(&sb, ":%d", f.Line)
				}
				sb.WriteString(" ")
			}
			sb.WriteString(f.Message)
			sb.WriteString("\n")
		}
		if r.FindingsOmitted > 0 {
			fmt.Fprintf(&sb, "  ... %d more finding(s) not shown\n", r.FindingsOmitted)
		}
	}

	if len(r.Evidence) > 0 {
		fmt.Fprintf(&sb, "evidence cited: %s", strings.Join(r.Evidence, " "))
		if r.EvidenceOmitted > 0 {
			fmt.Fprintf(&sb, " (+%d more)", r.EvidenceOmitted)
		}
		sb.WriteString("\n")
	}

	if len(r.Changed) > 0 {
		fmt.Fprintf(&sb, "changed: %s", strings.Join(r.Changed, " "))
		if r.ChangedOmitted > 0 {
			fmt.Fprintf(&sb, " (+%d more)", r.ChangedOmitted)
		}
		sb.WriteString("\n")
	}

	for _, v := range r.Verification {
		sb.WriteString("verification: ")
		sb.WriteString(v.Text())
		sb.WriteString("\n")
	}

	if len(r.Outline) > 0 {
		sb.WriteString("sections (line title):\n")
		for _, sec := range r.Outline {
			fmt.Fprintf(&sb, "  %d %s%s\n", sec.Line, strings.Repeat("  ", max(sec.Depth-1, 0)), sec.Title)
		}
		if r.OutlineOmitted > 0 {
			fmt.Fprintf(&sb, "  ... %d more section(s) not listed\n", r.OutlineOmitted)
		}
	}

	if len(r.Uncertainty) > 0 {
		sb.WriteString("unsettled:\n")
		for _, u := range r.Uncertainty {
			fmt.Fprintf(&sb, "  %s\n", u)
		}
		if r.UncertaintyOmitted > 0 {
			fmt.Fprintf(&sb, "  ... %d more\n", r.UncertaintyOmitted)
		}
	}

	if r.Verbatim != "" {
		sb.WriteString(r.Verbatim)
		if !strings.HasSuffix(r.Verbatim, "\n") {
			sb.WriteString("\n")
		}
		return sb.String()
	}

	if r.Handle != "" {
		// The handle is useless unless the reader is told the verb that redeems
		// it, and saying the expansion never re-runs the subagent is what makes
		// it safe to rely on: the lines it returns are the lines this
		// projection was computed from, produced by the run that already
		// happened.
		fmt.Fprintf(&sb, "transcript retained as %s (%d bytes, %d lines)", r.Handle, r.Bytes, r.Lines)
		if expandVerb != "" {
			fmt.Fprintf(&sb, " — %s handle=%s reads it without re-running the subagent", expandVerb, r.Handle)
		}
		sb.WriteString("\n")
	} else if r.Bytes > 0 {
		// Nothing retained and nothing carried: say so rather than let the
		// elided transcript read as an absence of one.
		fmt.Fprintf(&sb, "transcript (%d bytes) was not retained and cannot be reopened\n", r.Bytes)
	}
	return sb.String()
}

// Text renders one verification the way the parent has to read it: what ran,
// what it said, and whether anybody watched.
func (v Verification) Text() string {
	var sb strings.Builder
	sb.WriteString(v.Kind)
	switch {
	case !v.Ran:
		sb.WriteString(" did not run")
	case v.OK:
		sb.WriteString(" passed")
	default:
		sb.WriteString(" FAILED")
	}
	if v.Passed > 0 || v.Failed > 0 {
		fmt.Fprintf(&sb, " (%d passed, %d failed)", v.Passed, v.Failed)
	}
	if len(v.FailedNames) > 0 {
		fmt.Fprintf(&sb, " %s", strings.Join(v.FailedNames, " "))
	}
	if v.Source != "" {
		fmt.Fprintf(&sb, " [%s]", v.Source)
	}
	if v.Detail != "" {
		fmt.Fprintf(&sb, ": %s", clampMessage(v.Detail, 200))
	}
	return sb.String()
}

// Text renders a hydration as the transcript lines the subagent produced.
func (h HydratedReturn) Text() string {
	var sb strings.Builder
	who := h.Agent
	if who == "" {
		who = "subagent"
	}
	fmt.Fprintf(&sb, "transcript of %s from %s: lines %d-%d of %d",
		who, h.Handle, h.windowStart(), h.Offset+len(h.Lines), h.Total)
	if h.Match != "" {
		fmt.Fprintf(&sb, " matching %q", h.Match)
	}
	sb.WriteString("\n")
	if h.Failure != "" {
		fmt.Fprintf(&sb, "failure: %s\n", clampMessage(h.Failure, 400))
	}
	for _, line := range h.Lines {
		sb.WriteString(line)
		sb.WriteString("\n")
	}
	if h.NextOffset > 0 {
		fmt.Fprintf(&sb, "... %d more line(s); continue with offset=%d\n",
			h.Total-h.NextOffset, h.NextOffset)
	}
	return sb.String()
}

func (h HydratedReturn) windowStart() int {
	if len(h.Lines) == 0 {
		return h.Offset
	}
	return h.Offset + 1
}
