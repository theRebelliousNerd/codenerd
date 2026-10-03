package mcp_test

import (
	"strings"
	"testing"
	"time"

	"codenerd/internal/mangle"
	"codenerd/internal/mcp"
)

func TestFactEmitter_WhenValueNeedsEscaping_ShouldProduceParseableFacts(t *testing.T) {
	tool := &mcp.MCPTool{
		ToolID:       "srv/weird",
		ServerID:     "srv",
		Name:         "weird",
		Description:  "quotes \" backslash \\ newline \n emoji ✅ " + strings.Repeat("x", 600),
		Categories:   []string{"Code Analysis", "3d-render"},
		Capabilities: []string{"/read"},
		Domain:       "/general",
		RegisteredAt: time.Unix(1700000000, 0),
	}

	for _, fact := range mcp.ToolFactsForTest(tool) {
		if _, err := mangle.ParseAtom(fact); err != nil {
			t.Errorf("emitted fact is not parseable: %s (%v)", fact, err)
		}
	}
}
