package perception

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

type fakeStreamingLLM struct {
	chunks []string
	delay  time.Duration

	mu         sync.Mutex
	streamDone chan struct{}
}

func (f *fakeStreamingLLM) Complete(ctx context.Context, prompt string) (string, error) {
	return "", nil
}

func (f *fakeStreamingLLM) CompleteWithSystem(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	return "", nil
}

func (f *fakeStreamingLLM) CompleteWithStreaming(ctx context.Context, systemPrompt, userPrompt string, enableThinking bool) (<-chan string, <-chan error) {
	contentChan := make(chan string, 8)
	errorChan := make(chan error, 1)

	done := make(chan struct{})
	f.mu.Lock()
	f.streamDone = done
	f.mu.Unlock()

	go func() {
		defer close(contentChan)
		defer close(errorChan)
		defer close(done)

		for _, c := range f.chunks {
			select {
			case <-ctx.Done():
				errorChan <- ctx.Err()
				return
			default:
			}

			contentChan <- c

			if f.delay > 0 {
				select {
				case <-time.After(f.delay):
				case <-ctx.Done():
					errorChan <- ctx.Err()
					return
				}
			}
		}
	}()

	return contentChan, errorChan
}

func (f *fakeStreamingLLM) waitDone(t *testing.T) {
	t.Helper()
	f.mu.Lock()
	done := f.streamDone
	f.mu.Unlock()
	if done == nil {
		return
	}
	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatalf("stream did not stop in time")
	}
}

func TestCompleteWithGCDStreaming_AbortsWhenSurfaceAppearsFirst(t *testing.T) {
	fake := &fakeStreamingLLM{
		chunks: []string{
			`{"surface_response":"hi",`,
			`"control_packet":{"mangle_updates":["user_intent(/current_intent,/query,/explain,\"t\",\"\")."]}}`,
		},
		delay: 10 * time.Millisecond,
	}
	tr := NewRealTransducer(fake)
	sysCtx := NewSystemLLMContext(fake, "transducer", "test")
	defer sysCtx.Clear()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, _, _, err := tr.completeWithGCDStreaming(ctx, sysCtx, "sys", "user", 512)
	if err == nil {
		t.Fatalf("expected abort error, got nil")
	}
	var abort *gcdStreamAbortError
	if !errors.As(err, &abort) {
		t.Fatalf("expected gcdStreamAbortError, got %T: %v", err, err)
	}
	fake.waitDone(t)
}

func TestCompleteWithGCDStreaming_AbortsOnInvalidMangleUpdates(t *testing.T) {
	fake := &fakeStreamingLLM{
		chunks: []string{
			`{"control_packet":{"mangle_updates":["user_intent(/query,/explain,\"t\",\"\")."]},`,
			`"surface_response":"hi"}`,
		},
		delay: 10 * time.Millisecond,
	}
	tr := NewRealTransducer(fake)
	sysCtx := NewSystemLLMContext(fake, "transducer", "test")
	defer sysCtx.Clear()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, _, _, err := tr.completeWithGCDStreaming(ctx, sysCtx, "sys", "user", 2048)
	if err == nil {
		t.Fatalf("expected abort error, got nil")
	}
	var abort *gcdStreamAbortError
	if !errors.As(err, &abort) {
		t.Fatalf("expected gcdStreamAbortError, got %T: %v", err, err)
	}
	if abort.Reason == "" {
		t.Fatalf("expected abort reason to be set")
	}
	fake.waitDone(t)
}

func TestCompleteWithGCDStreaming_ValidatesAndCompletes(t *testing.T) {
	fake := &fakeStreamingLLM{
		chunks: []string{
			`{"control_packet":{"mangle_updates":["user_intent(/current_intent,/query,/explain,\"t\",\"\")."]},`,
			`"surface_response":"hi"}`,
		},
		delay: 5 * time.Millisecond,
	}
	tr := NewRealTransducer(fake)
	sysCtx := NewSystemLLMContext(fake, "transducer", "test")
	defer sysCtx.Clear()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	raw, atoms, validated, err := tr.completeWithGCDStreaming(ctx, sysCtx, "sys", "user", 2048)
	if err != nil {
		t.Fatalf("expected nil err, got %v", err)
	}
	if raw == "" {
		t.Fatalf("expected non-empty raw response")
	}
	if !validated {
		t.Fatalf("expected validated=true")
	}
	if len(atoms) != 1 {
		t.Fatalf("expected 1 atom, got %d (%v)", len(atoms), atoms)
	}
	fake.waitDone(t)
}
