package init

import (
	"fmt"
	"slices"
	"strings"

	"codenerd/internal/broker"
	"codenerd/internal/perception"
	"codenerd/internal/types"
)

// llmSlot names the configured client an init call is sent to.
//
// Init has two kinds of model work. Bulk work produces one knowledge atom
// among many: per-chunk document extraction and grounded domain research. A
// few calls produce what every later phase and session reads: the document
// roles and north star orientation derives, the domain-expert roster, the
// strategic knowledge synthesized from documents, each specialist's standing
// prompt atoms, and the project command tools. Those run on the planner
// client when one is configured, because their quality is multiplied by
// everything downstream; bulk work stays on the main client.
type llmSlot string

const (
	// slotMain serves per-chunk document extraction and grounded domain
	// research, and every phase when no planner is configured.
	slotMain llmSlot = "main"
	// slotPlanner serves document roles, the north-star draft, the
	// domain-expert proposal, strategic-knowledge synthesis, specialist prompt
	// generation and project tool selection.
	slotPlanner llmSlot = "planner"
)

// llmRoute is the client one init call is sent to and what its JIT compile
// needs to know about that client.
type llmRoute struct {
	// slot is the slot that actually serves the call: a planner request with
	// no planner configured is served by the main client and says so.
	slot     llmSlot
	client   perception.LLMClient
	provider string
	model    string
	// window is the client's own input cap in tokens (broker.InputWindow);
	// 0 when it reports none.
	window int
}

// route resolves the client a call for slot is sent to. A planner request
// falls back to the main client when no planner client is configured.
func (i *Initializer) route(slot llmSlot) llmRoute {
	if slot == slotPlanner && i.config.PlannerClient != nil {
		return newLLMRoute(slotPlanner, i.config.PlannerClient)
	}
	return newLLMRoute(slotMain, i.config.LLMClient)
}

func newLLMRoute(slot llmSlot, client perception.LLMClient) llmRoute {
	route := llmRoute{slot: slot, client: client}
	if client == nil {
		return route
	}
	route.provider, route.model = clientIdentity(client)
	route.window = broker.InputWindow(client)
	return route
}

// clientIdentity is the provider and model the client chain reports: the
// outermost layer implementing types.ModelIdentifier answers, so a scheduling,
// tracing or metering decorator is walked through and a fallback pair reports
// the client it serves. Empty strings when no layer reports one; provider- and
// model-pinned atoms then sit out, which is the fail-closed reading.
func clientIdentity(client perception.LLMClient) (provider, model string) {
	broker.Walk(client, func(layer types.LLMClient) bool {
		identifier, ok := layer.(types.ModelIdentifier)
		if !ok {
			return true
		}
		provider, model = identifier.ModelIdentity()
		return false
	})
	return strings.TrimSpace(provider), strings.TrimSpace(model)
}

// InitLLMMetrics reports the model calls initialization made. LLM failures
// degrade enrichment but do not make structurally valid init artifacts
// unusable, so they are reported separately from required failures.
type InitLLMMetrics struct {
	Attempts  int    `json:"attempts"`
	Succeeded int    `json:"succeeded"`
	Failed    int    `json:"failed"`
	LastError string `json:"last_error,omitzero"`
	// Clients is one row per client that was sent a call, in the order each
	// was first used.
	Clients []InitLLMClientMetrics `json:"clients,omitzero"`
}

// InitLLMClientMetrics is one serving client's share of InitLLMMetrics.
type InitLLMClientMetrics struct {
	Slot      string   `json:"slot"`
	Provider  string   `json:"provider,omitzero"`
	Model     string   `json:"model,omitzero"`
	Phases    []string `json:"phases,omitzero"`
	Attempts  int      `json:"attempts"`
	Succeeded int      `json:"succeeded"`
	Failed    int      `json:"failed"`
	LastError string   `json:"last_error,omitzero"`
}

// Identity is "provider/model" as far as the client reported it.
func (m InitLLMClientMetrics) Identity() string {
	return formatLLMIdentity(m.Provider, m.Model)
}

// recordLLMCall records one provider attempt against the client that served it.
func (i *Initializer) recordLLMCall(route llmRoute, phase string, err error) {
	i.mu.Lock()
	defer i.mu.Unlock()

	index := slices.IndexFunc(i.llmMetrics.Clients, func(row InitLLMClientMetrics) bool {
		return row.Slot == string(route.slot)
	})
	if index < 0 {
		i.llmMetrics.Clients = append(i.llmMetrics.Clients, InitLLMClientMetrics{
			Slot:     string(route.slot),
			Provider: route.provider,
			Model:    route.model,
		})
		index = len(i.llmMetrics.Clients) - 1
	}
	row := &i.llmMetrics.Clients[index]
	if phase != "" && !slices.Contains(row.Phases, phase) {
		row.Phases = append(row.Phases, phase)
	}

	i.llmMetrics.Attempts++
	row.Attempts++
	if err == nil {
		i.llmMetrics.Succeeded++
		row.Succeeded++
		return
	}
	message := boundedInitError(err, 240)
	i.llmMetrics.Failed++
	i.llmMetrics.LastError = message
	row.Failed++
	row.LastError = message
}

func (i *Initializer) snapshotLLMMetrics() InitLLMMetrics {
	i.mu.RLock()
	defer i.mu.RUnlock()
	m := i.llmMetrics
	m.Clients = make([]InitLLMClientMetrics, len(i.llmMetrics.Clients))
	for index, row := range i.llmMetrics.Clients {
		row.Phases = append([]string(nil), row.Phases...)
		m.Clients[index] = row
	}
	if len(m.Clients) == 0 {
		m.Clients = nil
	}
	return m
}

// degradedEnrichmentWarnings names each client whose calls failed, so a
// failing planner is not reported as a vague total over two different models.
func degradedEnrichmentWarnings(metrics InitLLMMetrics) []string {
	var warnings []string
	for _, row := range metrics.Clients {
		if row.Failed == 0 {
			continue
		}
		warnings = append(warnings, fmt.Sprintf(
			"LLM enrichment degraded on the %s client (%s): %d/%d calls failed in %s; last error: %s",
			row.Slot, row.Identity(), row.Failed, row.Attempts, strings.Join(row.Phases, ", "), row.LastError))
	}
	return warnings
}

func boundedInitError(err error, maxRunes int) string {
	if err == nil {
		return ""
	}
	clean := strings.Join(strings.Fields(err.Error()), " ")
	clean = initSecretKVPattern.ReplaceAllString(clean, "${1}[redacted]")
	clean = initSecretPattern.ReplaceAllString(clean, "[redacted]")
	runes := []rune(clean)
	if maxRunes > 0 && len(runes) > maxRunes {
		return string(runes[:maxRunes]) + "..."
	}
	return clean
}

func formatLLMIdentity(provider, model string) string {
	provider = strings.TrimSpace(provider)
	model = strings.TrimSpace(model)
	switch {
	case provider != "" && model != "":
		return provider + "/" + model
	case provider != "":
		return provider
	case model != "":
		return model
	default:
		return "provider/model unreported"
	}
}
