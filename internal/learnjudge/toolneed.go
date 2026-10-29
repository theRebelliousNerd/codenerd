package learnjudge

import (
	"context"
	"fmt"
	"strings"

	"codenerd/internal/typesafe"
)

// Tool need: whether a request (and, when there is one, a failed attempt at
// it) shows a capability gap that a new generated tool should fill. It replaces
// missingCapabilityPatterns and the rule that made any failed attempt a tool
// need. The kernel decides (policy/jev_learning.mg, tool_gap_observed); the
// existing-tool duplicate check (toolcheck.go) still runs before generation.

// ToolNeedInput is the request, the failed attempt's evidence ("" when there
// was none), and the tools already registered.
type ToolNeedInput struct {
	Request       string
	FailedAttempt string
	Tools         []ExistingTool
}

// Question names, which are also the judgment names in the kernel
// (tool_need_judgment).
const (
	AsksForTool          = "asks_for_tool"
	NeedsMissingOp       = "needs_missing_operation"
	FailedForMissingTool = "failed_for_missing_tool"
	ToolKind             = "tool_kind"
)

// ToolKinds are the Choice options for what kind of tool the request needs; a
// hint to the spec writer, not a decision.
var ToolKinds = map[string]any{
	"validator": "Checks whether data, a file or a config is valid.",
	"converter": "Turns data from one format into another.",
	"parser":    "Reads structured data out of text, files or command output.",
	"analyzer":  "Inspects code or data and reports what it finds.",
	"formatter": "Rewrites text or data into a required layout.",
	"utility":   "Some other single operation.",
}

// harnessCapabilities is what codeNERD can do without a generated tool: a fact
// the model needs to judge a gap, stated once.
const harnessCapabilities = "codeNERD can already read, search, write and edit files in the workspace, run builds and tests, run allowed shell commands, query its code index, and write code to complete a task."

const toolNeedTextLimit = 8000

// ToolNeedJudgment holds Jev's answers. Failed is set only when there was a
// failed attempt to ask about.
type ToolNeedJudgment struct {
	AsksForTool, NeedsMissingOp float64
	FailedForMissingTool        float64
	AskedAboutFailure           bool
	Kind                        string
	Model                       string
}

// ToolNeedRequest builds the state and the questions.
func ToolNeedRequest(in ToolNeedInput) (map[string]any, map[string]typesafe.Question) {
	state := map[string]any{"harness_capabilities": harnessCapabilities}
	putBounded(state, "request", strings.TrimSpace(in.Request), toolNeedTextLimit)
	if len(in.Tools) > 0 {
		tools := make([]map[string]string, 0, len(in.Tools))
		for _, t := range in.Tools {
			tools = append(tools, map[string]string{"name": t.Name, "does": t.Description})
		}
		state["generated_tools"] = tools
	}
	qs := map[string]typesafe.Question{
		AsksForTool: typesafe.Noul{
			Instructions: "Is the user asking codeNERD to create a new tool?",
			Criteria: &typesafe.NoulCriteria{
				True:  "The request asks for a tool, command or utility to be built, generated or added to codeNERD.",
				False: "The request asks for a task to be done (code written, a bug fixed, a question answered), not for a tool to be made.",
			},
		},
		NeedsMissingOp: typesafe.Noul{
			Instructions: "Does `request` need an operation that neither `harness_capabilities` nor any of `generated_tools` provides?",
			Criteria: &typesafe.NoulCriteria{
				True:  "Doing the request needs an operation the harness and the generated tools cannot perform at all, such as reading a format nothing parses or calling a service nothing reaches.",
				False: "The harness or a generated tool can already perform every operation the request needs, even if the task itself is large.",
			},
		},
		ToolKind: typesafe.Choice{
			Instructions: "What kind of tool would perform the operation `request` needs?",
			Criteria:     ToolKinds,
		},
	}
	if f := strings.TrimSpace(in.FailedAttempt); f != "" {
		putBounded(state, "failed_attempt", f, toolNeedTextLimit)
		qs[FailedForMissingTool] = typesafe.Noul{
			Instructions: "Did `failed_attempt` fail because no available tool could perform an operation it needed?",
			Criteria: &typesafe.NoulCriteria{
				True:  "The attempt stopped because an operation it needed does not exist: no tool or command for it.",
				False: "The attempt failed for another reason: a wrong argument, bad code, a missing file, a failing test, or a transient error such as a timeout or rate limit.",
			},
		}
	}
	return state, qs
}

// JudgeToolNeed asks the questions. A failed call is an error, never a "no".
func JudgeToolNeed(ctx context.Context, a Asker, in ToolNeedInput) (*ToolNeedJudgment, error) {
	if a == nil {
		return nil, fmt.Errorf("learnjudge: no Jev client")
	}
	if strings.TrimSpace(in.Request) == "" {
		return nil, fmt.Errorf("learnjudge: empty request")
	}
	state, qs := ToolNeedRequest(in)
	resp, err := a.Ask(typesafe.WithOperation(ctx, OpToolCheck), state, qs)
	if err != nil {
		return nil, err
	}
	j := &ToolNeedJudgment{Model: resp.Model}
	if j.AsksForTool, err = resp.Noul(AsksForTool); err != nil {
		return nil, err
	}
	if j.NeedsMissingOp, err = resp.Noul(NeedsMissingOp); err != nil {
		return nil, err
	}
	kind, err := resp.Choice(ToolKind)
	if err != nil {
		return nil, err
	}
	j.Kind = kind.Choice
	if _, ok := qs[FailedForMissingTool]; ok {
		if j.FailedForMissingTool, err = resp.Noul(FailedForMissingTool); err != nil {
			return nil, err
		}
		j.AskedAboutFailure = true
	}
	return j, nil
}
