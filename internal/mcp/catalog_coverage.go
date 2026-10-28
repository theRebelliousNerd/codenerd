package mcp

import (
	"errors"
	"sort"
)

// CatalogCoverage records whether the last collection of one catalog kind
// reached the end of the server's list. A catalog that stopped early is
// descriptive evidence of what the server offers, never proof of what it no
// longer offers, and the control plane says so wherever it shows the catalog.
type CatalogCoverage struct {
	Kind      string `json:"kind"`
	Complete  bool   `json:"complete"`
	Collected int    `json:"collected"`
	Pages     int    `json:"pages,omitzero"`
	Reason    string `json:"reason,omitzero"`
}

func coverageFrom(kind string, collected int, err error) CatalogCoverage {
	var incomplete *IncompleteCatalogError
	if errors.As(err, &incomplete) {
		return CatalogCoverage{Kind: kind, Collected: incomplete.Collected, Pages: incomplete.Pages, Reason: incomplete.Error()}
	}
	return CatalogCoverage{Kind: kind, Complete: true, Collected: collected}
}

func (m *MCPClientManager) setCoverage(operation RemoteOperation, serverID string, coverage CatalogCoverage) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.coverage == nil {
		m.coverage = make(map[string]CatalogCoverage)
	}
	m.coverage[catalogGenerationKey(operation, serverID)] = coverage
}

// clearCoverageLocked forgets a server's coverage records; the caller holds
// m.mu. A new connection starts with no claim about its catalogs.
func (m *MCPClientManager) clearCoverageLocked(serverID string) {
	for _, operation := range []RemoteOperation{RemoteTool, RemoteResource, RemotePrompt} {
		delete(m.coverage, catalogGenerationKey(operation, serverID))
	}
}

// IncompleteCatalogs describes each of the server's catalogs whose last
// collection stopped before the server's list was exhausted, in a stable order.
func (m *MCPClientManager) IncompleteCatalogs(serverID string) []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var gaps []string
	for _, operation := range []RemoteOperation{RemoteTool, RemoteResource, RemotePrompt} {
		if coverage, ok := m.coverage[catalogGenerationKey(operation, serverID)]; ok && !coverage.Complete {
			gaps = append(gaps, coverage.Reason)
		}
	}
	sort.Strings(gaps)
	return gaps
}
