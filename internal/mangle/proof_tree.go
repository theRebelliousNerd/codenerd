package mangle

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/mangle/ast"
)

// ============================================================================
// Proof Tree Visualization - Cortex 1.5.0 §2.2
// Materializes derivation_trace and proof_tree_node facts during query execution
// ============================================================================

// DerivationNode represents a node in the proof tree.
type DerivationNode struct {
	ID        string            // Unique node ID
	ParentID  string            // Parent node ID (empty for root)
	Fact      Fact              // The derived fact
	RuleName  string            // Rule that derived this fact (empty for EDB facts)
	Source    DerivationSource  // EDB (base fact) or IDB (derived)
	Children  []*DerivationNode // Child nodes (premises)
	Depth     int               // Depth in the tree
	Timestamp time.Time         // When this node was created
}

// DerivationSource indicates whether a fact came from EDB or IDB.
type DerivationSource string

const (
	SourceEDB DerivationSource = "EDB" // Extensional - base facts
	SourceIDB DerivationSource = "IDB" // Intensional - derived by rules
)

// DerivationTrace represents a complete proof tree for a query.
type DerivationTrace struct {
	Query      string            // Original query
	RootNodes  []*DerivationNode // Top-level derivation nodes
	AllNodes   []*DerivationNode // All nodes in the tree (flat list)
	Duration   time.Duration     // Time to compute
	Timestamp  time.Time         // When the trace was created
	TotalFacts int               // Total number of derived facts
}

// ProofTreeTracer tracks derivations during query execution.
type ProofTreeTracer struct {
	mu        sync.RWMutex
	engine    *Engine
	traces    map[string]*DerivationTrace // Cache of recent traces by query
	maxCache  int                         // Max traces to cache
	nodeIDSeq int64                       // Sequence for generating node IDs
	ruleIndex map[string][]RuleSpec       // Index of rules by head predicate
}

// RuleSpec describes a rule from policy.mg for tracing purposes.
type RuleSpec struct {
	Name        string   // Rule identifier
	HeadPred    string   // Head predicate
	BodyPreds   []string // Body predicates (premises)
	IsRecursive bool     // Whether the rule is recursive
	Source      string   // Source location (file:line)
}

// NewProofTreeTracer creates a new tracer for an engine.
func NewProofTreeTracer(engine *Engine) *ProofTreeTracer {
	return &ProofTreeTracer{
		engine:    engine,
		traces:    make(map[string]*DerivationTrace),
		maxCache:  100,
		ruleIndex: make(map[string][]RuleSpec),
	}
}

// IndexRules extracts rule structure from the program info for tracing.
func (t *ProofTreeTracer) IndexRules() {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.engine.mu.RLock()
	programInfo := t.engine.programInfo
	t.engine.mu.RUnlock()

	if programInfo == nil {
		return
	}

	// Extract rules from program info
	for _, decl := range programInfo.Decls {
		pred := decl.DeclaredAtom.Predicate.Symbol

		// Check if this predicate has derivation rules (IDB)
		// This is a simplified indexing - full implementation would parse policy.mg
		if _, ok := t.ruleIndex[pred]; !ok {
			t.ruleIndex[pred] = []RuleSpec{}
		}
	}
}

// TraceQuery executes a query and builds a proof tree.
func (t *ProofTreeTracer) TraceQuery(ctx context.Context, query string) (*DerivationTrace, error) {
	start := time.Now()

	// Check cache first
	t.mu.RLock()
	if cached, ok := t.traces[query]; ok {
		t.mu.RUnlock()
		return cached, nil
	}
	t.mu.RUnlock()

	// Execute query and capture results
	result, err := t.engine.Query(ctx, query)
	if err != nil {
		return nil, err
	}

	// Parse query to understand structure
	shape, err := parseQueryShape(query)
	if err != nil {
		return nil, err
	}

	// Build derivation tree
	trace := &DerivationTrace{
		Query:     query,
		RootNodes: make([]*DerivationNode, 0),
		AllNodes:  make([]*DerivationNode, 0),
		Timestamp: time.Now(),
	}

	predicate := shape.atom.Predicate.Symbol

	// For each result binding, create a derivation node
	for _, binding := range result.Bindings {
		// Reconstruct the fact from bindings
		fact := Fact{
			Predicate: predicate,
			Args:      make([]interface{}, len(shape.atom.Args)),
			Timestamp: time.Now(),
		}

		for i, arg := range shape.atom.Args {
			if v, ok := arg.(ast.Variable); ok {
				if val, exists := binding[v.Symbol]; exists {
					fact.Args[i] = val
				} else {
					fact.Args[i] = v.Symbol
				}
			} else {
				fact.Args[i] = convertBaseTermToInterface(arg)
			}
		}

		// Build the derivation node with its tree
		node := t.buildDerivationNode(ctx, fact, "", 0)
		trace.RootNodes = append(trace.RootNodes, node)
		trace.AllNodes = append(trace.AllNodes, t.flattenTree(node)...)
	}

	trace.Duration = time.Since(start)

	// Cache the result
	t.mu.Lock()
	if len(t.traces) >= t.maxCache {
		// Evict oldest entry
		for k := range t.traces {
			delete(t.traces, k)
			break
		}
	}
	t.traces[query] = trace
	t.mu.Unlock()

	return trace, nil
}

// buildDerivationNode builds a derivation node and recursively finds premises.
func (t *ProofTreeTracer) buildDerivationNode(ctx context.Context, fact Fact, parentID string, depth int) *DerivationNode {
	t.mu.Lock()
	t.nodeIDSeq++
	nodeID := fmt.Sprintf("node_%d", t.nodeIDSeq)
	t.mu.Unlock()

	node := &DerivationNode{
		ID:        nodeID,
		ParentID:  parentID,
		Fact:      fact,
		Depth:     depth,
		Timestamp: time.Now(),
		Children:  make([]*DerivationNode, 0),
	}

	// Determine if this is EDB or IDB
	source, ruleName := t.classifyFact(fact)
	node.Source = source
	node.RuleName = ruleName

	// If IDB, try to find premises (recursively build the tree)
	if source == SourceIDB && depth < 10 { // Limit recursion depth
		premises := t.findPremises(ctx, fact, ruleName)
		for _, premise := range premises {
			child := t.buildDerivationNode(ctx, premise, nodeID, depth+1)
			node.Children = append(node.Children, child)
		}
	}

	return node
}

// classifyFact determines if a fact is from EDB or IDB.
func (t *ProofTreeTracer) classifyFact(fact Fact) (DerivationSource, string) {
	// EDB predicates (base facts from schemas.mg)
	edbPredicates := map[string]bool{
		"file_topology":    true,
		"file_content":     true,
		"symbol_graph":     true,
		"dependency_link":  true,
		"diagnostic":       true,
		"observation":      true,
		"user_intent":      true,
		"focus_resolution": true,
		"preference":       true,
		"shard_profile":    true,
		"knowledge_atom":   true,
	}

	if edbPredicates[fact.Predicate] {
		return SourceEDB, ""
	}

	// IDB predicates (derived by rules in policy.mg)
	idbRules := map[string]string{
		"next_action":          "strategy_selector",
		"permitted":            "permission_gate",
		"block_commit":         "commit_barrier",
		"impacted":             "transitive_impact",
		"clarification_needed": "focus_threshold",
		"unsafe_to_refactor":   "refactoring_guard",
		"test_state":           "tdd_loop",
		"context_atom":         "spreading_activation",
		"missing_hypothesis":   "abductive_repair",
		"delegate_task":        "shard_delegation",
		"activation":           "activation_rules",
	}

	if ruleName, ok := idbRules[fact.Predicate]; ok {
		return SourceIDB, ruleName
	}

	// Unknown predicates default to EDB
	return SourceEDB, ""
}

// findPremises finds the facts that were used to derive a given fact.
func (t *ProofTreeTracer) findPremises(ctx context.Context, fact Fact, ruleName string) []Fact {
	var premises []Fact

	// Rule-specific premise discovery
	switch ruleName {
	case "transitive_impact":
		// impacted(X) :- dependency_link(X, Y, _), modified(Y).
		// Look for dependency_link facts that could have derived this
		deps, _ := t.engine.GetFacts("dependency_link")
		for _, dep := range deps {
			if len(dep.Args) >= 2 && dep.Args[0] == fact.Args[0] {
				premises = append(premises, dep)
			}
		}

	case "permission_gate":
		// permitted(Action) :- safe_action(Action).
		safeFacts, _ := t.engine.GetFacts("safe_action")
		for _, sf := range safeFacts {
			if len(sf.Args) >= 1 && len(fact.Args) >= 1 && sf.Args[0] == fact.Args[0] {
				premises = append(premises, sf)
			}
		}

	case "spreading_activation":
		// context_atom is derived from activation rules
		activations, _ := t.engine.GetFacts("activation")
		for _, a := range activations {
			if len(a.Args) >= 1 && len(fact.Args) >= 1 && a.Args[0] == fact.Args[0] {
				premises = append(premises, a)
			}
		}

	case "focus_threshold":
		// clarification_needed(Ref) :- focus_resolution(Ref, _, _, Score), Score < 0.85.
		focuses, _ := t.engine.GetFacts("focus_resolution")
		for _, f := range focuses {
			if len(f.Args) >= 1 && len(fact.Args) >= 1 && f.Args[0] == fact.Args[0] {
				premises = append(premises, f)
			}
		}

	case "strategy_selector":
		// next_action derived from user_intent + file_topology
		intents, _ := t.engine.GetFacts("user_intent")
		premises = append(premises, intents...)

	default:
		// Generic: no premises identified
	}

	return premises
}

// flattenTree converts a tree to a flat list of all nodes.
func (t *ProofTreeTracer) flattenTree(node *DerivationNode) []*DerivationNode {
	nodes := []*DerivationNode{node}
	for _, child := range node.Children {
		nodes = append(nodes, t.flattenTree(child)...)
	}
	return nodes
}

// MaterializeToFacts stores the derivation trace as Mangle facts.
func (t *ProofTreeTracer) MaterializeToFacts(_ context.Context, trace *DerivationTrace) error {
	for _, node := range trace.AllNodes {
		// Create derivation_trace fact
		if err := t.engine.AddFact("derivation_trace",
			node.Fact.String(),    // Conclusion
			node.RuleName,         // RuleApplied
			t.premiseString(node), // Premises as string
		); err != nil {
			return err
		}

		// Create proof_tree_node fact
		if err := t.engine.AddFact("proof_tree_node",
			node.ID,
			node.ParentID,
			node.Fact.String(),
			node.RuleName,
		); err != nil {
			return err
		}
	}

	return nil
}

// premiseString formats child facts as a comma-separated string.
func (t *ProofTreeTracer) premiseString(node *DerivationNode) string {
	if len(node.Children) == 0 {
		return ""
	}

	var parts []string
	for _, child := range node.Children {
		parts = append(parts, child.Fact.String())
	}
	return strings.Join(parts, ", ")
}

// ClearCache clears the trace cache.
func (t *ProofTreeTracer) ClearCache() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.traces = make(map[string]*DerivationTrace)
}

// GetCachedTrace retrieves a cached trace if available.
func (t *ProofTreeTracer) GetCachedTrace(query string) *DerivationTrace {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.traces[query]
}

// ============================================================================
// Proof Tree Rendering
// ============================================================================

// RenderASCII renders a proof tree as ASCII art.
func (trace *DerivationTrace) RenderASCII() string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Query: %s\n", trace.Query))
	sb.WriteString(fmt.Sprintf("Duration: %v\n", trace.Duration))
	sb.WriteString(strings.Repeat("=", 60) + "\n")

	for i, root := range trace.RootNodes {
		sb.WriteString(fmt.Sprintf("\nDerivation %d:\n", i+1))
		renderNodeASCII(&sb, root, "", true)
	}

	return sb.String()
}

func renderNodeASCII(sb *strings.Builder, node *DerivationNode, prefix string, isLast bool) {
	// Draw connector
	connector := "├── "
	if isLast {
		connector = "└── "
	}

	// Format source indicator
	sourceIndicator := "[EDB]"
	if node.Source == SourceIDB {
		sourceIndicator = fmt.Sprintf("[IDB:%s]", node.RuleName)
	}

	// Write the node
	sb.WriteString(fmt.Sprintf("%s%s%s %s\n", prefix, connector, node.Fact.String(), sourceIndicator))

	// Prepare prefix for children
	childPrefix := prefix
	if isLast {
		childPrefix += "    "
	} else {
		childPrefix += "│   "
	}

	// Render children
	for i, child := range node.Children {
		renderNodeASCII(sb, child, childPrefix, i == len(node.Children)-1)
	}
}

// RenderJSON renders the proof tree as JSON.
func (trace *DerivationTrace) RenderJSON() ([]byte, error) {
	type jsonNode struct {
		ID       string      `json:"id"`
		ParentID string      `json:"parent_id,omitempty"`
		Fact     string      `json:"fact"`
		Source   string      `json:"source"`
		Rule     string      `json:"rule,omitempty"`
		Depth    int         `json:"depth"`
		Children []*jsonNode `json:"children,omitempty"`
	}

	var convertNode func(*DerivationNode) *jsonNode
	convertNode = func(n *DerivationNode) *jsonNode {
		jn := &jsonNode{
			ID:       n.ID,
			ParentID: n.ParentID,
			Fact:     n.Fact.String(),
			Source:   string(n.Source),
			Rule:     n.RuleName,
			Depth:    n.Depth,
		}
		for _, child := range n.Children {
			jn.Children = append(jn.Children, convertNode(child))
		}
		return jn
	}

	type jsonTrace struct {
		Query    string      `json:"query"`
		Duration string      `json:"duration"`
		Roots    []*jsonNode `json:"roots"`
	}

	jt := jsonTrace{
		Query:    trace.Query,
		Duration: trace.Duration.String(),
	}
	for _, root := range trace.RootNodes {
		jt.Roots = append(jt.Roots, convertNode(root))
	}

	return json.MarshalIndent(jt, "", "  ")
}
