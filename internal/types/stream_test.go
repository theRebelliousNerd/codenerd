package types

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestStreamWriter_FinishPublishesExactlyOneTerminalAfterTheLastChunk(t *testing.T) {
	w, content, done := NewStream()
	go func() {
		defer w.Close()
		_ = w.Content(context.Background(), "a")
		_ = w.Content(context.Background(), "b")
		w.Finish(StreamResult{StopReason: "stop"})
		w.Finish(StreamResult{StopReason: "second"}) // ignored
		w.Fail(errors.New("also ignored"))
	}()
	text, res := CollectStream(context.Background(), content, done, nil)
	if text != "ab" || res.Err != nil || res.StopReason != "stop" {
		t.Fatalf("got %q %+v, want ab and the first terminal", text, res)
	}
	if _, more := <-done; more {
		t.Fatal("terminal channel carried a second value")
	}
}

func TestStreamWriter_CloseWithoutFinishIsIncomplete(t *testing.T) {
	w, content, done := NewStream()
	go func() {
		_ = w.Content(context.Background(), "looks whole")
		w.Close()
	}()
	text, res := CollectStream(context.Background(), content, done, nil)
	if text != "looks whole" {
		t.Fatalf("text = %q", text)
	}
	if !errors.Is(res.Err, ErrStreamIncomplete) || res.Outcome() != StreamIncomplete {
		t.Fatalf("closed without a terminal: %v (%s), want incomplete", res.Err, res.Outcome())
	}
}

func TestAwaitStreamResult_NilChannelIsIncompleteAndContextWins(t *testing.T) {
	if res := AwaitStreamResult(context.Background(), nil); res.Outcome() != StreamIncomplete {
		t.Fatalf("nil terminal channel: %s", res.Outcome())
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if res := AwaitStreamResult(ctx, make(chan StreamResult)); res.Outcome() != StreamCanceled {
		t.Fatalf("canceled wait: %s", res.Outcome())
	}
}

func TestStreamOutcomeOf_ClassifiesEveryTerminal(t *testing.T) {
	cases := []struct {
		err  error
		want StreamOutcome
	}{
		{nil, StreamComplete},
		{&OutputTruncated{Reason: "length"}, StreamTruncated},
		{fmt.Errorf("wrapped: %w", &StreamError{Outcome: StreamRefused}), StreamRefused},
		{&StreamError{Outcome: StreamFiltered}, StreamFiltered},
		{&StreamError{Outcome: StreamProtocolFailed}, StreamProtocolFailed},
		{context.Canceled, StreamCanceled},
		{fmt.Errorf("x: %w", context.DeadlineExceeded), StreamCanceled},
		{errors.New("HTTP 500"), StreamFailed},
	}
	for _, tc := range cases {
		if got := StreamOutcomeOf(tc.err); got != tc.want {
			t.Errorf("StreamOutcomeOf(%v) = %s, want %s", tc.err, got, tc.want)
		}
	}
	if !errors.Is(&StreamError{Outcome: StreamIncomplete}, ErrStreamIncomplete) {
		t.Error("an incomplete StreamError does not match ErrStreamIncomplete")
	}
	if errors.Is(&StreamError{Outcome: StreamFailed}, ErrStreamIncomplete) {
		t.Error("a failed StreamError matches ErrStreamIncomplete")
	}
}

func TestStreamWhole_PublishesTextThenCompleteOrTheError(t *testing.T) {
	content, done := StreamWhole(context.Background(), func(context.Context) (string, error) { return "whole", nil })
	text, res := CollectStream(context.Background(), content, done, nil)
	if text != "whole" || res.Err != nil {
		t.Fatalf("got %q %v", text, res.Err)
	}
	boom := errors.New("boom")
	content, done = StreamWhole(context.Background(), func(context.Context) (string, error) { return "partial", boom })
	text, res = CollectStream(context.Background(), content, done, nil)
	if text != "" || !errors.Is(res.Err, boom) {
		t.Fatalf("failed whole call: text %q err %v, want no content and the error", text, res.Err)
	}
}

func TestRelayStream_ForwardsReportsEmissionAndReturnsTheSourceTerminal(t *testing.T) {
	srcW, srcContent, srcThoughts, srcDone := NewThoughtStream()
	go func() {
		defer srcW.Close()
		_ = srcW.Thought(context.Background(), "hmm")
		_ = srcW.Content(context.Background(), "x")
		srcW.Finish(StreamResult{StopReason: "STOP", Usage: UsageMetadata{InputTokens: 3, Known: UsageTokens}})
	}()
	w, content, thoughts, done := NewThoughtStream()
	var res StreamResult
	var emitted bool
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		res, emitted = RelayStream(context.Background(), w, srcContent, srcThoughts, srcDone)
		w.Finish(res)
	}()
	var gotThoughts strings.Builder
	go func() {
		for th := range thoughts {
			gotThoughts.WriteString(th)
		}
	}()
	text, out := CollectStream(context.Background(), content, done, nil)
	<-finished
	if text != "x" || out.StopReason != "STOP" || out.Usage.InputTokens != 3 || !emitted {
		t.Fatalf("relay: text %q terminal %+v emitted %v", text, out, emitted)
	}
}

// When the consumer is gone, the relay still drains the source to its close
// and keeps the usage the source saw; the result is the cancellation.
func TestRelayStream_CancelDrainsTheSourceAndKeepsItsUsage(t *testing.T) {
	srcW, srcContent, srcDone := NewStream()
	go func() {
		defer srcW.Close()
		for i := 0; i < 500; i++ {
			if err := srcW.Content(context.Background(), "chunk"); err != nil {
				return
			}
		}
		srcW.Finish(StreamResult{Usage: UsageMetadata{InputTokens: 7, Partial: true}})
	}()
	ctx, cancel := context.WithCancel(context.Background())
	w, _, _ := NewStream() // nobody reads it
	cancel()
	resCh := make(chan StreamResult, 1)
	go func() {
		res, _ := RelayStream(ctx, w, srcContent, nil, srcDone)
		resCh <- res
	}()
	select {
	case res := <-resCh:
		if res.Outcome() != StreamCanceled || res.Usage.InputTokens != 7 {
			t.Fatalf("relay after cancel: %s usage %+v, want canceled with the source's usage", res.Outcome(), res.Usage)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("relay did not finish after cancellation; the source was left blocked")
	}
}

func TestCaptureStream_CopiesEveryChunk(t *testing.T) {
	src := make(chan string, 3)
	src <- "a"
	src <- "b"
	close(src)
	var b strings.Builder
	var got strings.Builder
	for chunk := range CaptureStream(src, &b) {
		got.WriteString(chunk)
	}
	if b.String() != "ab" || got.String() != "ab" {
		t.Fatalf("captured %q forwarded %q", b.String(), got.String())
	}
	if CaptureStream(nil, &b) != nil {
		t.Fatal("a nil source must stay nil")
	}
}

func TestUsageMetadata_PlusAndSameCounts(t *testing.T) {
	a := UsageMetadata{InputTokens: 10, OutputTokens: 2, CachedContentTokens: 4, Known: UsageTokens | UsageCacheRead}
	b := UsageMetadata{InputTokens: 5, OutputTokens: 1, Known: UsageTokens, Partial: true}
	sum := a.Plus(b)
	if sum.InputTokens != 15 || sum.CachedContentTokens != 4 || sum.Known != UsageTokens || !sum.Partial {
		t.Fatalf("Plus = %+v: a category known on one side only is a lower bound, not known", sum)
	}
	if !a.SameCounts(UsageMetadata{InputTokens: 10, OutputTokens: 2, Known: UsageTokens}) {
		t.Error("reports agreeing on every shared category differ")
	}
	if a.SameCounts(UsageMetadata{InputTokens: 10, OutputTokens: 2, CachedContentTokens: 9, Known: UsageTokens | UsageCacheRead}) {
		t.Error("a cache disagreement went unnoticed")
	}
}
