package shards

import (
	"context"
	"testing"

	"codenerd/internal/types"
)

func newBaseAgent(perms ...types.ShardPermission) *BaseShardAgent {
	cfg := types.ShardConfig{
		Name:        "tester",
		Type:        types.ShardTypeEphemeral,
		Permissions: perms,
	}
	return NewBaseShardAgent("shard-1", cfg)
}

func TestBaseShardAgent_IdentityAndState(t *testing.T) {
	a := newBaseAgent()
	if a.GetID() != "shard-1" {
		t.Errorf("GetID=%q, want shard-1", a.GetID())
	}
	if a.GetConfig().Name != "tester" {
		t.Errorf("GetConfig().Name=%q, want tester", a.GetConfig().Name)
	}
	if a.GetState() != types.ShardStateIdle {
		t.Errorf("initial state=%q, want idle", a.GetState())
	}
	a.SetState(types.ShardStateRunning)
	if a.GetState() != types.ShardStateRunning {
		t.Errorf("state after SetState=%q, want running", a.GetState())
	}
}

func TestBaseShardAgent_SessionContext(t *testing.T) {
	a := newBaseAgent()
	if a.GetSessionContext() != nil {
		t.Error("expected nil session context before SetSessionContext")
	}
	sc := &types.SessionContext{}
	a.SetSessionContext(sc)
	if a.GetSessionContext() != sc {
		t.Error("GetSessionContext did not return the value set by SetSessionContext")
	}
}

func TestBaseShardAgent_Permissions(t *testing.T) {
	a := newBaseAgent(types.PermissionReadFile, types.PermissionResearch)
	if !a.HasPermission(types.PermissionReadFile) {
		t.Error("expected read_file permission to be granted")
	}
	if !a.HasPermission(types.PermissionResearch) {
		t.Error("expected research permission to be granted")
	}
	if a.HasPermission(types.PermissionWriteFile) {
		t.Error("write_file permission should not be granted")
	}
}

func TestBaseShardAgent_ProvidersNilWithoutClient(t *testing.T) {
	a := newBaseAgent()
	// With no LLM client wired, every capability probe must degrade to nil/false
	// rather than panic.
	if a.GetThinkingProvider() != nil {
		t.Error("GetThinkingProvider should be nil without a client")
	}
	if a.GetThoughtSignatureProvider() != nil {
		t.Error("GetThoughtSignatureProvider should be nil without a client")
	}
	if a.GetGroundingProvider() != nil {
		t.Error("GetGroundingProvider should be nil without a client")
	}
	if a.GetGroundingController() != nil {
		t.Error("GetGroundingController should be nil without a client")
	}
	if a.ShouldUsePiggybackTools() {
		t.Error("ShouldUsePiggybackTools should be false without a client")
	}
}

func TestBaseShardAgent_StopIsIdempotent(t *testing.T) {
	a := newBaseAgent()
	if err := a.Stop(); err != nil {
		t.Fatalf("Stop returned error: %v", err)
	}
	if a.GetState() != types.ShardStateCompleted {
		t.Errorf("state after Stop=%q, want completed", a.GetState())
	}
	// Second Stop must not panic on the already-closed channel.
	if err := a.Stop(); err != nil {
		t.Errorf("second Stop returned error: %v", err)
	}
}

func TestBaseShardAgent_ExecuteDefault(t *testing.T) {
	a := newBaseAgent()
	out, err := a.Execute(context.Background(), "do something")
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if out != "BaseShardAgent execution" {
		t.Errorf("Execute output=%q, want the base placeholder", out)
	}
}
