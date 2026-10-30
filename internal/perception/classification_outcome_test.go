package perception

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/types"
)

const outcomeSecret = "sk-live-do-not-leak"

// admissionEnvelope is a complete classification answer.
const admissionEnvelope = `{"understanding":{"primary_intent":"investigate","semantic_type":"location","action_type":"investigate","domain":"general","confidence":0.9},"surface_response":"Looking."}`

func TestParseResponse_AdmitsOnlyClassifications(t *testing.T) {
	cases := []struct {
		name      string
		response  string
		wantStage ClassificationStage // "" admits
		wantCode  string
	}{
		{"valid envelope", admissionEnvelope, "", ""},
		{"valid bare profile", `{"semantic_type":"location","action_type":"investigate","domain":"general","confidence":0.9}`, "", ""},
		{"envelope without optional summary", `{"understanding":{"action_type":"investigate","confidence":0.5}}`, "", ""},
		{"explicit zero confidence is a reading, not an omission", `{"understanding":{"action_type":"explain","confidence":0}}`, "", ""},
		{"vocabulary the kernel does not know is still admitted", `{"understanding":{"action_type":"frobnicate","domain":"quantum","confidence":0.8}}`, "", ""},
		{"percentage confidence", `{"understanding":{"action_type":"explain","confidence":85}}`, "", ""},
		// The review witness: the envelope's array cannot populate ActionType
		// string, and the bare decode used to ignore the nested object and return
		// an all-zero understanding with a nil error.
		{"envelope with a mistyped action", `{"understanding":{"primary_intent":"investigate","semantic_type":"location","action_type":["investigate"],"domain":"general","confidence":0.9},"surface_response":"I will investigate."}`,
			StageDecode, "envelope_field_type:understanding.action_type"},
		{"bare with a mistyped action", `{"semantic_type":"location","action_type":123,"confidence":0.9}`,
			StageDecode, "bare_field_type:action_type"},
		{"null understanding", `{"understanding":null,"surface_response":"hi"}`, StageDecode, "envelope_understanding_not_object"},
		{"scalar understanding", `{"understanding":"investigate"}`, StageDecode, "envelope_understanding_not_object"},
		{"empty object", `{}`, StageRequiredFields, "missing:action_type,confidence"},
		{"unrelated object", `{"weather":"sunny"}`, StageRequiredFields, "missing:action_type,confidence"},
		{"surface only", `{"surface_response":"hello there"}`, StageRequiredFields, "missing:action_type,confidence"},
		{"omitted confidence", `{"understanding":{"action_type":"explain"}}`, StageRequiredFields, "missing:confidence"},
		{"blank action", `{"understanding":{"action_type":"  ","confidence":0.9}}`, StageRequiredFields, "missing:action_type"},
		{"null confidence", `{"understanding":{"action_type":"explain","confidence":null}}`, StageRequiredFields, "missing:confidence"},
		{"confidence over 100", `{"understanding":{"action_type":"explain","confidence":250}}`, StageRequiredFields, "confidence_out_of_range"},
		{"negative confidence", `{"understanding":{"action_type":"explain","confidence":-0.2}}`, StageRequiredFields, "confidence_out_of_range"},
		{"array answer", `[1,2,3]`, StageDecode, "not_an_object"},
		{"prose", `I could not decide.`, StageOutput, "no_json"},
	}
	tr := &LLMTransducer{}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			u, err := tr.parseResponse(tc.response)
			if tc.wantStage == "" {
				if err != nil || u == nil {
					t.Fatalf("parseResponse = %v, %v; want an admitted understanding", u, err)
				}
				return
			}
			var ce *ClassificationError
			if !errors.As(err, &ce) {
				t.Fatalf("parseResponse err = %v (understanding %+v); want a ClassificationError", err, u)
			}
			if ce.Stage != tc.wantStage || ce.Code != tc.wantCode {
				t.Fatalf("stage/code = %s/%s, want %s/%s", ce.Stage, ce.Code, tc.wantStage, tc.wantCode)
			}
		})
	}
}

func TestParseResponse_PercentIsConvertedNotClamped(t *testing.T) {
	u, err := (&LLMTransducer{}).parseResponse(`{"understanding":{"action_type":"explain","confidence":85}}`)
	if err != nil {
		t.Fatal(err)
	}
	if u.Confidence != 0.85 {
		t.Fatalf("confidence = %v, want 0.85: a percentage must not clamp to certainty", u.Confidence)
	}
}

func TestParseResponse_BareAndEnvelopeAgree(t *testing.T) {
	tr := &LLMTransducer{}
	env, err := tr.parseResponse(`{"understanding":{"semantic_type":"Location","action_type":"Investigate","domain":"general","confidence":0.9,"scope":{"target":"a.go"}}}`)
	if err != nil {
		t.Fatal(err)
	}
	bare, err := tr.parseResponse(`{"semantic_type":"Location","action_type":"Investigate","domain":"general","confidence":0.9,"scope":{"target":"a.go"}}`)
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprintf("%+v", *env) != fmt.Sprintf("%+v", *bare) {
		t.Fatalf("envelope and bare profile normalized differently:\n%+v\n%+v", *env, *bare)
	}
}

func TestFailedOutcome_ClassifiesEveryCause(t *testing.T) {
	cases := []struct {
		name        string
		err         error
		disposition ClassificationDisposition
		stage       ClassificationStage
		failure     ProviderFailureClass
		status      int
		retryable   bool
	}{
		{"outage", fmt.Errorf("LLM: %w", errors.New("transient server error (503): upstream")), ClassificationProviderFailed, StageTransport, FailureTransient, 503, true},
		{"sentinel outage", fmt.Errorf("max retries: %w", ErrLLMUnavailable), ClassificationProviderFailed, StageTransport, FailureTransient, 0, true},
		{"rate limit", errors.New("rate limit exceeded (429)"), ClassificationProviderFailed, StageTransport, FailureRateLimited, 429, true},
		{"auth", errors.New("API request failed with status 401: " + outcomeSecret), ClassificationProviderFailed, StageTransport, FailureAuth, 401, false},
		{"rejected", errors.New("API request failed with status 413"), ClassificationProviderFailed, StageTransport, FailureInvalidRequest, 413, false},
		{"permanent", errors.New("boom"), ClassificationProviderFailed, StageTransport, FailurePermanent, 0, false},
		{"canceled", fmt.Errorf("send: %w", context.Canceled), ClassificationCanceled, StageTransport, FailureCanceled, 0, false},
		{"deadline", context.DeadlineExceeded, ClassificationCanceled, StageTransport, FailureCanceled, 0, false},
		{"unreadable", &ClassificationError{Stage: StageOutput, Code: "no_json"}, ClassificationInvalidResponse, StageOutput, "", 0, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := FailedOutcome(tc.err, MethodGeneric, 1)
			if got.Disposition != tc.disposition || got.Stage != tc.stage || got.Failure != tc.failure || got.Status != tc.status {
				t.Fatalf("outcome = %+v", got)
			}
			if got.Retryable() != tc.retryable || !got.Failed() {
				t.Fatalf("retryable=%v failed=%v for %+v", got.Retryable(), got.Failed(), got)
			}
			if strings.Contains(got.SafeMessage(), outcomeSecret) {
				t.Fatalf("safe message leaks the provider body: %q", got.SafeMessage())
			}
		})
	}
}

func TestOutcome_ZeroIsNotAClassificationAndAdmittedIsNotAFailure(t *testing.T) {
	var zero ClassificationOutcome
	if zero.Failed() {
		t.Fatal("a preset intent (zero outcome) must not read as a failure")
	}
	if _, ok := zero.Fact("/current_intent"); ok {
		t.Fatal("a zero outcome must project no row: not read is not an observation")
	}
	adm := admittedOutcome(MethodSchema, 1)
	if adm.Failed() {
		t.Fatal("admitted outcome reads as failed")
	}
	if f, ok := adm.Fact("/current_intent"); !ok || len(f.Args) != 5 || f.Args[4] != types.MangleAtom("/schema") {
		t.Fatalf("admitted fact = %v %v", f, ok)
	}
}

// ---- the degraded compatibility intent --------------------------------------

func TestDegradedIntent_CarriesTheTypedCauseAndAHonestResponse(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want string
	}{
		{"auth", fmt.Errorf("API request failed with status 401: %s", outcomeSecret), "credentials"},
		{"outage", errors.New("transient server error (503)"), "briefly overloaded"},
		{"unreadable", &ClassificationError{Stage: StageDecode, Code: "bare_field_type:action_type"}, "could not be read as a classification"},
		{"canceled", context.Canceled, "canceled"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			intent := degradedClassificationIntent(FailedOutcome(tc.err, MethodGeneric, 1))
			if intent.Verb != "/explain" || intent.Category != "/query" || intent.Confidence != 0 {
				t.Fatalf("degraded projection = %+v", intent)
			}
			if !intent.Outcome.Failed() {
				t.Fatalf("typed cause lost: %+v", intent.Outcome)
			}
			if !strings.Contains(intent.Response, tc.want) || strings.Contains(intent.Response, outcomeSecret) {
				t.Fatalf("response = %q, want %q and no provider body", intent.Response, tc.want)
			}
		})
	}
}

// ---- one operation behind every entry point ---------------------------------

// planClient scripts a thinking client that may enforce schemas.
type planClient struct {
	baseMockLLMClient
	schemaCapable bool
	schemaResp    string
	schemaErr     error
	freeResp      string
	freeErr       error
	schemaCalls   int
	freeCalls     int
	freeSystem    string
}

func (c *planClient) IsThinkingEnabled() bool  { return true }
func (c *planClient) GetThinkingLevel() string { return "high" }
func (c *planClient) SchemaCapable() bool      { return c.schemaCapable }
func (c *planClient) CompleteWithSchema(ctx context.Context, system, user, schema string) (string, error) {
	c.schemaCalls++
	return c.schemaResp, c.schemaErr
}
func (c *planClient) CompleteWithSystem(ctx context.Context, system, user string) (string, error) {
	c.freeCalls++
	c.freeSystem = system
	return c.freeResp, c.freeErr
}

func TestEveryEntryPointRunsTheSameSchemaOperation(t *testing.T) {
	ctx := context.Background()
	entries := map[string]func(tr Transducer) (Intent, error){
		"ParseIntent":            func(tr Transducer) (Intent, error) { return tr.ParseIntent(ctx, "investigate x.go") },
		"ParseIntentWithContext": func(tr Transducer) (Intent, error) { return tr.ParseIntentWithContext(ctx, "investigate x.go", nil) },
		"ParseIntentWithGCD": func(tr Transducer) (Intent, error) {
			intent, _, err := tr.ParseIntentWithGCD(ctx, "investigate x.go", nil, 3)
			return intent, err
		},
	}
	for name, call := range entries {
		t.Run(name, func(t *testing.T) {
			client := &planClient{schemaCapable: true, schemaResp: admissionEnvelope}
			tr := newCompiledUnderstandingTransducer(t, client)
			intent, err := call(tr)
			if err != nil {
				t.Fatal(err)
			}
			if client.schemaCalls != 1 || client.freeCalls != 0 {
				t.Fatalf("schema=%d free=%d; the GCD entry must not bypass the schema operation", client.schemaCalls, client.freeCalls)
			}
			if intent.Outcome.Disposition != ClassificationAdmitted || intent.Outcome.Method != MethodSchema || intent.Outcome.Attempts != 1 {
				t.Fatalf("outcome = %+v, want admitted via schema in one attempt", intent.Outcome)
			}
		})
	}
}

func TestNonThinkingClientUsesTheGenericOperation(t *testing.T) {
	plain := &genericCountingClient{response: admissionEnvelope}
	intent, err := newCompiledUnderstandingTransducer(t, plain).ParseIntentWithContext(context.Background(), "investigate x.go", nil)
	if err != nil {
		t.Fatal(err)
	}
	if plain.calls != 1 || intent.Outcome.Method != MethodGeneric {
		t.Fatalf("calls=%d outcome=%+v, want one generic completion", plain.calls, intent.Outcome)
	}
}

type genericCountingClient struct {
	baseMockLLMClient
	response string
	calls    int
}

func (c *genericCountingClient) CompleteWithSystem(ctx context.Context, system, user string) (string, error) {
	c.calls++
	return c.response, nil
}

func TestSchemaFailureTransitionsAreCauseAware(t *testing.T) {
	ctx := context.Background()
	t.Run("authentication does not reach free-form", func(t *testing.T) {
		client := &planClient{schemaCapable: true, schemaErr: errors.New("API request failed with status 401"), freeResp: admissionEnvelope}
		intent, _ := newCompiledUnderstandingTransducer(t, client).ParseIntentWithContext(ctx, "x", nil)
		if client.freeCalls != 0 || intent.Outcome.Failure != FailureAuth || intent.Outcome.Attempts != 1 {
			t.Fatalf("free=%d outcome=%+v; a terminal cause must not spend a second send", client.freeCalls, intent.Outcome)
		}
	})
	t.Run("cancellation does not reach free-form", func(t *testing.T) {
		client := &planClient{schemaCapable: true, schemaErr: fmt.Errorf("send: %w", context.Canceled), freeResp: admissionEnvelope}
		intent, _ := newCompiledUnderstandingTransducer(t, client).ParseIntentWithContext(ctx, "x", nil)
		if client.freeCalls != 0 || intent.Outcome.Disposition != ClassificationCanceled {
			t.Fatalf("free=%d outcome=%+v", client.freeCalls, intent.Outcome)
		}
	})
	t.Run("an outage after the adapter's retries does not reach free-form", func(t *testing.T) {
		client := &planClient{schemaCapable: true, schemaErr: errors.New("transient server error (503)"), freeResp: admissionEnvelope}
		intent, _ := newCompiledUnderstandingTransducer(t, client).ParseIntentWithContext(ctx, "x", nil)
		if client.freeCalls != 0 || !intent.Outcome.Retryable() {
			t.Fatalf("free=%d outcome=%+v", client.freeCalls, intent.Outcome)
		}
	})
	t.Run("a generic rejected request is not a missing capability", func(t *testing.T) {
		client := &planClient{schemaCapable: true, schemaErr: errors.New("API request failed with status 400"), freeResp: admissionEnvelope}
		intent, _ := newCompiledUnderstandingTransducer(t, client).ParseIntentWithContext(ctx, "x", nil)
		if client.freeCalls != 0 || intent.Outcome.Failure != FailureInvalidRequest {
			t.Fatalf("free=%d outcome=%+v", client.freeCalls, intent.Outcome)
		}
	})
	t.Run("a client that cannot enforce a schema falls to free-form", func(t *testing.T) {
		client := &planClient{schemaCapable: true, schemaErr: core.ErrSchemaNotSupported, freeResp: admissionEnvelope}
		intent, _ := newCompiledUnderstandingTransducer(t, client).ParseIntentWithContext(ctx, "x", nil)
		if client.schemaCalls != 1 || client.freeCalls != 1 || intent.Outcome.Method != MethodFreeForm || intent.Outcome.Attempts != 2 {
			t.Fatalf("schema=%d free=%d outcome=%+v", client.schemaCalls, client.freeCalls, intent.Outcome)
		}
		if !strings.Contains(client.freeSystem, "Thinking") {
			t.Fatal("the free-form attempt lost the thinking system prompt")
		}
	})
	t.Run("a schema answer that is not a classification falls to free-form once", func(t *testing.T) {
		client := &planClient{schemaCapable: true, schemaResp: `{"unrelated":true}`, freeResp: admissionEnvelope}
		intent, _ := newCompiledUnderstandingTransducer(t, client).ParseIntentWithContext(ctx, "x", nil)
		if client.schemaCalls != 1 || client.freeCalls != 1 || intent.Outcome.Disposition != ClassificationAdmitted || intent.Outcome.Attempts != 2 {
			t.Fatalf("schema=%d free=%d outcome=%+v", client.schemaCalls, client.freeCalls, intent.Outcome)
		}
	})
	t.Run("both methods unreadable ends in an invalid response after two attempts", func(t *testing.T) {
		client := &planClient{schemaCapable: true, schemaResp: `{"unrelated":true}`, freeResp: `{"weather":"sunny"}`}
		intent, _ := newCompiledUnderstandingTransducer(t, client).ParseIntentWithContext(ctx, "x", nil)
		if client.schemaCalls != 1 || client.freeCalls != 1 {
			t.Fatalf("schema=%d free=%d; the plan is bounded", client.schemaCalls, client.freeCalls)
		}
		got := intent.Outcome
		if got.Disposition != ClassificationInvalidResponse || got.Stage != StageRequiredFields || got.Attempts != 2 || got.Method != MethodFreeForm {
			t.Fatalf("outcome = %+v", got)
		}
		if intent.Verb != "/explain" || intent.Confidence != 0 {
			t.Fatalf("degraded projection = %+v", intent)
		}
	})
}

// A malformed explicit envelope is no longer a success with an empty
// interpretation: the review witness through the public entry point.
func TestMalformedEnvelopeIsNotAnAdmittedInterpretation(t *testing.T) {
	client := &genericCountingClient{response: `{"understanding":{"primary_intent":"investigate","semantic_type":"location","action_type":["investigate"],"domain":"general","confidence":0.9},"surface_response":"I will investigate."}`}
	intent, err := newCompiledUnderstandingTransducer(t, client).ParseIntentWithContext(context.Background(), "investigate x.go", nil)
	if err != nil {
		t.Fatal(err)
	}
	if intent.Outcome.Disposition != ClassificationInvalidResponse || intent.Outcome.Stage != StageDecode {
		t.Fatalf("outcome = %+v, want an invalid response at the decode stage", intent.Outcome)
	}
	if intent.Response == "I will investigate." {
		t.Fatal("the surface of a rejected envelope was shown as the answer")
	}
}

// ---- the kernel's disposition ------------------------------------------------

func outcomeKernel(t *testing.T) *core.RealKernel {
	t.Helper()
	k, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	return k
}

func stopAction(t *testing.T, k *core.RealKernel, intent Intent) *ClassificationStop {
	t.Helper()
	if err := PublishClassificationOutcome(k, "/current_intent", intent.Outcome); err != nil {
		t.Fatalf("publish: %v", err)
	}
	stop, err := ClassificationStopFor(k, "/current_intent", intent)
	if err != nil {
		t.Fatalf("ClassificationStopFor: %v", err)
	}
	return stop
}

func TestKernelDerivesTheDispositionFromTheCause(t *testing.T) {
	cases := []struct {
		name   string
		err    error
		action string
		notice string
	}{
		{"outage", errors.New("transient server error (503)"), "/retry", "couldn't reach the language model"},
		{"rate limit", errors.New("rate limit exceeded (429)"), "/retry", "rate limiting"},
		{"authentication", errors.New("API request failed with status 403"), "/blocker", "rejected the configured credentials"},
		{"rejected request", errors.New("API request failed with status 422"), "/blocker", "rejected the request"},
		{"other failure", errors.New("boom"), "/blocker", "call failed"},
		{"unreadable answer", &ClassificationError{Stage: StageDecode, Code: "x"}, "/retry", "could not be understood"},
		{"canceled", context.Canceled, "/canceled", "canceled before it finished"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			k := outcomeKernel(t)
			stop := stopAction(t, k, degradedClassificationIntent(FailedOutcome(tc.err, MethodGeneric, 1)))
			if stop == nil || stop.Action != tc.action || !strings.Contains(stop.Notice, tc.notice) {
				t.Fatalf("stop = %+v, want %s containing %q", stop, tc.action, tc.notice)
			}
		})
	}
}

func TestKernelDoesNotStopAnAdmittedOrUnclassifiedTurn(t *testing.T) {
	k := outcomeKernel(t)
	if stop := stopAction(t, k, Intent{Outcome: admittedOutcome(MethodGeneric, 1), Confidence: 0.1}); stop != nil {
		t.Fatalf("a low-confidence admitted reading was stopped: %+v", stop)
	}
	if stop := stopAction(t, k, Intent{}); stop != nil {
		t.Fatalf("a preset intent with no outcome was stopped: %+v", stop)
	}
	if rows, _ := k.Query("classification_outcome"); len(rows) != 0 {
		t.Fatalf("an unclassified intent published a row: %v", rows)
	}
}

func TestKernelStopsAnOutcomeItDoesNotKnow(t *testing.T) {
	k := outcomeKernel(t)
	if err := k.Assert(types.Fact{Predicate: "classification_outcome", Args: []any{
		types.MangleAtom("/current_intent"), types.MangleAtom("/mystery"), types.MangleAtom("/none"), types.MangleAtom("/none"), types.MangleAtom("/none")}}); err != nil {
		t.Fatal(err)
	}
	stop, err := ClassificationStopFor(k, "/current_intent", Intent{Outcome: ClassificationOutcome{Disposition: "mystery"}})
	if err != nil || stop == nil || stop.Action != "/blocker" {
		t.Fatalf("stop = %+v, %v; an unknown outcome is a blocker, never a success", stop, err)
	}
}

func TestPublishReplacesOnlyTheIntentsRow(t *testing.T) {
	k := outcomeKernel(t)
	failed := FailedOutcome(errors.New("API request failed with status 401"), MethodGeneric, 1)
	if err := PublishClassificationOutcome(k, "/task_intent_7", failed); err != nil {
		t.Fatal(err)
	}
	if err := PublishClassificationOutcome(k, "/current_intent", admittedOutcome(MethodGeneric, 1)); err != nil {
		t.Fatal(err)
	}
	if err := PublishClassificationOutcome(k, "/current_intent", failed); err != nil {
		t.Fatal(err)
	}
	rows, err := k.Query("classification_outcome")
	if err != nil || len(rows) != 2 {
		t.Fatalf("rows = %v, %v; want one row per intent", rows, err)
	}
	// Publishing an outcome no model read removes the intent's row.
	if err := PublishClassificationOutcome(k, "/current_intent", ClassificationOutcome{}); err != nil {
		t.Fatal(err)
	}
	if rows, _ = k.Query("classification_outcome"); len(rows) != 1 {
		t.Fatalf("rows after retract = %v, want the task intent's row to stand", rows)
	}
}

// Jev's judgments are independent evidence. They neither rescue a failed
// classification nor are lost to one.
func TestJevEvidenceIsIndependentOfAFailedClassification(t *testing.T) {
	assertAll := func(t *testing.T, k *core.RealKernel, facts ...types.Fact) {
		t.Helper()
		if err := k.AssertBatch(facts); err != nil {
			t.Fatal(err)
		}
	}
	thresholds := []types.Fact{
		{Predicate: "config_param", Args: []any{types.MangleAtom("/jev_yes_percent"), int64(80)}},
		{Predicate: "config_param", Args: []any{types.MangleAtom("/jev_no_percent"), int64(20)}},
	}
	judgment := func(name string, percent int64) types.Fact {
		return types.Fact{Predicate: "jev_judgment", Args: []any{name, percent}}
	}
	failed := degradedClassificationIntent(FailedOutcome(errors.New("API request failed with status 401"), MethodGeneric, 1))
	intentFact := types.Fact{Predicate: "user_intent", Args: []any{types.MangleAtom("/current_intent"), types.MangleAtom("/query"), types.MangleAtom("/explain"), "", ""}}

	t.Run("a Jev yes does not make a failed classification admitted", func(t *testing.T) {
		k := outcomeKernel(t)
		assertAll(t, k, append(thresholds, intentFact, judgment("changes_files", 99), judgment("names_target", 99))...)
		if stop := stopAction(t, k, failed); stop == nil || stop.Action != "/blocker" {
			t.Fatalf("stop = %+v; independent judgments must not clear the failure", stop)
		}
	})
	t.Run("Jev reading the go-ahead on held work keeps its precedence", func(t *testing.T) {
		k := outcomeKernel(t)
		assertAll(t, k, append(thresholds, intentFact,
			types.Fact{Predicate: "work_proposal_pending"},
			judgment("approves_held_work", 97), judgment("adds_to_held_work", 3))...)
		if stop := stopAction(t, k, failed); stop != nil {
			t.Fatalf("held work's go-ahead was swallowed by the failed classification: %+v", stop)
		}
	})
	t.Run("held work with no go-ahead evidence still stops", func(t *testing.T) {
		k := outcomeKernel(t)
		assertAll(t, k, append(thresholds, intentFact, types.Fact{Predicate: "work_proposal_pending"})...)
		if stop := stopAction(t, k, failed); stop == nil {
			t.Fatal("pending work alone must not let a failed classification through")
		}
	})
	t.Run("a stop request read by Jev is derived whatever the classification did", func(t *testing.T) {
		k := outcomeKernel(t)
		assertAll(t, k, append(thresholds, intentFact, judgment("wants_to_stop", 96),
			types.Fact{Predicate: "background_job", Args: []any{"job-1", "reviewer", "review it", types.MangleAtom("/running")}})...)
		if stop := stopAction(t, k, failed); stop == nil {
			t.Fatal("the classification failure should still be reported")
		}
		rows, err := k.Query("stop_background_requested")
		if err != nil || len(rows) == 0 {
			t.Fatalf("stop_background_requested = %v, %v; an independently admitted stop must survive a failed classification", rows, err)
		}
	})
}

func TestAdmitClassificationWithoutAKernelCannotClearAFailure(t *testing.T) {
	failed := degradedClassificationIntent(FailedOutcome(errors.New("API request failed with status 401"), MethodGeneric, 1))
	err := AdmitClassification(nil, "/current_intent", failed)
	var stop *ClassificationStop
	if !errors.As(err, &stop) || stop.Outcome.Failure != FailureAuth || stop.Notice != failed.Response {
		t.Fatalf("err = %v; a kernel that cannot be asked must leave the failure standing with the safe message", err)
	}
	if err := AdmitClassification(nil, "/current_intent", Intent{}); err != nil {
		t.Fatalf("an unclassified intent was refused without a kernel: %v", err)
	}
}
