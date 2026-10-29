// Package precondition retains what a file read observed, so that an edit made
// later can be proved to rest on it.
//
// A read whose result has gone stale is worse than no read at all. With no
// read, a model asks. With a stale one it edits, confidently, a file it no
// longer understands: line coordinates from before an insertion now address
// different code, and a replacement anchored on text somebody already changed
// either fails to match or matches somewhere nobody looked. This repo has the
// scar — see lineShiftNotice in internal/tools/codedom/lines.go, added after two
// edits from stale offsets produced duplicate declarations in one file.
//
// # What the precondition covers, and why it is two answers
//
// Two digests are kept, and they are deliberately not collapsed into one
// verdict, because the honest answers to "has the file changed" are different
// answers with different remedies.
//
// The REGION digest covers the exact bytes of the lines that were read, bound
// to the line numbers they were read at. It is the precondition proper. It
// fails when the text at those coordinates is not what the reasoning was built
// on — including when an insertion above pushed that text somewhere else, since
// verification re-reads the same line range and the moved text is no longer
// there. It deliberately does NOT fail for a change elsewhere in the file,
// because that is the ordinary case and refusing it would be unusable: an agent
// making three edits to one file would have its second and third refused, every
// time, on the strength of its own first edit.
//
// The FILE digest covers the whole file as read. It cannot be the precondition
// for the reason just given, and it is not merely decorative either: with the
// region intact and the file changed, the edit is sound but everything else the
// agent concluded from that read may not be. That is worth saying, and it is a
// warning rather than a refusal.
//
// The alternative — a single whole-file precondition — is safe and unusable.
// The alternative — a single content-only region hash — is usable and unsound:
// a region that moved hashes identically to one that did not, so an insertion
// above it would verify clean and a line-addressed edit would then be applied
// at coordinates that now point at something else.
//
// # Why this is its own package
//
// The projection half of the file-read codec lives in internal/observation and
// needs internal/tools/codedom to find the code elements a region should be
// snapped out to. The verbs that must CHECK a precondition — edit_lines,
// insert_lines and delete_lines — live in that same codedom package. One
// package cannot import another that imports it, and a precondition the
// line-addressed edit verbs cannot reach would leave unprotected the exact
// verbs whose staleness bug is documented above. Retention and verification
// therefore live here, depending on nothing but internal/retain, and both sides
// import this.
package precondition

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"

	"codenerd/internal/retain"
)

// Read is the raw observation a file read produced: the whole file exactly as
// it was at that moment, plus the region the caller asked about.
//
// The whole file is retained rather than the region alone. It is the only
// surviving record of the world the reasoning was built on once the file moves
// on, which is precisely the situation a failed precondition announces — and a
// refusal that cannot say what the text used to be sends the agent back to
// re-read a file whose old contents nobody kept.
type Read struct {
	// Path is the file's IDENTITY: the absolute, symlink-resolved path every
	// verb already computes on its way through the containment guard. It is
	// deliberately not the pretty name below, because identity has to be one
	// string whoever asks. read_file, edit_file and the three line verbs resolve
	// against the session workspace while the VirtualStore action resolves
	// against its own execution working directory, which is allowed to be a
	// subdirectory of it — so a workspace-relative name is two different strings
	// for one file, and a precondition is refused outright when the names
	// disagree. That refusal is safe, but it would be a refusal for a file
	// nobody touched.
	Path string `json:"path"`
	// Display is that pretty name, and it reaches only the rendered projection.
	// Echoing absolute paths back at the model costs the same long prefix on
	// every read and teaches it to cite files that way; the code-search codec
	// next door avoids it for the same reason. Empty falls back to Path.
	Display string `json:"display,omitempty"`
	Content string `json:"content"`
	// Start and End are the 1-indexed inclusive lines the caller asked about.
	// Zero on either means unset, and an unset range is the whole file: an
	// unanchored read asked about everything in it.
	Start int `json:"start,omitempty"`
	End   int `json:"end,omitempty"`
	// Truncated records that the read itself stopped early — the VirtualStore
	// action caps at 100 KB — so a precondition taken from a partial read is
	// not mistaken for one covering the file.
	Truncated bool `json:"truncated,omitempty"`
}

// Name is how a read refers to its file in anything a person or a model reads.
func (r Read) Name() string {
	if r.Display != "" {
		return r.Display
	}
	return r.Path
}

// Verification is the answer to "is the file still what I read?".
type Verification struct {
	Handle string `json:"handle"`
	Path   string `json:"path"`
	Start  int    `json:"start"`
	End    int    `json:"end"`

	// RegionIntact means lines Start..End now hold exactly the bytes they held
	// when they were read. An edit addressed at those coordinates is operating
	// on the text the reasoning was built from.
	RegionIntact bool `json:"region_intact"`
	// FileIntact means the whole file is byte-identical to what was read.
	// False alongside RegionIntact is the ordinary, safe case: something else
	// changed, often this same agent one edit earlier.
	FileIntact bool `json:"file_intact"`

	// MovedTo is the 1-indexed line where the region's text now begins, when it
	// moved intact and can be found exactly once. Zero means it was not found,
	// or was found in several places where naming one would be a guess. A
	// refusal that says where the text went costs one retry; a refusal that
	// only says "changed" costs a re-read of the whole file.
	MovedTo int `json:"moved_to,omitempty"`

	// RegionAsRead is the source of Start..End as it was observed. It comes out
	// of the retained bytes and never off disk.
	RegionAsRead string `json:"region_as_read,omitempty"`
	// Truncated carries forward that the originating read stopped early, so a
	// precondition covering the first 100 KB of a file is not read as covering
	// the file.
	Truncated bool `json:"truncated,omitempty"`
}

// ErrNotFound is returned for an unknown or expired handle. It is distinct
// because the caller's correct response differs from every other failure: read
// the file again, do not retry the check.
var ErrNotFound = retain.ErrNotFound

// ErrWrongFile is returned when a precondition is checked against a file it was
// not taken from. It is distinct from ErrNotFound because the handle is
// perfectly good — it is the pairing that is wrong — and because the message
// already says everything the caller needs, so Enforce passes it through
// instead of wrapping it in advice about expiry.
var ErrWrongFile = errors.New("precondition belongs to another file")

// retentionKind labels retained payloads. It is part of the content address, so
// identical bytes retained by the code-search codec get a different id and a
// handle from one can never resolve through the other.
const retentionKind = "file_read"

// HandlePrefix marks a precondition handle, and marks it as visibly not a
// code-search handle: the two are redeemed by different verbs, and a model that
// cannot tell them apart spends a turn learning which it is holding.
const HandlePrefix = "obs:fr:"

// Store retains file reads and checks preconditions against them.
//
// It holds a retain.Store and nothing else, on purpose. Every field on this
// type is something Verify could reach, and the one guarantee verification has
// to make is that the "before" side comes from the retained bytes rather than
// from the world as it now stands. A path root, a file reader or a clock stored
// here would each be a way for that to be quietly lost in a later edit; with
// none of them present, reading a file is not a thing this type can do.
type Store struct {
	store *retain.Store
}

// New creates a precondition store over its own retention.
func New(cfg retain.Config) *Store {
	cfg.Prefix = HandlePrefix
	return &Store{store: retain.New(cfg)}
}

var (
	sharedOnce sync.Once
	shared     *Store
)

// Shared returns the process-wide precondition store.
//
// One store, not one per caller. A precondition is minted by whichever path
// read the file — the read_file tool or the VirtualStore action — and checked
// by an edit verb in a different package entirely. Separate stores would make
// every precondition unresolvable, and the agent would discover that by having
// a correct edit refused.
func Shared() *Store {
	sharedOnce.Do(func() {
		shared = New(retain.DefaultConfig())
	})
	return shared
}

// Mint retains a read and returns the handle that checks against it, or "" when
// nothing was retained.
//
// Failing to retain is not an error the caller has to handle. The read itself
// is still correct and still useful; it simply cannot be checked later, and an
// empty handle says so. Refusing to return the source because the checkable
// copy did not fit would be a strictly worse trade.
func (s *Store) Mint(r Read) string {
	if s == nil {
		return ""
	}
	payload, err := json.Marshal(r)
	if err != nil {
		// A struct of strings, ints and a bool cannot fail to marshal today.
		// If a future field makes it possible, the caller must still get its
		// read back.
		return ""
	}
	return s.store.Mint(retentionKind, payload)
}

// Verify checks a precondition against the file as it stands now.
//
// current is an argument rather than something this type reads, and that is
// load-bearing twice over. It keeps Store structurally unable to consult the
// live world, the same guarantee the code-search codec makes about hydration.
// And it means the bytes checked are the bytes the caller is about to edit: a
// store that opened the file itself would be comparing one instant against
// another, and the edit could still land on content neither of them saw.
func (s *Store) Verify(handle, path string, current []byte) (Verification, error) {
	if s == nil {
		return Verification{}, ErrNotFound
	}
	handle = strings.TrimSpace(handle)
	if handle == "" {
		return Verification{}, fmt.Errorf("%w: empty precondition handle", ErrNotFound)
	}

	kind, payload, err := s.store.Get(handle)
	if err != nil {
		return Verification{}, err
	}
	if kind != retentionKind {
		return Verification{}, fmt.Errorf("%w: %s is not a file-read precondition", ErrNotFound, handle)
	}

	var r Read
	if err := json.Unmarshal(payload, &r); err != nil {
		return Verification{}, fmt.Errorf("retained read %s is unreadable: %w", handle, err)
	}

	// A precondition taken from another file is refused rather than checked.
	// Silently verifying a handle for A against file B would let an edit to B
	// claim the safety of a read of A — worse than having no precondition,
	// because it reads as proof.
	if path != "" && r.Path != "" && path != r.Path {
		return Verification{}, fmt.Errorf(
			"%w: %s was taken from %s, not the file being edited; read that file before editing it",
			ErrWrongFile, handle, r.Name())
	}

	observed := SplitLines(r.Content)
	start, end := ClampRegion(r.Start, r.End, len(observed))
	regionText := Region(observed, start, end)

	live := SplitLines(string(current))
	// A file now shorter than the region cannot be holding it. That is a
	// mismatch, not an out-of-range failure: the coordinates the reasoning is
	// carrying no longer address anything.
	liveText := ""
	inRange := end <= len(live)
	if inRange {
		liveText = Region(live, start, end)
	}

	// A capped read saw only a prefix of the file, so digesting the whole of
	// what is there now against that prefix could never match — every edit
	// after a capped read would carry a spurious "the rest of the file
	// changed". The honest comparison is over the part that was actually seen,
	// and Explain says that is what it was.
	seen := string(current)
	if r.Truncated && len(seen) > len(r.Content) {
		seen = seen[:len(r.Content)]
	}

	v := Verification{
		Handle:       handle,
		Path:         r.Name(),
		Start:        start,
		End:          end,
		RegionIntact: inRange && regionDigest(start, end, liveText) == regionDigest(start, end, regionText),
		FileIntact:   fileDigest(seen) == fileDigest(r.Content),
		RegionAsRead: regionText,
		Truncated:    r.Truncated,
	}

	if !v.RegionIntact && regionText != "" {
		// The region may still exist, just somewhere else — an insertion above
		// it is much the commonest way a precondition fails. Saying where it
		// went turns "your read is stale" into "add fourteen to your line
		// numbers", which is a retry rather than a restart. Only an unambiguous
		// single occurrence is reported: naming one of several would send the
		// next edit to a location nobody chose.
		if body := string(current); strings.Count(body, regionText) == 1 {
			v.MovedTo = strings.Count(body[:strings.Index(body, regionText)], "\n") + 1
		}
	}
	return v, nil
}

// Explain renders a verification as the sentence an edit verb hands back.
//
// The wording differs by verdict on purpose. "The file changed" is true of both
// failure shapes and actionable in neither: one of them means re-read before
// editing, the other means the edit is fine and something else you concluded is
// stale.
func (v Verification) Explain() string {
	switch {
	case v.RegionIntact && v.FileIntact:
		// "Byte-identical" would be a lie about a capped read, which only ever
		// saw a prefix — and a false all-clear is worse than a caveat.
		if v.Truncated {
			return fmt.Sprintf("precondition %s holds: everything of %s that read saw is unchanged, though it stopped at its size cap and never saw the rest",
				v.Handle, v.Path)
		}
		return fmt.Sprintf("precondition %s holds: %s is byte-identical to the read it was taken from",
			v.Handle, v.Path)
	case v.RegionIntact:
		return fmt.Sprintf(
			"precondition %s holds for lines %d-%d of %s, but the rest of the file changed after that read. "+
				"The edit is addressed at text that has not moved; anything else you concluded from that read may be stale",
			v.Handle, v.Start, v.End, v.Path)
	case v.MovedTo > 0:
		return fmt.Sprintf(
			"precondition %s FAILED: lines %d-%d of %s no longer hold what was read — that text now starts at line %d. "+
				"Shift your line numbers by %+d, or read %s again before editing it",
			v.Handle, v.Start, v.End, v.Path, v.MovedTo, v.MovedTo-v.Start, v.Path)
	default:
		// The retained copy is the only surviving record of what those lines
		// said, and this is the branch where the caller has nothing else to go
		// on: the text is not where it was and is not anywhere else either.
		// One line of it is enough to search for, and cheap; the whole region
		// would put back the bytes the codec spent the read eliding.
		return fmt.Sprintf(
			"precondition %s FAILED: lines %d-%d of %s changed after they were read, and that text is nowhere else in the file "+
				"(it began %q). Read %s again before editing it — an edit built on a stale read lands on content nobody looked at",
			v.Handle, v.Start, v.End, v.Path, firstLine(v.RegionAsRead), v.Path)
	}
}

// firstLine is the opening line of the region as it was read, trimmed to
// something quotable.
func firstLine(region string) string {
	line := region
	if idx := strings.IndexByte(line, '\n'); idx >= 0 {
		line = line[:idx]
	}
	line = strings.TrimSpace(line)
	const maxQuoted = 80
	if len(line) > maxQuoted {
		line = line[:maxQuoted] + "..."
	}
	return line
}

// fileDigest identifies one revision of a whole file.
//
// Truncated to 64 bits deliberately. This detects concurrent modification, not
// forgery: nothing here defends against an adversary choosing colliding file
// contents, and carrying two full sha256 digests on every read would spend the
// context budget the codec above exists to protect.
func fileDigest(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])[:16]
}

// regionDigest binds a region's text to the coordinates it was read at.
//
// A region that moved must never verify as unmoved, or an insertion above it
// would pass and a line-addressed edit would then be applied at coordinates
// that now point at something else. Verify gets that by re-reading the same
// line range, where the moved text no longer is. The coordinates go inside the
// hash as well so the guarantee does not rest on the caller having picked the
// same range: a digest taken over lines 40-80 cannot be satisfied by identical
// text at lines 54-94.
func regionDigest(start, end int, text string) string {
	sum := sha256.New()
	fmt.Fprintf(sum, "%d:%d\n", start, end)
	sum.Write([]byte(text))
	return hex.EncodeToString(sum.Sum(nil))[:16]
}

// SplitLines splits source into lines the way every line-addressed verb in this
// repo counts them.
//
// The trailing empty element that strings.Split leaves on a newline-terminated
// file is KEPT, deliberately, even though sed and most editors would say such a
// file has one line fewer. edit_lines, insert_lines and delete_lines all split
// with a bare strings.Split and address that element as a real line, and
// read_file has always numbered it as one. A precondition that counted
// differently would hand back coordinates the edit verbs resolve one line off —
// which is the very defect it exists to prevent, reintroduced by the thing
// preventing it.
//
// An empty file is zero lines rather than one empty line, because there is no
// line there to address and "lines 1-1 of 1" would invite an edit to it.
func SplitLines(content string) []string {
	if content == "" {
		return nil
	}
	return strings.Split(content, "\n")
}

// Region returns lines lo..hi inclusive, 1-indexed, clamped to what exists.
func Region(lines []string, lo, hi int) string {
	if len(lines) == 0 {
		return ""
	}
	lo = max(1, lo)
	hi = min(len(lines), hi)
	if hi < lo {
		return ""
	}
	return strings.Join(lines[lo-1:hi], "\n")
}

// ClampRegion resolves a requested range against a file that may be shorter
// than the request. Zero on either end means unset, and an unset range is the
// whole file.
func ClampRegion(start, end, total int) (int, int) {
	if total <= 0 {
		return 1, 0
	}
	if start <= 0 {
		start = 1
	}
	if end <= 0 {
		end = total
	}
	start = min(start, total)
	if end < start {
		end = start
	}
	end = min(end, total)
	return start, end
}

// Arg is the tool argument every edit verb reads a precondition from, and
// ArgDescription is the schema text all of them show.
//
// Both are stated once so five schemas cannot drift into describing five
// slightly different guarantees, and so a read result cannot advertise an
// argument that some verb spells differently. A model told to pass an argument
// that is silently ignored believes it is protected and is not.
const (
	Arg = "precondition"

	ArgDescription = "Handle from a previous read_file of this same path, reported as 'precondition obs:fr:...'. " +
		"When given, this edit is refused if the lines that read returned have changed since. " +
		"Pass it whenever you have read the file; omit it only when you have not."
)

// Enforce checks a precondition supplied as a tool argument against the file a
// verb is about to modify.
//
// It returns a warning to append to the verb's own result, or an error that
// must abort the edit. The three outcomes are distinct on purpose: no argument
// is not a failure (the argument is optional and an agent that never read the
// file has nothing to check), a changed region is a refusal, and a changed file
// with an intact region is an edit that proceeds carrying a caveat.
//
// Every edit verb calls exactly this, so a fix to what "changed" means reaches
// all of them at once. Five copies of a staleness check is how one of them ends
// up comparing the wrong thing — the same shape of defect as the containment
// guard that protected file_ops and not codedom.
func Enforce(args map[string]any, path string, current []byte) (string, error) {
	raw, _ := args[Arg].(string)
	handle := strings.TrimSpace(raw)
	if handle == "" {
		return "", nil
	}

	v, err := Shared().Verify(handle, path, current)
	switch {
	case errors.Is(err, ErrWrongFile):
		// Already a complete sentence naming both files; wrapping it in advice
		// about expiry would send the caller after the wrong remedy.
		return "", err
	case err != nil:
		// An expired or unknown handle is refused rather than ignored. Treating
		// it as "no precondition given" would silently downgrade an edit the
		// caller asked to have checked into an unchecked one, which is the worst
		// possible reading of an explicit safety argument.
		return "", fmt.Errorf("cannot check precondition %s for %s: %w. Read %s again to take a fresh one",
			handle, path, err, path)
	}
	if !v.RegionIntact {
		return "", fmt.Errorf("%s", v.Explain())
	}
	if !v.FileIntact {
		return v.Explain(), nil
	}
	return "", nil
}
