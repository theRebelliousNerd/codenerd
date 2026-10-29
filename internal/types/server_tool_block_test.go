package types

import (
	"bytes"
	"testing"
)

// A provider's server-tool block is neither a call to execute nor prose: the
// flat projection the executor reads carries no tool call and no text from
// it, and the block itself keeps the provider's bytes, in position, through
// a rewrite of the turn.
func TestServerToolBlock_IsNeitherACallNorText(t *testing.T) {
	raw := []byte(`{"type":"server_tool_use","id":"srvtoolu_1","name":"web_search","input":{"query":"q"}}`)
	tests := []struct {
		name   string
		blocks []ContentBlock
		text   string
		calls  int
	}{
		{name: "alone", blocks: []ContentBlock{ServerToolBlock(raw)}},
		{name: "beside an answer and a real call",
			blocks: []ContentBlock{ServerToolBlock(raw), TextBlock("answer"), ToolUseBlock("toolu_1", "read_file", map[string]any{"path": "a.go"})},
			text:   "answer", calls: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewAssistantMessage(tt.blocks...)
			if m.Text != tt.text || len(m.ToolCalls) != tt.calls {
				t.Fatalf("projection = (%q, %d calls), want (%q, %d)", m.Text, len(m.ToolCalls), tt.text, tt.calls)
			}
			for _, call := range m.ToolCalls {
				if call.Name == "web_search" {
					t.Fatalf("the server tool became a call the executor would run: %+v", call)
				}
			}
			if got := m.Content()[0]; got.Kind != BlockServerTool || !bytes.Equal(got.Raw, raw) {
				t.Fatalf("first block = %+v, want the server block verbatim", got)
			}
		})
	}

	resp := &LLMToolResponse{Blocks: []ContentBlock{ServerToolBlock(raw), TextBlock("old")}}
	resp.Rewrite("new", nil)
	if len(resp.Blocks) == 0 || resp.Blocks[0].Kind != BlockServerTool || !bytes.Equal(resp.Blocks[0].Raw, raw) {
		t.Fatalf("a rewrite dropped or altered the server block: %+v", resp.Blocks)
	}
}
