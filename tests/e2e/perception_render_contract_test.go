//go:build integration

package e2e_test

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
	"unicode/utf8"

	appconfig "codenerd/internal/config"
	"codenerd/internal/perception"
	"codenerd/internal/types"
)

const renderedPerceptionReply = `{"understanding":{"primary_intent":"explain","semantic_type":"mechanism","action_type":"explain","domain":"general","scope":{"level":"file","target":"auth.go"},"user_constraints":[],"implicit_assumptions":[],"confidence":0.9,"signals":{"is_question":true,"is_hypothetical":false,"is_multi_step":false,"is_negated":false,"requires_confirmation":false,"urgency":"normal"},"suggested_approach":{"mode":"normal","primary_shard":"reviewer","tools_needed":[],"context_needed":[]}},"surface_response":"I will explain."}`

func TestPerceptionRenderedCurrentInput(t *testing.T) {
	const head = "HEAD_KEEP_"
	const tail = "_TAIL_KEEP"
	const middle = "OMITTED_MIDDLE_SENTINEL"
	inputs := map[string]string{
		"short":       head + "short" + tail,
		"exact_limit": head + strings.Repeat("x", 50000-len(head)-len(tail)) + tail,
		"ascii_cut":   head + strings.Repeat("x", 50000) + middle + strings.Repeat("z", 50000) + tail,
		"unicode_cut": head + strings.Repeat("界🦊", 15000) + middle + strings.Repeat("界🦊", 15000) + tail,
	}
	for _, mode := range []string{"short", "exact_limit", "ascii_cut", "unicode_cut"} {
		t.Run(mode, func(t *testing.T) {
			input := inputs[mode]
			model := newPCEMockClient(renderedPerceptionReply)
			tr := newPerceptionTransducer(t, model)
			intent, err := tr.ParseIntentWithContext(context.Background(), input, nil)
			if err != nil || model.callCount() != 1 || intent.Verb != "/explain" {
				t.Fatalf("classification did not use its one scripted request: intent=%+v err=%v calls=%d", intent, err, model.callCount())
			}
			_, body, ok := strings.Cut(model.getRecordedUser(0), "## Current Request\n\n")
			if !ok || !utf8.ValidString(body) || !strings.HasPrefix(body, head) || !strings.HasSuffix(body, tail) {
				t.Fatalf("request lost its framing or valid head/tail: mode=%s bytes=%d", mode, len(body))
			}
			cut := mode == "ascii_cut" || mode == "unicode_cut"
			if types.IsClamped(body) != cut {
				t.Fatalf("wrong actual cut notice: mode=%s clamped=%v", mode, types.IsClamped(body))
			}
			if !cut {
				if body != input {
					t.Fatal("an intact current input was changed")
				}
				return
			}
			if len(body) > 50200 || len(body) >= len(input) || strings.Contains(body, middle) || !strings.Contains(body, "chars from user input") {
				t.Fatalf("cut was unbounded, silently head-only, or not described: mode=%s input=%d body=%d", mode, len(input), len(body))
			}
			if body != types.ClampText(input, 50000, "user input") {
				t.Fatal("provider request differs from canonical head/tail clamping")
			}
		})
	}
}

func TestPerceptionRenderedHistoryPolicy(t *testing.T) {
	original := appconfig.ResolvedClassificationHistory()
	for _, mode := range []string{"default", "lowered", "disabled", "byte_budget", "oversized_newest", "summary_only"} {
		t.Run(mode, func(t *testing.T) {
			prior := appconfig.ResolvedClassificationHistory()
			policy := appconfig.DefaultClassificationConfig().Resolve()
			var history []perception.ConversationTurn
			history = append(history, perception.ConversationTurn{Role: "assistant", ThoughtSummary: "SUMMARY_ONLY_MUST_NOT_CLASSIFY"})
			var contents []string
			for i := 0; i < 7; i++ {
				content := fmt.Sprintf("VISIBLE_%02d_HEAD", i) + strings.Repeat("x", 60) + fmt.Sprintf("VISIBLE_%02d_TAIL", i)
				contents = append(contents, content)
				history = append(history, perception.ConversationTurn{Role: "assistant", Content: content, ThoughtSummary: fmt.Sprintf("PROVIDER_SUMMARY_%02d_MUST_NOT_CLASSIFY", i)})
			}
			start := 7 - policy.TurnWindow
			switch mode {
			case "lowered":
				policy.TurnWindow = 2
				start = 5
			case "disabled":
				policy.TurnWindow = 0
				start = 7
			case "byte_budget":
				policy.CharBudget = 2*len(contents[0]) + 1
				start = 5
			case "oversized_newest":
				policy.CharBudget = 200
				contents[6] = "NEWEST_HEAD" + strings.Repeat("Q", 10000) + "NEWEST_TAIL"
				history[7].Content = contents[6]
				start = 6
			case "summary_only":
				history = history[:1]
				start = 7
			}
			appconfig.SetClassificationHistory(policy)
			t.Cleanup(func() { appconfig.SetClassificationHistory(prior) })
			before := append([]perception.ConversationTurn(nil), history...)
			model := newPCEMockClient(renderedPerceptionReply)
			tr := newPerceptionTransducer(t, model)
			_, err := tr.ParseIntentWithContext(context.Background(), "CURRENT_REQUEST_KEEP", history)
			if err != nil || model.callCount() != 1 {
				t.Fatalf("classification failed: err=%v calls=%d", err, model.callCount())
			}
			request := model.getRecordedUser(0)
			if !strings.Contains(request, "CURRENT_REQUEST_KEEP") || strings.Contains(request, "MUST_NOT_CLASSIFY") {
				t.Fatal("current input missing or provider summary disclosed")
			}
			if !reflect.DeepEqual(history, before) {
				t.Fatal("classification mutated caller-owned visible content or provider summary")
			}
			for i, content := range contents {
				want := i >= start
				if strings.Contains(request, content) != want {
					t.Fatalf("whole-turn policy mismatch: mode=%s turn=%d present=%v want=%v", mode, i, strings.Contains(request, content), want)
				}
			}
			withHistory := mode != "disabled" && mode != "summary_only"
			if strings.Contains(request, "## Recent Conversation") != withHistory || types.IsClamped(request) != withHistory {
				t.Fatalf("history section or eviction notice mismatch: mode=%s", mode)
			}
			if mode == "oversized_newest" && (!strings.Contains(request, "included whole") || !strings.Contains(request, "classification.history_char_budget 200")) {
				t.Fatal("oversized-newest exception was silent or sliced")
			}
		})
	}
	if appconfig.ResolvedClassificationHistory() != original {
		t.Fatal("a request-boundary control leaked global classification policy")
	}
}

func TestPerceptionRenderedParallelOwners(t *testing.T) {
	const owners = 8
	models := make([]*pceMockLLMClient, owners)
	transducers := make([]perception.Transducer, owners)
	for i := 0; i < owners; i++ {
		models[i] = newPCEMockClient(renderedPerceptionReply)
		transducers[i] = newPerceptionTransducer(t, models[i])
	}
	var wg sync.WaitGroup
	failures := make(chan error, owners)
	for i := 0; i < owners; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			input := fmt.Sprintf("OWNER_%02d_CURRENT", i)
			content := fmt.Sprintf("OWNER_%02d_VISIBLE_HISTORY", i)
			_, err := transducers[i].ParseIntentWithContext(context.Background(), input, []perception.ConversationTurn{{Role: "assistant", Content: content, ThoughtSummary: "PARALLEL_PROVIDER_SUMMARY_MUST_NOT_CLASSIFY"}})
			request := models[i].getRecordedUser(0)
			if err != nil || models[i].callCount() != 1 || !strings.Contains(request, input) || !strings.Contains(request, content) || strings.Contains(request, "MUST_NOT_CLASSIFY") {
				failures <- fmt.Errorf("owner %d lost its request/history or disclosed a provider summary: %v", i, err)
				return
			}
			for j := 0; j < owners; j++ {
				if j != i && strings.Contains(request, fmt.Sprintf("OWNER_%02d_", j)) {
					failures <- fmt.Errorf("owner %d received owner %d context", i, j)
					return
				}
			}
		}(i)
	}
	wg.Wait()
	close(failures)
	for err := range failures {
		t.Error(err)
	}
}
