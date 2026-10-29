package perception

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Meta's strict mode cannot express the canonical envelope: the free-form
// tool_args node ({"type":"object"} with no properties) can never carry
// additionalProperties:false, and Meta rejects any object without it with
// HTTP 400 "'additionalProperties' is required to be supplied and to be
// false." (measured live 2026-09-03: every piggyback call burned one rejected
// request before the retry). Meta piggyback traffic must therefore use plain
// JSON mode; every other vendor keeps the strict schema.
func TestPiggybackResponseFormat_VendorSelection(t *testing.T) {
	cases := []struct {
		name       string
		vendor     Provider
		wantType   string
		wantStrict bool
		wantSchema bool
	}{
		{"meta uses json_object", ProviderMeta, "json_object", false, false},
		{"openrouter keeps strict schema", ProviderOpenRouter, "json_schema", true, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := newTestCompatClient(t, tc.vendor, "https://example.invalid/v1")
			rf := c.piggybackResponseFormat()
			if rf == nil {
				t.Fatalf("piggybackResponseFormat() = nil, want Type %q", tc.wantType)
			}
			if rf.Type != tc.wantType {
				t.Errorf("Type = %q, want %q", rf.Type, tc.wantType)
			}
			if tc.wantSchema {
				if rf.JSONSchema == nil {
					t.Fatalf("JSONSchema = nil, want strict envelope schema")
				}
				if !rf.JSONSchema.Strict {
					t.Errorf("Strict = false, want true")
				}
				if rf.JSONSchema.Schema == nil {
					t.Error("Schema = nil, want the strict envelope schema")
				}
			} else if rf.JSONSchema != nil {
				t.Errorf("JSONSchema = %+v, want nil (plain JSON mode carries no schema)", rf.JSONSchema)
			}
		})
	}
}

// The Meta builder is JSON mode without a schema: the prompt already specifies
// the envelope, and no schema Meta can accept exists.
func TestBuildMetaPiggybackResponseFormat_IsJSONMode(t *testing.T) {
	rf := BuildMetaPiggybackResponseFormat()
	if rf == nil {
		t.Fatal("BuildMetaPiggybackResponseFormat() = nil")
	}
	if rf.Type != "json_object" {
		t.Errorf("Type = %q, want %q", rf.Type, "json_object")
	}
	if rf.JSONSchema != nil {
		t.Errorf("JSONSchema = %+v, want nil", rf.JSONSchema)
	}
}

// A Meta piggyback request must carry response_format.type == "json_object"
// with no json_schema key — the strict form is what Meta rejected with 400
// "'additionalProperties' is required to be supplied and to be false."
func TestMetaPiggybackRequest_UsesJSONMode(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var raw map[string]any
		if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
			t.Errorf("decode request body: %v", err)
		}
		gotBody = raw
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"hello"},"finish_reason":"stop"}]}`))
	}))
	defer srv.Close()

	c := newTestCompatClient(t, ProviderMeta, srv.URL)
	// The substring marks this as a piggyback (envelope) call; conversational
	// shards never set the explicit structured-output flag.
	if _, err := c.CompleteWithSystem(context.Background(), "return a control_packet envelope", "hi"); err != nil {
		t.Fatalf("CompleteWithSystem: %v", err)
	}

	rfRaw, ok := gotBody["response_format"]
	if !ok {
		t.Fatal("request has no response_format; piggyback envelope was not requested")
	}
	rf, ok := rfRaw.(map[string]any)
	if !ok {
		t.Fatalf("response_format = %T, want object", rfRaw)
	}
	if rf["type"] != "json_object" {
		t.Errorf("response_format.type = %v, want %q", rf["type"], "json_object")
	}
	if _, hasSchema := rf["json_schema"]; hasSchema {
		t.Errorf("response_format carries json_schema key %v; Meta rejects the strict schema", rf["json_schema"])
	}
}
