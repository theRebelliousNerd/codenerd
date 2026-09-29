package init

import (
	"context"
	"strings"
	"testing"

	"codenerd/internal/tools"
)

func fakeContext7(t *testing.T, exec tools.ExecuteFunc) *tools.Registry {
	t.Helper()
	reg := tools.NewRegistry()
	if err := reg.Register(&tools.Tool{
		Name:    "context7_fetch",
		Execute: exec,
	}); err != nil {
		t.Fatalf("register context7_fetch: %v", err)
	}
	return reg
}

// TestRunShardTopicResearch_NoOuterDeadline pins that a pass over several
// shard topics adds no clock of its own. Each context7_fetch keeps
// research.context7_timeout inside the tool; this fake records the context
// the loop handed it.
func TestRunShardTopicResearch_NoOuterDeadline(t *testing.T) {
	const body = "context7 documentation body that is long enough to be stored as a knowledge atom for this shard topic."
	var calls int
	var accepted int
	reg := fakeContext7(t, func(ctx context.Context, args map[string]any) (string, error) {
		calls++
		if _, ok := ctx.Deadline(); ok {
			t.Errorf("topic %v ran under a deadline", args["topic"])
		}
		return body, nil
	})

	runShardTopicResearch(context.Background(), reg, []string{"go", "mangle", "sqlite"}, func(topic, got string) {
		accepted++
		if got != body {
			t.Errorf("topic %s body = %q", topic, got)
		}
	})
	if calls != 3 || accepted != 3 {
		t.Fatalf("calls=%d accepted=%d, want 3 and 3", calls, accepted)
	}
}

// TestRunShardTopicResearch_StopsWhenCallerCancels pins that init
// cancellation ends the topic loop between queries. A query already in
// flight keeps the tool's own request bound; the next topic does not start.
func TestRunShardTopicResearch_StopsWhenCallerCancels(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	var seen []string
	reg := fakeContext7(t, func(ctx context.Context, args map[string]any) (string, error) {
		topic, _ := args["topic"].(string)
		seen = append(seen, topic)
		if topic == "first" {
			cancel()
		}
		return strings.Repeat("d", 120), nil
	})

	var accepted int
	runShardTopicResearch(ctx, reg, []string{"first", "second"}, func(string, string) { accepted++ })
	if len(seen) != 1 || seen[0] != "first" {
		t.Fatalf("topics fetched = %v, want only first", seen)
	}
	if accepted != 1 {
		t.Fatalf("accepted %d bodies, want 1", accepted)
	}

	done, cancelDone := context.WithCancel(context.Background())
	cancelDone()
	started := 0
	runShardTopicResearch(done, reg, []string{"later"}, func(string, string) { started++ })
	if started != 0 || len(seen) != 1 {
		t.Fatalf("cancelled context fetched more topics: seen=%v accepted=%d", seen, started)
	}

	runShardTopicResearch(context.Background(), nil, []string{"x"}, func(string, string) {
		t.Fatal("nil registry called accept")
	})
	runShardTopicResearch(context.Background(), reg, []string{"x"}, nil)
}
