// Package shards provides specialist agent management including consultation protocols.
// This file implements the cross-specialist consultation system, allowing:
// - Technical executors to get advice from strategic advisors
// - Specialists to consult each other for domain expertise
// - Background consultation gathering for context enrichment
package shards

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// =============================================================================
// CONSULTATION PROTOCOL
// =============================================================================

// ConsultationRequest represents a request for specialist advice.
type ConsultationRequest struct {
	RequestID   string            // Unique request identifier
	FromSpec    string            // Requesting specialist (or "system" for auto-consult)
	ToSpec      string            // Target specialist
	Question    string            // The question or topic to consult on
	Context     string            // Additional context for the consultation
	Priority    ConsultPriority   // Urgency of the consultation
	Metadata    map[string]string // Additional metadata
	RequestTime time.Time
}

// ConsultationResponse represents a specialist's response to consultation.
type ConsultationResponse struct {
	RequestID    string
	FromSpec     string            // Which specialist provided this
	ToSpec       string            // Who requested this
	Advice       string            // The advice/guidance provided
	Confidence   float64           // 0-1 confidence in the advice
	References   []string          // References or sources
	Caveats      []string          // Important caveats or limitations
	Metadata     map[string]string // Additional metadata
	ResponseTime time.Time
	Duration     time.Duration
}

// ConsultPriority indicates urgency of consultation.
type ConsultPriority int

const (
	// PriorityBackground - Async, can wait for opportune moment
	PriorityBackground ConsultPriority = iota
	// PriorityNormal - Standard priority, complete before task continues
	PriorityNormal
	// PriorityUrgent - Block until complete
	PriorityUrgent
)

// ConsultationSpawner interface for spawning consultation tasks.
type ConsultationSpawner interface {
	SpawnConsultation(ctx context.Context, specialistName, task string) (string, error)
}

// ConsultationManager handles cross-specialist consultations.
type ConsultationManager struct {
	mu sync.RWMutex

	// Pending consultations
	pending map[string]*ConsultationRequest

	// Completed consultations (cache for reuse)
	completed map[string]*ConsultationResponse

	// Configuration
	maxCacheSize   int
	defaultTimeout time.Duration

	// Spawner for consultation tasks
	spawner ConsultationSpawner
}

// NewConsultationManager creates a new consultation manager.
func NewConsultationManager(spawner ConsultationSpawner) *ConsultationManager {
	return &ConsultationManager{
		pending:        make(map[string]*ConsultationRequest),
		completed:      make(map[string]*ConsultationResponse),
		maxCacheSize:   100,
		defaultTimeout: 2 * time.Minute,
		spawner:        spawner,
	}
}

// RequestConsultation initiates a consultation request.
// NOTE: Consult mechanism is implemented. JIT collaboration uses this via ConsultationSpawner interface.
func (m *ConsultationManager) RequestConsultation(ctx context.Context, req ConsultationRequest) (*ConsultationResponse, error) {
	if req.RequestID == "" {
		req.RequestID = fmt.Sprintf("consult-%s-%d", req.ToSpec, time.Now().UnixNano())
	}
	req.RequestTime = time.Now()

	// Check if we have a cached response for similar question
	cacheKey := m.cacheKey(req.ToSpec, req.Question, req.Context)
	if cached := m.getCached(cacheKey); cached != nil {
		return cached, nil
	}

	// Store as pending
	m.mu.Lock()
	m.pending[req.RequestID] = &req
	m.mu.Unlock()

	// Build the consultation task prompt
	taskPrompt := m.buildConsultationPrompt(req)
	if m.spawner == nil {
		m.mu.Lock()
		delete(m.pending, req.RequestID)
		m.mu.Unlock()
		return nil, fmt.Errorf("consultation with %s failed: no consultation spawner configured", req.ToSpec)
	}

	// Spawn the consultation
	startTime := time.Now()
	result, err := m.spawner.SpawnConsultation(ctx, req.ToSpec, taskPrompt)
	duration := time.Since(startTime)

	// Remove from pending
	m.mu.Lock()
	delete(m.pending, req.RequestID)
	m.mu.Unlock()

	if err != nil {
		return nil, fmt.Errorf("consultation with %s failed: %w", req.ToSpec, err)
	}

	// Parse the response
	response := m.parseConsultationResponse(req, result, duration)

	// Cache the response
	m.cacheResponse(cacheKey, response)

	return response, nil
}

// RequestBatchConsultation consults multiple specialists in parallel.
func (m *ConsultationManager) RequestBatchConsultation(ctx context.Context, question, context string, specialists []string) ([]ConsultationResponse, error) {
	type batchResult struct {
		index    int
		response *ConsultationResponse
		err      error
	}

	var wg sync.WaitGroup
	results := make(chan batchResult, len(specialists))

	for i, spec := range specialists {
		index := i
		specialist := spec
		wg.Go(func() {
			req := ConsultationRequest{
				FromSpec: "system",
				ToSpec:   specialist,
				Question: question,
				Context:  context,
				Priority: PriorityNormal,
			}
			resp, err := m.RequestConsultation(ctx, req)
			if err != nil {
				results <- batchResult{index: index, err: err}
				return
			}
			results <- batchResult{index: index, response: resp}
		})
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	ordered := make([]batchResult, len(specialists))
	for result := range results {
		ordered[result.index] = result
	}

	responses := make([]ConsultationResponse, 0, len(specialists))
	batchErrors := make([]error, 0)
	for _, result := range ordered {
		if result.err != nil {
			batchErrors = append(batchErrors, result.err)
			continue
		}
		if result.response != nil {
			responses = append(responses, *result.response)
		}
	}

	return responses, errors.Join(batchErrors...)
}

// GetStrategicAdvisorsFor returns strategic advisors that can assist a given executor.
func GetStrategicAdvisorsFor(executorName string) []string {
	// Based on specialist_assists rule in shards.mg:
	// Strategic advisors can assist any technical executor
	var advisors []string
	for name, class := range DefaultSpecialistClassifications {
		if class.ExecutionMode == SpecialistModeAdvisor &&
			class.KnowledgeTier == TierStrategic {
			advisors = append(advisors, name)
		}
	}
	// Sorted: the list feeds user-visible delegation output, and map order
	// would shuffle the consultation phase from run to run.
	sort.Strings(advisors)
	return advisors
}

// ShouldConsultBeforeExecution determines if an executor should consult advisors first.
func ShouldConsultBeforeExecution(executorName string, taskComplexity string) bool {
	class, ok := GetSpecialistClassification(executorName)
	if !ok {
		return false
	}

	// Only executors consult advisors
	if class.ExecutionMode != SpecialistModeExecutor {
		return false
	}

	// High complexity tasks should consult strategic advisors
	return taskComplexity == "high" || taskComplexity == "complex"
}

// buildConsultationPrompt creates the task prompt for a consultation.
func (m *ConsultationManager) buildConsultationPrompt(req ConsultationRequest) string {
	var sb strings.Builder

	sb.WriteString("CONSULTATION REQUEST\n\n")
	sb.WriteString(fmt.Sprintf("From: %s\n", req.FromSpec))
	sb.WriteString(fmt.Sprintf("Question: %s\n\n", req.Question))

	if req.Context != "" {
		sb.WriteString(fmt.Sprintf("Context:\n%s\n\n", req.Context))
	}

	sb.WriteString(`Please provide your expert advice. Structure your response as:

ADVICE:
[Your main advice and guidance]

CONFIDENCE: [0-100]

REFERENCES:
[Any relevant references, patterns, or sources]

CAVEATS:
[Important limitations or edge cases to consider]`)

	return sb.String()
}

// parseConsultationResponse parses the specialist's response.
func (m *ConsultationManager) parseConsultationResponse(req ConsultationRequest, result string, duration time.Duration) *ConsultationResponse {
	response := &ConsultationResponse{
		RequestID:    req.RequestID,
		FromSpec:     req.ToSpec,
		ToSpec:       req.FromSpec,
		ResponseTime: time.Now(),
		Duration:     duration,
		Confidence:   0.7, // Default confidence
		Metadata:     make(map[string]string),
	}

	// Parse structured response
	lines := strings.Split(result, "\n")
	var currentSection string
	var sectionContent strings.Builder

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		switch {
		case strings.HasPrefix(trimmed, "ADVICE:"):
			if currentSection != "" {
				m.applySection(response, currentSection, sectionContent.String())
			}
			currentSection = "advice"
			sectionContent.Reset()
			remaining := strings.TrimPrefix(trimmed, "ADVICE:")
			sectionContent.WriteString(strings.TrimSpace(remaining))
		case strings.HasPrefix(trimmed, "CONFIDENCE:"):
			if currentSection != "" {
				m.applySection(response, currentSection, sectionContent.String())
			}
			currentSection = "confidence"
			sectionContent.Reset()
			remaining := strings.TrimPrefix(trimmed, "CONFIDENCE:")
			sectionContent.WriteString(strings.TrimSpace(remaining))
		case strings.HasPrefix(trimmed, "REFERENCES:"):
			if currentSection != "" {
				m.applySection(response, currentSection, sectionContent.String())
			}
			currentSection = "references"
			sectionContent.Reset()
		case strings.HasPrefix(trimmed, "CAVEATS:"):
			if currentSection != "" {
				m.applySection(response, currentSection, sectionContent.String())
			}
			currentSection = "caveats"
			sectionContent.Reset()
		default:
			if currentSection != "" && trimmed != "" {
				if sectionContent.Len() > 0 {
					sectionContent.WriteString("\n")
				}
				sectionContent.WriteString(trimmed)
			}
		}
	}

	// Apply final section
	if currentSection != "" {
		m.applySection(response, currentSection, sectionContent.String())
	}

	// If no structured response, use the whole result as advice
	if response.Advice == "" {
		response.Advice = result
	}

	return response
}

// applySection applies parsed section content to response.
func (m *ConsultationManager) applySection(resp *ConsultationResponse, section, content string) {
	content = strings.TrimSpace(content)
	switch section {
	case "advice":
		resp.Advice = content
	case "confidence":
		if conf, ok := parseConfidence(content); ok {
			resp.Confidence = conf
		}
	case "references":
		if content != "" {
			resp.References = strings.Split(content, "\n")
		}
	case "caveats":
		if content != "" {
			resp.Caveats = strings.Split(content, "\n")
		}
	}
}

// parseConfidence reads the CONFIDENCE section onto the 0-1 scale. The prompt
// asks for 0-100, but models also write ratios ("0.85") and out-of-range
// values; the first must not parse as zero and the second must not escape
// the scale the response documents.
func parseConfidence(content string) (float64, bool) {
	content = strings.TrimSuffix(strings.TrimSpace(content), "%")
	f, err := strconv.ParseFloat(strings.TrimSpace(content), 64)
	if err != nil {
		return 0, false
	}
	if f > 1 {
		f /= 100
	}
	if f < 0 {
		f = 0
	}
	if f > 1 {
		f = 1
	}
	return f, true
}

// cacheKey generates a cache key for consultation responses. The full
// question and context are hashed: truncating to a prefix would collide
// distinct questions, and the response depends on the context (it is part of
// the consultation prompt), so keying on the question alone would serve one
// consultation's advice for another's.
func (m *ConsultationManager) cacheKey(specialist, question, context string) string {
	sum := sha256.Sum256([]byte(specialist + "\x00" + question + "\x00" + context))
	return specialist + ":" + hex.EncodeToString(sum[:8])
}

// getCached retrieves a cached consultation response.
func (m *ConsultationManager) getCached(key string) *ConsultationResponse {
	m.mu.RLock()
	defer m.mu.RUnlock()

	resp, ok := m.completed[key]
	if !ok {
		return nil
	}

	// Check if cache is still fresh (5 minutes)
	if time.Since(resp.ResponseTime) > 5*time.Minute {
		return nil
	}

	return resp
}

// cacheResponse caches a consultation response.
func (m *ConsultationManager) cacheResponse(key string, resp *ConsultationResponse) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Evict old entries if cache is full
	if len(m.completed) >= m.maxCacheSize {
		// Simple eviction: remove oldest
		var oldestKey string
		var oldestTime time.Time
		for k, v := range m.completed {
			if oldestKey == "" || v.ResponseTime.Before(oldestTime) {
				oldestKey = k
				oldestTime = v.ResponseTime
			}
		}
		if oldestKey != "" {
			delete(m.completed, oldestKey)
		}
	}

	m.completed[key] = resp
}
