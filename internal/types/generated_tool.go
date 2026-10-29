package types

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
)

type GeneratedToolProtocol string

const (
	GeneratedStdinV1 GeneratedToolProtocol = "generated-stdin-v1"
	LegacyArgvV1     GeneratedToolProtocol = "legacy-argv-v1"
)

// GeneratedToolIdentity is host-owned registration data, never model input.
type GeneratedToolIdentity struct {
	Name       string
	BinaryPath string
	BinaryHash string
	Protocol   GeneratedToolProtocol
	Workspace  string
}

type GeneratedToolRequest struct {
	ScopeID         string
	CallID          string
	AuthorizationID string
	Action          MangleAtom
	Target          string
	CanonicalArgs   string
	Tool            GeneratedToolIdentity
}

func CanonicalGeneratedArgs(args map[string]any) (string, error) {
	if args == nil {
		args = map[string]any{}
	}
	encoded, err := json.Marshal(args)
	return string(encoded), err
}

func (r GeneratedToolRequest) Args() (map[string]any, error) {
	var args map[string]any
	decoder := json.NewDecoder(strings.NewReader(r.CanonicalArgs))
	decoder.UseNumber()
	if err := decoder.Decode(&args); err != nil {
		return nil, err
	}
	if args == nil {
		return nil, fmt.Errorf("generated arguments must be an object")
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return nil, fmt.Errorf("trailing generated arguments")
	}
	canonical, err := CanonicalGeneratedArgs(args)
	if err != nil || canonical != r.CanonicalArgs {
		return nil, fmt.Errorf("generated arguments are not canonical")
	}
	return args, nil
}

func (r GeneratedToolRequest) Validate() error {
	name := strings.TrimPrefix(r.Tool.Name, "/")
	if name == "" {
		return fmt.Errorf("empty generated action name")
	}
	for _, letter := range name {
		if !((letter >= 'a' && letter <= 'z') || (letter >= 'A' && letter <= 'Z') || (letter >= '0' && letter <= '9') || letter == '_') {
			return fmt.Errorf("invalid generated action name")
		}
	}
	if r.ScopeID == "" || r.CallID == "" || r.AuthorizationID != "exec-"+r.CallID ||
		string(r.Action) != "/"+strings.TrimPrefix(r.Tool.Name, "/") || r.Target == "" {
		return fmt.Errorf("invalid generated authorization identity")
	}
	if _, err := r.Args(); err != nil {
		return err
	}
	if len(r.CanonicalArgs) > 100*1024 {
		return fmt.Errorf("generated payload exceeds kernel action bound")
	}
	if r.Tool.BinaryPath == "" || len(r.Tool.BinaryHash) != sha256.Size*2 {
		return fmt.Errorf("missing immutable generated binary identity")
	}
	if _, err := hex.DecodeString(r.Tool.BinaryHash); err != nil {
		return fmt.Errorf("invalid binary digest: %w", err)
	}
	switch r.Tool.Protocol {
	case GeneratedStdinV1, LegacyArgvV1:
	default:
		return fmt.Errorf("unknown generated tool protocol %q", r.Tool.Protocol)
	}
	return nil
}

func (r GeneratedToolRequest) Key() string { return r.ScopeID + "/" + r.AuthorizationID }
func (r GeneratedToolRequest) Fingerprint() string {
	encoded, _ := json.Marshal(r)
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:])
}

type GeneratedLearningAck struct {
	ExecutionID string
	Fingerprint string
	Path        string
	CommittedAt time.Time
	Durable     bool
	Replayed    bool
}

type GeneratedToolReceipt struct {
	Request             GeneratedToolRequest
	Attempted           bool
	Replayed            bool
	SharedCallPending   bool
	ProcessStarted      bool
	ExitCode            int
	StartedAt           time.Time
	Duration            time.Duration
	Stdout              string
	Stderr              string
	Output              string
	PartialOutput       bool
	OutputTruncated     bool
	BackendError        error
	ValidationError     error
	FeedbackError       error
	ValidationCompleted bool
	ValidationPassed    bool
	Feedback            GeneratedLearningAck
}

func (r GeneratedToolReceipt) Err() error {
	return errors.Join(r.BackendError, r.ValidationError, r.FeedbackError)
}

type GeneratedExecutionError struct{ Receipt GeneratedToolReceipt }

func (e *GeneratedExecutionError) Error() string {
	return fmt.Sprintf("generated execution %s: %v", e.Receipt.Request.CallID, e.Receipt.Err())
}
func (e *GeneratedExecutionError) Unwrap() error { return e.Receipt.Err() }

type GeneratedToolValidator func(context.Context, GeneratedToolRequest, GeneratedToolReceipt) error
type GeneratedToolExecutor interface {
	ExecuteGeneratedToolCall(context.Context, GeneratedToolRequest) (GeneratedToolReceipt, error)
}
type GeneratedToolBackend interface {
	ExecuteGenerated(context.Context, GeneratedToolRequest, GeneratedToolValidator) (GeneratedToolReceipt, error)
}
type GeneratedIdentityProvider interface {
	GeneratedToolIdentity(string) (GeneratedToolIdentity, error)
}
type GeneratedFeedbackRetrier interface {
	RetryGeneratedFeedback(context.Context, GeneratedToolRequest) (GeneratedToolReceipt, error)
}

type generatedCallScopeKey struct{}

func WithGeneratedCallScope(ctx context.Context, scope string) context.Context {
	return context.WithValue(ctx, generatedCallScopeKey{}, scope)
}
func GeneratedCallScope(ctx context.Context) string {
	scope, _ := ctx.Value(generatedCallScopeKey{}).(string)
	return scope
}
