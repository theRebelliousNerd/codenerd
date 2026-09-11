package core

import (
	"codenerd/internal/tools"
	"context"
	"fmt"
)

// maxContextSearchRecords is the working store's ceiling on records per
// search (WorkingStore.Search).
const maxContextSearchRecords = 50

func RecallContextTool() *tools.Tool {
	return &tools.Tool{
		Name: "recall_context", Description: "Recover a page of an archived observation by its context record ID. Returns original revision and provenance; historical observations are not current verification.", Category: tools.CategoryGeneral, Priority: 65,
		Schema: tools.ToolSchema{Properties: map[string]tools.Property{
			"id":     {Type: "string", Description: "Observation ID from working context"},
			"query":  {Type: "string", Description: "Literal archive search when the observation ID is unknown; provide query or id"},
			"offset": {Type: "integer", Description: "Character offset, default zero"},
			"limit":  {Type: "integer", Description: "With id: page characters; omitted returns the rest of the body from offset. Page only when a whole body was reported as not fitting the request. With query: number of records, at most 50"},
		}}, Execute: func(ctx context.Context, args map[string]any) (string, error) {
			recall := tools.ContextRecallFrom(ctx)
			if recall == nil {
				return "", fmt.Errorf("working context recall unavailable")
			}
			id, _ := args["id"].(string)
			query, _ := args["query"].(string)
			if (id == "") == (query == "") {
				return "", fmt.Errorf("provide exactly one of id or query")
			}
			integer := func(key string, fallback int) (int, error) {
				switch v := args[key].(type) {
				case nil:
					return fallback, nil
				case int:
					return v, nil
				case float64:
					if v != float64(int(v)) {
						return 0, fmt.Errorf("%s must be integral", key)
					}
					return int(v), nil
				default:
					return 0, fmt.Errorf("%s must be integral", key)
				}
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
