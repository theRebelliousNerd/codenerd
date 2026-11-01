//go:build integration

package e2e_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"codenerd/internal/system"
	"codenerd/internal/types"
)

func pipelineEnvelopeRequest(system, input string) bool {
	return strings.Contains(system, "## Available Tools") || strings.HasPrefix(input, "Conversation so far:\n")
}

// Only the model protocol is scripted. A result is observed only when the
// production continuation renders the actual correlated tool_result block.
func (m *pipelineModel) completePipelineEnvelope(ctx context.Context, system, input string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	plan, ok := m.planFor(input)
	if !ok {
		return "", errors.New("text fixture lost its actual turn identity")
	}
	m.mu.Lock()
	m.envelopes++
	m.mu.Unlock()
	lines := strings.Split(input, "\n")
	for index, line := range lines {
		failed := line == "tool_result id="+plan.Call.ID+" status=error:"
		if failed || line == "tool_result id="+plan.Call.ID+" status=ok:" {
			m.mu.Lock()
			m.observed[plan.Call.ID]++
			m.mu.Unlock()
			surface := "Generated tool returned: " + strings.Join(lines[index+1:], "\n")
			if failed {
				surface = "Task incomplete; generated tool failed: " + strings.Join(lines[index+1:], "\n")
			}
			data, err := json.Marshal(map[string]any{"surface_response": surface, "control_packet": map[string]any{}})
			return string(data), err
		}
	}
	if !strings.Contains(system, "**"+plan.Call.Name+"**") {
		return "", errors.New("generated tool absent from this request's text catalog")
	}
	m.mu.Lock()
	if m.issued[plan.Call.ID] != 0 {
		m.mu.Unlock()
		return "", errors.New("text fixture never received the actual issued tool result")
	}
	m.issued[plan.Call.ID]++
	m.mu.Unlock()
	if plan.Panic {
		panic("injected articulation model completion panic")
	}
	data, err := json.Marshal(map[string]any{"surface_response": "Execute the literal requested effect.",
		"control_packet": map[string]any{"tool_requests": []map[string]any{{
			"id": plan.Call.ID, "tool_name": plan.Call.Name, "tool_args": plan.Call.Input,
		}}}})
	return string(data), err
}

// These adapters establish the channel contract independently of the repaired
// constructor, so the same controls compile and run against unchanged main.
type explicitPipelineTextModel struct{ *pipelineModel }

func (*explicitPipelineTextModel) ShouldUsePiggybackTools() bool { return true }
func (m *explicitPipelineTextModel) CompleteWithSystem(ctx context.Context, system, input string) (string, error) {
	if pipelineEnvelopeRequest(system, input) {
		return m.completePipelineEnvelope(ctx, system, input)
	}
	return m.pipelineModel.CompleteWithSystem(ctx, system, input)
}

type explicitPipelineNativeModel struct{ *pipelineModel }

func (*explicitPipelineNativeModel) ShouldUsePiggybackTools() bool { return false }

func rebootPipelineChannelFixture(t *testing.T, f *pipelineFixture, client types.LLMClient) {
	t.Helper()
	if err := f.cortex.Close(); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	cortex, err := system.BootCortexWithConfig(ctx, system.BootConfig{Workspace: f.root,
		SessionID: "generated-channel-control", UserConfigOverride: newPipelineUserConfig(t), LLMClientOverride: client})
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := cortex.Close(); err != nil {
			t.Errorf("close channel-control boot: %v", err)
		}
		cancel()
	})
	f.cortex = cortex
	f.configure(cortex.SessionExecutor)
}

// The unchanged witness checks real effects and durable learning, then still
// refuses acceptance for this unclassified write. Require that sole remaining
// error; never erase it or present this as a verified task/pipeline success.
func requirePipelineTransportWithWriteRefusal(t *testing.T, f *pipelineFixture, plan pipelinePlan) {
	t.Helper()
	requirePipelineTransportOutcomeWithWriteRefusal(t, f, plan, f.run(plan))
}

func requirePipelineTransportOutcomeWithWriteRefusal(t *testing.T, f *pipelineFixture, plan pipelinePlan, outcome pipelineOutcome) {
	t.Helper()
	want := "hollow success blocked: write-oriented intent /create completed without a recognized write-mutation tool (tool_calls=1)"
	if outcome.err == nil || outcome.result == nil || outcome.result.Error == nil || outcome.err.Error() != want || outcome.result.Error.Error() != want {
		t.Fatalf("both result and returned error must retain write-acceptance refusal: returned=%v result=%+v", outcome.err, outcome.result)
	}
	err := f.positiveWitness(plan, outcome)
	if err == nil || err.Error() != want+"\n"+want {
		t.Fatalf("expected actual execution/learning witnesses and the retained write-acceptance refusal, got: %v", err)
	}
}

func TestGeneratedPipelineFixtureChannelContract(t *testing.T) {
	t.Run("default_text_executes_but_keeps_write_acceptance_refusal", func(t *testing.T) {
		f := newPipelineFixture(t, false)
		plan := f.plan("channel-default", "default text-channel effect\n", "", "")
		f.approve(plan)
		requirePipelineTransportWithWriteRefusal(t, f, plan)
		f.model.mu.Lock()
		envelopes, native := f.model.envelopes, f.model.native
		f.model.mu.Unlock()
		if envelopes < 2 || native != 0 {
			t.Fatalf("expected real text request/result continuation, envelopes=%d native=%d", envelopes, native)
		}
		f.assertTotals(1, 1, 1)
		f.assertBaseline()
	})

	t.Run("explicit_text_executes_on_unchanged_production", func(t *testing.T) {
		f := newPipelineFixture(t, false)
		rebootPipelineChannelFixture(t, f, &explicitPipelineTextModel{f.model})
		plan := f.plan("channel-explicit", "explicit text-channel effect\n", "", "")
		f.approve(plan)
		requirePipelineTransportWithWriteRefusal(t, f, plan)
		f.assertTotals(1, 1, 1)
	})

	t.Run("native_channel_withholds_untyped_generated_tool", func(t *testing.T) {
		f := newPipelineFixture(t, false)
		rebootPipelineChannelFixture(t, f, &explicitPipelineNativeModel{f.model})
		plan := f.plan("channel-native", "must not execute on unsupported native channel\n", "", "")
		f.approve(plan)
		outcome := f.run(plan)
		assertPipelineFailed(t, outcome)
		if !strings.Contains(pipelineFailureText(outcome), "absent from compiled turn catalog") {
			t.Fatalf("native control did not hit its channel boundary: %s", pipelineFailureText(outcome))
		}
		f.assertNoEffect(plan)
		f.assertTotals(0, 0, 0)
	})

	t.Run("missing_registry_is_not_an_offer", func(t *testing.T) {
		f := newPipelineFixture(t, false)
		rebootPipelineChannelFixture(t, f, &explicitPipelineTextModel{f.model})
		f.cortex.SessionExecutor.SetOuroborosRegistry(nil)
		plan := f.plan("channel-unbound", "must not execute without a bound registry\n", "", "")
		f.approve(plan)
		assertPipelineFailed(t, f.run(plan))
		f.assertNoEffect(plan)
		f.assertTotals(0, 0, 0)
	})

	t.Run("text_catalog_does_not_grant_action_permission", func(t *testing.T) {
		f := newPipelineFixture(t, false)
		rebootPipelineChannelFixture(t, f, &explicitPipelineTextModel{f.model})
		plan := f.plan("channel-unapproved", "must not execute without exact approval\n", "", "")
		assertPipelineFailed(t, f.run(plan))
		f.model.mu.Lock()
		issued := f.model.issued[plan.Call.ID]
		f.model.mu.Unlock()
		if issued != 1 {
			t.Fatalf("permission control did not reach an offered tool call: issued=%d", issued)
		}
		f.assertNoEffect(plan)
		f.assertTotals(0, 0, 0)
	})
}

func TestGeneratedPipelineFixtureConcurrentTransport(t *testing.T) {
	f := newPipelineFixture(t, false)
	gate := newPipelineGate(t)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	plans := []pipelinePlan{f.plan("channel-a", "distinct channel A\n", "", gate.server.URL), f.plan("channel-b", "distinct channel B\n", "", gate.server.URL)}
	var completions []<-chan pipelineOutcome
	for _, plan := range plans {
		f.approve(plan)
		executor := f.cortex.SessionExecutor.CloneForTask()
		executor.SetSessionID("channel-isolation-" + plan.Call.ID)
		f.configure(executor)
		completions = append(completions, f.start(ctx, executor, plan))
	}
	// The production per-tool operation lease serializes the same generated
	// binary. Release the first admitted process before awaiting the second;
	// caller concurrency must preserve results, not bypass that lease.
	admitted := map[string]bool{awaitPipeline(t, gate.admitted, "first channel process admission"): true}
	gate.open()
	admitted[awaitPipeline(t, gate.admitted, "second channel process admission")] = true
	if len(admitted) != 2 || !admitted[plans[0].Call.ID] || !admitted[plans[1].Call.ID] {
		t.Fatalf("distinct concurrent process identities lost: %v", admitted)
	}
	for index, plan := range plans {
		requirePipelineTransportOutcomeWithWriteRefusal(t, f, plan, awaitPipeline(t, completions[index], "concurrent channel result"))
		f.assertReleased(plan)
	}
	for range plans {
		awaitPipeline(t, gate.drained, "concurrent child drain")
	}
	f.assertTotals(2, 2, 2)
	f.assertBaseline()
}
