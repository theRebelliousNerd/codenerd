package broker

import (
	"testing"

	"codenerd/internal/types"
)

// piggybackClient answers the Piggyback question the way the CLI engines and a
// grounded Gemini do.
type piggybackClient struct {
	*fakeClient
	piggyback bool
}

func (p *piggybackClient) ShouldUsePiggybackTools() bool { return p.piggyback }

// Every production client is metered, so whatever the broker answers here is
// what the session executor hears about every client in the process. It used
// to answer nothing: the wrapper did not implement the method, the executor's
// probe failed, and a Codex or Claude CLI engine -- which has no tool channel
// but the envelope -- was sent down the native tool path and failed on its
// first tool round.
//
// Both directions, through Wrap rather than a bare core, because Wrap is what
// every constructor calls and the answer has to survive whichever shape it
// picks.
func TestWrapAnswersThePiggybackQuestionForTheUnderlyingClient(t *testing.T) {
	cfg := testMeter(200000, 8000).ConfigFor(ProviderCreds{Provider: "fake", Model: "fake-model"})

	cases := []struct {
		name       string
		underlying types.LLMClient
		want       bool
	}{
		{"an envelope-only engine", &piggybackClient{fakeClient: newFakeClient(), piggyback: true}, true},
		{"a client that answers no", &piggybackClient{fakeClient: newFakeClient(), piggyback: false}, false},
		// No opinion is the unwrapped behaviour of a native client: the probe
		// fails and the executor takes the native path. Forwarding must land
		// on the same side.
		{"a client with no opinion", newFakeClient(), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			wrapped, err := Wrap(tc.underlying, cfg)
			if err != nil {
				t.Fatalf("Wrap: %v", err)
			}
			ptp, ok := wrapped.(types.PiggybackToolProvider)
			if !ok {
				t.Fatalf("the metered client (%T) does not answer the Piggyback question; "+
					"a CLI engine behind it is routed to a tool channel it does not have", wrapped)
			}
			if got := ptp.ShouldUsePiggybackTools(); got != tc.want {
				t.Fatalf("ShouldUsePiggybackTools() = %v through the broker, the client says %v", got, tc.want)
			}
		})
	}
}
