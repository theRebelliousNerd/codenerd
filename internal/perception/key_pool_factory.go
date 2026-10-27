package perception

import (
	"fmt"
	"net/http"
	"time"

	"codenerd/internal/broker"
	"codenerd/internal/config"
)

func newPooledClient(cfg *ProviderConfig, classification bool) (LLMClient, error) {
	if cfg.Engine != "" && cfg.Engine != "api" {
		return nil, fmt.Errorf("credential pools require an API engine")
	}
	if len(cfg.APIKeys) == 0 {
		return nil, fmt.Errorf("credential pool is empty")
	}
	if cfg.Provider == ProviderOllama {
		return nil, fmt.Errorf("Ollama does not use an API key pool")
	}
	validation := &config.UserConfig{APIKeyPools: map[string][]config.ProviderAPIKey{string(cfg.Provider): {}}}
	for _, key := range cfg.APIKeys {
		validation.APIKeyPools[string(cfg.Provider)] = append(validation.APIKeyPools[string(cfg.Provider)], config.ProviderAPIKey{Name: key.Name, APIKey: key.Key, WorkspaceID: key.WorkspaceID})
	}
	if _, err := validation.ResolveAPIKeyPool(string(cfg.Provider)); err != nil {
		return nil, err
	}
	pool, err := sharedKeyPool(cfg.Provider, cfg.APIKeys)
	if err != nil {
		return nil, err
	}
	router := &providerKeyRouter{provider: cfg.Provider, pool: pool, pinned: -1, timeout: 10 * time.Minute}
	for _, key := range cfg.APIKeys {
		leaf := *cfg
		leaf.APIKeys = nil
		leaf.APIKey = key.Key
		leaf.CredentialAlias = key.Name
		if cfg.Provider == ProviderAnthropic {
			options := config.AnthropicProviderConfig{}
			if cfg.Anthropic != nil {
				options = *cfg.Anthropic
			}
			if key.WorkspaceID != "" {
				options.WorkspaceID = key.WorkspaceID
			}
			leaf.Anthropic = &options
		}
		var raw LLMClient
		if classification {
			raw, err = newRawClassificationClientFromConfig(&leaf)
		} else {
			raw, err = newRawClientFromConfig(&leaf)
		}
		if err != nil {
			return nil, err
		}
		if raw == nil {
			return nil, nil
		}
		if !observePoolHTTP(raw) {
			return nil, fmt.Errorf("provider %q has no supported API-key pool transport", cfg.Provider)
		}
		if classification {
			leaf.Model = classificationModel(cfg)
		}
		wrapped, err := InstallBroker(raw, &leaf)
		if err != nil {
			return nil, err
		}
		router.clients = append(router.clients, wrapped)
		router.aliases = append(router.aliases, key.Name)
	}
	return broker.WrapRouter(router.clients[0], router)
}

func poolCounterHTTPClient(key string) *http.Client {
	return &http.Client{Transport: poolObservedTransport{base: sharedTransport, secret: key}, Timeout: 5 * time.Second}
}
