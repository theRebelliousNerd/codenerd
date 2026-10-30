package chat

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/config"
	"codenerd/internal/core"
	"codenerd/internal/learnjudge"
	"codenerd/internal/observation"
	"codenerd/internal/perception"
	"codenerd/internal/types"
	"codenerd/internal/typesafe"
)

// request_delivery_test.go: a delegated or multi-step request goes on with
// another step while a part of it is missing, and stops when nothing is
// missing, a blocker is named, or a step resolves nothing
// (request_delivery.go, policy/request_delivery.mg).

const jsonFlagRequest = "add a --json flag to nerd status and document it in the README"

// deliveryProbs answers the delivery questions: every fragment asked, each
// fragment's delivered probability from delivered, and the blocker question.
func deliveryProbs(blocked float64, delivered ...float64) map[string]float64 {
	p := map[string]float64{
		// The turn judgments a delegated fix reads: it changes files, names
		// its target, and is no what-if.
		"changes_files": 0.9, "names_target": 0.9, "hypothetical": 0.04, "rules_out_changes": 0.04,
		learnjudge.DeliveryNamesBlocker: blocked,
	}
	for i, d := range delivered {
		p[fmt.Sprintf("asked_%d", i)] = 0.97
		p[fmt.Sprintf("delivered_%d", i)] = d
	}
	return p
}

// flagTurn is the delegated turn: an instruction to the coder, which runs in
// the foreground without the verification loop a /mutation takes.
func flagTurn() perception.Intent {
	return perception.Intent{Category: "/instruction", Verb: "/fix", Target: "cmd/nerd/cmd_status.go", Confidence: 0.95, Response: "Adding the flag."}
}

// The kernel's truth table, over one real kernel: each case is its own
// request, and only the active request can raise a step.
func TestDeliveryPolicy_TruthTable(t *testing.T) {
	t.Parallel()
	k, err := core.NewRealKernel()
	if err != nil {
		t.Fatal(err)
	}
	if err := config.EnsureParams(k, (&config.TypeSafeConfig{}).Params()); err != nil {
		t.Fatal(err)
	}
	type round struct {
		asked, delivered []int64
		blocked          int64
		cut              bool
	}
	frags := []string{"add the flag", "document it"}
	for i, tc := range []struct {
		name     string
		rounds   []round
		inactive bool
		ran      bool // the offered step already produced a shard result
		outcome  string
		pending  bool
	}{
		{name: "everything delivered", rounds: []round{{asked: []int64{97, 97}, delivered: []int64{96, 91}, blocked: 2}}, outcome: "/delivered"},
		{name: "a missing part goes back", rounds: []round{{asked: []int64{97, 97}, delivered: []int64{96, 4}, blocked: 2}}, outcome: "/pushback", pending: true},
		{name: "a named blocker stops it", rounds: []round{{asked: []int64{97, 97}, delivered: []int64{96, 21}, blocked: 98}}, outcome: "/blocked"},
		{name: "an unsettled blocker still pushes", rounds: []round{{asked: []int64{97, 97}, delivered: []int64{96, 4}, blocked: 50}}, outcome: "/pushback", pending: true},
		{name: "an unsettled part is reported, not pushed", rounds: []round{{asked: []int64{97, 97}, delivered: []int64{96, 52}, blocked: 2}}, outcome: "/unsettled"},
		{name: "a cut result reads no part missing", rounds: []round{{asked: []int64{97, 97}, delivered: []int64{96, 4}, blocked: 2, cut: true}}, outcome: "/unsettled"},
		{name: "a part nobody asked for is ignored", rounds: []round{{asked: []int64{97, 9}, delivered: []int64{96, 3}, blocked: 2}}, outcome: "/delivered"},
		{name: "a round that resolves a part goes on", rounds: []round{
			{asked: []int64{97, 97}, delivered: []int64{5, 4}, blocked: 2},
			{asked: []int64{97, 97}, delivered: []int64{95, 4}, blocked: 2},
		}, outcome: "/pushback", pending: true},
		{name: "a round that resolves nothing stops", rounds: []round{
			{asked: []int64{97, 97}, delivered: []int64{96, 4}, blocked: 2},
			{asked: []int64{97, 97}, delivered: []int64{96, 6}, blocked: 2},
		}, outcome: "/no_progress"},
		{name: "an inactive request raises no step", rounds: []round{{asked: []int64{97, 97}, delivered: []int64{96, 4}, blocked: 2}}, inactive: true, outcome: "/pushback"},
		{name: "a step that ran is not raised again", rounds: []round{{asked: []int64{97, 97}, delivered: []int64{96, 4}, blocked: 2}}, ran: true, outcome: "/pushback"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := types.MangleAtom(fmt.Sprintf("/delivery_case_%d", i))
			desc := fmt.Sprintf("deliver case %d", i)
			_ = k.Retract("delivery_active")
			if !tc.inactive {
				mustAssert(t, k, types.Fact{Predicate: "delivery_active", Args: []any{req}})
			}
			last := len(tc.rounds)
			for r, rd := range tc.rounds {
				for f := range frags {
					mustAssert(t, k, types.Fact{Predicate: learnjudge.DeliveryReadingPredicate, Args: []any{req, int64(r + 1), frags[f], rd.asked[f], rd.delivered[f]}})
				}
				mustAssert(t, k, types.Fact{Predicate: learnjudge.DeliveryBlockerPredicate, Args: []any{req, int64(r + 1), rd.blocked}})
				if rd.cut {
					mustAssert(t, k, types.Fact{Predicate: learnjudge.DeliveryCutPredicate, Args: []any{req, int64(r + 1)}})
				}
			}
			mustAssert(t, k, types.Fact{Predicate: "delivery_step_offer", Args: []any{req, int64(last), fmt.Sprintf("task_%d", i), desc, types.MangleAtom("/coder")}})
			if tc.ran {
				mustAssert(t, k, types.Fact{Predicate: "shard_result", Args: []any{"t", "/unverified", types.MangleAtom("/coder"), desc, "did it"}})
			}
			var outcomes []string
			rows, _ := k.Query("delivery_outcome")
			for _, row := range rows {
				if types.ExtractString(row.Args[0]) == string(req) && roundArg(row.Args[1]) == last {
					outcomes = append(outcomes, types.ExtractString(row.Args[2]))
				}
			}
			if len(outcomes) != 1 || outcomes[0] != tc.outcome {
				t.Fatalf("delivery_outcome = %v, want exactly %s", outcomes, tc.outcome)
			}
			pending := false
			rows, _ = k.Query("has_pending_subtask")
			for _, row := range rows {
				if row.Args[1] == desc {
					pending = true
				}
			}
			if pending != tc.pending {
				t.Fatalf("has_pending_subtask for the offer = %v, want %v", pending, tc.pending)
			}
		})
	}
}

func mustAssert(t *testing.T, k types.Kernel, f types.Fact) {
	t.Helper()
	if err := k.Assert(f); err != nil {
		t.Fatalf("assert %v: %v", f, err)
	}
}

// A delegated turn whose shard did half the request goes on with a step that
// names the missing half; when that step delivers it, the continuation ends
// and its summary says so. Before the delivery rounds the turn ended at the
// first reply.
func TestDeliveryJourney_AMissingPartIsHandedBackUntilDelivered(t *testing.T) {
	t.Parallel()
	m, exec := journeyModel(t, config.WorkModeYolo, flagTurn())
	exec.response = "Done. Added a --json flag to the status command; it prints the status as JSON."
	jev := &fakeJevServer{probs: deliveryProbs(0.02, 0.96, 0.04)}
	m = withJev(t, m, jev)

	m, msg, _ := say(t, m, jsonFlagRequest)
	init, ok := msg.(continuationInitMsg)
	if !ok {
		t.Fatalf("the half-delivered request ended with %T, want another step", msg)
	}
	if !strings.Contains(init.next.description, "- document it in the README") || init.next.shardType != "coder" {
		t.Fatalf("the step = %q to %s, want the missing part to the coder", init.next.description, init.next.shardType)
	}
	if m.delivery == nil {
		t.Fatal("the continuation does not carry the request it delivers")
	}

	// The step documents it.
	exec.response = "Documented --json under Commands in README.md."
	jev.set(deliveryProbs(0.02, 0.97, 0.95))
	done, ok := m.executeSubtask(init.next.subtaskID, init.next.description, init.next.shardType)().(continuationDoneMsg)
	if !ok {
		t.Fatal("a delivered request asked for yet another step")
	}
	if !strings.Contains(done.summary, "Jev read every part of the request as delivered.") {
		t.Fatalf("summary = %q", done.summary)
	}
	next, _ := m.Update(done)
	if next.(Model).delivery != nil {
		t.Fatal("the delivery chain outlived its continuation")
	}
	if rows, _ := m.kernel.Query("delivery_active"); len(rows) != 0 {
		t.Fatalf("delivery_active = %v after the continuation ended", rows)
	}
}

// A step that delivers none of what it was handed is not handed it again.
func TestDeliveryJourney_AStepThatResolvesNothingStops(t *testing.T) {
	t.Parallel()
	m, exec := journeyModel(t, config.WorkModeYolo, flagTurn())
	exec.response = "Done. Added a --json flag to the status command."
	m = withJev(t, m, &fakeJevServer{probs: deliveryProbs(0.02, 0.96, 0.04)})

	m, msg, _ := say(t, m, jsonFlagRequest)
	init, ok := msg.(continuationInitMsg)
	if !ok {
		t.Fatalf("got %T, want another step", msg)
	}
	exec.response = "The flag is in place."
	done, ok := m.executeSubtask(init.next.subtaskID, init.next.description, init.next.shardType)().(continuationDoneMsg)
	if !ok {
		t.Fatal("a step that resolved nothing was handed the same parts again")
	}
	if !strings.Contains(done.summary, "Not delivered: document it in the README. The last step delivered none of them") {
		t.Fatalf("summary = %q", done.summary)
	}
	if exec.callCount() != 2 {
		t.Fatalf("%d shard runs, want the turn and one delivery step", exec.callCount())
	}
}

// A blocker the step names ends the request with the reply, and the reply
// says what was not delivered and why.
func TestDeliveryJourney_ANamedBlockerEndsTheRequest(t *testing.T) {
	t.Parallel()
	m, exec := journeyModel(t, config.WorkModeYolo, flagTurn())
	exec.response = "Added the flag. Writing README.md was denied by the permission gate; approve it and I will document the flag."
	m = withJev(t, m, &fakeJevServer{probs: deliveryProbs(0.97, 0.96, 0.05)})

	_, msg, _ := say(t, m, jsonFlagRequest)
	reply, ok := msg.(assistantMsg)
	if !ok {
		t.Fatalf("a blocked request went on as %T", msg)
	}
	if !strings.Contains(reply.Surface, "Not delivered: document it in the README. The steps name an obstacle codeNERD cannot clear by itself.") {
		t.Fatalf("the reply does not say what was not delivered:\n%s", reply.Surface)
	}
	if exec.callCount() != 1 {
		t.Fatalf("%d shard runs, want one", exec.callCount())
	}
}

// Jev failing judges nothing: the reply says delivery was not judged, and no
// step is invented.
func TestDeliveryJourney_JevFailingHandsNothingBack(t *testing.T) {
	t.Parallel()
	m, exec := journeyModel(t, config.WorkModeYolo, flagTurn())
	exec.response = "Done. Added a --json flag."
	jev := &fakeJevServer{probs: deliveryProbs(0.02, 0.96, 0.04)}
	m = withJev(t, m, jev)
	// The turn judgments answer; the delivery call (asked after the step) fails.
	exec.check = func() error { jev.mu.Lock(); jev.fail = true; jev.mu.Unlock(); return nil }

	_, msg, _ := say(t, m, jsonFlagRequest)
	reply, ok := msg.(assistantMsg)
	if !ok {
		t.Fatalf("got %T, want the reply", msg)
	}
	if !strings.Contains(reply.Surface, "was not judged (Jev:") {
		t.Fatalf("the reply does not say delivery went unjudged:\n%s", reply.Surface)
	}
}

// With Jev off there is no chain at all.
func TestDelivery_OffWithoutJev(t *testing.T) {
	t.Parallel()
	m, _ := journeyModel(t, config.WorkModeYolo)
	if dc := m.startDelivery(jsonFlagRequest, "coder"); dc != nil {
		t.Fatal("a delivery chain opened with no Jev client")
	}
	m = withJev(t, m, &fakeJevServer{})
	off := false
	m.Config.TypeSafe.InstructionCoverage = &off
	if dc := m.startDelivery(jsonFlagRequest, "coder"); dc != nil {
		t.Fatal("a delivery chain opened with instruction_coverage off")
	}
}

func TestGapShard_PrefersTheLastWritingStep(t *testing.T) {
	m, _ := journeyModel(t, config.WorkModeYolo)
	for _, tc := range []struct {
		ran  []string
		want string
	}{
		{[]string{"coder", "reviewer"}, "coder"},
		{[]string{"researcher", "reviewer"}, "reviewer"},
		{[]string{"tester", "reviewer"}, "tester"},
		{nil, ""},
	} {
		if got := m.gapShard(tc.ran); got != tc.want {
			t.Errorf("gapShard(%v) = %q, want %q", tc.ran, got, tc.want)
		}
	}
}

// liveJev is the configured Jev client, or a skip.
func liveJev(t *testing.T) (*typesafe.Client, *config.TypeSafeConfig) {
	t.Helper()
	cfg, err := config.LoadUserConfig(filepath.Join("..", "..", "..", ".nerd", "config.json"))
	if err != nil || cfg == nil || cfg.TypeSafe == nil {
		t.Skipf("no typesafe section in .nerd/config.json: %v", err)
	}
	c, err := typesafe.FromConfig(cfg.TypeSafe)
	if err != nil || c == nil {
		t.Skipf("typesafe section does not resolve: %v", err)
	}
	return c, cfg.TypeSafe
}

// The rounds over the real Jev: the shard's "done" after only the flag reads
// as the README part missing, and the step that documents it ends the
// request delivered. Without Jev configured this skips.
func TestDeliveryLive_ShardSaysDoneButMissedAPart(t *testing.T) {
	client, sec := liveJev(t)
	k, err := core.NewRealKernel()
	if err != nil {
		t.Fatal(err)
	}
	m := Model{kernel: k, jev: client, Config: &config.UserConfig{TypeSafe: sec}}
	dc := m.startDelivery(jsonFlagRequest, "coder")
	if dc == nil {
		t.Fatal("no delivery chain with Jev configured")
	}
	ctx := context.Background()
	dc.record(observation.Return{Output: "Done. Added a --json flag to the status command in cmd/nerd/cmd_status.go; it prints the status as JSON. Build and tests pass.", Changed: []string{"cmd/nerd/cmd_status.go"}})
	next := m.nextStep(ctx, dc)
	if next == nil {
		t.Fatalf("Jev read the half-done request as finished: %s", dc.summaryNote())
	}
	if !strings.Contains(next.description, "README") {
		t.Fatalf("the step does not name the README part: %q", next.description)
	}
	m.injectShardResultFacts(next.shardType, next.description, observation.Return{}, nil)
	dc.record(observation.Return{Output: "Documented the new --json flag of nerd status in README.md under Commands, with an example of its output.", Changed: []string{"README.md"}})
	if again := m.nextStep(ctx, dc); again != nil {
		t.Fatalf("a delivered request asked for another step: %q (%s)", again.description, dc.summaryNote())
	}
	if !strings.Contains(dc.summaryNote(), "every part of the request as delivered") {
		t.Fatalf("note = %q", dc.summaryNote())
	}
}

// The multi-step lane: a plan whose steps left part of the request undone
// goes on with a step for it, to the plan's writing shard.
func TestDeliveryMultiStep_APlanThatMissedAPartGoesOn(t *testing.T) {
	t.Parallel()
	m, exec := journeyModel(t, config.WorkModeYolo)
	exec.response = "Done. Added a --json flag to the status command."
	m = withJev(t, m, &fakeJevServer{probs: deliveryProbs(0.02, 0.96, 0.04)})
	steps := []TaskStep{
		{Verb: "/review", Target: "cmd/nerd/cmd_status.go", ShardType: "reviewer", Task: "review the status command"},
		{Verb: "/fix", Target: "cmd/nerd/cmd_status.go", ShardType: "coder", Task: "add a --json flag to nerd status"},
	}
	msg := m.executeMultiStepTask(m.shutdownCtx, perception.Intent{Response: "Two steps."}, jsonFlagRequest, steps)()
	init, ok := msg.(continuationInitMsg)
	if !ok {
		t.Fatalf("the plan that missed a part ended with %T", msg)
	}
	if init.next.shardType != "coder" || !strings.Contains(init.next.description, "document it in the README") {
		t.Fatalf("the step = %q to %s", init.next.description, init.next.shardType)
	}
	if !strings.Contains(init.completedSurface, "Multi-Step Task Execution") {
		t.Fatalf("the plan's own report was dropped:\n%s", init.completedSurface)
	}
}
