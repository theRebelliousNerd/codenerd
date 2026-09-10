// Command audit_committed_binaries fails when a compiled executable is tracked
// in git.
//
// Two were: a 9 MB `main` and a 24 MB `predicate_corpus_builder`, both at the
// repository root, both landed by the same commit -- a commit whose message is
// about versioning a documentation file. Between them they were roughly half
// the weight of .git, carried by every clone forever, and stale from the moment
// the source they were built from changed.
//
// Nobody committed them on purpose. `go build ./cmd/tools/foo` writes its
// output to the working directory, so running it once at the repository root
// leaves an executable there, and the next `git add -A` picks it up along with
// whatever was actually being committed. The mistake is invisible in review: a
// diff of a binary shows as one line saying the binary changed, and it sits
// below the files the reviewer came to look at.
//
// This is a gate rather than a .gitignore entry on purpose. Ignoring the two
// names that happened to appear would do nothing about the next tool somebody
// adds under cmd/, and a build output is never something this repository wants
// tracked under any name.
package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Magic numbers for the three executable formats a Go build can produce.
// Deliberately not "is this file binary": PNG fixtures and test corpora are
// binary and belong here. A compiled program does not.
var executableMagic = []struct {
	name  string
	magic []byte
}{
	{"ELF (Linux)", []byte{0x7F, 'E', 'L', 'F'}},
	{"PE (Windows)", []byte{'M', 'Z'}},
	{"Mach-O 64 (macOS)", []byte{0xCF, 0xFA, 0xED, 0xFE}},
	{"Mach-O 32 (macOS)", []byte{0xCE, 0xFA, 0xED, 0xFE}},
	{"Mach-O universal (macOS)", []byte{0xCA, 0xFE, 0xBA, 0xBE}},
}

func main() {
	files, err := trackedFiles()
	if err != nil {
		fmt.Fprintf(os.Stderr, "audit_committed_binaries: %v\n", err)
		os.Exit(2)
	}

	var found []string
	for _, path := range files {
		if kind, ok := executableKind(path); ok {
			info, statErr := os.Stat(path)
			size := "unknown size"
			if statErr == nil {
				size = fmt.Sprintf("%.1f MB", float64(info.Size())/(1<<20))
			}
			found = append(found, fmt.Sprintf("  %s  (%s, %s)", path, kind, size))
		}
	}

	if len(found) == 0 {
		fmt.Printf("audit_committed_binaries: %d tracked files, no compiled executables\n", len(files))
		return
	}

	fmt.Fprintf(os.Stderr, "%d compiled executable(s) tracked in git:\n%s\n\n",
		len(found), strings.Join(found, "\n"))
	fmt.Fprint(os.Stderr, `A build output does not belong in source control: it bloats every clone
permanently and is stale as soon as its source changes.

This is almost always `+"`go build ./cmd/tools/foo`"+` run from the repository root,
which writes its output next to you, followed by a `+"`git add -A`"+` that sweeps it up.
Use `+"`go run ./cmd/tools/foo`"+` instead, or `+"`go build -o`"+` to a path outside the tree.

To fix: git rm --cached <path> && rm <path>
`)
	os.Exit(1)
}

// trackedFiles asks git rather than walking the tree, so that ignored and
// untracked build output -- which is fine, and normal -- is not reported.
func trackedFiles() ([]string, error) {
	out, err := exec.Command("git", "ls-files", "-z").Output()
	if err != nil {
		return nil, fmt.Errorf("git ls-files: %w", err)
	}
	var files []string
	for _, name := range bytes.Split(out, []byte{0}) {
		if len(name) > 0 {
			files = append(files, string(name))
		}
	}
	return files, nil
}

func executableKind(path string) (string, bool) {
	f, err := os.Open(path)
	if err != nil {
		// A tracked path that will not open is a sparse checkout or a symlink
		// to nowhere. Neither is this gate's business, and failing the build
		// over one would make the gate the problem.
		return "", false
	}
	defer func() { _ = f.Close() }()

	header := make([]byte, 4)
	n, _ := f.Read(header)
	header = header[:n]

	for _, kind := range executableMagic {
		if bytes.HasPrefix(header, kind.magic) {
			return kind.name, true
		}
	}
	return "", false
}
