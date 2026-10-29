package perception

import (
	"strings"
	"testing"
)

// Meta's Model API rejects malformed tool payloads with HTTP 400 rather than
// degrading, and the error body does not say which tool was at fault. These
// tests pin the client-side guard so a bad payload fails locally, with the
// offending name in the message, instead of costing a round trip and an
// opaque 400.

func metaTestClient(t *testing.T, vendor Provider) *OpenAICompatClient {
	t.Helper()
	cfg := DefaultOpenAICompatConfig(vendor, "test-key")
	c, err := NewOpenAICompatClient(cfg)
	if err != nil {
		t.Fatalf("NewOpenAICompatClient(%s): %v", vendor, err)
	}
	return c
}

func TestValidateMetaTools_RejectsBadToolNames(t *testing.T) {
	c := metaTestClient(t, ProviderMeta)

	cases := []struct {
		name string
		tool string
	}{
		{"empty", ""},
		{"space", "read file"},
		{"slash", "fs/read"},
		{"colon", "mcp:read"},
		{"two dots", "a.b.c"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := c.validateMetaTools([]ToolDefinition{{Name: tc.tool}}, nil)
			if err == nil {
				t.Fatalf("validateMetaTools accepted tool name %q, which Meta rejects with HTTP 400", tc.tool)
			}
		})
	}
}

func TestValidateMetaTools_AcceptsLegalToolNames(t *testing.T) {
	c := metaTestClient(t, ProviderMeta)

	legal := []string{"read_file", "write-file", "mcp.read", "Tool123", "a.b"}
	for _, name := range legal {
		if err := c.validateMetaTools([]ToolDefinition{{Name: name}}, nil); err != nil {
			t.Errorf("validateMetaTools rejected legal tool name %q: %v", name, err)
		}
	}
}

func TestValidateMetaTools_RejectsOutOfRangeCallIDs(t *testing.T) {
	c := metaTestClient(t, ProviderMeta)
	long := strings.Repeat("x", 65)

	if err := c.validateMetaTools(nil, []OpenAIMessage{{Role: "tool", ToolCallID: long}}); err == nil {
		t.Error("validateMetaTools accepted a 65-character tool-result call_id; Meta's limit is 64")
	}
	if err := c.validateMetaTools(nil, []OpenAIMessage{{
		Role:      "assistant",
		ToolCalls: []OpenAIToolCall{{ID: long}},
	}}); err == nil {
		t.Error("validateMetaTools accepted a 65-character assistant call_id; Meta's limit is 64")
	}
	if err := c.validateMetaTools(nil, []OpenAIMessage{{
		Role:      "assistant",
		ToolCalls: []OpenAIToolCall{{ID: ""}},
	}}); err == nil {
		t.Error("validateMetaTools accepted an empty assistant call_id; Meta requires 1-64 characters")
	}

	ok := strings.Repeat("x", 64)
	if err := c.validateMetaTools(nil, []OpenAIMessage{{Role: "tool", ToolCallID: ok}}); err != nil {
		t.Errorf("validateMetaTools rejected a legal 64-character call_id: %v", err)
	}
}

// The guard is vendor-scoped on purpose: DashScope and Moonshot document no
// such limits, and applying Meta's constraints to them would reject payloads
// those vendors accept.
func TestValidateMetaTools_IsNoOpForOtherVendors(t *testing.T) {
	for _, vendor := range []Provider{ProviderDashScope, ProviderMoonshot} {
		c := metaTestClient(t, vendor)
		err := c.validateMetaTools(
			[]ToolDefinition{{Name: "illegal/name.with.dots"}},
			[]OpenAIMessage{{Role: "tool", ToolCallID: strings.Repeat("x", 200)}},
		)
		if err != nil {
			t.Errorf("validateMetaTools must not constrain %s traffic, got: %v", vendor, err)
		}
	}
}
