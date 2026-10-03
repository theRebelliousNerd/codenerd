package mcpctl_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/mcp"
	"codenerd/internal/tools/mcpctl"
)

type facadeAuthorityKernel struct{ kernel *core.RealKernel }

func (adapter *facadeAuthorityKernel) Assert(fact string) error {
	return adapter.kernel.AssertString(fact)
}
func (adapter *facadeAuthorityKernel) Retract(fact string) error {
	parsed, err := core.ParseFactString(fact)
	if err != nil {
		return err
	}
	return adapter.kernel.RetractExactFactsBatch([]core.Fact{parsed})
}
func (adapter *facadeAuthorityKernel) Query(query string) ([]map[string]any, error) {
	facts, err := adapter.kernel.Query(query)
	if err != nil {
		return nil, err
	}
	rows := make([]map[string]any, len(facts))
	for index := range rows {
		rows[index] = map[string]any{}
	}
	return rows, nil
}

func TestMCPCallFacade_ConfirmationCannotSupplyConstitutionalAuthority(t *testing.T) {
	var dispatches atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var rpc struct {
			ID     any    `json:"id"`
			Method string `json:"method"`
		}
		if err := json.NewDecoder(request.Body).Decode(&rpc); err != nil {
			http.Error(writer, err.Error(), http.StatusBadRequest)
			return
		}
		var result any
		switch rpc.Method {
		case "initialize":
			result = map[string]any{"capabilities": map[string]any{"tools": true}}
		case "tools/list":
			result = map[string]any{"tools": []any{map[string]any{
				"name": "remove", "description": "Remove a remote record", "inputSchema": map[string]any{"type": "object"},
				"annotations": map[string]any{"destructiveHint": true},
			}}}
		case "tools/call":
			dispatches.Add(1)
			result = map[string]any{"removed": true}
		default:
			result = map[string]any{}
		}
		if err := json.NewEncoder(writer).Encode(map[string]any{"jsonrpc": "2.0", "id": rpc.ID, "result": result}); err != nil {
			t.Error(err)
		}
	}))
	t.Cleanup(server.Close)
	kernel, err := core.NewRealKernelWithWorkspace(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	adapter := &facadeAuthorityKernel{kernel: kernel}
	emitter := mcp.NewFactEmitter(adapter)
	manager := mcp.NewMCPClientManager(nil, nil, map[string]mcp.MCPServerConfig{"srv": {ID: "srv", Enabled: true, Protocol: "http", BaseURL: server.URL, Timeout: "5s"}})
	manager.SetFactEmitter(emitter)
	t.Cleanup(manager.DisconnectAll)
	ctx := context.Background()
	if err := manager.Connect(ctx, "srv"); err != nil {
		t.Fatal(err)
	}
	if err := manager.DiscoverTools(ctx, "srv"); err != nil {
		t.Fatal(err)
	}
	catalog := manager.GetAllTools()
	if len(catalog) != 1 {
		t.Fatal("remote catalog missing")
	}
	if err := manager.SetReviewedToolEffect(mcp.ReviewedToolEffect{ToolID: catalog[0].ToolID, SchemaHash: catalog[0].SchemaHash, Effect: mcp.RemoteDelete, RiskOverride: mcp.RiskSafe}); err != nil {
		t.Fatal(err)
	}
	plane := mcp.NewControlPlane(manager, nil, emitter)
	plane.SetRiskGate(mcp.KernelRiskGate(adapter))
	mcpctl.SetControlPlane(plane)
	t.Cleanup(func() { mcpctl.SetControlPlane(nil) })
	call := func(wantSuccess bool, wantDispatches int64) {
		t.Helper()
		text, err := mcpctl.CallTool().Execute(ctx, map[string]any{"tool": "srv/remove", "args": map[string]any{}, "confirm_risk": true})
		if err != nil {
			t.Fatal(err)
		}
		var receipt struct {
			Success bool `json:"success"`
		}
		if err := json.Unmarshal([]byte(text), &receipt); err != nil {
			t.Fatal(err)
		}
		if receipt.Success != wantSuccess || dispatches.Load() != wantDispatches {
			t.Fatalf("success=%v dispatches=%d receipt=%s", receipt.Success, dispatches.Load(), text)
		}
	}
	call(false, 0)
	for _, fact := range []string{"signed_approval(/delete_file)", `admin_override("host-reviewer")`} {
		if err := adapter.Assert(fact); err != nil {
			t.Fatal(err)
		}
	}
	call(true, 1)
	if err := adapter.Retract("signed_approval(/delete_file)"); err != nil {
		t.Fatal(err)
	}
	call(false, 1)
}
