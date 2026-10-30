package chat

import (
	"codenerd/internal/perception"
	"context"
)

// clarifyStubTransducer is a deterministic perception stub for the clarify-lane
// pin. It returns one canned intent regardless of input, the same way the
// integration harness's chatLoopTransducer does, but it lives in the default
// build so this test runs under plain `go test ./...`.
type clarifyStubTransducer struct {
	intent perception.Intent
	calls  int
}

func (s *clarifyStubTransducer) ParseIntentWithContext(_ context.Context, _ string, _ []perception.ConversationTurn) (perception.Intent, error) {
	s.calls++
	return s.intent, nil
}

// ParseIntent satisfies the perception.Transducer interface. The clarify lane
// under test goes through ParseIntentWithContext; this entry point returns
// the same canned intent so the stub is assignable anywhere a Transducer is
// required.
func (s *clarifyStubTransducer) ParseIntent(_ context.Context, _ string) (perception.Intent, error) {
	s.calls++
	return s.intent, nil
}

// ParseIntentWithGCD satisfies the perception.Transducer interface. The
// clarify lane under test never calls it; the stub returns the canned intent
// with no suggestions.
func (s *clarifyStubTransducer) ParseIntentWithGCD(_ context.Context, _ string, _ []perception.ConversationTurn, _ int) (perception.Intent, []string, error) {
	return s.intent, nil, nil
}

// ResolveFocus satisfies the perception.Transducer interface. Focus
// resolution is unused by the clarify lane; the stub returns the zero value.
func (s *clarifyStubTransducer) ResolveFocus(_ context.Context, _ string, _ []string) (perception.FocusResolution, error) {
	var zero perception.FocusResolution
	return zero, nil
}

// SetPromptAssembler satisfies the perception.Transducer interface. The stub
// has no prompt assembly to configure.
func (s *clarifyStubTransducer) SetPromptAssembler(_ perception.PromptAssembler) {}

// SetStrategicContext satisfies the perception.Transducer interface. The stub
// carries no strategic context.
func (s *clarifyStubTransducer) SetStrategicContext(_ string) {}
