package gates

import (
	"bufio"
	"bytes"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// The numbers recurse's improvement step must move. A change is an
// improvement only if one of these moved the right way, measured before and
// after -- never because a model says so. Every one is read from the
// workspace itself, whatever its language.
const (
	// MetricTests counts the workspace's test functions. It only rises when
	// tests are added, and a change that lowers it deleted tests.
	MetricTests = "tests"
	// MetricLines counts a node's non-blank source lines, tests excluded.
	MetricLines = "lines"
	// MetricCoverage is a node's statement coverage in basis points
	// (7325 = 73.25%), where its test run reports one.
	MetricCoverage = "coverage"
)

// skipDirs are directories that are not the workspace's own code:
// dependencies, build output, virtualenvs, fixtures.
var skipDirs = map[string]bool{
	"node_modules": true, "vendor": true, "dist": true, "build": true, "out": true,
	"target": true, "venv": true, "env": true, "__pycache__": true, "testdata": true,
	"coverage": true, "site-packages": true,
}

// SkipDir reports whether a directory named name is outside the workspace's
// own code: hidden (VCS, agent state) or one of the dependency, output and
// fixture directories every toolchain here leaves in a tree.
func SkipDir(name string) bool {
	return strings.HasPrefix(name, ".") || skipDirs[name]
}

var sourceExts = map[string]bool{
	".go": true, ".py": true, ".rs": true,
	".js": true, ".jsx": true, ".ts": true, ".tsx": true, ".mjs": true, ".cjs": true, ".mts": true, ".cts": true,
}

var (
	goTestFunc   = regexp.MustCompile(`(?m)^func ((?:Test|Fuzz)\w*)\(`)
	pyTestFunc   = regexp.MustCompile(`(?m)^\s*(?:async\s+)?def test_?\w*\s*\(`)
	jsTestCall   = regexp.MustCompile("(?m)^\\s*(?:it|test)(?:\\.(?:only|skip|each\\([^)]*\\)))?\\s*\\(\\s*['\"`]")
	rustTestAttr = regexp.MustCompile(`(?m)^\s*#\[(?:tokio::)?test\]`)
)

// isTestFile reports whether a source file holds tests, by each toolchain's
// own naming convention.
func isTestFile(rel string) bool {
	name := filepath.Base(rel)
	switch filepath.Ext(name) {
	case ".go":
		return strings.HasSuffix(name, "_test.go")
	case ".py":
		return strings.HasPrefix(name, "test_") || strings.HasSuffix(name, "_test.py")
	case ".rs":
		return false // Rust tests live beside the code; counted by attribute.
	}
	return strings.Contains(name, ".test.") || strings.Contains(name, ".spec.") ||
		strings.Contains(filepath.ToSlash(rel), "/__tests__/")
}

// testsIn counts the test functions in one file's content.
func testsIn(rel string, data []byte) int {
	switch filepath.Ext(rel) {
	case ".go":
		if !isTestFile(rel) {
			return 0
		}
		n := 0
		for _, m := range goTestFunc.FindAllSubmatch(data, -1) {
			if string(m[1]) != "TestMain" {
				n++
			}
		}
		return n
	case ".py":
		if !isTestFile(rel) {
			return 0
		}
		return len(pyTestFunc.FindAll(data, -1))
	case ".rs":
		return len(rustTestAttr.FindAll(data, -1))
	}
	if !isTestFile(rel) {
		return 0
	}
	return len(jsTestCall.FindAll(data, -1))
}

// CountTests counts the test functions in the workspace's own source files.
func CountTests(root string) (int, error) {
	total := 0
	err := walkSources(root, ".", true, func(rel string, data []byte) {
		total += testsIn(rel, data)
	})
	return total, err
}

// SourceLines counts the non-blank lines of the non-test source files in dirs
// (workspace-relative, "." for the root). A node's subdirectories are other
// nodes, so only the files directly in each dir count -- unless recursive,
// for a Rust crate whose modules are its subdirectories.
func SourceLines(root string, dirs []string, recursive bool) (int, error) {
	total := 0
	for _, dir := range dirs {
		err := walkSources(root, dir, recursive, func(rel string, data []byte) {
			if isTestFile(rel) {
				return
			}
			for _, line := range bytes.Split(data, []byte("\n")) {
				if len(bytes.TrimSpace(line)) > 0 {
					total++
				}
			}
		})
		if err != nil {
			return 0, err
		}
	}
	return total, nil
}

// walkSources calls fn for each source file under dir.
func walkSources(root, dir string, recursive bool, fn func(rel string, data []byte)) error {
	start := filepath.Join(root, filepath.FromSlash(dir))
	return filepath.WalkDir(start, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if p == start {
				return err
			}
			if d != nil && d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			if p != start && (!recursive || SkipDir(d.Name())) {
				return filepath.SkipDir
			}
			return nil
		}
		if !sourceExts[filepath.Ext(p)] {
			return nil
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		fn(filepath.ToSlash(rel), data)
		return nil
	})
}

var (
	// go test -cover: "ok  pkg  0.1s  coverage: 73.2% of statements".
	goCoverage = regexp.MustCompile(`coverage: (\d+(?:\.\d+)?)% of statements`)
	// pytest-cov's summary: "TOTAL   120   30   75%".
	pyCoverage = regexp.MustCompile(`(?m)^TOTAL\s+\d+\s+\d+(?:\s+\d+\s+\d+)?\s+(\d+(?:\.\d+)?)%`)
)

// Coverage reads the statement coverage a test run reported, in basis points.
// A run that reported several (a node spanning packages) reads as the lowest:
// the node is as covered as its least covered part.
func Coverage(output string) (int, bool) {
	var found []string
	sc := bufio.NewScanner(strings.NewReader(output))
	sc.Buffer(make([]byte, 0, bufio.MaxScanTokenSize), len(output)+1)
	for sc.Scan() {
		line := sc.Text()
		if m := goCoverage.FindStringSubmatch(line); m != nil {
			found = append(found, m[1])
		} else if m := pyCoverage.FindStringSubmatch(line); m != nil {
			found = append(found, m[1])
		}
	}
	best, ok := 0, false
	for _, f := range found {
		pct, err := strconv.ParseFloat(f, 64)
		if err != nil {
			continue
		}
		bp := int(math.Round(pct * 100))
		if !ok || bp < best {
			best, ok = bp, true
		}
	}
	return best, ok
}
