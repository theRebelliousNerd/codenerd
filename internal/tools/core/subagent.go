package core

import (
	"context"
	"fmt"
	"strings"

	"codenerd/internal/logging"
	"codenerd/internal/observation"
	"codenerd/internal/tools"
)

// SubagentExpandToolName is named once so that every producer of a subagent
// handle names the same redemption verb. A result that tells the model to call
// a verb by the wrong name publishes a handle nobody can redeem, and the model
// has no way to tell that from an expired one.
const SubagentExpandToolName = "subagent_expand"

// SubagentExpandTool returns the verb that redeems a subagent-return handle.
//
// It reads the retained transcript and nothing else, and here that guarantee is
// doing more work than it does for search_expand next door. Re-running a search
// answers from a world that has moved; re-running a SUBAGENT writes files,
// spends tokens and may open a pull request. "Show me the rest of what you
// already told me" must never be a second delegation, and the codec is built so
// that it cannot be: observation.Subagents holds a retain.Store and has no way
// to reach an executor.
//
// It is a verb of its own rather than an argument on delegate, which is the
// other place a handle could plausibly be redeemed. Delegation is an effectful
// action gated on depth, budget and constitutional permission; folding
// redemption into it would mean asking permission to SPAWN AN AGENT in order to
// re-read bytes that are already in this process, and a policy that stops
// further delegation — the ordinary case at a depth cap — would take the
// transcript of the delegation that already happened down with it. The
// redemption of a handle has to grant strictly less than the operation that
// minted it, which is the same reasoning that put safe_action(/search_expand)
// in the constitution.
func SubagentExpandTool() *tools.Tool {
	return &tools.Tool{
		Name:          SubagentExpandToolName,
		AltCategories: []tools.ToolCategory{tools.CategoryReview, tools.CategoryCode},
		Description:   "Read the raw transcript behind a subagent-return handle. This never re-runs the subagent, so the lines it returns are exactly the ones the findings, changed files and verification status were derived from — and no work is repeated and no file is written. Narrow with 'match' to the lines containing a string, and walk long transcripts with 'offset'. A handle that has expired means the transcript is gone; do not re-delegate to get it back unless the task itself needs doing again.",
		Category:      tools.CategoryGeneral,
		Priority:      84,
		Execute:       executeSubagentExpand,
		Schema: tools.ToolSchema{
			Required: []string{"handle"},
			Properties: map[string]tools.Property{
				"handle": {
					Type:        "string",
					Description: "Handle reported by a previous subagent return (starts with obs:sa:)",
				},
				"match": {
					Type:        "string",
					Description: "Keep only transcript lines containing this string",
				},
				"offset": {
					Type:        "integer",
					Description: "Skip this many transcript lines (use the offset a previous expansion reported)",
					Default:     0,
				},
				"max_lines": {
					Type:        "integer",
					Description: "Maximum transcript lines to return (default 60, hard cap 200)",
					Default:     60,
				},
			},
		},
	}
}

func executeSubagentExpand(_ context.Context, args map[string]any) (string, error) {
	// Arguments are validated before the store is consulted: a missing handle
	// is wrong whether or not anything is retained, and reporting "not found"
	// for it would send the caller off re-delegating over a typo — which, for
	// this codec, means running the whole task again.
	handle, _ := args["handle"].(string)
	handle = strings.TrimSpace(handle)
	if handle == "" {
		return "", fmt.Errorf("handle is required; it is reported at the end of a subagent's return")
	}

	window := observation.ReturnWindow{}
	if match, ok := args["match"].(string); ok {
		window.Match = strings.TrimSpace(match)
	}
	if v, ok := argInt(args, "offset"); ok && v > 0 {
		window.Offset = v
	}
	if v, ok := argInt(args, "max_lines"); ok && v > 0 {
		window.Limit = v
	}

	hydrated, err := observation.SharedSubagents().HydrateReturn(handle, window)
	if err != nil {
		return "", err
	}
	logging.Tools("subagent_expand: handle=%s returned %d of %d retained line(s)",
		handle, len(hydrated.Lines), hydrated.Total)
	return hydrated.Text(), nil
}
