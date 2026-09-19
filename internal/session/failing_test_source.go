package session

import (
	"os"
	"path/filepath"
	"strings"
)

// A repair round is handed the failing tests' source (external audit N24).
//
// The round's prompt carried the test runner's output and nothing else, and
// the round closes the read tools after its first round that writes nothing --
// so a model that wants to see the test it broke cannot open it. Ladder run
// R1-12 (2026-09-19): three attempts, eighteen model calls, 746.7k input
// tokens and twenty-six recall_context calls, no edit, and the turn died at
// the test gate with the same failure it started with. The removed-tests round
// already hands back each deleted test's source -- "a paste, not a
// reconstruction from memory" -- and this is the same answer for the tests a
// change broke.

// failingTestSection renders each test named in the runner's output as it is
// on disk. It returns "" when no name is parsed or no source is found: the
// prompt then says exactly what it said before.
func failingTestSection(workspace, output string, written []string) string {
	names := topLevelFailedTests(output)
	if len(names) == 0 {
		return ""
	}
	dirs := failingTestDirs(workspace, output, written)
	var b strings.Builder
	for _, name := range names {
		path, src := findTestFunc(workspace, dirs, name)
		if src == "" {
			continue
		}
		b.WriteString("\n" + path + ": " + name + "\n```go\n" + src + "\n```\n")
	}
	if b.Len() == 0 {
		return ""
	}
	return "\nThe failing tests, as they are on disk:\n" + b.String() +
		"\nThis is the contract the code must meet. Read it here rather than opening it: " +
		"the edit tools are what this round is for.\n"
}

// failingTestDirs are the directories to look in: the packages the runner
// reported as failing, and the directories the turn wrote to.
func failingTestDirs(workspace, output string, written []string) []string {
	seen := map[string]bool{}
	var dirs []string
	add := func(rel string) {
		rel = strings.TrimPrefix(filepath.ToSlash(rel), "./")
		if rel == "" || seen[rel] {
			return
		}
		if info, err := os.Stat(filepath.Join(workspace, filepath.FromSlash(rel))); err != nil || !info.IsDir() {
			return
		}
		seen[rel] = true
		dirs = append(dirs, rel)
	}
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) < 2 || fields[0] != "FAIL" {
			continue
		}
		// "FAIL codenerd/internal/tools/codedom": the import path with the
		// module's own name in front of it. The module path is not read from
		// go.mod -- the first segment is dropped and the rest tried as a
		// directory, then the whole path, so a module whose name is a domain
		// works the same way.
		pkg := filepath.ToSlash(fields[1])
		if i := strings.Index(pkg, "/"); i >= 0 {
			add(pkg[i+1:])
		}
		add(pkg)
	}
	for _, p := range written {
		add(filepath.ToSlash(filepath.Dir(p)))
	}
	return dirs
}

// findTestFunc returns the workspace-relative path and source of the first
// test function called name in dirs, as the default build sees them.
func findTestFunc(workspace string, dirs []string, name string) (string, string) {
	for _, dir := range dirs {
		abs := filepath.Join(workspace, filepath.FromSlash(dir))
		entries, err := os.ReadDir(abs)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() || !isTestPath(e.Name()) {
				continue
			}
			data, err := os.ReadFile(filepath.Join(abs, e.Name()))
			if err != nil {
				continue
			}
			if src := testFuncSource(string(data), name); src != "" {
				return dir + "/" + e.Name(), src
			}
		}
	}
	return "", ""
}
