package core

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"

	"codenerd/internal/observation"
	"codenerd/internal/tools"
)

// subagentFixtureSeq keeps each fixture's transcript unique across runs.
//
// Handles are content-addressed, so an identical transcript yields the
// identical handle on a second pass in the same process. That would let
// `go test -count=2` pass on a handle minted by the first pass rather than by
// the code under test.
var subagentFixtureSeq atomic.Int64

// seedRetainedReturn mints a handle through the shared codec the way a live
// producer does, and returns the handle plus a line only the transcript holds.
func seedRetainedReturn(t *testing.T) (handle, buried string) {
	t.Helper()
	n := subagentFixtureSeq.Add(1)
	buried = fmt.Sprintf("I opened internal/widget/store_%d.go and walked the write path.", n)

	var sb strings.Builder
	sb.WriteString(buried + "\n")
	for i := 0; i < 30; i++ {
		fmt.Fprintf(&sb, "Considering internal/widget/file%d_%d.go: nothing to report here.\n", i, n)
	}
	fmt.Fprintf(&sb, "- [CRITICAL] internal/widget/store_%d.go:88: Flush error discarded\n", n)

	result := observation.SharedSubagents().EncodeReturn(observation.Return{
		Agent:  "reviewer",
		Task:   "review internal/widget",
		Output: sb.String(),
	}, observation.ReturnLimits{})
	if result.Handle == "" {
		t.Fatal("the codec retained nothing, so there is no handle to redeem")
	}
	return result.Handle, buried
}

func TestSubagentExpand_ShouldReturnTheRetainedTranscript(t *testing.T) {
	t.Parallel()
	handle, buried := seedRetainedReturn(t)

	out, err := executeSubagentExpand(context.Background(), map[string]any{"handle": handle})
	if err != nil {
		t.Fatalf("subagent_expand: %v", err)
	}
	if !strings.Contains(out, buried) {
		t.Errorf("expansion did not return the retained transcript:\n%s", out)
	}
}

func TestSubagentExpand_WhenMatchIsGiven_ShouldNarrowToThoseLines(t *testing.T) {
	t.Parallel()
	handle, buried := seedRetainedReturn(t)

	out, err := executeSubagentExpand(context.Background(), map[string]any{
		"handle": handle,
		"match":  "walked the write path",
	})
	if err != nil {
		t.Fatalf("subagent_expand: %v", err)
	}
	if !strings.Contains(out, buried) {
		t.Fatalf("the matching line was not returned:\n%s", out)
	}
	if strings.Contains(out, "nothing to report here") {
		t.Errorf("a narrowed expansion returned the lines it was asked to exclude:\n%s", out)
	}
}

func TestSubagentExpand_WhenHandleIsMissingOrUnknown_ShouldSayWhichFailureItIs(t *testing.T) {
	t.Parallel()

	if _, err := executeSubagentExpand(context.Background(), map[string]any{}); err == nil ||
		!strings.Contains(err.Error(), "handle is required") {
		t.Fatalf("error for a missing handle = %v, want a message about the argument; reporting 'not found' would send the caller off re-delegating over a typo", err)
	}
	_, err := executeSubagentExpand(context.Background(), map[string]any{"handle": "obs:sa:nothinghere"})
	if !errors.Is(err, observation.ErrNotFound) {
		t.Fatalf("error for an unknown handle = %v, want ErrNotFound", err)
	}
}

// TestSubagentExpand_IsDeclaredReadOnly pins the property the whole elision
// rests on. A redemption verb classified as anything but a read would be gated
// like the delegation that minted the handle, which is the arrangement this
// tool exists specifically not to have.
func TestSubagentExpand_IsDeclaredReadOnly(t *testing.T) {
	t.Parallel()

	effect, err := SubagentExpandTool().DeclaredEffect()
	if err != nil {
		t.Fatalf("declared effect: %v", err)
	}
	if effect != tools.EffectRead {
		t.Errorf("effect = %q, want %q: redeeming a handle must grant strictly less than the delegation that minted it", effect, tools.EffectRead)
	}
}

// TestRegistry_SubagentExpandIsWiredToTheCodec is the end-to-end check that the
// handle a delegation publishes is redeemable through the registry an agent
// actually sees. Registering the codec without the verb, or the verb without
// the codec, publishes a promise the model is structurally unable to keep.
func TestRegistry_SubagentExpandIsWiredToTheCodec(t *testing.T) {
	t.Parallel()
	handle, buried := seedRetainedReturn(t)

	registry := tools.NewRegistry()
	if err := RegisterAll(registry); err != nil {
		t.Fatalf("RegisterAll: %v", err)
	}

	expand := registry.Get(SubagentExpandToolName)
	if expand == nil {
		t.Fatalf("%s is not registered, so every handle a subagent return publishes is unredeemable", SubagentExpandToolName)
	}

	out, err := expand.Execute(context.Background(), map[string]any{"handle": handle})
	if err != nil {
		t.Fatalf("registered %s: %v", SubagentExpandToolName, err)
	}
	if !strings.Contains(out, buried) {
		t.Errorf("the registered verb did not return the retained transcript:\n%s", out)
	}
}
