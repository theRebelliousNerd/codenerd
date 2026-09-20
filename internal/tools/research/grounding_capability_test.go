package research

import (
	"context"
	"testing"

	"codenerd/internal/types"
)

// plainClient controls no grounding. It is what every non-Gemini provider is.
type plainClient struct{}

func (plainClient) Complete(context.Context, string) (string, error) { return "", nil }
func (plainClient) CompleteWithSystem(context.Context, string, string) (string, error) {
	return "", nil
}
func (plainClient) CompleteWithTools(context.Context, string, string, []types.ToolDefinition) (*types.LLMToolResponse, error) {
	return nil, nil
}
func (plainClient) CompleteWithStreaming(context.Context, string, string, bool) (<-chan string, <-chan error) {
	return nil, nil
}
func (plainClient) GetModel() string { return "plain" }
func (plainClient) SetModel(string)  {}

// groundingClient controls grounding, as the Gemini client does.
type groundingClient struct {
	plainClient
	search bool
	urls   []string
}

func (g *groundingClient) GetLastGroundingSources() []string { return nil }
func (g *groundingClient) IsGoogleSearchEnabled() bool       { return g.search }
func (g *groundingClient) IsURLContextEnabled() bool         { return len(g.urls) > 0 }
func (g *groundingClient) SetEnableGoogleSearch(v bool)      { g.search = v }
func (g *groundingClient) SetEnableURLContext(bool)          {}
func (g *groundingClient) SetURLContextURLs(u []string)      { g.urls = u }

// meteredPlain is what a plain client looks like once the broker has wrapped
// it: the setters forward (harmlessly, to nothing), and the wrapper answers
// truthfully about what the client underneath can actually do.
type meteredPlain struct {
	plainClient
	grounds bool
}

func (m meteredPlain) GetLastGroundingSources() []string { return nil }
func (m meteredPlain) IsGoogleSearchEnabled() bool       { return false }
func (m meteredPlain) IsURLContextEnabled() bool         { return false }
func (m meteredPlain) SetEnableGoogleSearch(bool)        {}
func (m meteredPlain) SetEnableURLContext(bool)          {}
func (m meteredPlain) SetURLContextURLs([]string)        {}
func (m meteredPlain) SupportsGrounding() bool           { return m.grounds }

// Every one of the 11 nerd fix runs on disk logged
//
//	[INFO] [autopoiesis] Gemini grounding enabled for autopoiesis (Google Search active)
//	[DEBUG] Gemini grounding: Google Search enabled
//
// on a config whose provider is meta and whose model is
// muse-spark-1.3-contributor -- a client that implements none of the grounding
// control methods. Both lines sit inside a branch entered only when the helper
// reports the client can control grounding, and fourteen production call sites
// branch on that same report: they build documentation URL lists, enable URL
// context, and call CompleteWithGrounding instead of the client's own Complete.
//
// The helper decided by type assertion, and the broker implements the grounding
// interface unconditionally for every client it meters -- deliberately, because
// forwarding a setter to a client that ignores it is indistinguishable from not
// calling it (broker/passthrough.go). That argument is sound for the setters and
// silent about the capability question, which is not a setter: it selects a
// control flow, and broker/optional.go already says those must stay conditional.
//
// So the wrapper keeps forwarding and starts answering. A client that reports
// what it supports is believed over the shape of its method set.
func TestNewGroundingHelper_BelievesAReportedCapabilityOverTheMethodSet(t *testing.T) {
	t.Run("aMeteredPlainClientIsNotGrounding", func(t *testing.T) {
		h := NewGroundingHelper(meteredPlain{grounds: false})
		if h.IsGroundingAvailable() {
			t.Fatal("a metered client whose provider controls no grounding reports that it does; fourteen call sites take the grounded path on that report")
		}
		if h.IsGemini() {
			t.Fatal("IsGemini is true for a client that controls no grounding")
		}
	})

	t.Run("aMeteredGeminiIsStillGrounding", func(t *testing.T) {
		h := NewGroundingHelper(meteredPlain{grounds: true})
		if !h.IsGroundingAvailable() {
			t.Fatal("a metered client whose provider DOES control grounding reports that it does not; grounding would be lost for Gemini")
		}
	})

	t.Run("anUnwrappedGroundingClientStillWorks", func(t *testing.T) {
		// No capability report: the method set is the only evidence there is,
		// and it is conclusive for a client nothing has wrapped.
		h := NewGroundingHelper(&groundingClient{})
		if !h.IsGroundingAvailable() {
			t.Fatal("an unwrapped client that implements the control methods is not recognised")
		}
	})

	t.Run("anUnwrappedPlainClientIsNotGrounding", func(t *testing.T) {
		h := NewGroundingHelper(plainClient{})
		if h.IsGroundingAvailable() {
			t.Fatal("a bare client with no grounding methods reports grounding")
		}
	})
}
