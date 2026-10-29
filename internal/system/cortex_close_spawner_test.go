package system

import (
	"context"
	"errors"
	"testing"

	"codenerd/internal/session"
)

// Closing the Cortex shuts the delegated-task spawner: admission closes before
// the kernel and stores its workers hold are torn down.
func TestCortexClose_ShutsTheSessionSpawner(t *testing.T) {
	spawner := session.NewSpawner(nil, nil, nil, nil, nil, nil, session.DefaultSpawnerConfig())
	c := &Cortex{SessionSpawner: spawner}
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := spawner.Spawn(context.Background(), session.SpawnRequest{Name: "late", Task: "x"}); !errors.Is(err, session.ErrSpawnerClosed) {
		t.Fatalf("Spawn after Cortex.Close = %v; want ErrSpawnerClosed", err)
	}
}
