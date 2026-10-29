package mcp

import "testing"

// Configured reviews are the production source of host reviews: a manager
// built from config installs them when its fact emitter is set, and a new
// emitter epoch rebuilds them from config rather than losing them.
func TestConfiguredReviewsInstallWithTheEmitter(t *testing.T) {
	review := ReviewedToolEffect{ToolID: "srv/write_note", SchemaHash: "abc123", Effect: RemoteWrite, TargetArgument: "path"}
	bad := ReviewedToolEffect{ToolID: "srv/bogus", SchemaHash: "", Effect: RemoteWrite}
	manager := NewMCPClientManager(nil, nil, map[string]MCPServerConfig{
		"srv": {ID: "srv", Enabled: true, Reviews: []ReviewedToolEffect{review, bad}},
	})
	if _, ok := manager.ReviewFor("srv/write_note"); ok {
		t.Fatal("a review was installed before any emitter")
	}
	manager.SetFactEmitter(NewFactEmitter(nil))
	got, ok := manager.ReviewFor("srv/write_note")
	if !ok || got.Operation != RemoteTool || got.SchemaHash != "abc123" || got.Effect != RemoteWrite {
		t.Fatalf("ReviewFor = %+v, %v; want the configured review, normalized", got, ok)
	}
	if _, ok := manager.ReviewFor("srv/bogus"); ok {
		t.Fatal("a review with no schema hash was installed")
	}
	manager.SetFactEmitter(NewFactEmitter(nil))
	if _, ok := manager.ReviewFor("srv/write_note"); !ok {
		t.Fatal("a new emitter epoch dropped the configured review")
	}
}
