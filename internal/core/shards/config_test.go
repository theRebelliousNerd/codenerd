package shards

import (
	"testing"
	"time"

	"codenerd/internal/types"
)

func TestDefaultGeneralistConfig(t *testing.T) {
	cfg := DefaultGeneralistConfig("coder")
	if cfg.Name != "coder" {
		t.Fatalf("expected name coder, got %q", cfg.Name)
	}
	if cfg.Type != types.ShardTypeEphemeral {
		t.Fatalf("expected ephemeral type, got %q", cfg.Type)
	}
	if cfg.Timeout != 15*time.Minute {
		t.Fatalf("unexpected timeout: %v", cfg.Timeout)
	}
	if cfg.Model.Capability != types.CapabilityBalanced {
		t.Fatalf("unexpected model capability: %q", cfg.Model.Capability)
	}
	if !containsPermission(cfg.Permissions, types.PermissionReadFile) ||
		!containsPermission(cfg.Permissions, types.PermissionWriteFile) ||
		!containsPermission(cfg.Permissions, types.PermissionNetwork) {
		t.Fatalf("missing expected permissions")
	}
}

func TestDefaultSpecialistConfig(t *testing.T) {
	cfg := DefaultSpecialistConfig("researcher", "path/to/db")
	if cfg.Type != types.ShardTypePersistent {
		t.Fatalf("expected persistent type, got %q", cfg.Type)
	}
	if cfg.BaseType != "researcher" {
		t.Fatalf("expected base type researcher, got %q", cfg.BaseType)
	}
	if cfg.KnowledgePath != "path/to/db" {
		t.Fatalf("unexpected knowledge path: %q", cfg.KnowledgePath)
	}
	if cfg.Timeout != 30*time.Minute {
		t.Fatalf("unexpected timeout: %v", cfg.Timeout)
	}
	if cfg.Model.Capability != types.CapabilityHighReasoning {
		t.Fatalf("unexpected model capability: %q", cfg.Model.Capability)
	}
	if !containsPermission(cfg.Permissions, types.PermissionBrowser) ||
		!containsPermission(cfg.Permissions, types.PermissionResearch) {
		t.Fatalf("missing expected specialist permissions")
	}
}

func TestDefaultSystemConfig(t *testing.T) {
	cfg := DefaultSystemConfig("executive_policy")
	if cfg.Type != types.ShardTypeSystem {
		t.Fatalf("expected system type, got %q", cfg.Type)
	}
	if cfg.Timeout != 24*time.Hour {
		t.Fatalf("unexpected timeout: %v", cfg.Timeout)
	}
	if !containsPermission(cfg.Permissions, types.PermissionExecCmd) {
		t.Fatalf("expected exec permission for system shard")
	}
}

func containsPermission(perms []types.ShardPermission, p types.ShardPermission) bool {
	for _, perm := range perms {
		if perm == p {
			return true
		}
	}
	return false
}
