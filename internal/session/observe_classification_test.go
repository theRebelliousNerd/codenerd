package session

import (
	"context"
	"errors"
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/perception"
)

// A session turn that perceives its own intent does not run on the
// compatibility /explain a failed classification leaves: observation ends with
// the kernel's disposition, typed, so a caller can tell a blocker from a retry.
func TestExecutor_Observe_FailedClassificationEndsTheTurn(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		err    error
		action string
	}{
		{"authentication", errors.New("API request failed with status 401"), "/blocker"},
		{"outage", errors.New("transient server error (503)"), "/retry"},
		{"canceled", context.Canceled, "/canceled"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			kernel, err := core.NewRealKernel()
			if err != nil {
				t.Fatal(err)
			}
			failed := perception.Intent{Verb: "/explain", Category: "/query",
				Outcome: perception.FailedOutcome(tc.err, perception.MethodGeneric, 1)}
			tr := &MockTransducer{ParseIntentWithContextFunc: func(context.Context, string, []perception.ConversationTurn) (perception.Intent, error) {
				return failed, nil
			}}
			executor := NewExecutor(kernel, nil, nil, nil, nil, tr)
			_, err = executor.Process(context.Background(), "investigate the parser")
			var stop *perception.ClassificationStop
			if !errors.As(err, &stop) || stop.Action != tc.action {
				t.Fatalf("Process error = %v, want a ClassificationStop %s", err, tc.action)
			}
		})
	}
}

// A turn no model classified (a preset) and an admitted reading are observed
// without a classification stop.
func TestExecutor_Observe_AdmittedAndUnclassifiedPass(t *testing.T) {
	t.Parallel()
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatal(err)
	}
	for name, intent := range map[string]perception.Intent{
		"unclassified": {Verb: "/explain", Category: "/query"},
		"low confidence": {Verb: "/explain", Category: "/query", Confidence: 0.1,
			Outcome: perception.ClassificationOutcome{Disposition: perception.ClassificationAdmitted, Method: perception.MethodGeneric, Attempts: 1}},
	} {
		tr := &MockTransducer{ParseIntentWithContextFunc: func(context.Context, string, []perception.ConversationTurn) (perception.Intent, error) {
			return intent, nil
		}}
		executor := NewExecutor(kernel, nil, nil, nil, nil, tr)
		got, err := executor.observe(context.Background(), "hello")
		if err != nil || got.Verb != "/explain" {
			t.Fatalf("%s: observe = %+v, %v", name, got, err)
		}
	}
}
