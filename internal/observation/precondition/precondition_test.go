package precondition

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"codenerd/internal/retain"
)

const source = `package widget

import "fmt"

func Encode(v int) int {
	return v + 1
}

func caller() int {
	return Encode(1)
}
`

// readOf is a read of lines 5-7 of source: the body of Encode.
func readOf(content string) Read {
	return Read{Path: "widget.go", Content: content, Start: 5, End: 7}
}

func mustVerify(t *testing.T, s *Store, handle, path string, current string) Verification {
	t.Helper()
	v, err := s.Verify(handle, path, []byte(current))
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	return v
}

func TestVerify_WhenNothingChanged_ShouldHoldOnBothCounts(t *testing.T) {
	t.Parallel()

	s := New(retain.DefaultConfig())
	handle := s.Mint(readOf(source))
	if handle == "" {
		t.Fatal("a read must mint a handle, or no edit can be checked against it")
	}

	v := mustVerify(t, s, handle, "widget.go", source)
	if !v.RegionIntact || !v.FileIntact {
		t.Errorf("an unchanged file must satisfy both digests; got region=%v file=%v", v.RegionIntact, v.FileIntact)
	}
}

// TestVerify_WhenAnUnrelatedPartChanged_ShouldStillPermitTheEdit is the reason
// the precondition is the region rather than the whole file.
//
// A whole-file precondition is safe and unusable: an agent making three edits to
// one file would have its second and third refused, every time, on the strength
// of its own first edit. The region has to survive a change somewhere else.
func TestVerify_WhenAnUnrelatedPartChanged_ShouldStillPermitTheEdit(t *testing.T) {
	t.Parallel()

	s := New(retain.DefaultConfig())
	handle := s.Mint(readOf(source))

	// Rewrite caller(), which is below the region and the same number of lines.
	changed := strings.Replace(source, "return Encode(1)", "return Encode(9)", 1)

	v := mustVerify(t, s, handle, "widget.go", changed)
	if !v.RegionIntact {
		t.Errorf("a change outside the region must not fail the precondition, or every second edit to one file is refused")
	}
	if v.FileIntact {
		t.Errorf("the file digest must notice the change; reporting the file unchanged hides that other conclusions went stale")
	}
	if !strings.Contains(v.Explain(), "the rest of the file changed") {
		t.Errorf("an intact region in a changed file must say which is which:\n%s", v.Explain())
	}
}

func TestVerify_WhenTheRegionItselfChanged_ShouldRefuse(t *testing.T) {
	t.Parallel()

	s := New(retain.DefaultConfig())
	handle := s.Mint(readOf(source))

	changed := strings.Replace(source, "return v + 1", "return v * 2", 1)

	v := mustVerify(t, s, handle, "widget.go", changed)
	if v.RegionIntact {
		t.Fatal("the lines the reasoning was built on were rewritten and the precondition still held; an edit on them lands on content nobody looked at")
	}
	if !strings.Contains(v.Explain(), "FAILED") {
		t.Errorf("a failed precondition must say so plainly:\n%s", v.Explain())
	}
}

// TestVerify_WhenTextMovedWithoutChanging_ShouldRefuseAndSayWhereItWent is the
// case a content-only window hash gets wrong.
//
// Hashing only the region's bytes, a region that moved hashes identically to
// one that did not. An insertion above it would then verify clean and a
// line-addressed edit would be applied at coordinates now pointing at something
// else — silently, which is the worst way for this to fail.
func TestVerify_WhenTextMovedWithoutChanging_ShouldRefuseAndSayWhereItWent(t *testing.T) {
	t.Parallel()

	s := New(retain.DefaultConfig())
	handle := s.Mint(readOf(source))

	// Four lines inserted above the region. Every byte of the region survives;
	// only its address changed.
	moved := strings.Replace(source, "package widget\n", "package widget\n\n// one\n// two\n// three\n", 1)

	v := mustVerify(t, s, handle, "widget.go", moved)
	if v.RegionIntact {
		t.Fatal("text that moved verified as unmoved; an edit addressed at lines 5-7 would now land four lines above where the reasoning was")
	}
	if v.MovedTo != 9 {
		t.Errorf("moved-to line = %d, want 9: a refusal that cannot say where the text went costs a re-read of the whole file", v.MovedTo)
	}
	if !strings.Contains(v.Explain(), "+4") {
		t.Errorf("the refusal must name the shift so the caller can retry rather than restart:\n%s", v.Explain())
	}
}

func TestVerify_WhenTheFileGotShorterThanTheRegion_ShouldRefuse(t *testing.T) {
	t.Parallel()

	s := New(retain.DefaultConfig())
	handle := s.Mint(readOf(source))

	v := mustVerify(t, s, handle, "widget.go", "package widget\n")
	if v.RegionIntact {
		t.Error("a file too short to hold the region satisfied the precondition; the coordinates address nothing at all now")
	}
}

// TestVerify_ShouldNotReadTheFileItself pins the structural half of the
// guarantee: the "before" side comes from the retained bytes, and the "after"
// side is whatever the caller hands over — which is the same buffer the caller
// is about to edit. A store that opened the file itself would compare two
// instants and prove nothing about the third one the edit lands on.
func TestVerify_ShouldNotReadTheFileItself(t *testing.T) {
	t.Parallel()

	s := New(retain.DefaultConfig())
	handle := s.Mint(readOf(source))

	// Bytes that exist nowhere on disk. If Verify consulted a filesystem, this
	// could not be the thing it compared against.
	invented := strings.Replace(source, "func caller", "func invented", 1)
	v := mustVerify(t, s, handle, "widget.go", invented)
	if !v.RegionIntact {
		t.Error("verification did not compare against the caller's bytes")
	}
	if v.FileIntact {
		t.Error("verification did not notice a difference present only in the caller's bytes")
	}
}

// TestStore_ShouldHoldNothingItCouldReadAFileWith is the reflection guard the
// code-search codec has for the same reason. The comment on Store says
// verification cannot consult the live world because the type has nothing to
// consult it with; a later edit adding a workspace root, a file reader or a
// clock would quietly make that false while every other test still passed.
func TestStore_ShouldHoldNothingItCouldReadAFileWith(t *testing.T) {
	t.Parallel()

	typ := reflect.TypeOf(Store{})
	if typ.NumField() != 1 {
		t.Fatalf("Store has %d fields; it must hold only its retention, or verification can grow a way to read the file itself", typ.NumField())
	}
	if got := typ.Field(0).Type; got != reflect.TypeOf((*retain.Store)(nil)) {
		t.Fatalf("Store's only field is %s, want *retain.Store", got)
	}
}

func TestVerify_WhenHandleIsForAnotherFile_ShouldRefuseRatherThanCheck(t *testing.T) {
	t.Parallel()

	s := New(retain.DefaultConfig())
	handle := s.Mint(readOf(source))

	_, err := s.Verify(handle, "other.go", []byte(source))
	if !errors.Is(err, ErrWrongFile) {
		t.Fatalf("error = %v, want ErrWrongFile: a precondition taken from one file must not be checked against another, or an edit claims safety it was never granted", err)
	}
	if !strings.Contains(err.Error(), "widget.go") {
		t.Errorf("the refusal must name the file the handle came from:\n%v", err)
	}
}

func TestVerify_WhenHandleIsUnknownOrEmpty_ShouldSaySoDistinctly(t *testing.T) {
	t.Parallel()

	s := New(retain.Config{TTL: time.Minute})

	if _, err := s.Verify("obs:fr:deadbeef0000", "widget.go", []byte(source)); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown handle error = %v, want ErrNotFound: the caller's answer is to read the file again, not to retry the check", err)
	}
	if _, err := s.Verify("   ", "widget.go", []byte(source)); !errors.Is(err, ErrNotFound) {
		t.Errorf("empty handle error = %v, want ErrNotFound", err)
	}
}

func TestMint_WhenTheSameReadRepeats_ShouldReuseOneHandle(t *testing.T) {
	t.Parallel()

	s := New(retain.DefaultConfig())
	first := s.Mint(readOf(source))
	second := s.Mint(readOf(source))

	if first == "" || first != second {
		t.Errorf("handles = %q and %q, want one content-addressed handle: a handle quoted back from an earlier turn has to still resolve while the same read is live",
			first, second)
	}
}

func TestEnforce_WhenNoPreconditionIsGiven_ShouldNotBlockTheEdit(t *testing.T) {
	t.Parallel()

	warning, err := Enforce(map[string]any{}, "widget.go", []byte(source))
	if err != nil || warning != "" {
		t.Errorf("an edit with no precondition must proceed silently; got warning=%q err=%v", warning, err)
	}
}

// TestEnforce_WhenTheHandleCannotBeResolved_ShouldRefuse protects against the
// worst reading of an explicit safety argument: treating an expired handle as
// "no precondition given" silently downgrades an edit the caller asked to have
// checked into an unchecked one.
func TestEnforce_WhenTheHandleCannotBeResolved_ShouldRefuse(t *testing.T) {
	t.Parallel()

	_, err := Enforce(map[string]any{Arg: "obs:fr:000000000000"}, "widget.go", []byte(source))
	if err == nil {
		t.Fatal("an unresolvable precondition was ignored; the caller believes the edit was checked and it was not")
	}
	if !strings.Contains(err.Error(), "Read widget.go again") {
		t.Errorf("the refusal must name the recovery:\n%v", err)
	}
}

func TestEnforce_ShouldRefuseAStaleRegionAndOnlyWarnOnAChangedFile(t *testing.T) {
	t.Parallel()

	handle := Shared().Mint(Read{
		Path:    "enforce_test_widget.go",
		Content: source,
		Start:   5,
		End:     7,
	})
	args := map[string]any{Arg: handle}

	stale := strings.Replace(source, "return v + 1", "return v * 2", 1)
	if _, err := Enforce(args, "enforce_test_widget.go", []byte(stale)); err == nil {
		t.Error("an edit whose region changed was allowed through")
	}

	elsewhere := strings.Replace(source, "return Encode(1)", "return Encode(9)", 1)
	warning, err := Enforce(args, "enforce_test_widget.go", []byte(elsewhere))
	if err != nil {
		t.Fatalf("a change outside the region must not refuse the edit: %v", err)
	}
	if warning == "" {
		t.Error("a change outside the region must still be reported; anything else concluded from that read may be stale")
	}
}

func TestShared_ShouldBeOneStoreForEveryCaller(t *testing.T) {
	t.Parallel()

	// A precondition is minted by read_file or the VirtualStore action and
	// checked by an edit verb in another package. Separate stores would make
	// every one of them unresolvable, and the agent would discover that by
	// having a correct edit refused.
	if Shared() != Shared() {
		t.Fatal("Shared returned two stores; a handle minted through one would not resolve through the other")
	}

	handle := Shared().Mint(Read{Path: "shared_test.go", Content: source, Start: 5, End: 7})
	if _, err := Shared().Verify(handle, "shared_test.go", []byte(source)); err != nil {
		t.Fatalf("a handle minted through Shared must resolve through Shared: %v", err)
	}
}

// TestSplitLines_ShouldCountTheWayTheEditVerbsCount is a boundary the whole
// mechanism rests on. edit_lines, insert_lines and delete_lines all split with
// a bare strings.Split, so a newline-terminated file has a final empty line they
// will happily address. A precondition that counted one line fewer would report
// coordinates those verbs resolve one line off — the very defect it exists to
// prevent, reintroduced by the thing preventing it.
func TestSplitLines_ShouldCountTheWayTheEditVerbsCount(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		content string
		want    int
	}{
		{"", 0},
		{"a", 1},
		{"a\n", 2},
		{"a\nb", 2},
		{"a\nb\n", 3},
	} {
		if got := len(SplitLines(tc.content)); got != tc.want {
			t.Errorf("SplitLines(%q) counted %d lines, want %d — the count edit_lines uses", tc.content, got, tc.want)
		}
		if got := len(strings.Split(tc.content, "\n")); tc.content != "" && got != tc.want {
			t.Errorf("strings.Split(%q) counted %d, SplitLines says %d; the two must not diverge", tc.content, got, tc.want)
		}
	}
}

func TestRegionDigest_ShouldNotBeSatisfiedByTheSameTextAtOtherCoordinates(t *testing.T) {
	t.Parallel()

	// Verify re-reads the same line range, so this is belt as well as braces:
	// it means a digest cannot be reused against a range nobody took it over.
	text := "func Encode(v int) int {"
	if regionDigest(40, 80, text) == regionDigest(54, 94, text) {
		t.Error("identical text at different coordinates produced the same digest; a moved region could then be checked as unmoved")
	}
}

func TestFileDigest_ShouldSeparateRevisions(t *testing.T) {
	t.Parallel()

	if fileDigest(source) == fileDigest(source+"\n") {
		t.Error("a trailing newline is a different revision of the file and must digest differently")
	}
	if a, b := fileDigest(source), fileDigest(source); a != b {
		t.Errorf("the same content digested to %s and %s", a, b)
	}
}

func TestMint_WhenTheStoreIsNil_ShouldReturnNoHandleRatherThanPanic(t *testing.T) {
	t.Parallel()

	var s *Store
	if got := s.Mint(readOf(source)); got != "" {
		t.Errorf("nil store minted %q; a read must survive having no retention behind it", got)
	}
	if _, err := s.Verify("obs:fr:x", "widget.go", nil); !errors.Is(err, ErrNotFound) {
		t.Errorf("nil store Verify error = %v, want ErrNotFound", err)
	}
}

func ExampleVerification_Explain() {
	v := Verification{
		Handle: "obs:fr:abc123", Path: "widget.go", Start: 5, End: 7,
		RegionIntact: true, FileIntact: true,
	}
	fmt.Println(v.Explain())
	// Output: precondition obs:fr:abc123 holds: widget.go is byte-identical to the read it was taken from
}

// TestVerify_WhenTheReadWasCapped_ShouldNotReportEveryFileAsChanged is a defect
// this had before the test existed.
//
// A capped read retains only a prefix. Digesting the whole of what is on disk
// now against that prefix can never match, so every edit after a capped read
// carried a spurious "the rest of the file changed" — a warning that is always
// present is a warning nobody reads, and it would have discredited the one case
// where the warning is real.
func TestVerify_WhenTheReadWasCapped_ShouldNotReportEveryFileAsChanged(t *testing.T) {
	t.Parallel()

	prefix := "package widget\n\nfunc Encode(v int) int {\n\treturn v + 1\n}\n"
	whole := prefix + "\nfunc neverRead() {}\n"

	s := New(retain.DefaultConfig())
	handle := s.Mint(Read{Path: "widget.go", Content: prefix, Start: 3, End: 5, Truncated: true})

	v := mustVerify(t, s, handle, "widget.go", whole)
	if !v.RegionIntact {
		t.Error("the region a capped read did see is unchanged and must verify")
	}
	if !v.FileIntact {
		t.Error("a capped read compared its prefix against the whole file and called that a change; every edit after one would carry a false warning")
	}
	if !strings.Contains(v.Explain(), "size cap") {
		t.Errorf("an all-clear from a capped read must not read as 'byte-identical', which it cannot know:\n%s", v.Explain())
	}
}

// TestExplain_WhenTheTextIsGoneEntirely_ShouldQuoteWhatItWas covers the branch
// where the caller has nothing else to go on: the lines changed and the text is
// not elsewhere in the file either. The retained copy is the only surviving
// record of what the reasoning was looking at, and a refusal that cannot say
// what that was leaves nothing to search for.
func TestExplain_WhenTheTextIsGoneEntirely_ShouldQuoteWhatItWas(t *testing.T) {
	t.Parallel()

	s := New(retain.DefaultConfig())
	handle := s.Mint(readOf(source))

	v := mustVerify(t, s, handle, "widget.go", "package widget\n\nfunc Other() {}\n\nfunc more() {}\n\nfunc yet() {}\n")
	if v.RegionIntact {
		t.Fatal("the region was replaced wholesale and still verified")
	}
	if v.MovedTo != 0 {
		t.Fatalf("moved-to = %d, want 0: that text is nowhere in the new content", v.MovedTo)
	}
	if !strings.Contains(v.Explain(), "func Encode(v int) int {") {
		t.Errorf("the refusal does not say what those lines used to be, so there is nothing to look for:\n%s", v.Explain())
	}
}

func TestFirstLine_ShouldStayQuotable(t *testing.T) {
	t.Parallel()

	if got := firstLine("\tfunc Encode() {\n\treturn\n}"); got != "func Encode() {" {
		t.Errorf("firstLine = %q, want the opening line trimmed", got)
	}
	long := strings.Repeat("x", 200)
	if got := firstLine(long); len(got) > 90 {
		t.Errorf("firstLine returned %d bytes; a refusal must not put the region back into context one line at a time", len(got))
	}
	if got := firstLine(""); got != "" {
		t.Errorf("firstLine(\"\") = %q, want empty", got)
	}
}
