package core

import (
	"context"
	"fmt"
	"strings"

	"codenerd/internal/tools"
)

// maxContextSearchRecords is the working store's ceiling on records per
// search (WorkingStore.Search).
const maxContextSearchRecords = 50

// retainedHandleRedeemers are the other handle kinds the harness hands the
// model, each with the read-only verb that redeems it. recall_context accepts
// them too: a model holding an id reaches for the one recall verb, and it used
// to be told "no archived observation has id obs:sa:..." for a subagent handle
// that subagent_expand would have read (observed 2026-09-22, campaign
// 7b853890). Their offset and limit count lines.
var retainedHandleRedeemers = map[string]func(context.Context, map[string]any) (string, error){
	"obs:sa:": executeSubagentExpand,
	"obs:cs:": executeSearchExpand,
}

func RecallContextTool() *tools.Tool {
	return &tools.Tool{
		Name: "recall_context", Description: "Recover a page of an archived observation by its context record ID, or the retained text behind any handle the harness gave you (a subagent return obs:sa:..., a code search obs:cs:...). Returns original revision and provenance; historical observations are not current verification.", Category: tools.CategoryGeneral, Priority: 65,
		Schema: tools.ToolSchema{Properties: map[string]tools.Property{
			"id":     {Type: "string", Description: "Observation ID from working context, or a retained handle (obs:sa:..., obs:cs:...)"},
			"query":  {Type: "string", Description: "Literal archive search when the observation ID is unknown; provide query or id"},
			"offset": {Type: "integer", Description: "Character offset, default zero (lines, for a retained handle)"},
			"limit":  {Type: "integer", Description: "With id: page characters; omitted returns the rest of the body from offset. Page only when a whole body was reported as not fitting the request. With query: number of records, at most 50. Lines, for a retained handle"},
		}}, Execute: func(ctx context.Context, args map[string]any) (string, error) {
			id, _ := args["id"].(string)
			query, _ := args["query"].(string)
			if (id == "") == (query == "") {
				return "", fmt.Errorf("provide exactly one of id or query")
			}
			for prefix, redeem := range retainedHandleRedeemers {
				if strings.HasPrefix(strings.TrimSpace(id), prefix) {
					handleArgs := map[string]any{"handle": strings.TrimSpace(id)}
					if v, ok := args["offset"]; ok {
						handleArgs["offset"] = v
					}
					if v, ok := args["limit"]; ok {
						handleArgs["max_lines"] = v
					}
					return redeem(ctx, handleArgs)
				}
			}
			recall := tools.ContextRecallFrom(ctx)
			if recall == nil {
				return "", fmt.Errorf("working context recall unavailable")
			}
			integer := func(key string, fallback int) (int, error) {
				raw, present := args[key]
				if !present || raw == nil {
					return fallback, nil
				}
				// Exact page addresses read through the canonical strict
				// helper: fractional bounds are refused, and Mangle-sourced
				// int64 values — which the old local switch rejected
				// outright — are honored like every other integral shape.
				v, ok := tools.ArgIntStrict(args, key)
				if !ok {
					return 0, fmt.Errorf("%s must be integral", key)
				}
				return v, nil
			}
			offset, err := integer("offset", 0)
			if err != nil {
				return "", err
			}
			// A body is returned whole unless the caller pages: the default
			// page used to be 2000 characters, and a model handed a 14 KB
			// observation in 2000-character pages re-read the file instead.
			defaultLimit := 0
			if query != "" {
				defaultLimit = 10
			}
			limit, err := integer("limit", defaultLimit)
			if err != nil {
				return "", err
			}
			if query != "" {
				search, ok := recall.(interface {
					Search(context.Context, string, int, int) (string, error)
				})
				if !ok {
					return "", fmt.Errorf("context search unavailable")
				}
				// A search answers records, not characters. A model that
				// pages bodies by the thousand and then searches with the
				// same number was refused ("invalid context search bounds");
				// the store's record maximum is what it meant.
				if limit > maxContextSearchRecords {
					limit = maxContextSearchRecords
				}
				return search.Search(ctx, query, offset, limit)
			}
			return recall.Recall(ctx, id, offset, limit)
		},
	}
}
