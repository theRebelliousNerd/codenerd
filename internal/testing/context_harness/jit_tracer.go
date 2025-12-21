package context_harness

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"time"
)

// JITTracer traces JIT prompt compilation decisions.
type JITTracer struct {
	writer  io.Writer
	verbose bool
}

// NewJITTracer creates a new JIT tracer.
func NewJITTracer(writer io.Writer, verbose bool) *JITTracer {
	return &JITTracer{
		writer:  writer,
		verbose: verbose,
	}
}

// CompilationSnapshot captures a JIT compilation event.
type CompilationSnapshot struct {
	Timestamp      time.Time
	TurnNumber     int
	ShardType      string
	OperationalMode string // /debugging, /dream, etc.
	CampaignPhase  string
	Language       string
	Framework      string

	// Compilation Context
	TokenBudget    int
	ContextDimensions []string // List of active context dimensions

	// Atom Selection
	TotalAtomsAvailable int
	TotalAtomTokens     int
	SelectedAtoms       []CompiledAtom
	RejectedAtoms       []CompiledAtom

	// Budget Allocation
	SystemAtomTokens   int
	ContextAtomTokens  int
	SpecialistTokens   int
	DynamicTokens      int

	// Compilation Stats
	CompilationLatency time.Duration
	CacheHits          int
	CacheMisses        int
}

// CompiledAtom represents a prompt atom in the compilation process.
type CompiledAtom struct {
	ID           string
	Category     string
	FilePath     string
	Content      string
	Tokens       int
	Priority     int
	SelectionReason string // Why it was selected (or rejected)

	// Context Selectors (metadata that determines when this atom is active)
	RequiredMode      string   // Required operational mode
	RequiredShard     string   // Required shard type
	RequiredLanguage  string   // Required language
	RequiredFramework string   // Required framework
	Tags              []string // General-purpose tags
}

// TraceCompilation logs a JIT compilation event.
func (t *JITTracer) TraceCompilation(snapshot *CompilationSnapshot) {
	var sb strings.Builder

	// Header
	sb.WriteString("═══════════════════════════════════════════════════════════════\n")
	sb.WriteString(fmt.Sprintf("JIT PROMPT COMPILATION - TURN %d\n", snapshot.TurnNumber))
	sb.WriteString("═══════════════════════════════════════════════════════════════\n")
	sb.WriteString(fmt.Sprintf("Timestamp: %s\n", snapshot.Timestamp.Format("2006-01-02 15:04:05")))
	sb.WriteString(fmt.Sprintf("Compilation Latency: %v\n\n", snapshot.CompilationLatency.Round(time.Millisecond)))

	// Compilation Context
	sb.WriteString("COMPILATION CONTEXT:\n")
	sb.WriteString("───────────────────────────────────────────────────────────────\n")
	sb.WriteString(fmt.Sprintf("  Shard Type:       %s\n", snapshot.ShardType))
	sb.WriteString(fmt.Sprintf("  Operational Mode: %s\n", snapshot.OperationalMode))
	if snapshot.CampaignPhase != "" {
		sb.WriteString(fmt.Sprintf("  Campaign Phase:   %s\n", snapshot.CampaignPhase))
	}
	if snapshot.Language != "" {
		sb.WriteString(fmt.Sprintf("  Language:         %s\n", snapshot.Language))
	}
	if snapshot.Framework != "" {
		sb.WriteString(fmt.Sprintf("  Framework:        %s\n", snapshot.Framework))
	}
	sb.WriteString(fmt.Sprintf("  Token Budget:     %s\n", formatNumber(snapshot.TokenBudget)))
	sb.WriteString("\n")

	if len(snapshot.ContextDimensions) > 0 {
		sb.WriteString("Active Context Dimensions:\n")
		for _, dim := range snapshot.ContextDimensions {
			sb.WriteString(fmt.Sprintf("  - %s\n", dim))
		}
		sb.WriteString("\n")
	}

	// Atom Selection Overview
	sb.WriteString("ATOM SELECTION:\n")
	sb.WriteString("───────────────────────────────────────────────────────────────\n")
	sb.WriteString(fmt.Sprintf("  Total Available:  %s atoms (%s tokens)\n",
		formatNumber(snapshot.TotalAtomsAvailable),
		formatNumber(snapshot.TotalAtomTokens)))
	sb.WriteString(fmt.Sprintf("  Selected:         %d atoms (%s tokens)\n",
		len(snapshot.SelectedAtoms),
		formatNumber(sumTokens(snapshot.SelectedAtoms))))
	sb.WriteString(fmt.Sprintf("  Rejected:         %d atoms (%s tokens)\n\n",
		len(snapshot.RejectedAtoms),
		formatNumber(sumTokens(snapshot.RejectedAtoms))))

	// Budget Allocation
	sb.WriteString("TOKEN BUDGET ALLOCATION:\n")
	sb.WriteString("───────────────────────────────────────────────────────────────\n")
	totalUsed := snapshot.SystemAtomTokens + snapshot.ContextAtomTokens +
		snapshot.SpecialistTokens + snapshot.DynamicTokens

	sb.WriteString(fmt.Sprintf("  System Atoms:     %s tokens (%.1f%%)\n",
		formatNumber(snapshot.SystemAtomTokens),
		percent(snapshot.SystemAtomTokens, totalUsed)))
	sb.WriteString(fmt.Sprintf("  Context Atoms:    %s tokens (%.1f%%)\n",
		formatNumber(snapshot.ContextAtomTokens),
		percent(snapshot.ContextAtomTokens, totalUsed)))
	sb.WriteString(fmt.Sprintf("  Specialist:       %s tokens (%.1f%%)\n",
		formatNumber(snapshot.SpecialistTokens),
		percent(snapshot.SpecialistTokens, totalUsed)))
	sb.WriteString(fmt.Sprintf("  Dynamic:          %s tokens (%.1f%%)\n",
		formatNumber(snapshot.DynamicTokens),
		percent(snapshot.DynamicTokens, totalUsed)))
	sb.WriteString(fmt.Sprintf("  ────────────────────────────────\n"))
	sb.WriteString(fmt.Sprintf("  Total Used:       %s / %s (%.1f%% of budget)\n\n",
		formatNumber(totalUsed),
		formatNumber(snapshot.TokenBudget),
		percent(totalUsed, snapshot.TokenBudget)))

	// Cache Stats
	if snapshot.CacheHits+snapshot.CacheMisses > 0 {
		sb.WriteString("CACHE PERFORMANCE:\n")
		sb.WriteString("───────────────────────────────────────────────────────────────\n")
		sb.WriteString(fmt.Sprintf("  Hits:    %d\n", snapshot.CacheHits))
		sb.WriteString(fmt.Sprintf("  Misses:  %d\n", snapshot.CacheMisses))
		sb.WriteString(fmt.Sprintf("  Hit Rate: %.1f%%\n\n",
			percent(snapshot.CacheHits, snapshot.CacheHits+snapshot.CacheMisses)))
	}

	// Selected Atoms (grouped by category)
	if len(snapshot.SelectedAtoms) > 0 {
		sb.WriteString("SELECTED ATOMS:\n")
		sb.WriteString("───────────────────────────────────────────────────────────────\n")

		// Group by category
		byCategory := groupByCategory(snapshot.SelectedAtoms)
		categories := make([]string, 0, len(byCategory))
		for cat := range byCategory {
			categories = append(categories, cat)
		}
		sort.Strings(categories)

		for _, category := range categories {
			atoms := byCategory[category]
			categoryTokens := sumTokens(atoms)

			sb.WriteString(fmt.Sprintf("\n## %s (%d atoms, %s tokens)\n\n",
				category, len(atoms), formatNumber(categoryTokens)))

			// Sort by priority descending
			sort.Slice(atoms, func(i, j int) bool {
				return atoms[i].Priority > atoms[j].Priority
			})

			for _, atom := range atoms {
				sb.WriteString(fmt.Sprintf("  [P%d] %s (%d tokens)\n",
					atom.Priority, atom.ID, atom.Tokens))
				sb.WriteString(fmt.Sprintf("        %s\n", atom.FilePath))

				if atom.SelectionReason != "" {
					sb.WriteString(fmt.Sprintf("        ✓ %s\n", atom.SelectionReason))
				}

				// Show context selectors
				selectors := make([]string, 0)
				if atom.RequiredMode != "" {
					selectors = append(selectors, fmt.Sprintf("mode=%s", atom.RequiredMode))
				}
				if atom.RequiredShard != "" {
					selectors = append(selectors, fmt.Sprintf("shard=%s", atom.RequiredShard))
				}
				if atom.RequiredLanguage != "" {
					selectors = append(selectors, fmt.Sprintf("lang=%s", atom.RequiredLanguage))
				}
				if atom.RequiredFramework != "" {
					selectors = append(selectors, fmt.Sprintf("framework=%s", atom.RequiredFramework))
				}
				if len(selectors) > 0 {
					sb.WriteString(fmt.Sprintf("        Context: [%s]\n", strings.Join(selectors, ", ")))
				}

				if t.verbose && atom.Content != "" {
					// Show first 200 chars of content
					preview := strings.TrimSpace(atom.Content)
					if len(preview) > 200 {
						preview = preview[:200] + "..."
					}
					// Indent each line
					lines := strings.Split(preview, "\n")
					for _, line := range lines {
						sb.WriteString(fmt.Sprintf("        │ %s\n", line))
					}
				}

				sb.WriteString("\n")
			}
		}
	}

	// Rejected Atoms (sample)
	if t.verbose && len(snapshot.RejectedAtoms) > 0 {
		sb.WriteString("REJECTED ATOMS (sample):\n")
		sb.WriteString("───────────────────────────────────────────────────────────────\n")

		// Sort by priority and show top rejections
		sortedRejected := make([]CompiledAtom, len(snapshot.RejectedAtoms))
		copy(sortedRejected, snapshot.RejectedAtoms)
		sort.Slice(sortedRejected, func(i, j int) bool {
			return sortedRejected[i].Priority > sortedRejected[j].Priority
		})

		sampleSize := 10
		if len(sortedRejected) < sampleSize {
			sampleSize = len(sortedRejected)
		}

		for i := 0; i < sampleSize; i++ {
			atom := sortedRejected[i]
			sb.WriteString(fmt.Sprintf("  [P%d] %s (%d tokens)\n",
				atom.Priority, atom.ID, atom.Tokens))
			if atom.SelectionReason != "" {
				sb.WriteString(fmt.Sprintf("        ✗ %s\n", atom.SelectionReason))
			}
		}
		sb.WriteString("\n")
	}

	sb.WriteString("═══════════════════════════════════════════════════════════════\n\n")

	t.writer.Write([]byte(sb.String()))
}

// Helper functions

func sumTokens(atoms []CompiledAtom) int {
	total := 0
	for _, a := range atoms {
		total += a.Tokens
	}
	return total
}

func groupByCategory(atoms []CompiledAtom) map[string][]CompiledAtom {
	groups := make(map[string][]CompiledAtom)
	for _, atom := range atoms {
		groups[atom.Category] = append(groups[atom.Category], atom)
	}
	return groups
}

func percent(value, total int) float64 {
	if total == 0 {
		return 0
	}
	return float64(value) / float64(total) * 100
}
