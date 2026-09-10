package perception

import (
	"strings"

	"codenerd/internal/broker"
	"codenerd/internal/logging"
)

// InstallBroker puts metering in front of an LLM client.
//
// Every exported constructor in this package routes through here, which is what
// makes "no un-metered inference call" a structural property rather than a
// convention somebody has to remember. Callers did not change: the returned
// value is still a plain LLMClient with the same optional-interface surface as
// the client it wraps.
//
// Idempotent. A client that already carries metering anywhere in its decorator
// chain is returned untouched, so a nested construction path cannot charge one
// request to the ledger twice.
func InstallBroker(client LLMClient, cfg *ProviderConfig) (LLMClient, error) {
	if client == nil {
		return nil, nil
	}
	if broker.IsBrokered(client) {
		return client, nil
	}

	creds := brokerCredsFor(cfg)
	wrapped, err := broker.Wrap(client, broker.Default().ConfigFor(creds))
	if err != nil {
		// Fail closed. Wrap only rejects a nil client or a missing counter,
		// both of which are wiring errors in this file rather than anything a
		// user can cause. Returning the bare client instead would silently
		// restore un-metered inference, which is the exact condition the broker
		// exists to end — so this surfaces as a construction failure.
		return nil, err
	}

	logging.Perception("broker: metering installed provider=%s model=%s counter=%T",
		creds.Provider, creds.Model, broker.Default().CounterFor(creds))
	return wrapped, nil
}

// brokerCredsFor derives counting credentials from a provider config.
//
// Only Anthropic currently exposes a token-counting endpoint, so only Anthropic
// needs a base URL and key here; every other provider gets the calibrating
// estimator, which needs neither.
func brokerCredsFor(cfg *ProviderConfig) broker.ProviderCreds {
	if cfg == nil {
		return broker.ProviderCreds{}
	}

	creds := broker.ProviderCreds{
		Provider: strings.ToLower(string(cfg.Provider)),
		Model:    cfg.Model,
	}

	// A CLI-backed engine is not the API provider even when an API key is
	// present for fallback: claude-cli shells out and never reaches
	// api.anthropic.com, so pointing the counter there would count a request
	// that is not the one being made.
	if cfg.Engine != "" && cfg.Engine != "api" {
		creds.Provider = cfg.Engine
		return creds
	}

	if creds.Provider == string(ProviderAnthropic) {
		creds.APIKey = cfg.APIKey
		creds.BaseURL = cfg.BaseURL
		if creds.BaseURL == "" {
			creds.BaseURL = DefaultAnthropicConfig(cfg.APIKey).BaseURL
		}
	}
	return creds
}

// InstallBrokerForProvider meters a client built outside the factory.
//
// One such path exists (a direct ZAI construction in internal/system), and the
// wiring test forbids new ones from appearing without going through a metered
// constructor.
func InstallBrokerForProvider(client LLMClient, provider Provider, model, apiKey string) (LLMClient, error) {
	return InstallBroker(client, &ProviderConfig{Provider: provider, Model: model, APIKey: apiKey})
}
