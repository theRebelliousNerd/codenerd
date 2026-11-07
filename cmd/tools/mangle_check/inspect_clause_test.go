package main

import (
	"bytes"
	"codeberg.org/TauCeti/mangle-go/ast"
	"errors"
	"github.com/google/go-cmp/cmp"
	"io"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestMainPrintsFields(t *testing.T) {
	output := captureStdout(t, func() {
		main()
	})
	const want = "Field: Head\nField: HeadTime\nField: Premises\nField: Transform\n"
	if output != want {
		t.Errorf("main() output mismatch:\nwant %q\ngot  %q", want, output)
	}
	if direct := clauseFieldListing(); direct != want {
		t.Errorf("clauseFieldListing() mismatch:\nwant %q\ngot  %q", want, direct)
	}
	if output != clauseFieldListing() {
		t.Errorf("main() output mismatch with clauseFieldListing():\nmain %q\nlisting %q", output, clauseFieldListing())
	}
	if got := clauseFieldListingFor(reflect.TypeFor[ast.Clause]()); got != want {
		t.Errorf("clauseFieldListingFor(ast.Clause) mismatch:\nwant %q\ngot  %q", want, got)
	}
	if got := clauseFieldListingFor(nil); got != "" {
		t.Errorf("clauseFieldListingFor(nil) = %q, want empty string", got)
	}
	if got := clauseFieldListingFor(reflect.TypeFor[int]()); got != "" {
		t.Errorf("clauseFieldListingFor(int) = %q, want empty string for non-struct", got)
	}
}

func TestMainPrintsCompleteClauseLayout(t *testing.T) {
	clauseType := reflect.TypeFor[ast.Clause]()
	if clauseType.NumField() == 0 {
		t.Fatalf("expected ast.Clause to have fields, got none")
	}
	var sb strings.Builder
	for i := 0; i < clauseType.NumField(); i++ {
		sb.WriteString("Field: ")
		sb.WriteString(clauseType.Field(i).Name)
		sb.WriteString("\n")
	}
	listing := sb.String()

	tests := []struct {
		name        string
		invocations int
	}{
		{name: "single invocation", invocations: 1},
		{name: "repeated invocations", invocations: 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// main writes process-global stdout; these cases must stay serial.
			got := captureStdout(t, func() {
				for range tt.invocations {
					main()
				}
			})
			want := strings.Repeat(listing, tt.invocations)
			if diff := cmp.Diff(want, got); diff != "" {
				t.Errorf("main() output mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestCaptureStdoutRestoresStdout(t *testing.T) {
	orig := os.Stdout
	got := captureStdout(t, func() {
		if _, err := os.Stdout.WriteString("probe"); err != nil {
			t.Errorf("write to captured stdout: %v", err)
		}
	})
	if got != "probe" {
		t.Errorf("captureStdout() = %q, want %q", got, "probe")
	}
	if os.Stdout != orig {
		t.Errorf("captureStdout() did not restore os.Stdout")
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stdout pipe: %v", err)
	}
	t.Cleanup(func() {
		os.Stdout = orig
		if err := w.Close(); err != nil && !errors.Is(err, os.ErrClosed) {
			t.Errorf("close stdout writer: %v", err)
		}
		if err := r.Close(); err != nil && !errors.Is(err, os.ErrClosed) {
			t.Errorf("close stdout reader: %v", err)
		}
	})
	os.Stdout = w
	defer func() {
		os.Stdout = orig
	}()

	fn()

	if err := w.Close(); err != nil {
		t.Fatalf("close captured stdout: %v", err)
	}
	os.Stdout = orig

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("read captured stdout: %v", err)
	}
	if err := r.Close(); err != nil && !errors.Is(err, os.ErrClosed) {
		t.Fatalf("close captured stdout reader: %v", err)
	}
	return buf.String()
}

func TestMainPrintsFieldLinesWithPrefix(t *testing.T) {
	output := captureStdout(t, func() {
		main()
	})
	if output == "" {
		t.Fatalf("main() produced no output; expected Clause field listing")
	}
	if !strings.HasSuffix(output, "\n") {
		t.Fatalf("expected main() output to end with newline, got %q", output)
	}
	lines := strings.Split(strings.TrimSuffix(output, "\n"), "\n")
	if len(lines) == 0 {
		t.Fatalf("expected at least one field line, got %q", output)
	}
	seen := make(map[string]struct{}, len(lines))
	for i, line := range lines {
		if !strings.HasPrefix(line, "Field: ") {
			t.Errorf("line %d: expected %q prefix, got %q", i, "Field: ", line)
			continue
		}
		name := strings.TrimPrefix(line, "Field: ")
		if name == "" {
			t.Errorf("line %d has empty field name", i)
			continue
		}
		if strings.Contains(name, " ") {
			t.Errorf("line %d has unexpected space in field name %q", i, name)
		}
		if _, dup := seen[line]; dup {
			t.Errorf("duplicate field line %q", line)
		}
		seen[line] = struct{}{}
	}
}

func TestMainPrintsKnownClauseFieldsBatched(t *testing.T) {
	output := captureStdout(t, func() {
		main()
	})
	const want = "Field: Head\nField: HeadTime\nField: Premises\nField: Transform\n"
	if output != want {
		t.Errorf("main() output mismatch:\nwant %q\ngot  %q", want, output)
	}
	src, err := os.ReadFile("inspect_clause.go")
	if err != nil {
		t.Fatalf("read production source: %v", err)
	}
	text := string(src)
	if !strings.Contains(text, "strings.Builder") {
		t.Errorf("main() should batch output with strings.Builder")
	}
	if !strings.Contains(text, "NumField") {
		t.Errorf("main() should iterate with NumField")
	}
	if strings.Contains(text, ".Fields()") {
		t.Errorf("main() should not use the Fields() iterator path")
	}
	if strings.Contains(text, "Printf") {
		t.Errorf("main() should not print per-field with Printf; want single batched Print")
	}
}

func TestMainOutputMatchesClauseFieldOrder(t *testing.T) {
	clauseType := reflect.TypeFor[ast.Clause]()
	if clauseType.NumField() == 0 {
		t.Fatalf("expected ast.Clause to have fields, got none")
	}
	output := captureStdout(t, func() {
		main()
	})
	lines := strings.Split(strings.TrimSuffix(output, "\n"), "\n")
	if len(lines) != clauseType.NumField() {
		t.Fatalf("main() printed %d lines, want %d for ast.Clause fields, got %q", len(lines), clauseType.NumField(), output)
	}
	for i := range lines {
		want := "Field: " + clauseType.Field(i).Name
		if lines[i] != want {
			t.Errorf("line %d: got %q, want %q", i, lines[i], want)
		}
	}
}

func TestMainBatchesSingleBufferedWrite(t *testing.T) {
	output := captureStdout(t, func() {
		main()
	})
	clauseType := reflect.TypeFor[ast.Clause]()
	if clauseType.NumField() == 0 {
		t.Fatalf("expected ast.Clause to have fields, got none")
	}
	var sb strings.Builder
	for i := 0; i < clauseType.NumField(); i++ {
		sb.WriteString("Field: ")
		sb.WriteString(clauseType.Field(i).Name)
		sb.WriteString("\n")
	}
	want := sb.String()
	if output != want {
		t.Errorf("main() output mismatch:\nwant %q\ngot  %q", want, output)
	}
	src, err := os.ReadFile("inspect_clause.go")
	if err != nil {
		t.Fatalf("read production source: %v", err)
	}
	text := string(src)
	if !strings.Contains(text, "strings.Builder") {
		t.Errorf("main() should batch output with strings.Builder")
	}
	if !strings.Contains(text, "NumField") {
		t.Errorf("main() should iterate with NumField")
	}
	if strings.Contains(text, "Printf") {
		t.Errorf("main() should not print per-field with Printf; want single batched Print")
	}
	if strings.Contains(text, ".Fields()") {
		t.Errorf("main() should not use the Fields() iterator path")
	}
	if strings.Contains(text, `"Field: " +`) {
		t.Errorf("main() should not concatenate inside WriteString; want separate writes")
	}
}

func TestMainExactSingleBufferedImplementation(t *testing.T) {
	output := captureStdout(t, func() {
		main()
	})
	const want = "Field: Head\nField: HeadTime\nField: Premises\nField: Transform\n"
	if output != want {
		t.Errorf("main() output mismatch:\nwant %q\ngot  %q", want, output)
	}
	src, err := os.ReadFile("inspect_clause.go")
	if err != nil {
		t.Fatalf("read production source: %v", err)
	}
	text := string(src)
	for _, snippet := range []string{
		"var b strings.Builder",
		"for i := 0; i < t.NumField(); i++",
		`b.WriteString("Field: ")`,
		"b.WriteString(t.Field(i).Name)",
		`b.WriteString("\n")`,
		"return b.String()",
		"fmt.Print(clauseFieldListing())",
		"clauseFieldListingFor(reflect.TypeFor[ast.Clause]())",
	} {
		if !strings.Contains(text, snippet) {
			t.Errorf("main() should contain %q to keep single buffered write by index", snippet)
		}
	}
	for _, forbidden := range []string{
		"Fprint",
		"Println",
		"Printf",
		"os.Stdout",
		".Fields()",
		`"Field: " +`,
		"Sprintf",
		"range t.NumField",
	} {
		if strings.Contains(text, forbidden) {
			t.Errorf("main() should not contain %q; want single fmt.Print of Builder content", forbidden)
		}
	}
}

func TestMainPinsTypeForIndexIteration(t *testing.T) {
	output := captureStdout(t, func() {
		main()
	})
	const want = "Field: Head\nField: HeadTime\nField: Premises\nField: Transform\n"
	if output != want {
		t.Errorf("main() output mismatch:\nwant %q\ngot  %q", want, output)
	}
	src, err := os.ReadFile("inspect_clause.go")
	if err != nil {
		t.Fatalf("read production source: %v", err)
	}
	text := string(src)
	if !strings.Contains(text, "reflect.TypeFor[ast.Clause]()") {
		t.Errorf("main() should derive fields via reflect.TypeFor[ast.Clause]() for stable index iteration")
	}
	for _, forbidden := range []string{
		"reflect.TypeOf",
		"VisibleFields",
		"FieldByName",
		"reflect.ValueOf",
	} {
		if strings.Contains(text, forbidden) {
			t.Errorf("main() should not contain %q; want TypeFor index iteration", forbidden)
		}
	}
}

func TestClauseFieldListingMatchesMainOutput(t *testing.T) {
	output := captureStdout(t, func() {
		main()
	})
	direct := clauseFieldListing()
	if direct == "" {
		t.Fatalf("clauseFieldListing() produced no output")
	}
	if output != direct {
		t.Errorf("main() output mismatch with clauseFieldListing():\nmain %q\ndirect %q", output, direct)
	}
	if again := clauseFieldListing(); again != direct {
		t.Errorf("clauseFieldListing() not deterministic:\nfirst %q\nsecond %q", direct, again)
	}
	clauseType := reflect.TypeFor[ast.Clause]()
	if clauseType.NumField() == 0 {
		t.Fatalf("expected ast.Clause to have fields, got none")
	}
	lines := strings.Split(strings.TrimSuffix(direct, "\n"), "\n")
	if len(lines) != clauseType.NumField() {
		t.Fatalf("clauseFieldListing() printed %d lines, want %d for ast.Clause fields, got %q", len(lines), clauseType.NumField(), direct)
	}
	for i := range lines {
		want := "Field: " + clauseType.Field(i).Name
		if lines[i] != want {
			t.Errorf("line %d: got %q, want %q", i, lines[i], want)
		}
	}
}

func TestMainDelegatesToClauseFieldListing(t *testing.T) {
	src, err := os.ReadFile("inspect_clause.go")
	if err != nil {
		t.Fatalf("read production source: %v", err)
	}
	text := string(src)
	idx := strings.Index(text, "func main()")
	if idx < 0 {
		t.Fatalf("func main() not found in inspect_clause.go")
	}
	mainBody := strings.TrimSpace(text[idx:])
	wantMain := "func main() {\n\tfmt.Print(clauseFieldListing())\n}"
	if strings.TrimSpace(mainBody) != strings.TrimSpace(wantMain) {
		t.Errorf("main() should be exactly %q to delegate single buffered write; got %q", wantMain, mainBody)
	}
	if !strings.Contains(mainBody, "clauseFieldListing()") {
		t.Errorf("main() should delegate to clauseFieldListing(); got main body %q", mainBody)
	}
	for _, dup := range []string{"reflect.TypeFor", "strings.Builder", "NumField", "t.Field(", "b.String()", "Println", "Fprint", "os.Stdout", "Sprintf"} {
		if strings.Contains(mainBody, dup) {
			t.Errorf("main() should not contain %q; want delegation to clauseFieldListing()", dup)
		}
	}
	output := captureStdout(t, func() { main() })
	if want := clauseFieldListing(); output != want {
		t.Errorf("main() output mismatch with clauseFieldListing():\nmain %q\nwant %q", output, want)
	}
}

func TestClauseFieldListingForPinsNilAndStructCases(t *testing.T) {
	if got := clauseFieldListingFor(nil); got != "" {
		t.Errorf("clauseFieldListingFor(nil) = %q, want empty string", got)
	}
	if got := clauseFieldListingFor(reflect.TypeFor[struct{}]()); got != "" {
		t.Errorf("clauseFieldListingFor(empty struct) = %q, want empty string", got)
	}
	type sample struct {
		Alpha int
		Beta  string
		Gamma bool
	}
	const wantSample = "Field: Alpha\nField: Beta\nField: Gamma\n"
	if got := clauseFieldListingFor(reflect.TypeFor[sample]()); got != wantSample {
		t.Errorf("clauseFieldListingFor(sample) mismatch:\nwant %q\ngot  %q", wantSample, got)
	}
	clauseType := reflect.TypeFor[ast.Clause]()
	if got, want := clauseFieldListing(), clauseFieldListingFor(clauseType); got != want {
		t.Errorf("clauseFieldListing() should equal clauseFieldListingFor(ast.Clause):\ngot  %q\nwant %q", got, want)
	}
	if again := clauseFieldListingFor(clauseType); again != clauseFieldListing() {
		t.Errorf("clauseFieldListingFor(ast.Clause) not stable:\nfirst  %q\nsecond %q", clauseFieldListing(), again)
	}
}

func TestClauseFieldListingDelegatesToFor(t *testing.T) {
	const want = "Field: Head\nField: HeadTime\nField: Premises\nField: Transform\n"
	if got := clauseFieldListing(); got != want {
		t.Errorf("clauseFieldListing() mismatch:\nwant %q\ngot  %q", want, got)
	}
	output := captureStdout(t, func() { main() })
	if output != clauseFieldListing() {
		t.Errorf("main() output mismatch with clauseFieldListing():\nmain %q\nlisting %q", output, clauseFieldListing())
	}
	src, err := os.ReadFile("inspect_clause.go")
	if err != nil {
		t.Fatalf("read production source: %v", err)
	}
	text := string(src)
	start := strings.Index(text, "func clauseFieldListing()")
	if start < 0 {
		t.Fatalf("func clauseFieldListing() not found in inspect_clause.go")
	}
	rest := text[start:]
	afterHeader := rest[len("func clauseFieldListing()"):]
	var body string
	if idx := strings.Index(afterHeader, "\nfunc "); idx < 0 {
		body = rest
	} else {
		body = rest[:len("func clauseFieldListing()")+idx]
	}
	wantBody := "func clauseFieldListing() string {\n\treturn clauseFieldListingFor(reflect.TypeFor[ast.Clause]())\n}"
	if strings.TrimSpace(body) != strings.TrimSpace(wantBody) {
		t.Errorf("clauseFieldListing() should be exactly %q to delegate single buffered write; got %q", wantBody, body)
	}
	if !strings.Contains(body, "clauseFieldListingFor(reflect.TypeFor[ast.Clause]())") {
		t.Errorf("clauseFieldListing() should delegate to clauseFieldListingFor(reflect.TypeFor[ast.Clause]())); got body %q", body)
	}
	for _, dup := range []string{"strings.Builder", "NumField", "WriteString", "t.Field("} {
		if strings.Contains(body, dup) {
			t.Errorf("clauseFieldListing() should not contain %q; want delegation to clauseFieldListingFor()", dup)
		}
	}
}
