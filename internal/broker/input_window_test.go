package broker

import (
	"testing"

	"codenerd/internal/types"
)

// cappedFake is a provider client that reports an input cap.
type cappedFake struct {
	*fakeClient
	window int
}

func (c *cappedFake) InputWindowTokens() int { return c.window }

// unwrapping is a decorator layer that reports nothing of its own and is
// walked through, like the scheduler, tracing and session adapters are.
type unwrapping struct {
	*fakeClient
	inner types.LLMClient
}

func (u *unwrapping) Unwrap() types.LLMClient { return u.inner }

// cappedDecorator is a layer that both reports a cap and wraps a client.
type cappedDecorator struct {
	unwrapping
	window int
}

func (c *cappedDecorator) InputWindowTokens() int { return c.window }

// InputWindow walks the whole chain: through the metering wrapper and any
// decorator with Unwrap to the client that reports, keeps the smallest
// positive cap when several layers report, and reads 0 as no cap.
func TestInputWindow(t *testing.T) {
	meter := testMeter(200000, 8000)
	metered := func(t *testing.T, inner types.LLMClient) types.LLMClient {
		t.Helper()
		wrapped, err := Wrap(inner, meter.ConfigFor(ProviderCreds{Provider: "fake"}))
		if err != nil {
			t.Fatalf("Wrap: %v", err)
		}
		return wrapped
	}

	tests := []struct {
		name   string
		client func(t *testing.T) types.LLMClient
		want   int
	}{
		{
			name:   "nil is uncapped",
			client: func(*testing.T) types.LLMClient { return nil },
			want:   0,
		},
		{
			name:   "a client that reports nothing is uncapped",
			client: func(*testing.T) types.LLMClient { return newFakeClient() },
			want:   0,
		},
		{
			name: "a cap is found through metering and a decorator",
			client: func(t *testing.T) types.LLMClient {
				return &unwrapping{fakeClient: newFakeClient(), inner: metered(t, &cappedFake{fakeClient: newFakeClient(), window: 100000})}
			},
			want: 100000,
		},
		{
			name: "a zero report is no cap",
			client: func(t *testing.T) types.LLMClient {
				return metered(t, &cappedFake{fakeClient: newFakeClient()})
			},
			want: 0,
		},
		{
			name: "the smallest positive cap on the chain binds",
			client: func(t *testing.T) types.LLMClient {
				inner := &cappedFake{fakeClient: newFakeClient(), window: 100000}
				return &cappedDecorator{unwrapping: unwrapping{fakeClient: newFakeClient(), inner: inner}, window: 60000}
			},
			want: 60000,
		},
		{
			name: "an outer zero does not hide an inner cap",
			client: func(t *testing.T) types.LLMClient {
				inner := &cappedFake{fakeClient: newFakeClient(), window: 100000}
				return &cappedDecorator{unwrapping: unwrapping{fakeClient: newFakeClient(), inner: inner}}
			},
			want: 100000,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := InputWindow(tt.client(t)); got != tt.want {
				t.Errorf("InputWindow() = %d, want %d", got, tt.want)
			}
		})
	}
}

// A cyclic Unwrap chain ends at Walk's bound instead of spinning.
func TestInputWindowTerminatesOnCyclicUnwrap(t *testing.T) {
	done := make(chan int, 1)
	go func() {
		done <- InputWindow(&selfUnwrapping{fakeClient: newFakeClient()})
	}()
	select {
	case got := <-done:
		if got != 0 {
			t.Errorf("InputWindow(cyclic) = %d, want 0", got)
		}
	case <-timeAfter():
		t.Fatal("InputWindow did not terminate on a cyclic Unwrap chain")
	}
}
