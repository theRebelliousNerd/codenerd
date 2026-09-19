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
// holds fixtures that may be unformatted on purpose.
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
		src = bytes.ReplaceAll(src, []byte("\r\n"), []byte("\n"))
		formatted, ferr := format.Source(src)
		if ferr != nil {
			continue
		}
		if !bytes.Equal(formatted, src) {
			unformatted = append(unformatted, rel)
		}
	}
	if len(unformatted) > 0 {
		sort.Strings(unformatted)
		t.Errorf("%d Go file(s) are not gofmt-clean; run gofmt -w on:\n%s", len(unformatted), strings.Join(unformatted, "\n"))
	}
}
