package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"codenerd/internal/logging"
)

// =============================================================================
// OLLAMA EMBEDDING ENGINE
// =============================================================================

// Preferred default when config says bare "embeddinggemma" — Ollama does not
// always publish an untagged :latest for this model; :300m is the common tag.
const defaultOllamaEmbedModel = "embeddinggemma:300m"

// OllamaEngine generates embeddings using local Ollama server.
// Supports embeddinggemma and other embedding models.
//
// Missing models are handled automatically:
//  1. Resolve bare names (embeddinggemma → embeddinggemma:300m if present)
//  2. Prefer any already-installed embedding-capable match
//  3. POST /api/pull for the resolved name (once per process/engine)
//  4. On embed 404 "not found", ensure+retry once
type OllamaEngine struct {
	endpoint string
	model    string
	client   *http.Client

	ensureMu     sync.Mutex
	modelReady   bool
	pullAttempted bool
}

// NewOllamaEngine creates a new Ollama embedding engine.
func NewOllamaEngine(endpoint, model string) (*OllamaEngine, error) {
	timer := logging.StartTimer(logging.CategoryEmbedding, "NewOllamaEngine")
	defer timer.Stop()

	if endpoint == "" {
		endpoint = "http://localhost:11434"
		logging.EmbeddingDebug("Ollama endpoint defaulted to: %s", endpoint)
	}
	if model == "" || model == "embeddinggemma" {
		// Bare "embeddinggemma" is a common config value but Ollama often only
		// ships tagged variants (e.g. embeddinggemma:300m). Prefer the tagged
		// default; EnsureModel will still remap to whatever is installed.
		model = defaultOllamaEmbedModel
		logging.EmbeddingDebug("Ollama model defaulted to: %s", model)
	}

	logging.Embedding("Creating Ollama engine: endpoint=%s, model=%s, timeout=60s (auto-pull enabled)", endpoint, model)

	engine := &OllamaEngine{
		endpoint: strings.TrimRight(endpoint, "/"),
		model:    model,
		client: &http.Client{
			// 60 seconds allows for:
			// - Ollama cold starts (model loading)
			// - High load scenarios (multiple concurrent requests)
			// - Larger text embeddings
			// Individual operations like JIT compilation use their own sub-deadlines.
			// Model pulls use a separate longer-timeout client in pullModel.
			Timeout: 60 * time.Second,
		},
	}

	logging.Embedding("Ollama engine created successfully")
	return engine, nil
}

// Embed generates an embedding for a single text.
func (e *OllamaEngine) Embed(ctx context.Context, text string) ([]float32, error) {
	timer := logging.StartTimer(logging.CategoryEmbedding, "Ollama.Embed")

	// Best-effort: make sure the model is installed before first use.
	// Non-fatal if Ollama is briefly unreachable — the request loop still runs.
	if err := e.EnsureModel(ctx); err != nil {
		logging.EmbeddingDebug("Ollama.Embed: EnsureModel preflight: %v (will still attempt embed)", err)
	}

	textLen := len(text)
	logging.EmbeddingDebug("Ollama.Embed: starting embed request, text_length=%d chars model=%s", textLen, e.model)

	// Retry transient Ollama runner/network failures.
	const maxRetries = 3
	backoff := 300 * time.Millisecond
	pulledOn404 := false

	for attempt := 1; attempt <= maxRetries; attempt++ {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		req := ollamaEmbedRequest{
			Model:  e.model,
			Prompt: text,
		}

		body, err := json.Marshal(req)
		if err != nil {
			logging.Get(logging.CategoryEmbedding).Error("Ollama.Embed: failed to marshal request: %v", err)
			return nil, fmt.Errorf("failed to marshal request: %w", err)
		}

		logging.EmbeddingDebug("Ollama.Embed: sending POST to %s/api/embeddings (attempt %d/%d model=%s)", e.endpoint, attempt, maxRetries, e.model)
		apiStart := time.Now()

		httpReq, err := http.NewRequestWithContext(ctx, "POST", e.endpoint+"/api/embeddings", bytes.NewReader(body))
		if err != nil {
			logging.Get(logging.CategoryEmbedding).Error("Ollama.Embed: failed to create HTTP request: %v", err)
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
		httpReq.Header.Set("Content-Type", "application/json")

		resp, err := e.client.Do(httpReq)
		apiLatency := time.Since(apiStart)

		if err != nil {
			if attempt < maxRetries && ctx.Err() == nil {
				logging.Get(logging.CategoryEmbedding).Warn("Ollama.Embed: request failed after %v (attempt %d/%d): %v; retrying in %v",
					apiLatency, attempt, maxRetries, err, backoff)
				time.Sleep(backoff)
				backoff *= 2
				continue
			}
			logging.Get(logging.CategoryEmbedding).Error("Ollama.Embed: request failed after %v: %v", apiLatency, err)
			return nil, fmt.Errorf("ollama request failed: %w", err)
		}

		respBytes, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if readErr != nil {
			if attempt < maxRetries && ctx.Err() == nil {
				logging.Get(logging.CategoryEmbedding).Warn("Ollama.Embed: failed to read response (attempt %d/%d): %v; retrying in %v",
					attempt, maxRetries, readErr, backoff)
				time.Sleep(backoff)
				backoff *= 2
				continue
			}
			return nil, fmt.Errorf("failed to read response: %w", readErr)
		}

		if resp.StatusCode != http.StatusOK {
			bodyStr := string(respBytes)

			// Model missing → auto-pull once, then retry the embed.
			if isModelNotFoundStatus(resp.StatusCode, bodyStr) && !pulledOn404 && ctx.Err() == nil {
				pulledOn404 = true
				e.modelReady = false // force re-ensure
				logging.Get(logging.CategoryEmbedding).Warn(
					"Ollama.Embed: model %q not found (%s); auto-pulling / resolving...", e.model, truncateForLog(bodyStr, 160))
				if ensureErr := e.EnsureModel(ctx); ensureErr != nil {
					logging.Get(logging.CategoryEmbedding).Error("Ollama.Embed: auto-pull failed: %v", ensureErr)
					return nil, fmt.Errorf("ollama model %q missing and auto-pull failed: %w", e.model, ensureErr)
				}
				// Reset attempt budget after a successful ensure.
				attempt = 0
				backoff = 300 * time.Millisecond
				continue
			}

			retryable := resp.StatusCode >= 500 && resp.StatusCode <= 599
			if !retryable && strings.Contains(bodyStr, "connection was forcibly closed") {
				retryable = true
			}

			if retryable && attempt < maxRetries && ctx.Err() == nil {
				logging.Get(logging.CategoryEmbedding).Warn("Ollama.Embed: non-OK status %d (attempt %d/%d): %s; retrying in %v",
					resp.StatusCode, attempt, maxRetries, bodyStr, backoff)
				time.Sleep(backoff)
				backoff *= 2
				continue
			}

			logging.Get(logging.CategoryEmbedding).Error("Ollama.Embed: non-OK status %d: %s", resp.StatusCode, bodyStr)
			return nil, fmt.Errorf("ollama returned status %d: %s", resp.StatusCode, bodyStr)
		}

		var result ollamaEmbedResponse
		if err := json.Unmarshal(respBytes, &result); err != nil {
			if attempt < maxRetries && ctx.Err() == nil {
				logging.Get(logging.CategoryEmbedding).Warn("Ollama.Embed: decode failed (attempt %d/%d): %v; retrying in %v",
					attempt, maxRetries, err, backoff)
				time.Sleep(backoff)
				backoff *= 2
				continue
			}
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}

		timer.Stop()
		logging.Embedding("Ollama.Embed: completed successfully, dimensions=%d, api_latency=%v, model=%s", len(result.Embedding), apiLatency, e.model)

		return result.Embedding, nil
	}

	return nil, fmt.Errorf("ollama embed failed after %d attempts", maxRetries)
}

// EmbedBatch generates embeddings for multiple texts.
// Ollama doesn't have native batch API, so we call Embed sequentially.
func (e *OllamaEngine) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	timer := logging.StartTimer(logging.CategoryEmbedding, "Ollama.EmbedBatch")
	defer timer.Stop()

	logging.Embedding("Ollama.EmbedBatch: starting batch embed for %d texts", len(texts))

	if len(texts) == 0 {
		logging.EmbeddingDebug("Ollama.EmbedBatch: empty input, returning nil")
		return nil, nil
	}

	// One ensure for the whole batch.
	if err := e.EnsureModel(ctx); err != nil {
		logging.EmbeddingDebug("Ollama.EmbedBatch: EnsureModel preflight: %v", err)
	}

	embeddings := make([][]float32, len(texts))

	for i, text := range texts {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("ollama EmbedBatch cancelled at text %d/%d: %w", i, len(texts), err)
		}
		logging.EmbeddingDebug("Ollama.EmbedBatch: processing text %d/%d (length=%d chars)", i+1, len(texts), len(text))

		embedding, err := e.Embed(ctx, text)
		if err != nil {
			logging.Get(logging.CategoryEmbedding).Error("Ollama.EmbedBatch: failed at text %d: %v", i, err)
			return nil, fmt.Errorf("failed to embed text %d: %w", i, err)
		}
		embeddings[i] = embedding
	}

	logging.Embedding("Ollama.EmbedBatch: completed successfully, processed %d texts", len(texts))
	return embeddings, nil
}

// Dimensions returns the dimensionality of embeddings.
// embeddinggemma / nomic-embed-text produce 768-dimensional vectors.
func (e *OllamaEngine) Dimensions() int {
	return 768
}

// Name returns the engine name.
func (e *OllamaEngine) Name() string {
	return fmt.Sprintf("ollama:%s", e.model)
}

// Model returns the currently selected model name (may change after ensure/pull).
func (e *OllamaEngine) Model() string {
	e.ensureMu.Lock()
	defer e.ensureMu.Unlock()
	return e.model
}

// HealthCheck verifies that the Ollama service is reachable.
// This should be called before batch embedding operations to fail fast
// instead of blocking for minutes with retries.
func (e *OllamaEngine) HealthCheck(ctx context.Context) error {
	timer := logging.StartTimer(logging.CategoryEmbedding, "Ollama.HealthCheck")
	defer timer.Stop()

	// Create a short timeout context for the health check
	checkCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	logging.EmbeddingDebug("Ollama.HealthCheck: checking endpoint %s/api/tags", e.endpoint)

	req, err := http.NewRequestWithContext(checkCtx, "GET", e.endpoint+"/api/tags", nil)
	if err != nil {
		return fmt.Errorf("failed to create health check request: %w", err)
	}

	resp, err := e.client.Do(req)
	if err != nil {
		logging.Get(logging.CategoryEmbedding).Warn("Ollama.HealthCheck: endpoint unreachable: %v", err)
		return fmt.Errorf("ollama unavailable at %s: %w", e.endpoint, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logging.Get(logging.CategoryEmbedding).Warn("Ollama.HealthCheck: endpoint returned status %d", resp.StatusCode)
		return fmt.Errorf("ollama returned status %d", resp.StatusCode)
	}

	logging.Embedding("Ollama.HealthCheck: endpoint healthy")
	return nil
}

// EnsureModel makes sure the configured embedding model is available locally.
// Safe to call repeatedly; only pulls once per engine when needed.
func (e *OllamaEngine) EnsureModel(ctx context.Context) error {
	e.ensureMu.Lock()
	defer e.ensureMu.Unlock()

	if e.modelReady {
		return nil
	}

	installed, err := e.listModels(ctx)
	if err != nil {
		return err
	}

	// 1) Exact / alias match against what's already installed.
	if resolved := resolveInstalledModel(e.model, installed); resolved != "" {
		if resolved != e.model {
			logging.Embedding("Ollama: resolved model %q → installed %q", e.model, resolved)
			e.model = resolved
		}
		e.modelReady = true
		return nil
	}

	// 2) Prefer any installed embedding-capable model matching the family.
	if alt := preferInstalledEmbeddingModel(e.model, installed); alt != "" {
		logging.Embedding("Ollama: using installed embedding model %q (configured %q not present)", alt, e.model)
		e.model = alt
		e.modelReady = true
		return nil
	}

	// 3) Auto-pull the best candidate name.
	pullName := pullTargetFor(e.model)
	if e.pullAttempted {
		return fmt.Errorf("model %q still unavailable after prior pull attempt", pullName)
	}
	e.pullAttempted = true

	logging.Embedding("Ollama: model %q not installed — pulling %q (this may take a few minutes)...", e.model, pullName)
	if err := e.pullModel(ctx, pullName); err != nil {
		// Last-resort fallback ONLY for known embed families — never remap
		// arbitrary/custom model names (would break tests and user configs).
		fallback := "nomic-embed-text"
		base := modelBase(e.model)
		_, isKnownFamily := knownEmbedFamilies[base]
		if isKnownFamily && pullName != fallback && pullName != fallback+":latest" {
			logging.Get(logging.CategoryEmbedding).Warn("Ollama: pull of %q failed (%v); trying fallback %q", pullName, err, fallback)
			if ferr := e.pullModel(ctx, fallback); ferr == nil {
				e.model = fallback
				e.modelReady = true
				logging.Embedding("Ollama: fallback model %q ready", fallback)
				return nil
			}
		}
		return fmt.Errorf("auto-pull %q failed: %w", pullName, err)
	}

	e.model = pullName
	e.modelReady = true
	logging.Embedding("Ollama: model %q pulled and ready", e.model)
	return nil
}

func (e *OllamaEngine) listModels(ctx context.Context) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", e.endpoint+"/api/tags", nil)
	if err != nil {
		return nil, fmt.Errorf("list models: %w", err)
	}
	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("list models: ollama unreachable at %s: %w", e.endpoint, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("list models: read: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("list models: status %d: %s", resp.StatusCode, truncateForLog(string(body), 200))
	}
	var tags ollamaTagsResponse
	if err := json.Unmarshal(body, &tags); err != nil {
		return nil, fmt.Errorf("list models: decode: %w", err)
	}
	names := make([]string, 0, len(tags.Models))
	for _, m := range tags.Models {
		if m.Name != "" {
			names = append(names, m.Name)
		} else if m.Model != "" {
			names = append(names, m.Model)
		}
	}
	return names, nil
}

func (e *OllamaEngine) pullModel(ctx context.Context, name string) error {
	payload, err := json.Marshal(ollamaPullRequest{Name: name, Stream: false})
	if err != nil {
		return err
	}

	// Pulls can take a long time (hundreds of MB). Use a dedicated client.
	pullClient := &http.Client{Timeout: 30 * time.Minute}
	req, err := http.NewRequestWithContext(ctx, "POST", e.endpoint+"/api/pull", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	start := time.Now()
	resp, err := pullClient.Do(req)
	if err != nil {
		return fmt.Errorf("pull request failed: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("pull status %d: %s", resp.StatusCode, truncateForLog(string(body), 300))
	}

	// Non-stream pull returns a final status object; stream=false still may
	// return newline-delimited JSON on some versions — accept either.
	if len(body) > 0 {
		var st ollamaPullStatus
		if err := json.Unmarshal(body, &st); err == nil && st.Error != "" {
			return fmt.Errorf("pull error: %s", st.Error)
		}
		// If NDJSON, scan last non-empty line for error/status.
		lines := strings.Split(string(body), "\n")
		for i := len(lines) - 1; i >= 0; i-- {
			line := strings.TrimSpace(lines[i])
			if line == "" {
				continue
			}
			var lineSt ollamaPullStatus
			if json.Unmarshal([]byte(line), &lineSt) == nil {
				if lineSt.Error != "" {
					return fmt.Errorf("pull error: %s", lineSt.Error)
				}
				break
			}
		}
	}

	logging.Embedding("Ollama: pull of %q finished in %v", name, time.Since(start).Round(time.Millisecond))
	return nil
}

// =============================================================================
// MODEL RESOLUTION HELPERS
// =============================================================================

func isModelNotFoundStatus(code int, body string) bool {
	lower := strings.ToLower(body)
	looksMissing := strings.Contains(lower, "not found") ||
		strings.Contains(lower, "try pulling") ||
		strings.Contains(lower, "pull it") ||
		strings.Contains(lower, "pulling it first")
	// Ollama typically returns 404; some builds use 400 with the same body.
	if code == http.StatusNotFound || code == http.StatusBadRequest {
		return looksMissing || code == http.StatusNotFound
	}
	// Any status whose body clearly says "not found, try pulling".
	return looksMissing && strings.Contains(lower, "pull")
}

// resolveInstalledModel returns an installed tag that satisfies want, or "".
func resolveInstalledModel(want string, installed []string) string {
	want = strings.TrimSpace(want)
	if want == "" {
		return ""
	}
	// Exact match
	for _, name := range installed {
		if name == want {
			return name
		}
	}
	// want without tag matches name base (embeddinggemma → embeddinggemma:300m)
	wantBase := modelBase(want)
	var candidates []string
	for _, name := range installed {
		if modelBase(name) == wantBase {
			candidates = append(candidates, name)
		}
	}
	if len(candidates) == 0 {
		// Also try if want is bare and installed has :latest
		for _, name := range installed {
			if name == want+":latest" {
				return name
			}
		}
		return ""
	}
	// Prefer :300m for embeddinggemma, else :latest, else first.
	for _, c := range candidates {
		if strings.HasSuffix(c, ":300m") {
			return c
		}
	}
	for _, c := range candidates {
		if strings.HasSuffix(c, ":latest") {
			return c
		}
	}
	return candidates[0]
}

// knownEmbedFamilies are the only bases we will auto-remap / auto-prefer when
// the configured model is missing. Arbitrary names (e.g. test-model in unit
// tests, or a user typo of a custom fine-tune) are NOT remapped — we try to
// pull the configured name instead.
var knownEmbedFamilies = map[string]struct{}{
	"embeddinggemma":    {},
	"nomic-embed-text":  {},
	"mxbai-embed-large": {},
	"all-minilm":        {},
	"bge-m3":            {},
	"bge-large":         {},
	"snowflake-arctic-embed": {},
}

// preferInstalledEmbeddingModel picks a sensible installed embed model when
// the configured one is absent. Only applies to known embedding families.
func preferInstalledEmbeddingModel(configured string, installed []string) string {
	cfgBase := modelBase(configured)
	if cfgBase != "" {
		if _, ok := knownEmbedFamilies[cfgBase]; !ok {
			return ""
		}
	}
	families := []string{"embeddinggemma", "nomic-embed-text", "mxbai-embed-large", "all-minilm", "bge-m3"}
	ordered := make([]string, 0, len(families)+1)
	if cfgBase != "" {
		ordered = append(ordered, cfgBase)
	}
	for _, f := range families {
		if f != cfgBase {
			ordered = append(ordered, f)
		}
	}
	for _, fam := range ordered {
		if hit := resolveInstalledModel(fam, installed); hit != "" {
			return hit
		}
	}
	return ""
}

func pullTargetFor(configured string) string {
	base := modelBase(configured)
	switch base {
	case "embeddinggemma", "":
		return defaultOllamaEmbedModel
	case "nomic-embed-text":
		return "nomic-embed-text"
	default:
		if strings.Contains(configured, ":") {
			return configured
		}
		return configured
	}
}

func modelBase(name string) string {
	if i := strings.IndexByte(name, ':'); i >= 0 {
		return name[:i]
	}
	return name
}

func truncateForLog(s string, max int) string {
	s = strings.TrimSpace(s)
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

// =============================================================================
// OLLAMA API TYPES
// =============================================================================

type ollamaEmbedRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

type ollamaEmbedResponse struct {
	Embedding []float32 `json:"embedding"`
}

type ollamaTagsResponse struct {
	Models []ollamaTagModel `json:"models"`
}

type ollamaTagModel struct {
	Name  string `json:"name"`
	Model string `json:"model"`
}

type ollamaPullRequest struct {
	Name   string `json:"name"`
	Stream bool   `json:"stream"`
}

type ollamaPullStatus struct {
	Status string `json:"status"`
	Error  string `json:"error"`
}
