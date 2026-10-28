package learnjudge

import (
	"context"
	"fmt"
	"strings"

	"codenerd/internal/typesafe"
)

// Tool value: observations about a generated tool that the tool-value rules
// read (policy/jev_learning.mg, tool_value_evidence). Whether a tool earns its
// place is the kernel's decision over these and the measured ledger (cost,
// condensation, use counts); this file only asks.
//
//   - Per call, whether the output gave the caller what its input asked for:
//     OutputGivesWhatWasAsked, asked in the same request as the tool-output
//     judgments (autopoiesis.go), so it costs no extra call.
//   - Per tool, whether another registered tool already does the same thing:
//     CheckDuplicate (toolcheck.go), with the tool's own description as the
//     request.
//   - Per tool, whether the harness or a shell command already does it:
//     HarnessCovers, below.

// HarnessCoversName is the question's name.
const HarnessCoversName = "harness_covers"

// HarnessCoversRequest builds the state and the question for one tool.
func HarnessCoversRequest(tool ExistingTool) (map[string]any, map[string]typesafe.Question) {
	state := map[string]any{
		"harness_capabilities": harnessCapabilities,
		"tool":                 map[string]string{"name": tool.Name, "does": strings.TrimSpace(tool.Description)},
	}
	return state, map[string]typesafe.Question{
		HarnessCoversName: typesafe.Noul{
			Instructions: "Could `harness_capabilities`, or one ordinary shell command, already do what `tool` does?",
			Criteria: &typesafe.NoulCriteria{
				True:  "The tool repeats something the harness does directly or one standard command does (grep, find, wc, jq, go vet, a test run).",
				False: "The tool performs an operation the harness lacks, or does in one call what would take many harness steps.",
			},
		},
	}
}

// JudgeHarnessCovers asks whether the harness already covers the tool. A
// failed call is an error, never a "no".
func JudgeHarnessCovers(ctx context.Context, a Asker, tool ExistingTool) (float64, error) {
	if a == nil {
		return 0, fmt.Errorf("learnjudge: no Jev client")
	}
	if strings.TrimSpace(tool.Description) == "" {
		return 0, fmt.Errorf("learnjudge: tool %s has no description to judge", tool.Name)
	}
	state, qs := HarnessCoversRequest(tool)
	resp, err := a.Ask(typesafe.WithOperation(ctx, OpToolCheck), state, qs)
	if err != nil {
		return 0, err
	}
	return resp.Noul(HarnessCoversName)
}
