package observation

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"codenerd/internal/observation/precondition"
)

const readSource = `package widget

import "fmt"

// Encode adds one.
func Encode(v int) int {
	fmt.Println(v)
	return v + 1
}

type Widget struct {
	Name string
}

func caller() int {
	total := Encode(1)
	return total
}
`

func readOf(start, end int) precondition.Read {
	return precondition.Read{Path: "widget.go", Content: readSource, Start: start, End: end}
}

// rawRead is what read_file returned before the codec: every line of the file,
// numbered. It is the baseline every cost claim here is measured against.
func rawRead(content string) string {
	return numberedRegion(content, 1)
}

func outlineNames(r FileReadResult) []string {
	out := make([]string, 0, len(r.Outline))
	for _, e := range r.Outline {
		out = append(out, e.Name)
	}
	return out
}

func TestProjectRead_WhenTheRangeLandsInsideAFunction_ShouldShowTheWholeFunction(t *testing.T) {
	t.Parallel()

	// Line 8 is `return v + 1`, inside Encode (lines 6-9). A read that showed
	// that line without the signature above it or the brace below it is how a
	// model composes an edit_lines call that drops a delimiter — which this
	// repo already refuses, one turn later, after the model has committed to it.
	r := ProjectRead(readOf(8, 8), ReadLimits{PadLines: 0})

	if r.Start > 6 || r.End < 9 {
		t.Fatalf("region = lines %d-%d, want it snapped out to cover Encode at 6-9", r.Start, r.End)
	}
	if !strings.Contains(r.Region, "func Encode(v int) int {") {
		t.Errorf("region omits the signature of the function being edited:\n%s", r.Region)
	}
	if !strings.Contains(r.Region, "}") {
		t.Errorf("region omits the closing brace of the function being edited:\n%s", r.Region)
	}
}

func TestProjectRead_ShouldOutlineOnlyWhatItDidNotPrint(t *testing.T) {
	t.Parallel()

	r := ProjectRead(readOf(6, 9), ReadLimits{PadLines: 0})

	names := outlineNames(r)
	for _, name := range names {
		if name == "Encode" {
			t.Errorf("Encode is printed in full and listed in the outline as well; the projection is paying for the same fact twice: %v", names)
		}
	}
	var sawCaller bool
	for _, name := range names {
		if name == "caller" {
			sawCaller = true
		}
	}
	if !sawCaller {
		t.Errorf("outline = %v, want the elements that were NOT shown; an elision that does not say what it hid reads as a whole file", names)
	}
}

func TestProjectRead_WhenTheWholeFileIsShown_ShouldOutlineNothing(t *testing.T) {
	t.Parallel()

	r := ProjectRead(precondition.Read{Path: "widget.go", Content: readSource}, ReadLimits{})

	if r.Elided != 0 {
		t.Fatalf("elided = %d, want 0: this file is far shorter than the region ceiling", r.Elided)
	}
	if len(r.Outline) != 0 {
		t.Errorf("outline = %v on a file shown in full; every one of those lines is a fact already on screen", outlineNames(r))
	}
}

func TestProjectRead_WhenTheFileIsLongerThanTheCeiling_ShouldElideAndSayHowMuch(t *testing.T) {
	t.Parallel()

	var src strings.Builder
	src.WriteString("package a\n")
	for i := 0; i < 60; i++ {
		fmt.Fprintf(&src, "\nfunc fn%d() {\n\treturn\n}\n", i)
	}
	content := src.String()

	r := ProjectRead(precondition.Read{Path: "a.go", Content: content}, ReadLimits{MaxRegionLines: 20})

	if r.End-r.Start+1 > 20 {
		t.Fatalf("region is %d lines, want no more than the 20-line ceiling", r.End-r.Start+1)
	}
	total := len(precondition.SplitLines(content))
	if r.Elided != total-(r.End-r.Start+1) {
		t.Errorf("elided = %d, want %d: a cap that hides its own omissions makes the projection look complete when it is not",
			r.Elided, total-(r.End-r.Start+1))
	}
	if len(r.Outline) == 0 {
		t.Error("a projection that elided most of a file must outline what it dropped, or the next read is a guess")
	}
}

func TestProjectRead_WhenTheOutlineExceedsItsLimit_ShouldCapAndSayHowMany(t *testing.T) {
	t.Parallel()

	var src strings.Builder
	src.WriteString("package a\n")
	for i := 0; i < 40; i++ {
		fmt.Fprintf(&src, "\nfunc fn%d() {\n\treturn\n}\n", i)
	}

	r := ProjectRead(precondition.Read{Path: "a.go", Content: src.String()}, ReadLimits{MaxRegionLines: 10, MaxOutline: 5})

	if len(r.Outline) != 5 {
		t.Fatalf("outline = %d entries, want the 5-entry cap", len(r.Outline))
	}
	if r.OutlineOmitted == 0 {
		t.Error("a capped outline that does not report its own omissions reads as the complete list of what is in the file")
	}
}

// TestProjectRead_WhenTheAnchorIsOverBudget_ShouldKeepTheAnchorAndDropContext
// protects the one thing a read cannot give up. The lines the caller asked
// about are why the read happened; surrendering those to keep the padding
// answers a question nobody asked.
func TestProjectRead_WhenTheAnchorIsOverBudget_ShouldKeepTheAnchorAndDropContext(t *testing.T) {
	t.Parallel()

	var src strings.Builder
	for i := 1; i <= 200; i++ {
		fmt.Fprintf(&src, "line %d\n", i)
	}

	r := ProjectRead(precondition.Read{Path: "a.txt", Content: src.String(), Start: 100, End: 110},
		ReadLimits{MaxRegionLines: 15, PadLines: 50})

	if r.Start > 100 || r.End < 110 {
		t.Fatalf("region = %d-%d, want it to still contain the requested 100-110 after the budget bit", r.Start, r.End)
	}
	if r.End-r.Start+1 > 15 {
		t.Fatalf("region is %d lines, over the 15-line ceiling", r.End-r.Start+1)
	}
}

// TestProjectRead_WhenLinesAreNotLineShaped_ShouldStillBound is the ceiling
// behind the ceiling. Four hundred lines of a minified bundle or a base64 asset
// is megabytes, and a line count alone would wave it straight through.
func TestProjectRead_WhenLinesAreNotLineShaped_ShouldStillBound(t *testing.T) {
	t.Parallel()

	var src strings.Builder
	for i := 0; i < 50; i++ {
		src.WriteString(strings.Repeat("x", 4000))
		src.WriteString("\n")
	}

	r := ProjectRead(precondition.Read{Path: "bundle.js", Content: src.String()}, ReadLimits{})

	if len(r.Region) > maxRegionBytes+4001 {
		t.Errorf("region is %d bytes against a %d-byte ceiling; a line count is not a cost bound", len(r.Region), maxRegionBytes)
	}
	if r.Elided == 0 {
		t.Error("a bounded projection of an oversized file must report that it dropped something")
	}
}

func TestProjectRead_ShouldProduceTheSameResultForTheSameObservation(t *testing.T) {
	t.Parallel()

	// codedom walks its pattern table with a map range, so element order is not
	// stable across runs. A projection that varied would make the omitted tail
	// of a capped outline a different set each turn, and would give the same
	// read two different handles.
	first := ProjectRead(readOf(6, 9), ReadLimits{})
	for i := 0; i < 25; i++ {
		if got := ProjectRead(readOf(6, 9), ReadLimits{}); !reflect.DeepEqual(got, first) {
			t.Fatalf("projection %d differs from the first for identical input:\n%+v\nvs\n%+v", i, got, first)
		}
	}
}

func TestProjectRead_WhenTheFileIsEmpty_ShouldSaySoRatherThanOfferLineOne(t *testing.T) {
	t.Parallel()

	r := ProjectRead(precondition.Read{Path: "empty.go", Content: ""}, ReadLimits{})
	if r.Lines != 0 {
		t.Errorf("an empty file has %d lines; reporting one invites an edit to a line that is not there", r.Lines)
	}
	if !strings.Contains(r.Text(), "(empty file)") {
		t.Errorf("an empty read must say so:\n%s", r.Text())
	}
}

func TestText_ShouldNumberTheRegionWithItsRealLineNumbers(t *testing.T) {
	t.Parallel()

	// A snapped region renumbered from 1 would look authoritative and be wrong
	// by the offset. Every file:line citation and every line-addressed edit in
	// this repo is built on these numbers.
	text := ProjectRead(readOf(15, 16), ReadLimits{PadLines: 0}).Text()

	if !strings.Contains(text, "15\tfunc caller() int {") {
		t.Errorf("region is not numbered at its real line:\n%s", text)
	}
	if strings.Contains(text, "1\tfunc caller() int {") {
		t.Errorf("region was renumbered from 1:\n%s", text)
	}
}

func TestText_ShouldNameThePreconditionAndHowToUseIt(t *testing.T) {
	t.Parallel()

	r := EncodeRead(precondition.Read{Path: "fileread_text_widget.go", Content: readSource, Start: 6, End: 9}, ReadLimits{})
	text := r.Text()

	if r.Handle == "" {
		t.Fatal("a read published no precondition, so no edit built on it can be checked")
	}
	for _, want := range []string{r.Handle, precondition.Arg + "=" + r.Handle, "edit_lines", "refused"} {
		if !strings.Contains(text, want) {
			t.Errorf("rendered read is missing %q; a precondition nobody is told how to use is a precondition nobody uses:\n%s", want, text)
		}
	}
}

func TestText_WhenTheReadWasCapped_ShouldNotLetTheReaderConcludeThatIsTheFile(t *testing.T) {
	t.Parallel()

	r := ProjectRead(precondition.Read{Path: "big.go", Content: readSource, Truncated: true}, ReadLimits{})
	if !strings.Contains(r.Text(), "size cap") {
		t.Errorf("a read that stopped at a size cap must say so, or everything below it reads as absent:\n%s", r.Text())
	}
}

// TestText_WhenNothingIsElided_ShouldCostOnlyAFixedHeaderAndFooter is the cost
// property, and it is the honest worst case rather than the flattering one.
//
// On a short file the codec elides nothing, so every byte it adds over the raw
// numbered read is pure overhead — one header line and one precondition line.
// That overhead must be a small constant. If it ever scaled with the file, the
// codec would be more expensive than the thing it replaced on the commonest
// call there is, which is decoration with a schema rather than a codec. This is
// the same failure the code-search codec shipped first and had to fix.
//
// Measured on this repository: 173 bytes, whether the file is 905 bytes
// (internal/tools/core/register.go, 1.20x) or 10092 (internal/retain/retain.go,
// 1.02x). Against the reads the codec exists for it is 0.20x on
// internal/session/executor.go read whole and 0.05x on twenty lines of it.
func TestText_WhenNothingIsElided_ShouldCostOnlyAFixedHeaderAndFooter(t *testing.T) {
	t.Parallel()

	small := readSource
	var bigger strings.Builder
	bigger.WriteString("package a\n")
	for i := 0; i < 100; i++ {
		fmt.Fprintf(&bigger, "\nfunc fn%d() {\n\treturn\n}\n", i)
	}

	overheadOf := func(name, content string) int {
		t.Helper()
		r := EncodeRead(precondition.Read{Path: name, Content: content}, ReadLimits{MaxRegionLines: 100000})
		if r.Elided != 0 {
			t.Fatalf("%s: this measurement is only meaningful when nothing was elided; elided=%d", name, r.Elided)
		}
		over := len(r.Text()) - len(rawRead(content))
		if over < 0 {
			t.Fatalf("%s: projection is shorter than the raw read with nothing elided, which cannot be right", name)
		}
		return over
	}

	smallOver := overheadOf("small_widget.go", small)
	biggerOver := overheadOf("bigger_widget.go", bigger.String())

	// Measured at 173 bytes. The headroom is for a longer path in the header,
	// not for another footer line: a second one would double the cost of the
	// case this codec is worst at.
	const ceiling = 220
	if smallOver > ceiling {
		t.Errorf("an unelided read costs %d bytes over the raw file; the header and precondition must stay under %d", smallOver, ceiling)
	}
	// The header carries the line count, so a longer file costs a few more
	// digits. Anything beyond that means the overhead scales with the input.
	if biggerOver-smallOver > 16 {
		t.Errorf("overhead grew from %d to %d bytes on a file %dx longer; a fixed header must not scale with the file",
			smallOver, biggerOver, len(bigger.String())/len(small))
	}
}

// TestText_WhenTheFileIsLong_ShouldCostFarLessThanTheWholeFile is the case the
// codec exists for. Without it, an oversized read is not delivered whole
// anyway: the tool-loop transcript clamps it head-and-tail, so the middle of
// the file vanishes leaving no trace of what was in it.
func TestText_WhenTheFileIsLong_ShouldCostFarLessThanTheWholeFile(t *testing.T) {
	t.Parallel()

	var src strings.Builder
	src.WriteString("package a\n")
	for i := 0; i < 400; i++ {
		fmt.Fprintf(&src, "\n// fn%d does a thing\nfunc fn%d(argument int) int {\n\treturn argument + %d\n}\n", i, i, i)
	}
	content := src.String()

	projected := len(ProjectRead(precondition.Read{Path: "long.go", Content: content}, ReadLimits{}).Text())
	raw := len(rawRead(content))

	if projected >= raw/2 {
		t.Errorf("projected read is %d bytes against %d raw; on a file this long the projection must be a fraction of it, not a rounding of it", projected, raw)
	}
}

func TestEncodeRead_WhenTheSameReadRepeats_ShouldReuseOneHandle(t *testing.T) {
	t.Parallel()

	r := precondition.Read{Path: "fileread_repeat_widget.go", Content: readSource, Start: 6, End: 9}
	first := EncodeRead(r, ReadLimits{})
	second := EncodeRead(r, ReadLimits{})

	if first.Handle == "" || first.Handle != second.Handle {
		t.Errorf("handles = %q and %q, want one content-addressed handle", first.Handle, second.Handle)
	}
}

func TestNumberedRegion_ShouldStartAtTheLineItWasGiven(t *testing.T) {
	t.Parallel()

	// Moved here from internal/tools/core when read_file's numbering became the
	// projection's job. A ranged read numbered from 1 is worse than no numbers
	// at all, because it looks authoritative.
	if got, want := numberedRegion("alpha\nbeta\ngamma", 1), "1\talpha\n2\tbeta\n3\tgamma"; got != want {
		t.Errorf("numberedRegion() = %q, want %q", got, want)
	}
	if got, want := numberedRegion("func validate()\n\treturn nil\n}", 200), "200\tfunc validate()\n201\t\treturn nil\n202\t}"; got != want {
		t.Errorf("numberedRegion() at offset = %q, want %q", got, want)
	}
	if got := numberedRegion("", 1); got != "" {
		t.Errorf("numberedRegion(\"\") = %q, want empty", got)
	}
	if got, want := numberedRegion("only", 7), "7\tonly"; got != want {
		t.Errorf("numberedRegion single line = %q, want %q", got, want)
	}
	// A newline-terminated file's final empty line is a line the edit verbs will
	// address, so it must be numbered as one.
	if got, want := numberedRegion("a\nb\n", 1), "1\ta\n2\tb\n3\t"; got != want {
		t.Errorf("numberedRegion with trailing newline = %q, want %q", got, want)
	}
}

// TestProjectRead_WhenTheFileIsOneEnormousLine_ShouldStillBound closes the last
// unbounded path. The line budget has nothing to trim on a file with no line
// breaks, and budgetRegion stops at one line by construction — so a minified
// bundle or a base64 asset, which is exactly one line of megabytes, walked
// straight through both ceilings and landed whole in context.
func TestProjectRead_WhenTheFileIsOneEnormousLine_ShouldStillBound(t *testing.T) {
	t.Parallel()

	oneLine := strings.Repeat("payload,", 40000)
	r := ProjectRead(precondition.Read{Path: "bundle.min.js", Content: oneLine}, ReadLimits{})

	if len(r.Region) > maxRegionBytes {
		t.Fatalf("region is %d bytes for a single-line file against a %d-byte ceiling", len(r.Region), maxRegionBytes)
	}
	if r.RegionCut == 0 {
		t.Error("a cut region that does not announce itself reads as the whole line, and a halved record reads as a whole one")
	}
	if !strings.Contains(r.Text(), "cut from the end") {
		t.Errorf("the rendered read must say the text was cut:\n%s", firstLine(r.Text()))
	}
}

func TestTrimToRune_ShouldNotLeaveAPartialCharacter(t *testing.T) {
	t.Parallel()

	// A cut inside a multi-byte character leaves bytes no consumer can decode.
	full := "héllo"
	cut := full[:2] // 'h' plus the first byte of 'é'
	if got := trimToRune(cut); got != "h" {
		t.Errorf("trimToRune(%q) = %q, want %q", cut, got, "h")
	}
	if got := trimToRune(full); got != full {
		t.Errorf("trimToRune trimmed valid text: %q", got)
	}
}

func firstLine(s string) string {
	if idx := strings.IndexByte(s, '\n'); idx >= 0 {
		return s[:idx]
	}
	return s
}
