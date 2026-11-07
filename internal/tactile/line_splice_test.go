package tactile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSpliceLineEndings(t *testing.T) {
	// mixed is the shape of the observed defect: a few LF lines at the top of
	// an otherwise CRLF file.
	const mixed = "# T\n\nintro\n\nsection\r\nbody\r\nmore\r\ntail\r\nend\r\n"
	tests := []struct {
		name   string
		orig   string
		edited string
		want   string
	}{
		{"uniform LF edit", "a\nb\nc\n", "a\nB\nc\n", "a\nB\nc\n"},
		{"uniform CRLF edit", "a\r\nb\r\nc\r\n", "a\nB\nc\n", "a\r\nB\r\nc\r\n"},
		{"uniform CRLF edit arrives as CRLF", "a\r\nb\r\nc\r\n", "a\r\nB\r\nc\r\n", "a\r\nB\r\nc\r\n"},
		{"uniform CRLF multi-line replacement", "a\r\nb\r\nc\r\n", "a\nx\ny\nz\nc\n", "a\r\nx\r\ny\r\nz\r\nc\r\n"},
		{"mixed, edit in the LF region", mixed, "# T\n\nINTRO\n\nsection\nbody\nmore\ntail\nend\n", "# T\n\nINTRO\n\nsection\r\nbody\r\nmore\r\ntail\r\nend\r\n"},
		{"mixed, edit in the CRLF region", mixed, "# T\n\nintro\n\nsection\nBODY\nmore\ntail\nend\n", "# T\n\nintro\n\nsection\r\nBODY\r\nmore\r\ntail\r\nend\r\n"},
		{"mixed, edit across the boundary pairs lines in order", mixed, "# T\n\nintro\nNEW\nBODY\nmore\ntail\nend\n", "# T\n\nintro\nNEW\nBODY\r\nmore\r\ntail\r\nend\r\n"},
		{"mixed, no change", mixed, "# T\n\nintro\n\nsection\nbody\nmore\ntail\nend\n", mixed},
		{"mixed, insert in CRLF region", mixed, "# T\n\nintro\n\nsection\nbody\nnew\nmore\ntail\nend\n", "# T\n\nintro\n\nsection\r\nbody\r\nnew\r\nmore\r\ntail\r\nend\r\n"},
		{"mixed, insert in LF region takes the dominant ending", mixed, "# T\n\nintro\nnew\n\nsection\nbody\nmore\ntail\nend\n", "# T\n\nintro\nnew\r\n\nsection\r\nbody\r\nmore\r\ntail\r\nend\r\n"},
		{"mixed, delete in LF region", mixed, "# T\nintro\n\nsection\nbody\nmore\ntail\nend\n", "# T\nintro\n\nsection\r\nbody\r\nmore\r\ntail\r\nend\r\n"},
		{"mixed, delete in CRLF region", mixed, "# T\n\nintro\n\nsection\nmore\ntail\nend\n", "# T\n\nintro\n\nsection\r\nmore\r\ntail\r\nend\r\n"},
		{"replacement lines pair with the lines they replace", "a\nb\r\nc\r\nd\r\n", "a\nX\nY\nZ\nd\n", "a\nX\r\nY\r\nZ\r\nd\r\n"},
		{"replacing an LF line with two lines", "a\r\nb\nc\r\n", "a\nX\nY\nc\n", "a\r\nX\nY\r\nc\r\n"},
		{"add a final newline to CRLF file", "a\r\nb\r\nc", "a\nb\nc\n", "a\r\nb\r\nc\r\n"},
		{"add a final newline to LF file", "a\nb\nc", "a\nb\nc\n", "a\nb\nc\n"},
		{"remove the final newline from CRLF file", "a\r\nb\r\nc\r\n", "a\nb\nc", "a\r\nb\r\nc"},
		{"remove the final newline from mixed file", "a\nb\r\nc\r\n", "a\nb\nc", "a\nb\r\nc"},
		{"append a line to a file without a final newline", "a\r\nb", "a\nb\nc\n", "a\r\nb\r\nc\r\n"},
		{"unterminated last line edited, stays unterminated", "a\r\nb", "a\nB", "a\r\nB"},
		{"empty file gets text", "", "a\nb\n", "a\nb\n"},
		{"empty file stays empty", "", "", ""},
		{"edit empties a CRLF file", "a\r\nb\r\n", "", ""},
		{"edit of an empty line only", "a\r\n\r\nb\r\n", "a\n\n\nb\n", "a\r\n\r\n\r\nb\r\n"},
		{"lone trailing CR is content, not a terminator", "a\r\nb\r", "a\nB\r", "a\r\nB\r"},
		{"lone trailing CR kept when untouched", "a\r\nb\r", "A\nb\r", "A\r\nb\r"},
		{"lone CR inside a line is content", "a\rb\r\nc\r\n", "a\rb\nC\n", "a\rb\r\nC\r\n"},
		{"duplicate lines do not confuse the overlap", "a\r\na\r\n", "a\na\na\n", "a\r\na\r\na\r\n"},
		{"suffix and prefix never overlap on a deletion", "x\r\nx\r\nx\r\n", "x\nx\n", "x\r\nx\r\n"},
		{"tie between LF and CRLF counts as LF for new lines", "a\nb\r\n", "a\nb\nc\n", "a\nb\r\nc\n"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := string(SpliceLineEndings([]byte(tc.orig), tc.edited))
			if got != tc.want {
				t.Fatalf("SpliceLineEndings(%q, %q)\n got  %q\n want %q", tc.orig, tc.edited, got, tc.want)
			}
		})
	}
}

// On a file with one convention throughout, the splice is exactly what
// normalizing the whole output to that convention gives, and a CR is never
// doubled.
func TestSpliceLineEndings_UniformFilesMatchWholeFileNormalization(t *testing.T) {
	edits := []string{
		"", "\n", "x", "x\n", "a\nb\nc\n", "a\nb\nc", "z\na\nb\nc\n", "a\nB\nc\nd\n",
		"a\r\nb\r\nc\r\n", "a\r\nb\nc\r\n", "\n\n\n", "a\rb\nc\n",
	}
	origs := map[string]string{
		"LF":                    "a\nb\nc\n",
		"CRLF":                  "a\r\nb\r\nc\r\n",
		"LF no final newline":   "a\nb\nc",
		"CRLF no final newline": "a\r\nb\r\nc",
	}
	for name, orig := range origs {
		ending := DetectLineEnding([]byte(orig))
		for _, edited := range edits {
			got := string(SpliceLineEndings([]byte(orig), edited))
			want := NormalizeLineEnding(edited, ending)
			if got != want {
				t.Errorf("%s: SpliceLineEndings(%q, %q) = %q, want %q", name, orig, edited, got, want)
			}
			for i := 1; i < len(got); i++ {
				if got[i] == '\r' && got[i-1] == '\r' && ending == "\r\n" && i+1 < len(got) && got[i+1] == '\n' {
					t.Errorf("%s: doubled CR in %q", name, got)
				}
			}
		}
	}
}

func TestSpliceOntoFile(t *testing.T) {
	dir := t.TempDir()
	existing := filepath.Join(dir, "mixed.md")
	if err := os.WriteFile(existing, []byte("a\nb\r\nc\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := SpliceOntoFile(existing, "a\nB\nc\n")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "a\nB\r\nc\r\n" {
		t.Fatalf("existing file spliced wrong: %q", got)
	}
	got, err = SpliceOntoFile(filepath.Join(dir, "new.md"), "x\r\ny\n")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "x\r\ny\n" {
		t.Fatalf("a new file keeps today's behavior (content as given): %q", got)
	}
	if _, err := SpliceOntoFile(dir, "x"); err == nil {
		t.Fatal("an unreadable existing path must be an error, not a silent new file")
	}
}

func TestFileEditorWriteFile_MixedFileKeepsUntouchedLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mixed.txt")
	if err := os.WriteFile(path, []byte("one\ntwo\r\nthree\r\nfour\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	editor := NewFileEditor()
	editor.SetWorkingDir(dir)
	if _, err := editor.EditLines(path, 3, 3, []string{"THREE"}); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if string(data) != "one\ntwo\r\nTHREE\r\nfour\r\n" {
		t.Fatalf("lines outside the edit changed: %q", data)
	}
}
