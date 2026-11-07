package build

import (
	"bytes"
	"go/format"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// Every Go file in the repository is gofmt-clean. Nothing held it: the
// session formats the Go a turn writes once, before its later forcing rounds
// write (ladder N18), and a change made by hand is formatted only when its
// author remembers. On 2026-09-19 28 of 2,542 files were not clean.
//
// The files are the ones the repository holds (git ls-files), not whatever
// the working copy has lying around: an ignored scratch program is nobody's
// to format. The working copy's line endings are git's to choose
// (core.autocrlf), not the source's, so a file is compared with CRLF read as
// LF. A file that does not parse is the build gate's to report, and testdata
// holds fixtures that may be unformatted on purpose. This package owns the
// tree-wide guard, so a failure may name a file outside internal/build and
// the fix lands where gofmt names it (run gofmt -w on the named file).
// Gofmt decides; this test only reports.
func TestRepository_EveryGoFileIsGofmtClean(t *testing.T) {
	root := repoRoot(t)
	out, err := exec.Command("git", "-C", root, "ls-files", "-z", "--", "*.go").Output()
	if err != nil {
		t.Fatalf("git ls-files in %s: %v", root, err)
	}
	var unformatted []string
	for _, rel := range strings.Split(string(out), "\x00") {
		if rel == "" || strings.Contains("/"+rel, "/testdata/") {
			continue
		}
		src, rerr := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if rerr != nil {
			continue
		}
		clean, ferr := isGofmtClean(src)
		if ferr != nil {
			continue
		}
		if !clean {
			unformatted = append(unformatted, rel)
		}
	}
	if len(unformatted) > 0 {
		sort.Strings(unformatted)
		t.Errorf("%d Go file(s) are not gofmt-clean; run gofmt -w on:\n%s", len(unformatted), strings.Join(unformatted, "\n"))
	}
}

// isGofmtClean reports whether src is gofmt-clean, normalizing CRLF to LF
// first. It returns an error when src does not parse; callers skip such
// files because unparsable code is the build gate's to report.
func isGofmtClean(src []byte) (bool, error) {
	normalized := bytes.ReplaceAll(src, []byte("\r\n"), []byte("\n"))
	formatted, err := format.Source(normalized)
	if err != nil {
		return false, err
	}
	return bytes.Equal(formatted, normalized), nil
}

func TestIsGofmtClean_PinsComparisonContract(t *testing.T) {
	t.Parallel()
	clean := []byte("package build\n\nfunc clean() {}\n")
	if ok, err := isGofmtClean(clean); err != nil || !ok {
		t.Fatalf("clean src: ok=%v err=%v, want ok=true err=nil", ok, err)
	}
	unformatted := []byte("package build\n\nfunc  clean()  {}\n")
	if ok, err := isGofmtClean(unformatted); err != nil || ok {
		t.Fatalf("unformatted src: ok=%v err=%v, want ok=false err=nil", ok, err)
	}
	crlf := bytes.ReplaceAll(clean, []byte("\n"), []byte("\r\n"))
	if ok, err := isGofmtClean(crlf); err != nil || !ok {
		t.Fatalf("crlf src: ok=%v err=%v, want ok=true err=nil", ok, err)
	}
	if _, err := isGofmtClean([]byte("package build\nfunc broken( {}\n")); err == nil {
		t.Fatal("unparsable src: got nil error, want non-nil")
	}
}
