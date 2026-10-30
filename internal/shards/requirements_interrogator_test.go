package shards

import (
	"codenerd/internal/types"
	"context"
	"testing"
)

func TestRequirementsInterrogatorIsAReadOnlyChallenger(t *testing.T) {
	shard := NewRequirementsInterrogatorShard()
	if shard.GetID() != "requirements_interrogator" {
		t.Fatal(shard.GetID())
	}
	for _, permission := range shard.GetConfig().Permissions {
		if permission == types.PermissionAskUser || permission == types.PermissionWriteFile || permission == types.PermissionExecCmd {
			t.Fatalf("challenger must not receive %s", permission)
		}
	}
	if _, err := shard.Execute(context.Background(), "{}"); err == nil {
		t.Fatal("unwired challenger fabricated questions")
	}
}
