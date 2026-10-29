package broker

import (
	"errors"
	"fmt"
	"go/scanner"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTokenCounterScanDetectsExecutableIdentifiers(test *testing.T) {
	names := []string{"charsPerToken", "CharsPerTokenEstimator", "NewTokenCounterWithEstimator"}
	forms := []struct {
		name   string
		source string
	}{
		{name: "variable declaration", source: "var %s = 4\n"},
		{name: "type declaration", source: "type %s struct{}\n"},
		{name: "function declaration", source: "func %s() {}\n"},
		{name: "field declaration", source: "type counter struct { %s int }\n"},
		{name: "selector reference", source: "func inspect() { _ = counter.%s }\n"},
		{name: "bare reference", source: "func inspect() { _ = %s }\n"},
	}
	paths := []string{
		"internal/example/counter.go",
		"internal/example/counter_test.go",
		"internal/broker/counter_test.go",
		"cmd/tools/audit_doc_citations/counter_test.go",
	}
	for _, name := range names {
		for _, form := range forms {
			for _, relative := range paths {
				test.Run(name+"/"+form.name+"/"+relative, func(test *testing.T) {
					root := test.TempDir()
					path := writeTokenCounterFixture(test, root, relative, fmt.Sprintf("package fixture\nvar historicalName = %q\n", name))
					findings, err := scanCompetingTokenCounters(root)
					if err != nil || len(findings) != 0 {
						test.Fatalf("literal baseline findings=%+v error=%v", findings, err)
					}
					writeTokenCounterFixture(test, root, relative, "package fixture\n"+fmt.Sprintf(form.source, name))
					findings, err = scanCompetingTokenCounters(root)
					if err != nil {
						test.Fatalf("scan executable mutation: %v", err)
					}
					if len(findings) != 1 {
						test.Fatalf("executable mutation findings=%+v, want exactly one", findings)
					}
					finding := findings[0]
					if finding.name != name || finding.why == "" || finding.position.Filename != path || finding.position.Line != 2 || finding.position.Column <= 0 {
						test.Fatalf("finding lost identifier or source location: %+v", finding)
					}
				})
			}
		}
	}
}

func TestTokenCounterScanIgnoresNonIdentifiers(test *testing.T) {
	cases := []struct {
		name   string
		source string
	}{
		{name: "quoted literals", source: "var removed = \"charsPerToken CharsPerTokenEstimator NewTokenCounterWithEstimator\"\n"},
		{name: "raw literals", source: "var removed = `charsPerToken CharsPerTokenEstimator NewTokenCounterWithEstimator`\n"},
		{name: "line comments", source: "// charsPerToken CharsPerTokenEstimator NewTokenCounterWithEstimator\nvar current = 4\n"},
		{name: "block comments", source: "/* charsPerToken CharsPerTokenEstimator NewTokenCounterWithEstimator */\nvar current = 4\n"},
		{name: "struct tags", source: "type counter struct { Current int `json:\"charsPerToken CharsPerTokenEstimator NewTokenCounterWithEstimator\"` }\n"},
		{name: "similar names", source: "var charsPerTokenLegacy = 4\ntype OldCharsPerTokenEstimator struct{}\nfunc NewTokenCounterWithEstimatorLegacy() {}\n"},
		{name: "citation auditor selector literal", source: "func inspect() { checker.stdlibHas(\"context\", \"TokenCounter.charsPerToken\") }\n"},
	}
	for _, testcase := range cases {
		test.Run(testcase.name, func(test *testing.T) {
			root := test.TempDir()
			writeTokenCounterFixture(test, root, "cmd/tools/audit_doc_citations/audit_test.go", "package fixture\n"+testcase.source)
			findings, err := scanCompetingTokenCounters(root)
			if err != nil || len(findings) != 0 {
				test.Fatalf("non-identifier fixture findings=%+v error=%v", findings, err)
			}
		})
	}
}

func TestTokenCounterScanReportsAllIdentifiers(test *testing.T) {
	root := test.TempDir()
	writeTokenCounterFixture(test, root, "counter.go", "package fixture\nvar charsPerToken = 4\ntype CharsPerTokenEstimator struct{}\nfunc NewTokenCounterWithEstimator() { _ = counter.charsPerToken }\n")
	writeTokenCounterFixture(test, root, "counter_test.go", "package fixture\nfunc inspect() { _ = counter.CharsPerTokenEstimator; counter.NewTokenCounterWithEstimator() }\n")
	findings, err := scanCompetingTokenCounters(root)
	if err != nil {
		test.Fatal(err)
	}
	expected := map[string]int{"charsPerToken": 2, "CharsPerTokenEstimator": 2, "NewTokenCounterWithEstimator": 2}
	observed := make(map[string]int)
	for _, finding := range findings {
		observed[finding.name]++
	}
	if len(findings) != 6 || len(observed) != len(expected) {
		test.Fatalf("findings=%+v, want all six declarations and references", findings)
	}
	for name, count := range expected {
		if observed[name] != count {
			test.Errorf("identifier %s occurrences=%d, want %d", name, observed[name], count)
		}
	}
}

func TestTokenCounterScanPropagatesParseErrors(test *testing.T) {
	cases := []struct {
		name   string
		source string
	}{
		{name: "malformed declaration", source: "package fixture\nvar charsPerToken =\n"},
		{name: "unterminated literal", source: "package fixture\nvar historicalName = \"charsPerToken\n"},
		{name: "unterminated comment", source: "package fixture\n/* charsPerToken\n"},
	}
	for _, testcase := range cases {
		test.Run(testcase.name, func(test *testing.T) {
			root := test.TempDir()
			path := writeTokenCounterFixture(test, root, "internal/example/broken_test.go", testcase.source)
			findings, err := scanCompetingTokenCounters(root)
			if err == nil || len(findings) != 0 {
				test.Fatalf("malformed source yielded findings=%+v error=%v", findings, err)
			}
			var parseErrors scanner.ErrorList
			if !errors.As(err, &parseErrors) || !strings.Contains(err.Error(), "parse "+path) {
				test.Fatalf("parse failure lost cause or path: %v", err)
			}
		})
	}
}

func TestTokenCounterScanPropagatesReadErrors(test *testing.T) {
	root := test.TempDir()
	test.Run("missing source", func(test *testing.T) {
		path := filepath.Join(root, "missing.go")
		findings, err := scanTokenCounterSource(path)
		if err == nil || len(findings) != 0 || !errors.Is(err, os.ErrNotExist) || !strings.Contains(err.Error(), "read "+path) {
			test.Fatalf("missing source yielded findings=%+v error=%v", findings, err)
		}
	})
	test.Run("directory cannot be read as source", func(test *testing.T) {
		findings, err := scanTokenCounterSource(root)
		var pathError *os.PathError
		if err == nil || len(findings) != 0 || !errors.As(err, &pathError) || !strings.Contains(err.Error(), "read "+root) {
			test.Fatalf("unreadable source yielded findings=%+v error=%v", findings, err)
		}
	})
	test.Run("missing scan root", func(test *testing.T) {
		path := filepath.Join(root, "missing")
		findings, err := scanCompetingTokenCounters(path)
		if err == nil || len(findings) != 0 || !errors.Is(err, os.ErrNotExist) || !strings.Contains(err.Error(), "walk "+path) {
			test.Fatalf("failed walk yielded findings=%+v error=%v", findings, err)
		}
	})
}

func writeTokenCounterFixture(test *testing.T, root, relative, source string) string {
	test.Helper()
	path := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		test.Fatalf("create source fixture directory: %v", err)
	}
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		test.Fatalf("write source fixture: %v", err)
	}
	return path
}
