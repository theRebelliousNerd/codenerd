// Package xaioauth implements a Hermes-style SuperGrok / X Premium+ OAuth LLM
// client for codeNERD.
//
// This package is intentionally independent of perception.XAIClient (API-key
// path). Subscription OAuth credentials are loaded from codeNERD's own store
// (~/.nerd/xai_oauth.json) and/or imported from the official Grok CLI
// (~/.grok/auth.json).
//
// Architecture (modular files):
//
//	config.go            defaults, engine constants
//	errors.go            typed auth/rate/tier errors
//	store.go             codeNERD credential persistence
//	grok_auth_import.go  read-only ~/.grok/auth.json import
//	auth_device.go       OAuth 2.0 device-code login
//	token.go             EnsureValid + refresh_token grant
//	transport.go         Bearer HTTP helpers
//	client.go            Client + constructors
//	chat.go              Complete / CompleteWithSystem
//	streaming.go         CompleteWithStreaming
//	tools.go             CompleteWithTools
//	probe.go             health probe for nerd auth status
//
// Protocol notes (validated 2026-07-12):
//
//   - OIDC issuer: https://auth.x.ai
//   - Device code: POST /oauth2/device/code
//   - Token: POST /oauth2/token (grant_types: refresh_token, device_code)
//   - Grok CLI client_id: b1a00492-073a-47ea-816f-4c329264a828
//   - Grok CLI stores access token in field "key" (not "access_token")
//   - Scopes include api:access and offline_access
//   - Inference: Authorization Bearer against https://api.x.ai/v1/chat/completions
//   - Default model: grok-4.5 (Grok Build flagship; also listed under SuperGrok OAuth /v1/models)
//
// codeNERD remains the executive; this client is a model backend only.
package xaioauth
