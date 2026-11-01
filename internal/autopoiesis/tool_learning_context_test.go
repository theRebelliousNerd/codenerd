package autopoiesis

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

const learningContextCode = "package tools\nfunc SnapshotTool() {}"

func TestToolGeneratorLearningContextReplacements(t *testing.T) {
	var observed string
	generator := NewToolGenerator(&MockLLMClient{CompleteWithSystemFunc: func(_ context.Context, _, prompt string) (string, error) {
		observed = prompt
		return learningContextCode, nil
	}}, t.TempDir())
	generator.SetPromptAssembler(stagePromptAssembler{})
	need := &ToolNeed{Name: "snapshot", Purpose: "Replace and clear learning context", InputType: "string", OutputType: "string"}
	for _, value := range []string{"learning-alpha", "learning-beta", ""} {
		generator.SetLearningsContext(value)
		if code, err := generator.generateToolCode(context.Background(), need); err != nil || code != learningContextCode {
			t.Fatalf("normal generation changed: code=%q err=%v", code, err)
		}
		if value == "" {
			if strings.Contains(observed, "LEARNINGS FROM PAST TOOL GENERATION:") {
				t.Fatal("cleared learning context survived in the next prompt")
			}
		} else if !strings.Contains(observed, "LEARNINGS FROM PAST TOOL GENERATION:\n"+value+"\n") {
			t.Fatalf("next request missed the latest complete learning context: %q", observed)
		}
	}
}

func TestToolGeneratorLearningContextSnapshots(t *testing.T) {
	for _, before := range []string{"", "learning-alpha", "learning-beta"} {
		t.Run(fmt.Sprintf("initial_%q", before), func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			seen, release, finished := make(chan string, 1), make(chan struct{}), make(chan struct{})
			result := make(chan error, 1)
			client := &MockLLMClient{CompleteWithSystemFunc: func(ctx context.Context, system, prompt string) (string, error) {
				if system != "stage prompt /specification" {
					return "", fmt.Errorf("generation lost JIT stage: %q", system)
				}
				seen <- prompt
				select {
				case <-release:
					return learningContextCode, nil
				case <-ctx.Done():
					return "", ctx.Err()
				}
			}}
			generator := NewToolGenerator(client, t.TempDir())
			generator.SetPromptAssembler(stagePromptAssembler{})
			generator.SetLearningsContext(before)
			need := &ToolNeed{Name: "snapshot", Purpose: "Observe an immutable prompt snapshot", InputType: "string", OutputType: "string"}
			go func() {
				defer close(finished)
				_, err := generator.generateToolCode(ctx, need)
				result <- err
			}()
			defer func() { cancel(); close(release); <-finished }()
			var prompt string
			select {
			case prompt = <-seen:
			case <-time.After(3 * time.Second):
				t.Fatal("generation did not reach the actual model-request boundary")
			}
			updated := make(chan struct{})
			go func() { generator.SetLearningsContext("learning-later"); close(updated) }()
			select {
			case <-updated:
			case <-time.After(3 * time.Second):
				t.Fatal("learning refresh held a lock across the model call")
			}
			if before == "" {
				if strings.Contains(prompt, "LEARNINGS FROM PAST TOOL GENERATION:") {
					t.Fatal("empty learning context produced an injection block")
				}
			} else if !strings.Contains(prompt, "LEARNINGS FROM PAST TOOL GENERATION:\n"+before+"\n") {
				t.Fatalf("request did not preserve its complete initial learning snapshot: %q", prompt)
			}
			if strings.Contains(prompt, "learning-later") {
				t.Fatal("in-flight request changed after its owner captured the prompt")
			}
			// The already captured request is independent of a later refresh.
			cancel()
			if err := <-result; err != context.Canceled {
				t.Fatalf("model-call cancellation changed: %v", err)
			}
		})
	}
}

func TestToolGeneratorLearningContextConcurrentUpdates(t *testing.T) {
	const workers, iterations = 16, 64
	var calls atomic.Int64
	client := &MockLLMClient{CompleteWithSystemFunc: func(_ context.Context, _, prompt string) (string, error) {
		calls.Add(1)
		if strings.Contains(prompt, "LEARNINGS FROM PAST TOOL GENERATION:") &&
			!strings.Contains(prompt, "LEARNINGS FROM PAST TOOL GENERATION:\nlearning-alpha\n") &&
			!strings.Contains(prompt, "LEARNINGS FROM PAST TOOL GENERATION:\nlearning-beta\n") {
			return "", fmt.Errorf("learning block was not one whole published snapshot")
		}
		return learningContextCode, nil
	}}
	generator := NewToolGenerator(client, t.TempDir())
	generator.SetPromptAssembler(stagePromptAssembler{})
	need := &ToolNeed{Name: "snapshot", Purpose: "Exercise concurrent refresh and prompt capture", InputType: "string", OutputType: "string"}
	start := make(chan struct{})
	errors := make(chan error, workers)
	var joined sync.WaitGroup
	for worker := range workers {
		joined.Add(2)
		go func() {
			defer joined.Done()
			<-start
			values := []string{"learning-alpha", "learning-beta", ""}
			for index := range iterations {
				generator.SetLearningsContext(values[(index+worker)%len(values)])
			}
		}()
		go func() {
			defer joined.Done()
			<-start
			for range iterations {
				code, err := generator.generateToolCode(context.Background(), need)
				if err != nil || code != learningContextCode {
					errors <- fmt.Errorf("actual generation request: code=%q err=%v", code, err)
					return
				}
			}
		}()
	}
	close(start)
	joined.Wait()
	close(errors)
	for err := range errors {
		t.Error(err)
	}
	if calls.Load() != workers*iterations {
		t.Fatalf("generation requests lost: %d", calls.Load())
	}
}
