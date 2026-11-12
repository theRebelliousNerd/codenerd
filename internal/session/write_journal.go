package session

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"codenerd/internal/testfacts"
)

// A write journal is the record of every write a span of turns made, kept by
// the caller that owns the span so it can put exactly those writes back.
//
// A self-repair fix turn is the span it exists for (chat self_repair.go): the
// turn runs as a chain step, its verified delegation may retry the coder, and
// each attempt is a turn of its own executor (an isolated clone or a
// subagent). None of those executors outlives the turn, and the chat only
// sees the delegation's verdict. So the chat puts a journal on the context
// the fix turn runs under; every turn that runs under it adds what it wrote
// when it ends (ProcessWithIntent), and the chat undoes the span when the fix
// does not end verified and installed. The first record of a path keeps what
// it held before the span; the last keeps what the span itself last wrote, so
// the undo leaves a file another writer changed since as found
// (turnFiles.restore) -- the user's concurrent edit is never reverted.
//
// The journal also keeps the span's last failing tests, so the caller can say
// what the attempt broke without parsing its error text.

type writeJournalKey struct{}

// WithWriteJournal returns ctx under which every turn records its writes to j.
func WithWriteJournal(ctx context.Context, j *WriteJournal) context.Context {
	if j == nil {
		return ctx
	}
	return context.WithValue(ctx, writeJournalKey{}, j)
}

// WriteJournalFrom is the journal ctx carries, or nil.
func WriteJournalFrom(ctx context.Context) *WriteJournal {
	if ctx == nil {
		return nil
	}
	j, _ := ctx.Value(writeJournalKey{}).(*WriteJournal)
	return j
}

// WriteJournal is safe for concurrent use: a verified delegation's attempts
// and a turn's nested turns record into it from their own goroutines.
type WriteJournal struct {
	mu sync.Mutex
	// entries is keyed by workspace, then by the turn's own path key
	// (workspace-relative slash form, as PreWriteContents keys it).
	entries  map[string]map[string]PreImage
	failures []string
}

// NewWriteJournal starts an empty journal.
func NewWriteJournal() *WriteJournal {
	return &WriteJournal{entries: map[string]map[string]PreImage{}}
}

// Record merges one turn's writes: the paths it wrote and their preimages,
// each carrying the hash of what the turn last wrote there. The first
// preimage of a path is what it held before the span; the last write hash is
// the latest turn's.
func (j *WriteJournal) Record(workspace string, written []string, pre map[string]PreImage) {
	if j == nil || len(written) == 0 {
		return
	}
	workspace = filepath.Clean(workspace)
	j.mu.Lock()
	defer j.mu.Unlock()
	paths := j.entries[workspace]
	if paths == nil {
		paths = map[string]PreImage{}
		j.entries[workspace] = paths
	}
	for _, p := range written {
		img, ok := preImageFor(workspace, p, pre)
		if !ok {
			// A write with no preimage cannot be undone; it is named as
			// unknown, never guessed at.
			img = PreImage{Unknown: "the turn recorded no preimage for it"}
		}
		first, seen := paths[p]
		if !seen {
			paths[p] = img
			continue
		}
		first.LastWriteHash, first.LastWriteAbsent = img.LastWriteHash, img.LastWriteAbsent
		paths[p] = first
	}
}

// NoteTestFailures replaces the span's failing tests with the latest turn's:
// one line per failing test, "<package>.<Test>: <file>:<line>: <message>".
func (j *WriteJournal) NoteTestFailures(lines []string) {
	if j == nil {
		return
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	j.failures = append([]string(nil), lines...)
}

// TestFailures is the span's last failing tests.
func (j *WriteJournal) TestFailures() []string {
	if j == nil {
		return nil
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	return append([]string(nil), j.failures...)
}

// Paths is every path the span wrote, workspace-relative, sorted.
func (j *WriteJournal) Paths() []string {
	if j == nil {
		return nil
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	var out []string
	for _, paths := range j.entries {
		for p := range paths {
			out = append(out, p)
		}
	}
	sort.Strings(out)
	return out
}

// JournalUndo is what undoing a span did.
type JournalUndo struct {
	// Patch is the span's whole diff, taken before anything was put back.
	Patch string
	// Restored are the paths put back as the span found them.
	Restored []string
	// Left are the paths left as found, each with why: another writer
	// changed it since the span wrote it, or what it held before is unknown.
	Left []string
	// Err joins every restore that failed.
	Err error
}

// Undo renders the span's diff and puts every path it wrote back as the span
// found it, except a path another writer changed since: that one is left as
// found and named. The journal is emptied: what it recorded has been settled.
func (j *WriteJournal) Undo() JournalUndo {
	if j == nil {
		return JournalUndo{}
	}
	j.mu.Lock()
	entries := j.entries
	j.entries = map[string]map[string]PreImage{}
	j.mu.Unlock()

	var out JournalUndo
	var patches []string
	var errs []string
	workspaces := make([]string, 0, len(entries))
	for ws := range entries {
		workspaces = append(workspaces, ws)
	}
	sort.Strings(workspaces)
	for _, ws := range workspaces {
		res := &ExecutionResult{PreWriteContents: map[string]PreImage{}}
		for p, img := range entries[ws] {
			if !img.Known() {
				out.Left = append(out.Left, p+" (what it held before is unknown: "+img.Unknown+")")
				continue
			}
			res.WrittenPaths = append(res.WrittenPaths, p)
			res.PreWriteContents[p] = img
		}
		sort.Strings(res.WrittenPaths)
		if len(res.WrittenPaths) == 0 {
			continue
		}
		if patch := turnDiffPatch(ws, res.WrittenPaths, res.PreWriteContents); strings.TrimSpace(patch) != "" {
			patches = append(patches, patch)
		}
		restored, conflicts, err := turnFiles{pre: map[string]PreImage{}}.restore(ws, res)
		out.Restored = append(out.Restored, restored...)
		for _, c := range conflicts {
			out.Left = append(out.Left, c+" (changed since the attempt wrote it)")
		}
		if err != nil {
			errs = append(errs, err.Error())
		}
	}
	sort.Strings(out.Restored)
	sort.Strings(out.Left)
	out.Patch = strings.Join(patches, "\n")
	if len(errs) > 0 {
		out.Err = fmt.Errorf("undo the attempt's writes: %s", strings.Join(errs, "; "))
	}
	return out
}

// journalTurn records a finished turn into the journal on ctx, if any.
func journalTurn(ctx context.Context, workspace string, result *ExecutionResult) {
	j := WriteJournalFrom(ctx)
	if j == nil || result == nil {
		return
	}
	j.Record(workspace, result.WrittenPaths, result.PreWriteContents)
	if lines := turnTestFailureLines(result); lines != nil {
		j.NoteTestFailures(lines)
	}
}

// turnTestFailureLines is the turn's failing tests, own run then importers,
// or nil when neither run failed.
func turnTestFailureLines(result *ExecutionResult) []string {
	var lines []string
	failed := false
	for _, v := range []TestVerification{result.TestCheck, result.ImporterCheck} {
		if v.Verdict() != VerifyFailed {
			continue
		}
		failed = true
		lines = append(lines, testFailureLines(v.Result)...)
	}
	if !failed {
		return nil
	}
	return lines
}

// testFailureLines renders a run's first failure per test as one line each.
func testFailureLines(res *testfacts.Result) []string {
	var out []string
	for _, f := range res.FirstFailures() {
		name := f.Package
		if f.Test != "" {
			name += "." + f.Test
		}
		where := ""
		if f.File != "" {
			where = fmt.Sprintf("%s:%d: ", f.File, f.Line)
		}
		out = append(out, name+": "+where+f.Message)
	}
	return out
}
