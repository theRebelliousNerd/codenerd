// Package shards provides specialist agent management including consultation protocols.
// This file implements the cross-specialist consultation system, allowing:
// - Technical executors to get advice from strategic advisors
// - Specialists to consult each other for domain expertise
// - Background consultation gathering for context enrichment
package shards

import (
	"context"
	"fmt"
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
	cacheKey := m.cacheKey(req.ToSpec, req.Question)
	if cached := m.getCached(cacheKey); cached != nil {
		return cached, nil
	}

	// Store as pending
	m.mu.Lock()
	m.pending[req.RequestID] = &req
	m.mu.Unlock()

	// Build the consultation task prompt
	taskPrompt := m.buildConsultationPrompt(req)

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
	var wg sync.WaitGroup
	results := make(chan ConsultationResponse, len(specialists))
	errors := make(chan error, len(specialists))

	for _, spec := range specialists {
		wg.Add(1)
		go func(specialist string) {
			defer wg.Done()
			req := ConsultationRequest{
				FromSpec: "system",
				ToSpec:   specialist,
				Question: question,
				Context:  context,
				Priority: PriorityNormal,
			}
			resp, err := m.RequestConsultation(ctx, req)
			if err != nil {
				errors <- err
				return
			}
			results <- *resp
		}(spec)
	}

	go func() {
		wg.Wait()
		close(results)
		close(errors)
	}()

	var responses []ConsultationResponse
	for resp := range results {
		responses = append(responses, resp)
	}

	return responses, nil
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
		var conf int
		if _, err := fmt.Sscanf(content, "%d", &conf); err == nil {
			resp.Confidence = float64(conf) / 100.0
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

// cacheKey generates a cache key for consultation responses.
func (m *ConsultationManager) cacheKey(specialist, question string) string {
	// Simple cache key - in production might use hash
	q := question
	if len(q) > 100 {
		q = q[:100]
	}
	return fmt.Sprintf("%s:%s", specialist, q)
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

// FormatConsultationAdvice formats consultation responses for injection into context.
func FormatConsultationAdvice(responses []ConsultationResponse) string {
	if len(responses) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("## Specialist Consultation Results\n\n")

	for _, resp := range responses {
		sb.WriteString(fmt.Sprintf("### %s (Confidence: %.0f%%)\n\n", resp.FromSpec, resp.Confidence*100))
		sb.WriteString(resp.Advice)
		sb.WriteString("\n\n")

		if len(resp.Caveats) > 0 {
			sb.WriteString("**Caveats:**\n")
			for _, c := range resp.Caveats {
				if strings.TrimSpace(c) != "" {
					sb.WriteString(fmt.Sprintf("- %s\n", c))
				}
			}
			sb.WriteString("\n")
		}
	}

	return sb.String()
}
