package core

import (
	"context"
	"testing"

	"codenerd/internal/tools"
)

func TestHandleModularTool_WebSearchTargetIsQuery(t *testing.T) {
	vs, _ := createActionsTestVS(t)
	var got map[string]any
	err := vs.modularTools.Register(&tools.Tool{
		Effect:   tools.EffectRead,
		Name:     "web_search",
		Category: tools.CategoryResearch,
		Execute: func(ctx context.Context, args map[string]any) (string, error) {
			got = args
			return "ok", nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	res, err := vs.handleModularTool(context.Background(), ActionRequest{
		Type:    ActionWebSearch,
		Target:  "rod browser",
		Payload: map[string]any{"max_results": 3, "query": "ignored"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Success {
		t.Fatalf("handleModularTool: %+v", res)
	}
	if got["query"] != "rod browser" {
		t.Fatalf("query = %#v, want the action target", got["query"])
	}
	if got["max_results"] != 3 {
		t.Fatalf("max_results = %#v, want the payload value", got["max_results"])
	}

	got = nil
	res, err = vs.handleModularTool(context.Background(), ActionRequest{
		Type:    ActionWebSearch,
		Payload: map[string]any{"query": "from-payload"},
	})
	if err != nil || !res.Success {
		t.Fatalf("empty target: %v %+v", err, res)
	}
	if got["query"] != "from-payload" {
		t.Fatalf("empty target query = %#v, want the payload query", got["query"])
	}
}
