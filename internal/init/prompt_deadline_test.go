package init

import (
	"bufio"
	"context"
	"errors"
	"os"
	"testing"
	"time"
)

// openTTYLikeReader returns a reader over a pipe whose write end is held open
// and never written to. That is the shape of the environment that caused the
// hang: a real file descriptor that satisfies every "is someone there?" check,
// supplies no data, and — unlike /dev/null or a closed pipe — never returns
// EOF either. `docker run -t`, `docker compose run` and CI wrappers that
// allocate a pty all present exactly this to `nerd init`.
func openTTYLikeReader(t *testing.T) *bufio.Reader {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	t.Cleanup(func() {
		_ = w.Close() // held open for the duration of the test on purpose
		_ = r.Close()
	})
	return bufio.NewReader(r)
}

// TestReadInput_WhenNobodyAnswers_ShouldGiveUpRatherThanBlockForever is the
// regression test for the hang.
//
// Before this, readInput was a bare bufio.Reader.ReadString('\n'). Under an
// allocated pty with no input it blocked for the life of the process, and the
// surrounding 25-minute operation timeout could not touch it: the phase loop
// only re-checks ctx after the read returns, so a cancelled context waited
// behind a blocked syscall. `nerd init` inside `docker run -t` never finished.
//
// The test asserts termination, not speed — a bound that only holds on a fast
// machine is not a bound.
func TestReadInput_WhenNobodyAnswers_ShouldGiveUpRatherThanBlockForever(t *testing.T) {
	reader := openTTYLikeReader(t)

	done := make(chan error, 1)
	start := time.Now()
	go func() {
		_, err := readInput(context.Background(), reader, 150*time.Millisecond)
		done <- err
	}()

	select {
	case err := <-done:
		if !errors.Is(err, errPromptUnanswered) {
			t.Fatalf("err = %v, want errPromptUnanswered", err)
		}
		if elapsed := time.Since(start); elapsed < 150*time.Millisecond {
			t.Errorf("returned after %s, before the %s deadline — the timer is not what released it",
				elapsed, 150*time.Millisecond)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("readInput did not return 30s after a 150ms deadline; the prompt still hangs")
	}
}

// TestReadInput_WhenRunIsCancelled_ShouldReturnPromptly covers the other half:
// the operation timeout and Ctrl-C must be able to interrupt a prompt, which is
// the specific thing the unbounded read made impossible.
func TestReadInput_WhenRunIsCancelled_ShouldReturnPromptly(t *testing.T) {
	reader := openTTYLikeReader(t)
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		// A deadline far beyond the test's patience: only ctx can release this.
		_, err := readInput(ctx, reader, time.Hour)
		done <- err
	}()

	cancel()

	select {
	case err := <-done:
		if !errors.Is(err, errPromptUnanswered) || !errors.Is(err, context.Canceled) {
			t.Fatalf("err = %v, want both errPromptUnanswered and context.Canceled", err)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("cancelling the run did not interrupt the prompt")
	}
}

// TestReadInput_WhenAnswered_ShouldReturnTheAnswer keeps the fix from being a
// regression in the other direction: a deadline that also drops real input
// would be worse than the hang.
func TestReadInput_WhenAnswered_ShouldReturnTheAnswer(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	defer r.Close()

	go func() {
		defer w.Close()
		_, _ = w.WriteString("  c  \n")
	}()

	got, err := readInput(context.Background(), bufio.NewReader(r), 30*time.Second)
	if err != nil {
		t.Fatalf("readInput: %v", err)
	}
	if got != "c" {
		t.Errorf("got %q, want %q (surrounding whitespace must be trimmed)", got, "c")
	}
}

// TestCurateAgents_WhenPromptGoesUnanswered_ShouldKeepRecommendedAgents is the
// behavioural half: init must not just stop hanging, it must finish with the
// same agent set a non-interactive run would have produced. Losing the user's
// workspace setup to an unanswered prompt is not an acceptable way to
// terminate.
func TestCurateAgents_WhenPromptGoesUnanswered_ShouldKeepRecommendedAgents(t *testing.T) {
	workspace := t.TempDir()
	if err := os.MkdirAll(workspace+"/.nerd", 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	ini := &Initializer{config: InitConfig{
		Workspace:   workspace,
		Interactive: true,
		InteractiveIO: &InteractiveConfig{
			Reader:        openTTYLikeReader(t),
			Writer:        os.Stdout,
			PromptTimeout: 150 * time.Millisecond,
		},
	}}

	offered := []RecommendedAgent{
		{Name: "GoExpert", Type: "language", Reason: "go.mod present"},
		{Name: "RedisExpert", Type: "dependency", Reason: "redis in go.mod"},
	}
	result := &InitResult{}

	done := make(chan []RecommendedAgent, 1)
	go func() {
		done <- ini.curateAgents(context.Background(), offered, ProjectProfile{Language: "go"}, result)
	}()

	select {
	case curated := <-done:
		if len(curated) != len(offered) {
			t.Fatalf("kept %d agents, want all %d recommended when the prompt goes unanswered",
				len(curated), len(offered))
		}
		if len(result.Warnings) == 0 {
			t.Error("init finished silently; an unanswered prompt must leave a warning explaining " +
				"why the user was not asked")
		}
	case <-time.After(30 * time.Second):
		t.Fatal("curateAgents did not return; `nerd init` still hangs on an unanswered prompt")
	}
}
