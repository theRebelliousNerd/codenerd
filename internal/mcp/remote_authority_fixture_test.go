package mcp

import (
	"encoding/json"
	"testing"
)

var RemoteAuthorityKernelForTest func(*testing.T) KernelInterface

func installRemoteAuthorityFixture(test *testing.T, manager *MCPClientManager, effects map[string]RemoteEffect) KernelInterface {
	test.Helper()
	if RemoteAuthorityKernelForTest == nil {
		test.Fatal("real MCP authority kernel fixture is not installed")
	}
	kernel := RemoteAuthorityKernelForTest(test)
	emitter := NewFactEmitter(kernel)
	manager.SetFactEmitter(emitter)
	for serverID, connection := range manager.servers {
		connection.Server.ID = serverID
		connection.Server.Status = ServerStatusConnected
		for toolID := range effects {
			boundServer, toolName := parseToolID(toolID)
			if boundServer != serverID {
				continue
			}
			found := false
			for _, tool := range connection.Tools {
				if tool.ToolID == toolID {
					found = true
				}
			}
			if !found {
				connection.Tools = append(connection.Tools, &MCPTool{
					ToolID: toolID, ServerID: serverID, Name: toolName,
					InputSchema: json.RawMessage(`{}`), Risk: RiskSafe, RiskSource: SourceAnnotation,
				})
			}
		}
		emitter.EmitServer(connection.Server)
		for _, tool := range connection.Tools {
			tool.SchemaHash = remoteSchemaHash(tool)
			emitter.EmitTool(tool)
			if effect, reviewed := effects[tool.ToolID]; reviewed {
				override := RiskClass("")
				if effect == RemoteRead {
					override = RiskSafe
				}
				if err := manager.SetReviewedToolEffect(ReviewedToolEffect{ToolID: tool.ToolID, SchemaHash: tool.SchemaHash, Effect: effect, RiskOverride: override}); err != nil {
					test.Fatal(err)
				}
			}
		}
		emitter.EmitResources(serverID, manager.resources[serverID])
		emitter.EmitPrompts(serverID, manager.prompts[serverID])
	}
	if err := emitter.Error(); err != nil {
		test.Fatal(err)
	}
	return kernel
}

func reviewRemoteFixtureSubject(test *testing.T, manager *MCPClientManager, operation RemoteOperation, subject *MCPTool) {
	test.Helper()
	if err := manager.SetReviewedToolEffect(ReviewedToolEffect{Operation: operation, ToolID: subject.ToolID, SchemaHash: subject.SchemaHash, Effect: RemoteRead}); err != nil {
		test.Fatal(err)
	}
}

type observedAuthorityKernel struct {
	KernelInterface
	recorder *recordingKernel
}

func (kernel *observedAuthorityKernel) Assert(fact string) error {
	if err := kernel.recorder.Assert(fact); err != nil {
		return err
	}
	return kernel.KernelInterface.Assert(fact)
}

func (kernel *observedAuthorityKernel) Retract(fact string) error {
	if err := kernel.recorder.Retract(fact); err != nil {
		return err
	}
	return kernel.KernelInterface.Retract(fact)
}
