// Package reviewer implements the Reviewer ShardAgent per §7.0 Sharding.
// This file contains file analysis and architecture analysis functions.
package reviewer

import (
	"context"
	"strings"

	"codenerd/internal/logging"
)

// =============================================================================
// FILE ANALYSIS
// =============================================================================

// analyzeFile runs all analysis checks on a file (no dependency context).
func (r *ReviewerShard) analyzeFile(ctx context.Context, filePath, content string) ([]ReviewFinding, string) {
	return r.analyzeFileWithDeps(ctx, filePath, content, nil, nil)
}

// ArchitectureAnalysis holds the "Holographic" view of the code.
type ArchitectureAnalysis struct {
	Module      string   `json:"module"`       // The module this file belongs to
	Layer       string   `json:"layer"`        // e.g., "core", "api", "data"
	Related     []string `json:"related"`      // Semantically related entities
	Role        string   `json:"role"`         // Deduced role (adapter, service, model)
	SystemValue string   `json:"system_value"` // High-level system purpose
}

// analyzeArchitecture performs a "Holographic" analysis using the knowledge graph.
func (r *ReviewerShard) analyzeArchitecture(ctx context.Context, filePath string) *ArchitectureAnalysis {
	analysis := &ArchitectureAnalysis{
		Module: "unknown",
		Layer:  "unknown",
		Role:   "unknown",
	}

	if r.virtualStore == nil {
		return analysis
	}

	localDB := r.virtualStore.GetLocalDB()
	if localDB == nil {
		return analysis
	}

	// 1. Determine Module/Layer from path
	// Simple heuristic fallback if graph is empty
	parts := strings.Split(filePath, "/")
	if len(parts) > 1 {
		for i, part := range parts {
			if part == "internal" || part == "pkg" || part == "cmd" {
				if i+1 < len(parts) {
					analysis.Module = parts[i+1]
				}
				analysis.Layer = part
				break
			}
		}
	}

	// 2. Query Knowledge Graph for relationships
	// Using "contains" or "defines" relations
	links, err := localDB.QueryLinks(filePath, "incoming")
	if err == nil {
		for _, link := range links {
			if link.Relation == "defines" || link.Relation == "contains" {
				// The container (directory/package) is the entity A
				analysis.Module = link.EntityA
			}
		}
	}

	// 3. Find related entities (semantic neighbors)
	// Using "imports" or "calls"
	outgoing, err := localDB.QueryLinks(filePath, "outgoing")
	if err == nil {
		for _, link := range outgoing {
			if link.Relation == "imports" || link.Relation == "depends_on" {
				analysis.Related = append(analysis.Related, link.EntityB)
			}
		}
	}

	return analysis
}

// analyzeFileWithDeps runs all analysis checks on a file with optional dependency and architectural context.
// Returns findings and the LLM analysis report (if any).
func (r *ReviewerShard) analyzeFileWithDeps(ctx context.Context, filePath, content string, depCtx *DependencyContext, archCtx *ArchitectureAnalysis) ([]ReviewFinding, string) {
	findings := make([]ReviewFinding, 0)
	var report string

	// Code DOM safety checks (check kernel facts first)
	findings = append(findings, r.checkCodeDOMSafety(filePath)...)

	// Security checks
	findings = append(findings, r.checkSecurity(filePath, content)...)

	// Style checks
	findings = append(findings, r.checkStyle(filePath, content)...)

	// Bug pattern checks
	findings = append(findings, r.checkBugPatterns(filePath, content)...)

	// Custom rules checks (user-defined patterns)
	findings = append(findings, r.checkCustomRules(filePath, content)...)

	// LLM-powered semantic analysis (if available) - now with dependency and architectural context
	if r.llmClient != nil {
		var err error
		var llmFindings []ReviewFinding
		llmFindings, report, err = r.llmAnalysisWithDeps(ctx, filePath, content, depCtx, archCtx)
		if err == nil {
			findings = append(findings, llmFindings...)
		} else {
			// Log LLM failure but continue with regex-based checks
			logging.Get(logging.CategoryReviewer).Warn("LLM analysis failed for %s, continuing with regex checks: %v", filePath, err)
		}
	}

	// Check against learned anti-patterns
	findings = append(findings, r.checkLearnedPatterns(filePath, content)...)

	return findings, report
}

// analyzeFileWithHolographic runs all analysis checks with full holographic context.
// This is the enhanced version that includes package sibling awareness.
func (r *ReviewerShard) analyzeFileWithHolographic(ctx context.Context, filePath, content string, depCtx *DependencyContext, archCtx *ArchitectureAnalysis, holoCtx *HolographicContext) ([]ReviewFinding, string) {
	findings := make([]ReviewFinding, 0)
	var report string

	// Code DOM safety checks (check kernel facts first)
	findings = append(findings, r.checkCodeDOMSafety(filePath)...)

	// Security checks
	findings = append(findings, r.checkSecurity(filePath, content)...)

	// Style checks
	findings = append(findings, r.checkStyle(filePath, content)...)

	// Bug pattern checks
	findings = append(findings, r.checkBugPatterns(filePath, content)...)

	// Custom rules checks (user-defined patterns)
	findings = append(findings, r.checkCustomRules(filePath, content)...)

	// LLM-powered semantic analysis with FULL holographic context
	if r.llmClient != nil {
		var err error
		var llmFindings []ReviewFinding
		llmFindings, report, err = r.llmAnalysisWithHolographic(ctx, filePath, content, depCtx, archCtx, holoCtx)
		if err == nil {
			findings = append(findings, llmFindings...)
		} else {
			// Log LLM failure but continue with regex-based checks
			logging.Get(logging.CategoryReviewer).Warn("LLM analysis failed for %s, continuing with regex checks: %v", filePath, err)
		}
	}

	// Check against learned anti-patterns
	findings = append(findings, r.checkLearnedPatterns(filePath, content)...)

	return findings, report
}
