#!/usr/bin/env python3
"""
Mangle Stratification Diagnostic Tool v1.0

Analyzes Google Mangle programs for stratification issues - the #1 cause of
"unsafe" or "cannot compute fixpoint" errors in Datalog programs.

WHAT IS STRATIFICATION?
=======================
Stratification ensures negation has well-defined semantics. A program is
stratified if predicates can be partitioned into "strata" where:
  - Positive dependencies: can go to same or higher strata
  - Negative dependencies: must go to STRICTLY LOWER strata

A stratification VIOLATION occurs when a negative dependency creates a cycle:
  A -> !B -> A  (A depends negatively on B, B depends on A)

USAGE
=====
    python3 diagnose_stratification.py <mangle_file> [options]
    python3 diagnose_stratification.py --check-string "<mangle_code>"

OPTIONS
=======
    --verbose, -v     Show detailed dependency analysis
    --graph, -g       Output dependency graph in DOT format
    --json, -j        Output results as JSON
    --fix             Suggest automated fixes

EXIT CODES
==========
    0 - Program is stratified (no issues)
    1 - Stratification violations found
    2 - Parse error or fatal error

EXAMPLES
========
    # Check a file
    python3 diagnose_stratification.py policy.mg

    # Get detailed analysis with graph
    python3 diagnose_stratification.py policy.mg -v -g > deps.dot

    # Check inline code
    python3 diagnose_stratification.py --check-string "bad(X) :- !bad(X)."

Compatible with Mangle v0.4.0 (November 2024)
"""

import sys
import re
import argparse
import json
from pathlib import Path
from dataclasses import dataclass, field
from typing import List, Dict, Set, Optional, Tuple
from collections import defaultdict
from enum import Enum


class EdgeType(Enum):
    POSITIVE = "positive"
    NEGATIVE = "negative"
    AGGREGATION = "aggregation"


@dataclass
class Edge:
    """Dependency edge in the predicate graph."""
    source: str       # Predicate that depends
    target: str       # Predicate being depended on
    edge_type: EdgeType
    line: int
    context: str = ""  # The rule fragment where this dependency appears


@dataclass
class Predicate:
    """Information about a predicate."""
    name: str
    arity: int = -1
    defined_at: List[int] = field(default_factory=list)  # Lines where rules define this
    used_at: List[int] = field(default_factory=list)     # Lines where this is used
    is_edb: bool = False  # Base fact (extensional)
    is_idb: bool = False  # Derived (intensional)
    stratum: int = -1     # Assigned stratum (-1 = not computed)


@dataclass
class StratificationViolation:
    """A detected stratification problem."""
    cycle: List[str]         # Predicates in the cycle
    negative_edges: List[Edge]  # Negative edges causing the violation
    lines: List[int]         # Source lines involved
    severity: str            # "error" or "warning"
    message: str
    suggestion: str


class StratificationAnalyzer:
    """
    Analyzes Mangle programs for stratification issues.

    Algorithm:
    1. Parse program to extract predicates and dependencies
    2. Build dependency graph with edge types (positive/negative)
    3. Find strongly connected components (SCCs)
    4. Check each SCC for negative edges (violation!)
    5. Compute stratum assignment via topological sort
    """

    # Regex patterns for parsing
    PATTERNS = {
        'predicate': re.compile(r'\b([a-z][a-z0-9_]*)\s*\('),
        'negation_bang': re.compile(r'!\s*([a-z][a-z0-9_]*)\s*\('),
        'negation_not': re.compile(r'\bnot\s+([a-z][a-z0-9_]*)\s*\('),
        'rule_arrow': re.compile(r':-|<-|⟸'),
        'decl': re.compile(r'^\s*Decl\s+([a-z][a-z0-9_]*)\s*\('),
        'pipeline': re.compile(r'\|>'),
        'aggregation_fn': re.compile(r'fn:(Count|Sum|Min|Max|Avg|Collect)'),
        'comment': re.compile(r'#.*$'),
        'string': re.compile(r'"(?:[^"\\]|\\.)*"|\'(?:[^\'\\]|\\.)*\'|`[^`]*`'),
    }

    # Built-in predicates that don't need to be tracked
    BUILTINS = {
        'match_cons', 'match_nil', 'match_field', 'match_entry',
        'list:member', 'list_length', 'time_diff',
    }

    def __init__(self, verbose: bool = False):
        self.verbose = verbose
        self.predicates: Dict[str, Predicate] = {}
        self.edges: List[Edge] = []
        self.violations: List[StratificationViolation] = []
        self.strata: Dict[str, int] = {}

    def analyze_file(self, filepath: Path) -> bool:
        """Analyze a Mangle file. Returns True if stratified."""
        with open(filepath, encoding='utf-8') as f:
            content = f.read()
        return self.analyze(content, str(filepath))

    def analyze(self, content: str, source: str = "<string>") -> bool:
        """
        Analyze Mangle source for stratification issues.
        Returns True if the program is properly stratified.
        """
        self.predicates = {}
        self.edges = []
        self.violations = []
        self.strata = {}

        # Phase 1: Extract predicates and dependencies
        self._extract_dependencies(content)

        # Phase 2: Build graph and find SCCs
        sccs = self._find_sccs()

        # Phase 3: Check each SCC for negative edges
        self._check_sccs_for_violations(sccs)

        # Phase 4: Compute strata (if no violations)
        if not self.violations:
            self._compute_strata()

        return len(self.violations) == 0

    def _extract_dependencies(self, content: str):
        """Parse content and extract predicate dependencies."""
        # First, join multi-line statements
        statements = self._split_into_statements(content)

        for stmt, start_line in statements:
            # Skip empty statements
            stmt = stmt.strip()
            if not stmt:
                continue

            # Skip declarations (they define schema, not dependencies)
            if stmt.startswith('Decl '):
                decl_match = self.PATTERNS['decl'].match(stmt)
                if decl_match:
                    pred_name = decl_match.group(1)
                    self._register_predicate(pred_name, start_line, is_edb=True)
                continue

            # Skip package statements
            if stmt.startswith('Package ') or stmt.startswith('Uses '):
                continue

            # Find rule arrow to separate head from body
            arrow_match = self.PATTERNS['rule_arrow'].search(stmt)

            if arrow_match:
                # It's a rule
                head = stmt[:arrow_match.start()]
                body = stmt[arrow_match.end():]
                self._analyze_rule(head, body, start_line, stmt)
            elif stmt.endswith('.') and '(' in stmt:
                # It's a fact (EDB)
                pred_match = self.PATTERNS['predicate'].match(stmt)
                if pred_match:
                    pred_name = pred_match.group(1)
                    self._register_predicate(pred_name, start_line, is_edb=True)

    def _split_into_statements(self, content: str) -> List[Tuple[str, int]]:
        """
        Split content into statements, handling multi-line rules.
        Returns list of (statement_text, start_line).
        """
        statements = []
        lines = content.split('\n')

        current_stmt = []
        start_line = 1
        in_statement = False

        for i, line in enumerate(lines, 1):
            # Remove comments
            line = self.PATTERNS['comment'].sub('', line)
            stripped = line.strip()

            if not stripped:
                continue

            if not in_statement:
                # Starting a new statement
                start_line = i
                in_statement = True

            current_stmt.append(stripped)

            # Check if statement is complete (ends with period or !)
            if stripped.endswith('.') or stripped.endswith('!'):
                full_stmt = ' '.join(current_stmt)
                statements.append((full_stmt, start_line))
                current_stmt = []
                in_statement = False

        # Handle any incomplete statement at end
        if current_stmt:
            full_stmt = ' '.join(current_stmt)
            statements.append((full_stmt, start_line))

        return statements

    def _analyze_rule(self, head: str, body: str, line: int, full_rule: str):
        """Analyze a single rule for dependencies."""
        # Remove strings to avoid false matches
        body_clean = self.PATTERNS['string'].sub('""', body)

        # Extract head predicate
        head_match = self.PATTERNS['predicate'].match(head.strip())
        if not head_match:
            return

        head_pred = head_match.group(1)
        self._register_predicate(head_pred, line, is_idb=True)

        # Check for aggregation (affects stratification)
        has_aggregation = bool(self.PATTERNS['aggregation_fn'].search(body_clean))

        # Find all positive predicate uses (excluding negated ones)
        # First, identify positions of negated predicates
        neg_positions = set()
        for pattern in [self.PATTERNS['negation_bang'], self.PATTERNS['negation_not']]:
            for match in pattern.finditer(body_clean):
                neg_positions.add(match.start())

        # Find all predicates in body
        for match in self.PATTERNS['predicate'].finditer(body_clean):
            pred_name = match.group(1)

            if pred_name in self.BUILTINS:
                continue

            # Check if this occurrence is negated
            is_negated = False
            for neg_pattern in [self.PATTERNS['negation_bang'], self.PATTERNS['negation_not']]:
                for neg_match in neg_pattern.finditer(body_clean):
                    if neg_match.group(1) == pred_name and abs(neg_match.start() - match.start()) < 5:
                        is_negated = True
                        break

            self._register_predicate(pred_name, line)

            edge_type = EdgeType.NEGATIVE if is_negated else EdgeType.POSITIVE
            if has_aggregation and edge_type == EdgeType.POSITIVE:
                edge_type = EdgeType.AGGREGATION

            self.edges.append(Edge(
                source=head_pred,
                target=pred_name,
                edge_type=edge_type,
                line=line,
                context=full_rule[:80] + ("..." if len(full_rule) > 80 else "")
            ))

        # Also explicitly find negated predicates
        for pattern in [self.PATTERNS['negation_bang'], self.PATTERNS['negation_not']]:
            for match in pattern.finditer(body_clean):
                neg_pred = match.group(1)
                if neg_pred in self.BUILTINS:
                    continue

                self._register_predicate(neg_pred, line)

                # Add negative edge (might duplicate, but edges list handles it)
                self.edges.append(Edge(
                    source=head_pred,
                    target=neg_pred,
                    edge_type=EdgeType.NEGATIVE,
                    line=line,
                    context=full_rule[:80] + ("..." if len(full_rule) > 80 else "")
                ))

    def _register_predicate(self, name: str, line: int,
                           is_edb: bool = False, is_idb: bool = False):
        """Register a predicate occurrence."""
        if name not in self.predicates:
            self.predicates[name] = Predicate(name=name)

        pred = self.predicates[name]
        if is_edb:
            pred.is_edb = True
            pred.defined_at.append(line)
        if is_idb:
            pred.is_idb = True
            pred.defined_at.append(line)
        pred.used_at.append(line)

    def _find_sccs(self) -> List[Set[str]]:
        """Find strongly connected components using Tarjan's algorithm."""
        # Build adjacency list
        graph: Dict[str, Set[str]] = defaultdict(set)
        for edge in self.edges:
            graph[edge.source].add(edge.target)

        # Tarjan's algorithm
        index_counter = [0]
        stack = []
        lowlinks = {}
        index = {}
        on_stack = {}
        sccs = []

        def strongconnect(v):
            index[v] = index_counter[0]
            lowlinks[v] = index_counter[0]
            index_counter[0] += 1
            stack.append(v)
            on_stack[v] = True

            for w in graph.get(v, []):
                if w not in index:
                    strongconnect(w)
                    lowlinks[v] = min(lowlinks[v], lowlinks[w])
                elif on_stack.get(w, False):
                    lowlinks[v] = min(lowlinks[v], index[w])

            if lowlinks[v] == index[v]:
                scc = set()
                while True:
                    w = stack.pop()
                    on_stack[w] = False
                    scc.add(w)
                    if w == v:
                        break
                if len(scc) > 1 or v in graph.get(v, []):
                    sccs.append(scc)

        for v in list(self.predicates.keys()):
            if v not in index:
                strongconnect(v)

        return sccs

    def _check_sccs_for_violations(self, sccs: List[Set[str]]):
        """Check each SCC for negative edges (stratification violations)."""
        for scc in sccs:
            # Find negative edges within this SCC
            neg_edges_in_scc = []
            for edge in self.edges:
                if (edge.source in scc and
                    edge.target in scc and
                    edge.edge_type == EdgeType.NEGATIVE):
                    neg_edges_in_scc.append(edge)

            if neg_edges_in_scc:
                # Find the cycle path for better diagnostics
                cycle_path = self._find_cycle_path(scc, neg_edges_in_scc)
                lines = sorted(set(e.line for e in neg_edges_in_scc))

                # Determine severity
                severity = "error"

                # Build message
                predicates_str = " -> ".join(cycle_path) if cycle_path else ", ".join(sorted(scc))
                neg_edges_str = ", ".join(f"{e.source} -> !{e.target}" for e in neg_edges_in_scc[:3])

                violation = StratificationViolation(
                    cycle=cycle_path if cycle_path else list(scc),
                    negative_edges=neg_edges_in_scc,
                    lines=lines,
                    severity=severity,
                    message=f"Negative cycle detected: {predicates_str}. "
                           f"Negative edges: {neg_edges_str}",
                    suggestion=self._generate_fix_suggestion(scc, neg_edges_in_scc)
                )
                self.violations.append(violation)

    def _find_cycle_path(self, scc: Set[str], neg_edges: List[Edge]) -> List[str]:
        """Find a representative cycle path through the SCC."""
        if not neg_edges:
            return list(scc)

        # Build local graph
        graph: Dict[str, Set[str]] = defaultdict(set)
        for edge in self.edges:
            if edge.source in scc and edge.target in scc:
                graph[edge.source].add(edge.target)

        # Find path from first neg edge source back to itself
        start = neg_edges[0].source
        visited = {start}
        path = [start]
        current = neg_edges[0].target

        while current != start and current in scc:
            if current in visited:
                # We're looping but not back to start
                break
            path.append(current)
            visited.add(current)

            # Find next node
            neighbors = graph.get(current, set())
            next_node = None
            for n in neighbors:
                if n == start:
                    next_node = n
                    break
                if n not in visited:
                    next_node = n
                    break

            if next_node is None:
                break
            current = next_node

        if current == start:
            path.append(start)

        return path

    def _generate_fix_suggestion(self, scc: Set[str], neg_edges: List[Edge]) -> str:
        """Generate a suggestion for fixing the stratification violation."""
        suggestions = []

        # Pattern 1: Direct negative self-reference
        for edge in neg_edges:
            if edge.source == edge.target:
                suggestions.append(
                    f"DIRECT SELF-NEGATION: '{edge.source}' depends on !{edge.target}\n"
                    f"  Line {edge.line}: {edge.context}\n"
                    f"  FIX: This pattern has no stable truth value. Consider:\n"
                    f"       1. Use a base case with explicit facts\n"
                    f"       2. Split into separate predicates\n"
                    f"       3. Use a helper predicate with different semantics"
                )

        # Pattern 2: Mutual recursion through negation
        if len(scc) == 2 and not any(e.source == e.target for e in neg_edges):
            preds = list(scc)
            suggestions.append(
                f"MUTUAL NEGATION: '{preds[0]}' and '{preds[1]}' are mutually recursive\n"
                f"  through negation, creating an unstable definition.\n"
                f"  FIX: Break the cycle by:\n"
                f"       1. Making one predicate EDB (ground facts only)\n"
                f"       2. Adding a base case that doesn't depend on the other\n"
                f"       3. Using explicit strata with different predicate names"
            )

        # Pattern 3: Complex cycle (game theory, etc.)
        if len(scc) > 2:
            cycle_str = " -> ".join(list(scc)[:4]) + (" -> ..." if len(scc) > 4 else "")
            suggestions.append(
                f"COMPLEX NEGATIVE CYCLE involving {len(scc)} predicates:\n"
                f"  {cycle_str}\n"
                f"  FIX: This often occurs in game-theoretic definitions.\n"
                f"       Consider these approaches:\n"
                f"       1. Add termination conditions (terminal states)\n"
                f"       2. Use bounded depth/iteration counters\n"
                f"       3. Restructure to use well-founded semantics\n"
                f"       4. Split into multiple stratified levels"
            )

        # Pattern 4: Classic winning/losing game
        if 'winning' in scc or 'losing' in scc:
            suggestions.append(
                "GAME THEORY PATTERN DETECTED:\n"
                "  The classic 'winning(X) :- move(X,Y), losing(Y)' with\n"
                "  'losing(X) :- !winning(X)' is inherently non-stratified.\n"
                "  FIX:\n"
                "    # Define terminal positions first (base case)\n"
                "    losing(X) :- position(X), !has_move(X).\n"
                "    has_move(X) :- move(X, _).\n"
                "    # Then winning depends only on computed losing\n"
                "    winning(X) :- move(X, Y), losing(Y)."
            )

        # Add the helper pattern suggestion
        helper_predicates = []
        for edge in neg_edges:
            helper_predicates.append(
                f"    # Helper for safe negation of {edge.target}\n"
                f"    has_{edge.target}() :- {edge.target}(_).  # Or appropriate args\n"
                f"    # Then use: !has_{edge.target}() instead of !{edge.target}(_)"
            )

        if helper_predicates and len(suggestions) < 3:
            suggestions.append(
                "HELPER PATTERN: Create helper predicates for safe negation:\n" +
                "\n".join(helper_predicates[:3])
            )

        return "\n\n".join(suggestions) if suggestions else "Review the cycle and break it by introducing base cases or restructuring predicates."

    def _compute_strata(self):
        """
        Compute stratum assignment for each predicate.
        Predicates with no dependencies are in stratum 0.
        """
        # Build dependency graph
        pos_deps: Dict[str, Set[str]] = defaultdict(set)
        neg_deps: Dict[str, Set[str]] = defaultdict(set)

        for edge in self.edges:
            if edge.edge_type == EdgeType.NEGATIVE:
                neg_deps[edge.source].add(edge.target)
            else:
                pos_deps[edge.source].add(edge.target)

        # Initialize strata
        for pred in self.predicates:
            self.strata[pred] = 0

        # Fixed-point iteration
        changed = True
        iterations = 0
        max_iterations = len(self.predicates) + 10

        while changed and iterations < max_iterations:
            changed = False
            iterations += 1

            for pred in self.predicates:
                # Positive deps: same or higher stratum
                for dep in pos_deps.get(pred, []):
                    if dep in self.strata:
                        if self.strata[dep] > self.strata[pred]:
                            self.strata[pred] = self.strata[dep]
                            changed = True

                # Negative deps: strictly higher stratum
                for dep in neg_deps.get(pred, []):
                    if dep in self.strata:
                        if self.strata[dep] >= self.strata[pred]:
                            self.strata[pred] = self.strata[dep] + 1
                            changed = True

        # Update predicate objects
        for name, stratum in self.strata.items():
            if name in self.predicates:
                self.predicates[name].stratum = stratum

    def get_report(self, source: str = "<string>") -> str:
        """Generate a human-readable report."""
        lines = []
        lines.append("=" * 70)
        lines.append("MANGLE STRATIFICATION ANALYSIS")
        lines.append(f"Source: {source}")
        lines.append("=" * 70)

        # Summary
        lines.append(f"\nPredicates analyzed: {len(self.predicates)}")
        lines.append(f"Dependencies found: {len(self.edges)}")
        neg_count = sum(1 for e in self.edges if e.edge_type == EdgeType.NEGATIVE)
        lines.append(f"Negative dependencies: {neg_count}")

        # Violations
        if self.violations:
            lines.append(f"\n{'!'*60}")
            lines.append(f"STRATIFICATION VIOLATIONS: {len(self.violations)}")
            lines.append(f"{'!'*60}")

            for i, v in enumerate(self.violations, 1):
                lines.append(f"\n--- Violation #{i} ({v.severity.upper()}) ---")
                lines.append(f"Message: {v.message}")
                lines.append(f"Lines: {', '.join(map(str, v.lines))}")
                lines.append(f"\nSuggested Fix:")
                for line in v.suggestion.split('\n'):
                    lines.append(f"  {line}")
        else:
            lines.append(f"\n{'*'*60}")
            lines.append("RESULT: Program is STRATIFIED")
            lines.append(f"{'*'*60}")

            # Show strata assignment
            if self.strata:
                max_stratum = max(self.strata.values()) if self.strata else 0
                lines.append(f"\nStrata assignment ({max_stratum + 1} strata):")

                by_stratum: Dict[int, List[str]] = defaultdict(list)
                for pred, s in sorted(self.strata.items()):
                    by_stratum[s].append(pred)

                for s in sorted(by_stratum.keys()):
                    preds = sorted(by_stratum[s])
                    if len(preds) <= 10:
                        lines.append(f"  Stratum {s}: {', '.join(preds)}")
                    else:
                        lines.append(f"  Stratum {s}: {', '.join(preds[:10])} ... ({len(preds)} total)")

        # Verbose output
        if self.verbose:
            lines.append("\n" + "-" * 60)
            lines.append("DETAILED DEPENDENCY ANALYSIS")
            lines.append("-" * 60)

            # Group edges by source predicate
            by_source: Dict[str, List[Edge]] = defaultdict(list)
            for edge in self.edges:
                by_source[edge.source].append(edge)

            for source in sorted(by_source.keys()):
                lines.append(f"\n{source}:")
                edges = by_source[source]
                for edge in edges:
                    symbol = "!" if edge.edge_type == EdgeType.NEGATIVE else "+"
                    lines.append(f"  {symbol} {edge.target} (line {edge.line})")

        lines.append("\n" + "=" * 70)
        return "\n".join(lines)

    def get_dot_graph(self) -> str:
        """Generate DOT format graph for visualization."""
        lines = [
            'digraph stratification {',
            '  rankdir=TB;',
            '  node [shape=box, style=filled];',
            ''
        ]

        # Color nodes by stratum
        colors = ['lightblue', 'lightgreen', 'lightyellow', 'lightpink', 'lightgray']
        for pred, stratum in self.strata.items():
            color = colors[stratum % len(colors)]
            lines.append(f'  "{pred}" [fillcolor={color}, label="{pred}\\nS{stratum}"];')

        lines.append('')

        # Add edges
        seen_edges = set()
        for edge in self.edges:
            key = (edge.source, edge.target, edge.edge_type)
            if key in seen_edges:
                continue
            seen_edges.add(key)

            if edge.edge_type == EdgeType.NEGATIVE:
                lines.append(f'  "{edge.source}" -> "{edge.target}" [color=red, style=dashed, label="!"];')
            elif edge.edge_type == EdgeType.AGGREGATION:
                lines.append(f'  "{edge.source}" -> "{edge.target}" [color=blue, label="agg"];')
            else:
                lines.append(f'  "{edge.source}" -> "{edge.target}";')

        lines.append('}')
        return '\n'.join(lines)

    def get_json_result(self) -> dict:
        """Return analysis results as JSON-serializable dict."""
        return {
            'stratified': len(self.violations) == 0,
            'predicates': {
                name: {
                    'stratum': p.stratum,
                    'is_edb': p.is_edb,
                    'is_idb': p.is_idb,
                    'defined_at': p.defined_at,
                }
                for name, p in self.predicates.items()
            },
            'violations': [
                {
                    'cycle': v.cycle,
                    'lines': v.lines,
                    'severity': v.severity,
                    'message': v.message,
                    'suggestion': v.suggestion,
                }
                for v in self.violations
            ],
            'strata': self.strata,
            'edge_count': {
                'positive': sum(1 for e in self.edges if e.edge_type == EdgeType.POSITIVE),
                'negative': sum(1 for e in self.edges if e.edge_type == EdgeType.NEGATIVE),
                'aggregation': sum(1 for e in self.edges if e.edge_type == EdgeType.AGGREGATION),
            }
        }


def main():
    parser = argparse.ArgumentParser(
        description="Diagnose stratification issues in Mangle programs",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog=__doc__
    )
    parser.add_argument('file', nargs='?', help='Mangle file to analyze')
    parser.add_argument('--check-string', '-s', help='Analyze inline Mangle code')
    parser.add_argument('--verbose', '-v', action='store_true', help='Show detailed analysis')
    parser.add_argument('--graph', '-g', action='store_true', help='Output DOT graph')
    parser.add_argument('--json', '-j', action='store_true', help='Output as JSON')
    parser.add_argument('--fix', action='store_true', help='Show fix suggestions (default on)')

    args = parser.parse_args()

    if not args.file and not args.check_string:
        parser.print_help()
        sys.exit(1)

    analyzer = StratificationAnalyzer(verbose=args.verbose)

    if args.check_string:
        is_stratified = analyzer.analyze(args.check_string, "<inline>")
        source = "<inline>"
    else:
        filepath = Path(args.file)
        if not filepath.exists():
            print(f"Error: File not found: {filepath}", file=sys.stderr)
            sys.exit(2)
        is_stratified = analyzer.analyze_file(filepath)
        source = str(filepath)

    # Output results
    if args.json:
        print(json.dumps(analyzer.get_json_result(), indent=2))
    elif args.graph:
        print(analyzer.get_dot_graph())
    else:
        print(analyzer.get_report(source))

    sys.exit(0 if is_stratified else 1)


if __name__ == "__main__":
    main()
