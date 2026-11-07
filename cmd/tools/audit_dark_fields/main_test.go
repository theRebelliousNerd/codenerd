package main

import (
	"context"
	"errors"
	"flag"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestCollectUses_ClassifiesSelectors(t *testing.T) {
	cases := []struct {
		name, body    string
		writes, reads []string
	}{
		{name: "read", body: "_ = x.Dark", reads: []string{"Dark"}},
		{name: "assignment", body: "x.Filled = x.Source", writes: []string{"Filled"}, reads: []string{"Filled", "Source"}},
		{name: "indexed assignment", body: "x.Queue[0] = 1", writes: []string{"Queue"}, reads: []string{"Queue"}},
		{name: "wrapped assignment", body: "(*(x.Buffer))[0] = 1", writes: []string{"Buffer"}, reads: []string{"Buffer"}},
		{name: "sliced assignment", body: "x.Buffer[:][0] = 1", writes: []string{"Buffer"}, reads: []string{"Buffer"}},
		{name: "increment", body: "x.Count++", writes: []string{"Count"}, reads: []string{"Count"}},
		{name: "address", body: "scan(&x.Decoded)", writes: []string{"Decoded"}, reads: []string{"Decoded"}},
		{name: "method call", body: "x.Run()"},
		{name: "parenthesized method call", body: "(x.Run)()"},
		{name: "nested parenthesized method call", body: "(((x.Run)))()"},
		{name: "method value is a read", body: "_ = (x.Run)", reads: []string{"Run"}},
		{name: "receiver and argument still read", body: "(x.Client.Run)(x.Argument)", reads: []string{"Client", "Argument"}},
		{name: "literal closure is traversed", body: "_ = T{Callback: func() { x.Filled = x.Dark }}", writes: []string{"Callback", "Filled"}, reads: []string{"Filled", "Dark"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f, err := parser.ParseFile(token.NewFileSet(), "fixture.go", "package fixture; func use() {"+tc.body+"}", parser.SkipObjectResolution)
			if err != nil {
				t.Fatal(err)
			}
			writes, reads := map[string]bool{}, map[string]bool{}
			collectUses(f, nil, nil, writes, reads)
			for label, pair := range map[string]struct {
				got  map[string]bool
				want []string
			}{"writes": {writes, tc.writes}, "reads": {reads, tc.reads}} {
				want := map[string]bool{}
				for _, name := range pair.want {
					want[name] = true
				}
				if !reflect.DeepEqual(pair.got, want) {
					t.Errorf("%s = %v, want %v", label, pair.got, want)
				}
			}
		})
	}
}

func TestCollectFields_PreservesPositionalSlots(t *testing.T) {
	const source = "package fixture\ntype Embedded struct{}\ntype Item struct {\n Embedded\n hidden int\n Tagged int `json:\"tagged\"`\n First, Second int\n}\ntype Alias = int\n"
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "fixture.go", source, parser.SkipObjectResolution)
	if err != nil {
		t.Fatal(err)
	}
	fields := map[string][]field{}
	order, types := map[string][]string{}, map[string][]string{}
	collectFields(fset, f, "internal/fixture.go", fields, order, types)
	wantFields := map[string][]field{
		"First":  {{Name: "First", Type: "Item", File: "internal/fixture.go", Line: 7}},
		"Second": {{Name: "Second", Type: "Item", File: "internal/fixture.go", Line: 7}},
	}
	if !reflect.DeepEqual(fields, wantFields) {
		t.Errorf("fields = %#v, want %#v", fields, wantFields)
	}
	if want := []string{"", "hidden", "Tagged", "First", "Second"}; !reflect.DeepEqual(order["Item"], want) {
		t.Errorf("order = %v, want %v", order["Item"], want)
	}
	if want := []string{"Embedded", "int", "int", "int", "int"}; !reflect.DeepEqual(types["Item"], want) {
		t.Errorf("types = %v, want %v", types["Item"], want)
	}
}

func TestCollectUses_CompositeWrites(t *testing.T) {
	cases := []struct {
		name, expr string
		want       []string
	}{
		{"positional", "Item{1, 2}", []string{"First", "Second"}},
		{"keyed", "Item{Second: 2}", []string{"Second"}},
		{"elided slice", "[]Item{{1, 2}}", []string{"First", "Second"}},
		{"elided map", "map[string]Item{\"key\": {1, 2}}", []string{"First", "Second"}},
		{"qualified pointer elements", "[]*other.Item{{1, 2}}", []string{"First", "Second"}},
		{"nested composite occupies slot", "Outer{Item{1, 2}}", []string{"Child", "First", "Second"}},
		{"embedded placeholder", "WithEmbedded{Embedded{}, 1}", []string{"Visible"}},
		{"unknown positional type", "Unknown{1, 2}", nil},
	}
	order := map[string][]string{"Item": {"First", "Second"}, "Outer": {"Child"}, "WithEmbedded": {"", "Visible"}}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f, err := parser.ParseFile(token.NewFileSet(), "fixture.go", "package fixture; var value = "+tc.expr, parser.SkipObjectResolution)
			if err != nil {
				t.Fatal(err)
			}
			writes := map[string]bool{}
			collectUses(f, order, nil, writes, map[string]bool{})
			want := map[string]bool{}
			for _, name := range tc.want {
				want[name] = true
			}
			if !reflect.DeepEqual(writes, want) {
				t.Errorf("writes = %v, want %v", writes, want)
			}
		})
	}
}

func TestCollectUses_SyscallBuffers(t *testing.T) {
	cases := []struct {
		name, setup, call string
		written           bool
	}{
		{"declared", "var buf Buffer", "unsafe.Pointer(&buf)", true},
		{"short declaration", "buf := Buffer{}", "unsafe.Pointer(&buf)", true},
		{"pointer declaration", "buf := &Buffer{}", "unsafe.Pointer(buf)", true},
		{"wrapped indexed", "var buf [1]Buffer", "unsafe.Pointer((&buf[0]))", true},
		{"other package", "var buf Buffer", "other.Pointer(&buf)", false},
		{"other function", "var buf Buffer", "unsafe.Sizeof(&buf)", false},
		{"wrong arity", "var buf Buffer", "unsafe.Pointer(&buf, 0)", false},
		{"non address unary", "var buf Buffer", "unsafe.Pointer(-buf)", false},
		{"unknown type", "var buf int", "unsafe.Pointer(&buf)", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			source := "package fixture; type Buffer struct { Nested Inner; Next *Buffer }; type Inner struct { Count int }; func use() {" + tc.setup + "; _ = " + tc.call + "}"
			fset := token.NewFileSet()
			f, err := parser.ParseFile(fset, "fixture.go", source, parser.SkipObjectResolution)
			if err != nil {
				t.Fatal(err)
			}
			order, types := map[string][]string{}, map[string][]string{}
			collectFields(fset, f, "fixture.go", map[string][]field{}, order, types)
			writes := map[string]bool{}
			collectUses(f, order, types, writes, map[string]bool{})
			want := map[string]bool{}
			if tc.written {
				want = map[string]bool{"Nested": true, "Next": true, "Count": true}
			}
			if !reflect.DeepEqual(writes, want) {
				t.Errorf("writes = %v, want %v", writes, want)
			}
		})
	}
}

func TestDedupe_SortsWithoutMutatingInput(t *testing.T) {
	in := []string{"Z.Last", "A.First", "Z.Last", "M.Middle"}
	before := append([]string(nil), in...)
	got := dedupe(in)
	want := []string{"A.First", "M.Middle", "Z.Last"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("dedupe = %v, want %v", got, want)
	}
	if !reflect.DeepEqual(in, before) {
		t.Errorf("input changed: %v, want %v", in, before)
	}
	if got := dedupe(nil); got != nil {
		t.Errorf("dedupe(nil) = %v, want nil", got)
	}
}

func TestAuditDarkFields_Command(t *testing.T) {
	if os.Getenv("AUDIT_DARK_FIELDS_TEST_HELPER") == "1" {
		flag.CommandLine = flag.NewFlagSet("audit_dark_fields", flag.ExitOnError)
		os.Args = []string{"audit_dark_fields"}
		if os.Getenv("AUDIT_DARK_FIELDS_TEST_UPDATE") == "1" {
			os.Args = append(os.Args, "-update")
		}
		main()
		return
	}
	const dark = "package fixture\ntype Item struct { Dark int }\nfunc read(x Item) int { return x.Dark }\n"
	cases := []struct {
		name, source, baseline               string
		extra                                map[string]string
		missingBaseline, missingRoot, update bool
		exit                                 int
		output, saved                        string
	}{
		{name: "unchanged budget", source: dark, baseline: "# comment\n\n Item.Dark\told.go:2\treason\n", output: "budget holds: 1 fields"},
		{name: "new dark field", source: dark, exit: 1, output: "\ninternal/fixture.go:2: dark field (read, never written): Item.Dark\n"},
		{name: "removed dark field", source: "package fixture", baseline: "Item.Dark\told.go:2\n", exit: 1, output: "Fields that are no longer dark (1):\n  Item.Dark"},
		{name: "missing baseline", source: dark, missingBaseline: true, exit: 2, output: "no baseline at"},
		{name: "parse error", source: "package fixture; type", exit: 2, output: "parse internal/fixture.go:"},
		{name: "missing root", source: dark, missingRoot: true, exit: 2, output: "internal"},
		{name: "parenthesized method is not a field read", source: "package fixture; type Item struct { Run int }; type Runner struct{}; func (Runner) Run() {}; func call(x Runner) { (x.Run)() }", output: "budget holds: 0 fields"},
		{name: "test writers excluded", source: dark, extra: map[string]string{"internal/fixture_test.go": "package fixture; func fill(x *Item) { x.Dark = 1 }"}, exit: 1, output: "New dark fields (1)"},
		{name: "test only trees excluded", source: "package fixture", extra: map[string]string{"internal/testing/bad.go": "not Go", "internal/types/typestest/bad.go": "not Go", "cmd/fixture_test.go": "not Go", "internal/notes.txt": "not Go"}, output: "budget holds: 0 fields"},
		{name: "update baseline", source: dark, update: true, output: "Baseline updated: 1 dark fields.", saved: "Item.Dark\tinternal/fixture.go:2\n"},
		{name: "annotate dead file", source: dark, update: true, extra: map[string]string{deadcodeBaselinePath: "# comment\n\n internal/fixture.go\tunusedFunction\n"}, output: "Baseline updated: 1 dark fields.", saved: "Item.Dark\tinternal/fixture.go:2\t(dead file)\n"},
		{name: "new dark fields sorted", source: "package fixture\ntype Item struct {\nZed int\nAlpha int\n}\nfunc read(x Item) int { return x.Zed + x.Alpha }\n", exit: 1, output: "\ninternal/fixture.go:4: dark field (read, never written): Item.Alpha\ninternal/fixture.go:3: dark field (read, never written): Item.Zed\n"},
		{name: "update baseline sorts entries", source: "package fixture\ntype Item struct {\nZed int\nAlpha int\n}\nfunc read(x Item) int { return x.Zed + x.Alpha }\n", update: true, output: "Baseline updated: 2 dark fields.", saved: "Item.Alpha\tinternal/fixture.go:4\nItem.Zed\tinternal/fixture.go:3\n"},
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			put := func(path, content string) {
				t.Helper()
				target := filepath.Join(dir, path)
				if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(target, []byte(content), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.MkdirAll(filepath.Join(dir, "cmd"), 0o755); err != nil {
				t.Fatal(err)
			}
			if !tc.missingRoot {
				put("internal/fixture.go", tc.source)
			}
			if !tc.missingBaseline {
				put(baselinePath, tc.baseline)
			}
			for path, content := range tc.extra {
				put(path, content)
			}
			ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, executable, "-test.run=^TestAuditDarkFields_Command$")
			cmd.Dir = dir
			cmd.Env = []string{"AUDIT_DARK_FIELDS_TEST_HELPER=1"}
			if tc.update {
				cmd.Env = append(cmd.Env, "AUDIT_DARK_FIELDS_TEST_UPDATE=1")
			}
			output, err := cmd.CombinedOutput()
			exit := 0
			if err != nil {
				var exitErr *exec.ExitError
				if !errors.As(err, &exitErr) {
					t.Fatalf("run audit: %v\n%s", err, output)
				}
				exit = exitErr.ExitCode()
			}
			if exit != tc.exit {
				t.Fatalf("exit = %d, want %d\n%s", exit, tc.exit, output)
			}
			if !strings.Contains(string(output), tc.output) {
				t.Errorf("output = %q, want substring %q", output, tc.output)
			}
			if tc.saved != "" {
				raw, err := os.ReadFile(filepath.Join(dir, baselinePath))
				if err != nil {
					t.Fatal(err)
				}
				var entries []string
				for _, line := range strings.Split(string(raw), "\n") {
					if line != "" && !strings.HasPrefix(line, "#") {
						entries = append(entries, line)
					}
				}
				if got := strings.Join(entries, "\n") + "\n"; got != tc.saved {
					t.Errorf("baseline entries = %q, want %q", got, tc.saved)
				}
			}
		})
	}
}

func TestWriteBaseline_SortsLinesDeterministically(t *testing.T) {
	if err := os.MkdirAll(filepath.Dir(baselinePath), 0o755); err != nil {
		t.Fatalf("mkdir baseline dir: %v", err)
	}
	orig, readErr := os.ReadFile(baselinePath)
	hadOrig := readErr == nil
	t.Cleanup(func() {
		if hadOrig {
			_ = os.WriteFile(baselinePath, orig, 0o644)
		} else {
			_ = os.Remove(baselinePath)
		}
	})
	unsorted := []string{"Zebra.Field", "Apple.Field", "Mango.Field"}
	before := append([]string(nil), unsorted...)
	writeBaseline(unsorted, nil)
	for i := range unsorted {
		if unsorted[i] != before[i] {
			t.Fatalf("writeBaseline mutated input: got %v want %v", unsorted, before)
		}
	}
	raw, err := os.ReadFile(baselinePath)
	if err != nil {
		t.Fatalf("read baseline: %v", err)
	}
	var got []string
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		got = append(got, strings.Split(line, "\t")[0])
	}
	want := []string{"Apple.Field", "Mango.Field", "Zebra.Field"}
	if len(got) != len(want) {
		t.Fatalf("baseline entries = %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("baseline entries = %v want %v", got, want)
		}
	}
}

func TestCompare_HoldsOnUnsortedInput(t *testing.T) {
	if err := os.MkdirAll(filepath.Dir(baselinePath), 0o755); err != nil {
		t.Fatalf("mkdir baseline dir: %v", err)
	}
	orig, readErr := os.ReadFile(baselinePath)
	hadOrig := readErr == nil
	t.Cleanup(func() {
		if hadOrig {
			_ = os.WriteFile(baselinePath, orig, 0o644)
		} else {
			_ = os.Remove(baselinePath)
		}
	})
	sorted := []string{"Apple.Field", "Mango.Field", "Zebra.Field"}
	unsorted := []string{"Zebra.Field", "Apple.Field", "Mango.Field"}
	writeBaseline(sorted, nil)
	compare(unsorted, nil)
	compare(sorted, nil)
	writeBaseline(nil, nil)
	compare(nil, nil)
}
