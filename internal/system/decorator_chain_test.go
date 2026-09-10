package system

import (
	"testing"

	"codenerd/internal/broker"
	"codenerd/internal/config"
	"codenerd/internal/core"
	"codenerd/internal/perception"
)

// Boot does not hand anyone a bare client. It builds a stack:
//
//	ScheduledLLMCall -> TracingLLMClient -> broker wrapper -> concrete client
//
// Every layer of that stack is tested. The stack is not, and that is where the
// defect lived: broker.Base and broker.IsBrokered walk the chain by looking for
// an Unwrap() LLMClient method, and for a long time the broker's own wrapper was
// the only thing in the repo that had one. Both outer layers were opaque, so
// both functions stopped at the first layer and reported on it instead of on the
// chain.
//
// Nothing failed. That is the point. broker.Base returned a perfectly good
// LLMClient -- the wrong one -- and every caller of Base does a comma-ok type
// assertion on the result, so the miss reads as "not a Codex client" rather than
// as an error. The codex_cli framework tag simply stopped being set, silently,
// for every session.
//
// The per-layer tests could not catch it because each one builds a chain one
// deep, where Base has nothing to walk past.
func productionChain(t *testing.T) (base *perception.CodexCLIClient, chain perception.LLMClient) {
	t.Helper()

	base = perception.NewCodexCLIClient(&config.CodexCLIConfig{})
	metered, err := perception.InstallBrokerForProvider(base, perception.ProviderAnthropic, "test-model", "")
	if err != nil {
		t.Fatalf("InstallBrokerForProvider: %v", err)
	}
	// The exact order internal/system/factory.go builds: metering innermost,
	// then tracing when a local store opened, then scheduling.
	traced := perception.NewTracingLLMClient(metered, nil)
	return base, core.NewScheduledLLMCall("main", traced)
}

// The concrete engine type has to stay reachable through the whole stack, or
// engine-specific prompt selection silently stops happening.
func TestBaseReachesThroughTheProductionChain(t *testing.T) {
	base, chain := productionChain(t)

	got := broker.Base(chain)
	if got == nil {
		t.Fatal("broker.Base returned nil for a live chain")
	}
	codex, ok := got.(*perception.CodexCLIClient)
	if !ok {
		t.Fatalf("broker.Base stopped at %T instead of reaching *perception.CodexCLIClient.\n"+
			"Every decorator in the chain must implement Unwrap() LLMClient. Without it the "+
			"codex_cli framework tag never reaches JIT atom selection, and the miss is invisible "+
			"because the call site is a comma-ok assertion.", got)
	}
	if codex != base {
		t.Error("broker.Base reached a *CodexCLIClient that is not the one the chain was built on")
	}
}

// And metering has to stay visible through it, or the guard against double
// counting is looking at the wrong object.
//
// InstallBroker returns a client untouched when IsBrokered says it is already
// metered. An opaque decorator makes that answer false for a client that is in
// fact metered, so the guard stops guarding: wrapping an already-metered chain
// would install a second wrapper and charge one request to the ledger twice.
func TestIsBrokeredSeesThroughTheProductionChain(t *testing.T) {
	_, chain := productionChain(t)

	if !broker.IsBrokered(chain) {
		t.Fatal("broker.IsBrokered reports a metered chain as un-metered.\n" +
			"InstallBroker's idempotency guard reads this, so it would wrap an " +
			"already-metered client a second time and double-count its spend.")
	}

	// The guard is what actually has to hold, so assert on it rather than only
	// on the predicate behind it.
	again, err := perception.InstallBroker(chain, &perception.ProviderConfig{
		Provider: perception.ProviderAnthropic,
		Model:    "test-model",
	})
	if err != nil {
		t.Fatalf("InstallBroker on an already-metered chain: %v", err)
	}
	if again != chain {
		t.Errorf("InstallBroker wrapped an already-metered chain again (%T); "+
			"its spend would now be counted twice", again)
	}
}
