package learnjudge

import (
	"context"
	"fmt"
	"strings"

	"codenerd/internal/typesafe"
)

// Ouroboros tool checks (internal/autopoiesis/tool_jev.go reads the answers).
// Before a tool is generated, Jev is asked whether a tool codeNERD already
// generated does what the request needs, and which one; after, whether the
// generated code does what the request asked and whether it also does
// something the request did not ask for.

// ToolRequest is the tool a request asks for, as a person would read it. The
// proposed name is left out of the duplicate check: a name shared with an
// existing tool would pull the choice toward it whatever the tools do.
type ToolRequest struct {
	Name    string
	Purpose string
	Input   string
	Output  string
}

// ExistingTool is one tool already registered: its name and what it does.
type ExistingTool struct {
	Name        string
	Description string
}

// MaxToolOptions is the most options one Choice takes (the API's limit).
const MaxToolOptions = 255

// DuplicateCheck is Jev's answer to whether an existing tool already does what
// a request needs: AnyExists is the companion yes/no, Tool the Choice's top
// option and ToolProbability its probability. Code reads Tool only when
// AnyExists reads yes.
type DuplicateCheck struct {
	AnyExists       float64
	Tool            string
	ToolProbability float64
	Model           string
}

// Question names, for the request and the recorded probes.
const (
	QAnyToolExists  = "any_tool_does_this"
	QWhichTool      = "which_tool_does_this"
	QCodeDoesAsked  = "code_does_what_was_asked"
	QCodeDoesExtra  = "code_does_unrequested_things"
	codeStateLimit  = 60000 // bytes of code in one question: well inside Jev's 32k-token question context
	toolStateDetail = "codeNERD is a coding agent that writes small Go command-line tools for itself."
)

// CheckDuplicate asks, in one request, whether any of tools already does what
// req needs and which one. tools must be non-empty and at most MaxToolOptions;
// the caller decides what to do when there are more.
func CheckDuplicate(ctx context.Context, a Asker, req ToolRequest, tools []ExistingTool) (DuplicateCheck, error) {
	state, questions, err := DuplicateRequest(req, tools)
	if err != nil {
		return DuplicateCheck{}, err
	}
	resp, err := a.Ask(typesafe.WithOperation(ctx, OpToolCheck), state, questions)
	if err != nil {
		return DuplicateCheck{}, err
	}
	exists, err := resp.Noul(QAnyToolExists)
	if err != nil {
		return DuplicateCheck{}, err
	}
	which, err := resp.Choice(QWhichTool)
	if err != nil {
		return DuplicateCheck{}, err
	}
	return DuplicateCheck{AnyExists: exists, Tool: which.Choice, ToolProbability: which.Probabilities[which.Choice], Model: resp.Model}, nil
}

// DuplicateRequest builds the state and questions of the duplicate check.
func DuplicateRequest(req ToolRequest, tools []ExistingTool) (map[string]any, map[string]typesafe.Question, error) {
	if len(tools) == 0 {
		return nil, nil, fmt.Errorf("duplicate check needs at least one existing tool")
	}
	if len(tools) > MaxToolOptions {
		return nil, nil, fmt.Errorf("duplicate check takes at most %d tools, got %d", MaxToolOptions, len(tools))
	}
	options := make(map[string]any, len(tools))
	listed := make([]map[string]string, 0, len(tools))
	for _, t := range tools {
		var desc any
		if d := strings.TrimSpace(t.Description); d != "" {
			desc = d
		}
		options[t.Name] = desc
		listed = append(listed, map[string]string{"name": t.Name, "does": strings.TrimSpace(t.Description)})
	}
	state := map[string]any{
		"context":        toolStateDetail,
		"request":        requestFields(req, false),
		"existing_tools": listed,
	}
	questions := map[string]typesafe.Question{
		QAnyToolExists: typesafe.Noul{
			Instructions: "Does one of `existing_tools` already do what `request` needs?",
			Criteria: &typesafe.NoulCriteria{
				True:  "Running one of the existing tools, as it is, would give the user what the request asks for.",
				False: "Each existing tool does a different job, or only part of the job, or works on a different kind of input or output than the request needs.",
			},
		},
		QWhichTool: typesafe.Choice{
			Instructions: "Which of these existing tools comes closest to doing what `request` needs?",
			Criteria:     options,
		},
	}
	return state, questions, nil
}

// CodeCheck is Jev's reading of generated code against its request: the
// probability that the code does what was asked, and that it also does
// something the request did not ask for.
type CodeCheck struct {
	DoesAsked float64
	DoesExtra float64
	Model     string
}

// CheckCode asks, in one request, whether code does what req asks and whether
// it does things req did not ask for.
func CheckCode(ctx context.Context, a Asker, req ToolRequest, code string) (CodeCheck, error) {
	state, questions := CodeRequest(req, code)
	resp, err := a.Ask(typesafe.WithOperation(ctx, OpToolCheck), state, questions)
	if err != nil {
		return CodeCheck{}, err
	}
	asked, err := resp.Noul(QCodeDoesAsked)
	if err != nil {
		return CodeCheck{}, err
	}
	extra, err := resp.Noul(QCodeDoesExtra)
	if err != nil {
		return CodeCheck{}, err
	}
	return CodeCheck{DoesAsked: asked, DoesExtra: extra, Model: resp.Model}, nil
}

// CodeRequest builds the state and questions of the code check. The code goes
// whole up to codeStateLimit; past it, the head and tail go and the state
// says how much was left out.
func CodeRequest(req ToolRequest, code string) (map[string]any, map[string]typesafe.Question) {
	state := map[string]any{
		"context": toolStateDetail,
		"request": requestFields(req, true),
	}
	putBounded(state, "code", code, codeStateLimit)
	questions := map[string]typesafe.Question{
		QCodeDoesAsked: typesafe.Noul{
			Instructions: "Does `code` do what `request` asks the tool to do?",
			Criteria: &typesafe.NoulCriteria{
				True:  "Given the input the request describes, the code produces the output the request describes.",
				False: "The code does a different job, leaves out part of what the request asks, returns a placeholder or fixed answer, or reads or produces a different kind of data than the request describes.",
			},
		},
		QCodeDoesExtra: typesafe.Noul{
			Instructions: "Does `code` also do something that `request` does not ask for?",
			Criteria: &typesafe.NoulCriteria{
				True:  "The code also writes, deletes, moves or sends something, starts another program, or does a second task that the request does not mention.",
				False: "Everything the code does serves the request. Reading its input, parsing, validation, error messages and printing the result all serve the request.",
			},
		},
	}
	return state, questions
}

// requestFields is the request as named fields; empty fields are left out,
// since an empty value reads to the model as a signal.
func requestFields(req ToolRequest, withName bool) map[string]string {
	out := map[string]string{}
	if withName && strings.TrimSpace(req.Name) != "" {
		out["tool_name"] = strings.TrimSpace(req.Name)
	}
	if v := strings.TrimSpace(req.Purpose); v != "" {
		out["purpose"] = v
	}
	if v := strings.TrimSpace(req.Input); v != "" {
		out["input"] = v
	}
	if v := strings.TrimSpace(req.Output); v != "" {
		out["output"] = v
	}
	return out
}
