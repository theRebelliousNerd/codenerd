//go:build jevlive

package learnjudge

import (
	"context"
	"fmt"
	"testing"
)

var harnessCases = []struct {
	tool ExistingTool
	want bool
}{
	{ExistingTool{"line_count", "Counts the lines in a file."}, true},
	{ExistingTool{"grep_todo", "Lists every line containing TODO in the workspace's Go files."}, true},
	{ExistingTool{"go_vet_pkg", "Runs go vet on one package and returns its output."}, true},
	{ExistingTool{"avro_to_csv", "Decodes Avro container files and writes their records as CSV."}, false},
	{ExistingTool{"mangle_stratify", "Parses a Mangle program and reports its strata and every negation cycle."}, false},
	{ExistingTool{"pdf_tables", "Extracts tables from PDF files into markdown tables."}, false},
}

var callCases = []struct {
	name, tool, input, output string
	want                      bool
}{
	{"count-ok", "line_count", "internal/core/kernel.go", "412", true},
	{"count-err", "line_count", "internal/core/kernel.go", "open internal/core/kernel.go: no such file or directory", false},
	{"json-ok", "json_check", ".nerd/config.json", "valid JSON (14 top-level keys)", true},
	{"json-wrong-file", "json_check", ".nerd/config.json", "valid YAML", false},
	{"avro-partial", "avro_to_csv", "data/events.avro", "id,ts\n1,2026-10-01\n[stopped after 2 of 48000 records: schema evolution not supported]", false},
	{"stratify-ok", "mangle_stratify", "policy/jev_learning.mg", "3 strata; no negation cycles; tool_gap_observed in stratum 2", true},
}

func TestLiveToolValue(t *testing.T) {
	c, _ := liveClient(t, "tool_value")
	tally := map[string][3]int{}
	var misses, rows []string
	for _, tc := range harnessCases {
		p, err := JudgeHarnessCovers(probeLabel(context.Background(), "harness-"+tc.tool.Name), c, tc.tool)
		if err != nil {
			t.Fatal(err)
		}
		rows = append(rows, fmt.Sprintf("harness %-16s p=%.2f want=%v", tc.tool.Name, p, tc.want))
		scoreLine(tally, &misses, tc.tool.Name, HarnessCoversName, tc.tool.Description, p, tc.want)
	}
	for _, tc := range callCases {
		got, err := JudgeToolOutput(probeLabel(context.Background(), "call-"+tc.name), c, tc.tool, tc.input, tc.output)
		if err != nil {
			t.Fatal(err)
		}
		p := got[OutputGivesWhatWasAsked]
		rows = append(rows, fmt.Sprintf("call    %-16s p=%.2f want=%v", tc.name, p, tc.want))
		scoreLine(tally, &misses, tc.name, OutputGivesWhatWasAsked, tc.output, p, tc.want)
	}
	dup, err := CheckDuplicate(probeLabel(context.Background(), "dup-json"), c, ToolRequest{Purpose: "Validates that a file is well-formed JSON."},
		[]ExistingTool{{"json_lint", "Checks a JSON file parses and reports the first syntax error."}, {"line_count", "Counts the lines in a file."}})
	if err != nil {
		t.Fatal(err)
	}
	rows = append(rows, fmt.Sprintf("duplicate json_check -> %s any=%.2f (want yes)", dup.Tool, dup.AnyExists))
	scoreLine(tally, &misses, "dup-json", QAnyToolExists, "json_check", dup.AnyExists, true)
	dup2, err := CheckDuplicate(probeLabel(context.Background(), "dup-avro"), c, ToolRequest{Purpose: "Decodes Avro container files and writes their records as CSV."},
		[]ExistingTool{{"json_lint", "Checks a JSON file parses and reports the first syntax error."}, {"line_count", "Counts the lines in a file."}})
	if err != nil {
		t.Fatal(err)
	}
	rows = append(rows, fmt.Sprintf("duplicate avro_to_csv -> %s any=%.2f (want no)", dup2.Tool, dup2.AnyExists))
	scoreLine(tally, &misses, "dup-avro", QAnyToolExists, "avro_to_csv", dup2.AnyExists, false)
	lines := append([]string{"tool value probe"}, rows...)
	lines = append(lines, tallyLines(tally)...)
	probeSummary(t, "tool_value", append(append(lines, "misses (x wrong, ~ unsettled):"), misses...))
}
