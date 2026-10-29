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
	"unicode/utf8"

	"codenerd/internal/logging"
)

// =============================================================================
// OLLAMA EMBEDDING ENGINE
// =============================================================================

// OllamaEngine generates embeddings using local Ollama server.
// It runs exactly the configured model and never substitutes another: a
// different embedding model returns vectors the index was not built with.
//
// A missing model is handled by:
//  1. Matching the configured name against installed tags (bare = :latest)
//  2. POST /api/pull for the configured name (once per process/engine)
//  3. On embed 404 "not found", ensure+retry once
type OllamaEngine struct {
	endpoint string
	model    string
	// dimensions is the vector length the user configured for this model. It
	// used to be a literal 768 whatever the model was.
	dimensions int
	client     *http.Client

	ensureMu      sync.Mutex
	modelReady    bool
	pullAttempted bool

	// ceiling is the model's input window as learned from its refusals; a
	// longer text is split and pooled (long_input.go).
	ceiling inputCeiling

	// keepAlive is sent with every request (embedding.ollama_keep_alive):
	// how long Ollama keeps the model loaded after it. Ollama's own default
	// unloads it after five idle minutes, and the next embed then reloads it
	// (~2.6 s measured 2026-10-27) in the middle of a turn.
	keepAlive string
}

// NewOllamaEngine creates a new Ollama embedding engine.
func NewOllamaEngine(endpoint, model string, dimensions int) (*OllamaEngine, error) {
	timer := logging.StartTimer(logging.CategoryEmbedding, "NewOllamaEngine")
	defer timer.Stop()

	if endpoint == "" {
		endpoint = "http://localhost:11434"
		logging.EmbeddingDebug("Ollama endpoint defaulted to: %s", endpoint)
	}
	model = strings.TrimSpace(model)
	if model == "" {
		// No invented model. An embedding model that differs from the one the
		// stored vectors were built with returns wrong neighbours silently, so
		// a guess here is worse than a refusal.
		return nil, fmt.Errorf("no Ollama embedding model configured: set embedding.ollama_model in .nerd/config.json")
	}
	if dimensions <= 0 {
		// It sizes the vector index, so a guess is a corrupt index.
		return nil, fmt.Errorf("no embedding dimensions configured for Ollama model %q: set embedding.dimensions in .nerd/config.json to the length of the vectors it returns", model)
	}

	// One embed HTTP call. The bound is embedding.request_timeout, published
	// into this package by config.SetEmbeddingRequestTimeout: this package
	// cannot import config, because config imports embedding. Zero means
	// nothing has been published yet, and the caller's context is then the
	// only bound. A model pull uses its own client in pullModel.
	timeout := EmbedRequestTimeout()
	logging.Embedding("Creating Ollama engine: endpoint=%s, model=%s, timeout=%s (auto-pull enabled)", endpoint, model, timeout)

	engine := &OllamaEngine{
		endpoint:   strings.TrimRight(endpoint, "/"),
		model:      model,
		dimensions: dimensions,
		client: &http.Client{
			Timeout: timeout,
		},
	}

	logging.Embedding("Ollama engine created successfully")
	return engine, nil
}

// Embed generates an embedding for a single text of any length. A text longer
// than the model's input window is split at natural boundaries and its pieces'
// vectors pooled (embedLong), so no caller has to know the window.
func (e *OllamaEngine) Embed(ctx context.Context, text string) ([]float32, error) {
	timer := logging.StartTimer(logging.CategoryEmbedding, "Ollama.Embed")
	defer timer.Stop()

	if err := checkEmbeddable(text); err != nil {
		return nil, err
	}
	return embedLong(ctx, text, &e.ceiling, e.embedOnce)
}

// embedOnce is one embed request. An input the model refuses as too long is
// an *InputTooLongError, which embedLong answers by splitting.
func (e *OllamaEngine) embedOnce(ctx context.Context, text string) ([]float32, error) {

	// Best-effort: make sure the model is installed before first use.
	// Non-fatal if Ollama is briefly unreachable — the request loop still runs.
	if err := e.EnsureModel(ctx); err != nil {
		logging.EmbeddingDebug("Ollama.Embed: EnsureModel preflight: %v (will still attempt embed)", err)
	}

	textLen := len(text)
	logging.EmbeddingDebug("Ollama.Embed: starting embed request, text_length=%d chars model=%s", textLen, e.Model())

	// Retry transient Ollama runner/network failures.
	const maxRetries = 3
	backoff := 300 * time.Millisecond
	pulledOn404 := false

	for attempt := 1; attempt <= maxRetries; attempt++ {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		model := e.Model()

		req := ollamaEmbedRequest{
			Model:     model,
			Prompt:    text,
			KeepAlive: e.keepAlive,
		}

		body, err := json.Marshal(req)
		if err != nil {
			logging.Get(logging.CategoryEmbedding).Error("Ollama.Embed: failed to marshal request: %v", err)
			return nil, fmt.Errorf("failed to marshal request: %w", err)
		}

		logging.EmbeddingDebug("Ollama.Embed: sending POST to %s/api/embeddings (attempt %d/%d model=%s)", e.endpoint, attempt, maxRetries, model)
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
				if err := waitForRetry(ctx, backoff); err != nil {
					return nil, fmt.Errorf("ollama embed retry cancelled: %w", err)
				}
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
				if err := waitForRetry(ctx, backoff); err != nil {
					return nil, fmt.Errorf("ollama embed retry cancelled: %w", err)
				}
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
				e.invalidateModel() // force re-ensure under the model-state lock
				logging.Get(logging.CategoryEmbedding).Warn(
					"Ollama.Embed: model %q not found (%s); auto-pulling / resolving...", model, truncateForLog(bodyStr, 160))
				if ensureErr := e.EnsureModel(ctx); ensureErr != nil {
					logging.Get(logging.CategoryEmbedding).Error("Ollama.Embed: auto-pull failed: %v", ensureErr)
					return nil, fmt.Errorf("ollama model %q missing and auto-pull failed: %w", model, ensureErr)
				}
				// Reset attempt budget after a successful ensure.
				attempt = 0
				backoff = 300 * time.Millisecond
				continue
			}

			// Ollama answers an input longer than the model's context with a
			// 500. It is the input, not the server: the same text fails the same
			// way every time, so it is not retried, and the error says what the
			// caller has to change. Observed 2026-09-21: a campaign task brief
			// sent as a recall query, retried three times on every task.
			if strings.Contains(bodyStr, "exceeds the context length") {
				logging.EmbeddingDebug("Ollama.Embed: %d characters exceed model %s's context; splitting", utf8.RuneCountInString(text), model)
				return nil, &InputTooLongError{Chars: utf8.RuneCountInString(text), Model: model}
			}
			retryable := resp.StatusCode >= 500 && resp.StatusCode <= 599
			if !retryable && strings.Contains(bodyStr, "connection was forcibly closed") {
				retryable = true
			}

			if retryable && attempt < maxRetries && ctx.Err() == nil {
				logging.Get(logging.CategoryEmbedding).Warn("Ollama.Embed: non-OK status %d (attempt %d/%d): %s; retrying in %v",
					resp.StatusCode, attempt, maxRetries, bodyStr, backoff)
				if err := waitForRetry(ctx, backoff); err != nil {
					return nil, fmt.Errorf("ollama embed retry cancelled: %w", err)
				}
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
				if err := waitForRetry(ctx, backoff); err != nil {
					return nil, fmt.Errorf("ollama embed retry cancelled: %w", err)
				}
				backoff *= 2
				continue
			}
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}
		if validationErr := validateEmbeddingVector(result.Embedding); validationErr != nil {
			if attempt < maxRetries && ctx.Err() == nil {
				logging.Get(logging.CategoryEmbedding).Warn(
					"Ollama.Embed: invalid embedding response (attempt %d/%d): %v; retrying in %v",
					attempt, maxRetries, validationErr, backoff)
				if err := waitForRetry(ctx, backoff); err != nil {
					return nil, fmt.Errorf("ollama embed retry cancelled: %w", err)
				}
				backoff *= 2
				continue
			}
			return nil, fmt.Errorf("ollama returned invalid embedding: %w", validationErr)
		}

		if got := len(result.Embedding); got != e.dimensions {
			// Not retried: the model will return the same length every time. A
			// vector of the wrong length written into the index is unsearchable.
			return nil, fmt.Errorf("Ollama model %q returned a %d-dimensional vector and embedding.dimensions is %d: set embedding.dimensions to %d in .nerd/config.json, then `nerd embedding reembed`", model, got, e.dimensions, got)
		}
		logging.Embedding("Ollama.Embed: completed successfully, dimensions=%d, api_latency=%v, model=%s", len(result.Embedding), apiLatency, model)

		return result.Embedding, nil
	}

	return nil, fmt.Errorf("ollama embed failed after %d attempts", maxRetries)
}

const (
	ollamaBatchChunkTexts = 32
	ollamaBatchChunkBytes = 256 << 10
)

// embedBatchNative sends one chunk to Ollama's native /api/embed endpoint.
// It returns errors without logging them as final: EmbedBatch falls back to
// per-text Embed for the chunk, which is also the path that splits an input
// longer than the model's window.
func (e *OllamaEngine) embedBatchNative(ctx context.Context, texts []string) ([][]float32, error) {
	// Truncate is sent explicitly, as false. /api/embed defaults it to true,
	// and a text longer than the model's window would then come back as a
	// valid, correctly sized vector of its opening only: the batch succeeds, the
	// per-text split-and-pool path never runs, and nothing downstream can tell.
	// With false the server refuses the chunk, and EmbedBatch embeds it text by
	// text through Embed, which covers the whole input.
	noTruncate := false
	body, err := json.Marshal(ollamaBatchEmbedRequest{Model: e.Model(), Input: texts, Truncate: &noTruncate, KeepAlive: e.keepAlive})
	if err != nil {
		return nil, fmt.Errorf("marshal batch request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, "POST", e.endpoint+"/api/embed", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create batch request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ollama batch request failed: %w", err)
	}
	defer resp.Body.Close()
	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read batch response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama batch returned status %d: %s", resp.StatusCode, truncateForLog(string(respBytes), 200))
	}
	var result ollamaBatchEmbedResponse
	if err := json.Unmarshal(respBytes, &result); err != nil {
		return nil, fmt.Errorf("decode batch response: %w", err)
	}
	if len(result.Embeddings) != len(texts) {
		return nil, fmt.Errorf("ollama batch returned %d vectors for %d texts", len(result.Embeddings), len(texts))
	}
	for i, vec := range result.Embeddings {
		if err := validateEmbeddingVector(vec); err != nil {
			return nil, fmt.Errorf("ollama batch vector %d: %w", i, err)
		}
		if got := len(vec); got != e.dimensions {
			return nil, fmt.Errorf("Ollama model %q returned a %d-dimensional vector and embedding.dimensions is %d: set embedding.dimensions to %d in .nerd/config.json, then `nerd embedding reembed`", e.Model(), got, e.dimensions, got)
		}
	}
	return result.Embeddings, nil
}

// EmbedBatch generates embeddings for multiple texts. Modern Ollama has a
// native batch endpoint; use it in bounded chunks. Sequential per-text calls
// made a 256-text batch take ~45ms of HTTP work per text and predictably
// outlived short caller budgets, discarding every completed vector. A chunk
// the native endpoint refuses (an old server, or one overlong text) falls
// back to Embed, preserving long-input splitting and model recovery.
func (e *OllamaEngine) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	timer := logging.StartTimer(logging.CategoryEmbedding, "Ollama.EmbedBatch")
	defer timer.Stop()

	logging.Embedding("Ollama.EmbedBatch: starting batch embed for %d texts", len(texts))

	if len(texts) == 0 {
		logging.EmbeddingDebug("Ollama.EmbedBatch: empty input, returning nil")
		return nil, nil
	}
	// Before the first request: an empty text late in the batch would
	// otherwise cost every embed ahead of it.
	if err := checkEmbeddableBatch(texts); err != nil {
		return nil, err
	}

	// One ensure for the whole batch.
	if err := e.EnsureModel(ctx); err != nil {
		logging.EmbeddingDebug("Ollama.EmbedBatch: EnsureModel preflight: %v", err)
	}

	embeddings := make([][]float32, len(texts))

	for start := 0; start < len(texts); {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("ollama EmbedBatch cancelled at text %d/%d: %w", start, len(texts), err)
		}
		end := start
		chunkBytes := 0
		for end < len(texts) && end-start < ollamaBatchChunkTexts {
			if end > start && chunkBytes+len(texts[end]) > ollamaBatchChunkBytes {
				break
			}
			chunkBytes += len(texts[end])
			end++
		}
		chunk := texts[start:end]
		if len(chunk) > 1 {
			if vecs, nativeErr := e.embedBatchNative(ctx, chunk); nativeErr == nil {
				copy(embeddings[start:end], vecs)
				start = end
				continue
			} else if ctx.Err() != nil {
				return nil, fmt.Errorf("ollama EmbedBatch cancelled at text %d/%d: %w", start, len(texts), ctx.Err())
			} else {
				logging.EmbeddingDebug("Ollama.EmbedBatch: native batch for texts %d-%d unavailable (%v); embedding individually", start, end-1, nativeErr)
			}
		}
		for i, text := range chunk {
			idx := start + i
			if err := ctx.Err(); err != nil {
				return nil, fmt.Errorf("ollama EmbedBatch cancelled at text %d/%d: %w", idx, len(texts), err)
			}
			embedding, err := e.Embed(ctx, text)
			if err != nil {
				logging.Get(logging.CategoryEmbedding).Error("Ollama.EmbedBatch: failed at text %d: %v", idx, err)
				return nil, fmt.Errorf("failed to embed text %d: %w", idx, err)
			}
			embeddings[idx] = embedding
		}
		start = end
	}

	logging.Embedding("Ollama.EmbedBatch: completed successfully, processed %d texts", len(texts))
	return embeddings, nil
}

// Dimensions returns the configured vector length (embedding.dimensions).
func (e *OllamaEngine) Dimensions() int {
	return e.dimensions
}

// Name returns the engine name.
func (e *OllamaEngine) Name() string {
	return fmt.Sprintf("ollama:%s", e.Model())
}

// Model returns the currently selected model name (may change after ensure/pull).
func (e *OllamaEngine) Model() string {
	e.ensureMu.Lock()
	defer e.ensureMu.Unlock()
	return e.model
}

func (e *OllamaEngine) invalidateModel() {
	e.ensureMu.Lock()
	defer e.ensureMu.Unlock()
	e.modelReady = false
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

	// 1) The configured model, as installed.
	if resolved := resolveInstalledModel(e.model, installed); resolved != "" {
		if resolved != e.model {
			logging.Embedding("Ollama: resolved model %q → installed %q", e.model, resolved)
			e.model = resolved
		}
		e.modelReady = true
		return nil
	}

	// 2) Pull exactly the configured model.
	pullName := e.model
	if e.pullAttempted {
		return fmt.Errorf("model %q still unavailable after prior pull attempt", pullName)
	}
	e.pullAttempted = true

	logging.Embedding("Ollama: model %q not installed — pulling %q (client timeout %s)...", e.model, pullName, PullTimeout())
	if err := e.pullModel(ctx, pullName); err != nil {
		// The factory performs a deliberately short best-effort ensure. A
		// bootstrap deadline must not permanently poison the engine: the first
		// real Embed call gets its own context and must be able to retry the pull.
		if ctx.Err() != nil {
			e.pullAttempted = false
		}
		return fmt.Errorf("auto-pull %q failed: %w", pullName, err)
	}

	e.model = pullName
	e.modelReady = true
	logging.Embedding("Ollama: model %q pulled and ready", e.model)
	return nil
}

func waitForRetry(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
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

	// One POST /api/pull (stream:false) is one request: the body is the
	// download. It does not follow the caller's context deadline. EnsureModel
	// runs under embed and boot contexts (embedding.request_timeout, and the
	// short ensure at engine init) that are shorter than a model download,
	// and a cancelled caller still aborts this request through ctx. The
	// client timeout is embedding.pull_timeout, published into this package
	// by config.SetEmbeddingPullTimeout. Zero means nothing has been
	// published yet, and the caller's context is then the only bound. An
	// absent key publishes as 30m.
	pullClient := &http.Client{Timeout: PullTimeout()}
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

// resolveInstalledModel returns the installed tag that is the configured
// model, or "". A bare name means its :latest tag, as Ollama itself resolves
// it. Nothing else matches: an embedding model other than the configured one
// returns vectors the index was not built with, so a near miss is a refusal.
func resolveInstalledModel(want string, installed []string) string {
	want = strings.TrimSpace(want)
	if want == "" {
		return ""
	}
	for _, name := range installed {
		if name == want || (!strings.Contains(want, ":") && name == want+":latest") {
			return name
		}
	}
	return ""
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
	Model     string `json:"model"`
	Prompt    string `json:"prompt"`
	KeepAlive string `json:"keep_alive,omitempty"`
}

type ollamaEmbedResponse struct {
	Embedding []float32 `json:"embedding"`
}

type ollamaBatchEmbedRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
	// Truncate is a pointer so false is sent: the server default is true.
	Truncate  *bool  `json:"truncate,omitempty"`
	KeepAlive string `json:"keep_alive,omitempty"`
}

type ollamaBatchEmbedResponse struct {
	Embeddings [][]float32 `json:"embeddings"`
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
