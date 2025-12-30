#!/usr/bin/env python3
"""
Mangle Performance Analysis Tool v1.0

Static analysis to estimate Cartesian explosion risks, identify expensive rules,
and suggest optimizations for Google Mangle programs.

WHAT IS CARTESIAN EXPLOSION?
=============================
Cartesian explosion occurs when predicates in a rule body create a cross-product
before filtering, causing exponential blowup:

    BAD:  result(X, Y) :- big_table(X), big_table(Y), filter(X, Y).
          # 10K rows × 10K rows = 100M combinations before filter!

    GOOD: result(X, Y) :- filter(X, Y), big_table(X), big_table(Y).
          # Only 100 filter matches × 2 lookups = 200 total operations

USAGE
=====
    python profile_rules.py <file.mg> [options]
    python profile_rules.py policy.mg --warn-expensive
    python profile_rules.py policy.mg --threshold medium
    python profile_rules.py *.mg --json
    python profile_rules.py policy.mg --suggest-rewrites
    python profile_rules.py policy.mg --estimate-sizes sizes.json

OPTIONS
=======
    --warn-expensive       Exit with error if high-risk rules found
    --threshold LEVEL      Only show LEVEL+ risk (low/medium/high)
    --json                 Output results as JSON
    --suggest-rewrites     Show rewritten rule suggestions
    --estimate-sizes FILE  Load predicate size estimates from JSON file
    --verbose, -v          Show detailed analysis for all rules
    --output FILE, -o      Write results to file instead of stdout

EXIT CODES
==========
    0 - No high-risk rules found (or --warn-expensive not set)
    1 - High-risk rules found and --warn-expensive set
    2 - Parse error or fatal error

EXAMPLES
========
    # Basic analysis
    python profile_rules.py policy.mg

    # CI/CD check
    python profile_rules.py policy.mg --warn-expensive --threshold medium

    # Full analysis with rewrites
    python profile_rules.py policy.mg -v --suggest-rewrites

    # With size estimates
    echo '{"big_table": 10000, "filter": 100}' > sizes.json
    python profile_rules.py policy.mg --estimate-sizes sizes.json

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


class RiskLevel(Enum):
    LOW = "low"
    MEDIUM = "medium"
    HIGH = "high"


@dataclass
class Variable:
    """Represents a variable in a rule."""
    name: str
    first_occurrence: int  # Index of first predicate where it appears
    predicates: List[str] = field(default_factory=list)  # Predicates that bind it


@dataclass
class PredicateOccurrence:
    """A predicate occurrence in a rule body."""
    name: str
    position: int  # Position in body (0-indexed)
    variables: Set[str]
    is_negated: bool = False
    is_comparison: bool = False
    is_builtin: bool = False


@dataclass
class Rule:
    """Represents a Mangle rule."""
    head: str
    body: str
    full_text: str
    line_start: int
    line_end: int
    head_predicate: str
    body_predicates: List[PredicateOccurrence]
    is_recursive: bool = False
    recursion_depth: int = 0  # 0 = not recursive, 1+ = recursion depth


@dataclass
class PerformanceIssue:
    """A detected performance issue."""
    rule: Rule
    risk_level: RiskLevel
    issue_type: str
    description: str
    estimated_cost: str
    suggestion: str
    rewritten_rule: Optional[str] = None


class PerformanceAnalyzer:
    """
    Analyzes Mangle programs for performance issues.

    Detection patterns:
    1. Cartesian products (multiple predicates with disjoint variables)
    2. Late filtering (comparisons after expensive joins)
    3. Unbounded recursion
    4. Large predicate joins without selective filters
    5. Negation after expensive operations
    """

    # Regex patterns (reused from diagnose_stratification.py)
    PATTERNS = {
        'predicate': re.compile(r'\b([a-z][a-z0-9_]*)\s*\('),
        'negation_bang': re.compile(r'!\s*([a-z][a-z0-9_]*)\s*\('),
        'negation_not': re.compile(r'\bnot\s+([a-z][a-z0-9_]*)\s*\('),
        'rule_arrow': re.compile(r':-|<-|⟸'),
        'decl': re.compile(r'^\s*Decl\s+([a-z][a-z0-9_]*)\s*\('),
        'comment': re.compile(r'#.*$'),
        'string': re.compile(r'"(?:[^"\\]|\\.)*"|\'(?:[^\'\\]|\\.)*\'|`[^`]*`'),
        'variable': re.compile(r'\b([A-Z][a-zA-Z0-9_]*)\b'),
        'comparison': re.compile(r'(<|>|<=|>=|==|!=)'),
        'aggregation': re.compile(r'fn:(Count|Sum|Min|Max|Avg|Collect)'),
    }

    # Built-in predicates
    BUILTINS = {
        'match_cons', 'match_nil', 'match_field', 'match_entry',
        'list:member', 'list_length', 'time_diff',
    }

    # Comparison predicates
    COMPARISONS = {'<', '>', '<=', '>=', '==', '!=', 'lt', 'gt', 'le', 'ge', 'eq', 'ne'}

    def __init__(self, verbose: bool = False, size_estimates: Optional[Dict[str, int]] = None):
        self.verbose = verbose
        self.size_estimates = size_estimates or {}
        self.rules: List[Rule] = []
        self.issues: List[PerformanceIssue] = []
        self.predicate_definitions: Dict[str, List[Rule]] = defaultdict(list)

    def analyze_file(self, filepath: Path) -> bool:
        """Analyze a Mangle file. Returns True if no high-risk issues."""
        with open(filepath, encoding='utf-8') as f:
            content = f.read()
        return self.analyze(content, str(filepath))

    def analyze(self, content: str, source: str = "<string>") -> bool:
        """
        Analyze Mangle source for performance issues.
        Returns True if no high-risk issues found.
        """
        self.rules = []
        self.issues = []
        self.predicate_definitions = defaultdict(list)

        # Phase 1: Parse rules
        self._parse_rules(content)

        # Phase 2: Identify recursion
        self._identify_recursion()

        # Phase 3: Analyze each rule for performance issues
        for rule in self.rules:
            self._analyze_rule(rule)

        # Sort issues by risk level
        risk_order = {RiskLevel.HIGH: 0, RiskLevel.MEDIUM: 1, RiskLevel.LOW: 2}
        self.issues.sort(key=lambda x: (risk_order[x.risk_level], x.rule.line_start))

        return not any(issue.risk_level == RiskLevel.HIGH for issue in self.issues)

    def _parse_rules(self, content: str):
        """Parse content and extract rules."""
        statements = self._split_into_statements(content)

        for stmt, start_line, end_line in statements:
            stmt = stmt.strip()
            if not stmt:
                continue

            # Skip declarations and package statements
            if stmt.startswith('Decl ') or stmt.startswith('Package ') or stmt.startswith('Uses '):
                continue

            # Find rule arrow
            arrow_match = self.PATTERNS['rule_arrow'].search(stmt)
            if not arrow_match:
                continue

            head = stmt[:arrow_match.start()].strip()
            body = stmt[arrow_match.end():].strip()

            # Remove trailing period
            if body.endswith('.'):
                body = body[:-1].strip()

            rule = self._parse_rule(head, body, stmt, start_line, end_line)
            if rule:
                self.rules.append(rule)
                self.predicate_definitions[rule.head_predicate].append(rule)

    def _split_into_statements(self, content: str) -> List[Tuple[str, int, int]]:
        """
        Split content into statements, handling multi-line rules.
        Returns list of (statement_text, start_line, end_line).
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
                start_line = i
                in_statement = True

            current_stmt.append(stripped)

            # Check if statement is complete
            if stripped.endswith('.') or stripped.endswith('!'):
                full_stmt = ' '.join(current_stmt)
                statements.append((full_stmt, start_line, i))
                current_stmt = []
                in_statement = False

        # Handle any incomplete statement
        if current_stmt:
            full_stmt = ' '.join(current_stmt)
            statements.append((full_stmt, start_line, len(lines)))

        return statements

    def _parse_rule(self, head: str, body: str, full_text: str,
                    start_line: int, end_line: int) -> Optional[Rule]:
        """Parse a single rule into structured form."""
        # Extract head predicate
        head_match = self.PATTERNS['predicate'].match(head)
        if not head_match:
            return None

        head_pred = head_match.group(1)

        # Parse body predicates
        body_predicates = self._parse_body(body)

        return Rule(
            head=head,
            body=body,
            full_text=full_text,
            line_start=start_line,
            line_end=end_line,
            head_predicate=head_pred,
            body_predicates=body_predicates,
        )

    def _split_at_top_level_commas(self, text: str) -> List[str]:
        """Split text at commas, but only at top level (not inside parentheses)."""
        parts = []
        current = []
        paren_depth = 0

        for char in text:
            if char == '(':
                paren_depth += 1
                current.append(char)
            elif char == ')':
                paren_depth -= 1
                current.append(char)
            elif char == ',' and paren_depth == 0:
                # Top-level comma - split here
                parts.append(''.join(current).strip())
                current = []
            else:
                current.append(char)

        # Add last part
        if current:
            parts.append(''.join(current).strip())

        return parts

    def _parse_body(self, body: str) -> List[PredicateOccurrence]:
        """Parse body into predicate occurrences."""
        # Remove strings
        body_clean = self.PATTERNS['string'].sub('""', body)

        predicates = []
        position = 0

        # Split by comma, but only at top level (not inside parens)
        parts = self._split_at_top_level_commas(body_clean)

        for part in parts:
            # Check for negation
            is_negated = False
            for neg_pattern in [self.PATTERNS['negation_bang'], self.PATTERNS['negation_not']]:
                if neg_pattern.search(part):
                    is_negated = True
                    break

            # Check for comparison
            is_comparison = bool(self.PATTERNS['comparison'].search(part))

            # Extract predicate name
            pred_match = self.PATTERNS['predicate'].search(part)
            if not pred_match:
                # Might be a comparison like "X < Y"
                if is_comparison:
                    variables = set(self.PATTERNS['variable'].findall(part))
                    predicates.append(PredicateOccurrence(
                        name='<comparison>',
                        position=position,
                        variables=variables,
                        is_comparison=True,
                    ))
                    position += 1
                continue

            pred_name = pred_match.group(1)

            # Extract variables from this predicate
            # Find the predicate call: pred_name(args)
            pred_start = pred_match.start()
            paren_count = 0
            pred_end = pred_start
            found_open = False

            for i in range(pred_start, len(part)):
                if part[i] == '(':
                    paren_count += 1
                    found_open = True
                elif part[i] == ')':
                    paren_count -= 1
                    if paren_count == 0:
                        pred_end = i + 1
                        break

            if found_open:
                pred_call = part[pred_start:pred_end]
                variables = set(self.PATTERNS['variable'].findall(pred_call))
            else:
                variables = set()

            is_builtin = pred_name in self.BUILTINS

            predicates.append(PredicateOccurrence(
                name=pred_name,
                position=position,
                variables=variables,
                is_negated=is_negated,
                is_comparison=is_comparison,
                is_builtin=is_builtin,
            ))
            position += 1

        return predicates

    def _identify_recursion(self):
        """Identify recursive rules and compute recursion depth."""
        # Build dependency graph
        depends_on: Dict[str, Set[str]] = defaultdict(set)

        for rule in self.rules:
            for pred in rule.body_predicates:
                if not pred.is_builtin:
                    depends_on[rule.head_predicate].add(pred.name)

        # Check each rule for recursion
        for rule in self.rules:
            if self._is_recursive(rule.head_predicate, rule.head_predicate, depends_on, set()):
                rule.is_recursive = True
                rule.recursion_depth = self._compute_recursion_depth(
                    rule.head_predicate, depends_on, set()
                )

    def _is_recursive(self, start: str, current: str, graph: Dict[str, Set[str]],
                      visited: Set[str]) -> bool:
        """Check if a predicate is recursive (depends on itself)."""
        if current in visited:
            return False

        if current in graph:
            if start in graph[current]:
                return True

            visited.add(current)
            for neighbor in graph[current]:
                if self._is_recursive(start, neighbor, graph, visited):
                    return True

        return False

    def _compute_recursion_depth(self, pred: str, graph: Dict[str, Set[str]],
                                 visited: Set[str], depth: int = 0) -> int:
        """Estimate recursion depth (conservative heuristic)."""
        if pred in visited:
            return depth

        if depth > 10:  # Cap at 10 to prevent infinite loops
            return 10

        visited.add(pred)
        max_depth = depth

        if pred in graph:
            for neighbor in graph[pred]:
                neighbor_depth = self._compute_recursion_depth(neighbor, graph, visited, depth + 1)
                max_depth = max(max_depth, neighbor_depth)

        return max_depth

    def _analyze_rule(self, rule: Rule):
        """Analyze a single rule for performance issues."""
        if not rule.body_predicates:
            return

        # Check for various performance patterns
        self._check_cartesian_product(rule)
        self._check_late_filtering(rule)
        self._check_unbounded_recursion(rule)
        self._check_expensive_negation(rule)
        self._check_predicate_ordering(rule)

    def _check_cartesian_product(self, rule: Rule):
        """Detect Cartesian product patterns."""
        # Track which variables are bound at each position
        bound_vars = set()

        for i, pred1 in enumerate(rule.body_predicates):
            # Update bound variables for all predicates (including comparisons/builtins)
            # This is important because filter(X, Y) binds both X and Y
            if pred1.is_negated:
                # Negated predicates don't bind variables
                continue

            # Check if this predicate introduces independent variables
            # compared to what's already bound
            new_vars = pred1.variables - bound_vars

            if new_vars and bound_vars and not pred1.is_comparison and not pred1.is_builtin:
                # pred1 introduces new vars - check if they connect to existing vars
                shared_with_bound = pred1.variables & bound_vars

                if not shared_with_bound:
                    # This predicate is independent! This is a Cartesian product
                    # Estimate cost
                    size1 = 1000  # Default estimate for accumulated predicates
                    size2 = self.size_estimates.get(pred1.name, 1000)
                    cost = size1 * size2

                    self.issues.append(PerformanceIssue(
                        rule=rule,
                        risk_level=RiskLevel.HIGH if cost > 100000 else RiskLevel.MEDIUM,
                        issue_type="Cartesian Product",
                        description=f"Predicate '{pred1.name}' at position {i} shares no variables with prior predicates, creating cross-product",
                        estimated_cost=f"O(N x M) where N = prior results, M = |{pred1.name}| ~ {cost:,} combinations",
                        suggestion=f"Reorder predicates to share variables earlier, or add a filtering constraint before '{pred1.name}'",
                        rewritten_rule=None
                    ))

            # Update bound variables
            bound_vars.update(pred1.variables)

    def _check_late_filtering(self, rule: Rule):
        """Detect filtering (comparisons) after expensive joins."""
        # Find comparisons
        comparison_positions = [
            pred.position for pred in rule.body_predicates
            if pred.is_comparison
        ]

        if not comparison_positions:
            return

        # Find first comparison
        first_comparison = min(comparison_positions)

        # Count non-builtin predicates before it
        expensive_preds = [
            pred for pred in rule.body_predicates
            if pred.position < first_comparison
            and not pred.is_builtin
            and not pred.is_negated
        ]

        if len(expensive_preds) >= 3:
            self.issues.append(PerformanceIssue(
                rule=rule,
                risk_level=RiskLevel.MEDIUM,
                issue_type="Late Filtering",
                description=f"Comparison appears after {len(expensive_preds)} joins",
                estimated_cost=f"Filtering {len(expensive_preds)} joined tables instead of filtering early",
                suggestion="Move comparisons earlier in the rule body to reduce intermediate result size",
                rewritten_rule=self._suggest_early_filtering(rule)
            ))

    def _check_unbounded_recursion(self, rule: Rule):
        """Detect potentially unbounded recursion."""
        if not rule.is_recursive:
            return

        # Check if THIS rule is the base case (non-recursive body)
        is_base_case = not any(
            pred.name == rule.head_predicate
            for pred in rule.body_predicates
            if not pred.is_builtin
        )

        if is_base_case:
            # This rule is a base case, no issue
            return

        # This is a recursive rule - check for base case (non-recursive rule for same predicate)
        has_base_case = False
        for other_rule in self.predicate_definitions[rule.head_predicate]:
            if other_rule == rule:
                continue
            # Check if other_rule is non-recursive
            other_is_recursive = any(
                pred.name == rule.head_predicate
                for pred in other_rule.body_predicates
                if not pred.is_builtin
            )
            if not other_is_recursive:
                has_base_case = True
                break

        # Check for depth limiting
        has_depth_limit = any(
            'depth' in str(pred.variables).lower() or
            'count' in str(pred.variables).lower() or
            'limit' in str(pred.variables).lower()
            for pred in rule.body_predicates
        )

        if not has_base_case:
            self.issues.append(PerformanceIssue(
                rule=rule,
                risk_level=RiskLevel.HIGH,
                issue_type="Unbounded Recursion",
                description=f"Recursive rule without apparent base case for '{rule.head_predicate}'",
                estimated_cost="Could diverge or explode on large/cyclic data",
                suggestion="Add a non-recursive base case or termination condition",
            ))
        elif not has_depth_limit and rule.recursion_depth > 3:
            self.issues.append(PerformanceIssue(
                rule=rule,
                risk_level=RiskLevel.MEDIUM,
                issue_type="Deep Recursion",
                description=f"Recursive rule with depth {rule.recursion_depth} without explicit depth limit",
                estimated_cost="Could be expensive on dense graphs",
                suggestion="Consider adding depth limit or visited tracking if graph is large",
            ))

    def _check_expensive_negation(self, rule: Rule):
        """Detect negation after expensive operations."""
        negated_preds = [
            pred for pred in rule.body_predicates if pred.is_negated
        ]

        if not negated_preds:
            return

        for neg_pred in negated_preds:
            # Count expensive predicates before this negation
            expensive_before = [
                pred for pred in rule.body_predicates
                if pred.position < neg_pred.position
                and not pred.is_builtin
                and not pred.is_comparison
            ]

            if len(expensive_before) >= 3:
                self.issues.append(PerformanceIssue(
                    rule=rule,
                    risk_level=RiskLevel.MEDIUM,
                    issue_type="Late Negation",
                    description=f"Negation of '{neg_pred.name}' after {len(expensive_before)} joins",
                    estimated_cost="Computing large intermediate result before checking negation",
                    suggestion="Move negation checks earlier if possible (ensure safety)",
                ))

    def _check_predicate_ordering(self, rule: Rule):
        """Check if predicates are ordered from most to least selective."""
        # This is a heuristic: smaller predicates should come first
        non_builtin_preds = [
            pred for pred in rule.body_predicates
            if not pred.is_builtin and not pred.is_comparison
        ]

        if len(non_builtin_preds) < 2:
            return

        # Check ordering based on size estimates
        for i in range(len(non_builtin_preds) - 1):
            curr_pred = non_builtin_preds[i]
            next_pred = non_builtin_preds[i + 1]

            curr_size = self.size_estimates.get(curr_pred.name, 1000)
            next_size = self.size_estimates.get(next_pred.name, 1000)

            # If next predicate is significantly smaller, suggest reordering
            if next_size < curr_size / 10:  # 10x smaller
                self.issues.append(PerformanceIssue(
                    rule=rule,
                    risk_level=RiskLevel.LOW,
                    issue_type="Suboptimal Ordering",
                    description=f"'{next_pred.name}' (est. {next_size} rows) appears after '{curr_pred.name}' (est. {curr_size} rows)",
                    estimated_cost=f"Processing {curr_size} rows before filtering to {next_size}",
                    suggestion=f"Consider moving '{next_pred.name}' before '{curr_pred.name}' if variables allow",
                ))

    def _suggest_reorder_for_cartesian(self, rule: Rule, pos1: int, pos2: int) -> Optional[str]:
        """Suggest a reordered rule to avoid Cartesian product."""
        # Find the connecting predicate
        pred1 = rule.body_predicates[pos1]
        pred2 = rule.body_predicates[pos2]

        for i, pred in enumerate(rule.body_predicates):
            if i > pos2 and (pred.variables & pred1.variables) and (pred.variables & pred2.variables):
                # Found connector - suggest moving it earlier
                new_body_preds = rule.body_predicates.copy()
                connector = new_body_preds.pop(i)

                # Insert after pred1
                insert_pos = pos1 + 1
                new_body_preds.insert(insert_pos, connector)

                return self._format_rewritten_rule(rule, new_body_preds)

        return None

    def _suggest_early_filtering(self, rule: Rule) -> Optional[str]:
        """Suggest moving comparisons earlier."""
        # Find comparisons
        comparisons = [pred for pred in rule.body_predicates if pred.is_comparison]
        non_comparisons = [pred for pred in rule.body_predicates if not pred.is_comparison]

        if not comparisons:
            return None

        # Try to move comparisons right after variables are bound
        new_body = []
        used_comparisons = set()

        for pred in non_comparisons:
            new_body.append(pred)

            # Add any comparisons that can now be evaluated
            for comp in comparisons:
                if comp.position in used_comparisons:
                    continue

                # Check if all variables in comparison are bound
                bound_vars = set()
                for p in new_body:
                    bound_vars.update(p.variables)

                if comp.variables.issubset(bound_vars):
                    new_body.append(comp)
                    used_comparisons.add(comp.position)

        # Add any remaining comparisons at the end
        for comp in comparisons:
            if comp.position not in used_comparisons:
                new_body.append(comp)

        return self._format_rewritten_rule(rule, new_body)

    def _format_rewritten_rule(self, rule: Rule, new_body_preds: List[PredicateOccurrence]) -> str:
        """Format a rewritten rule."""
        # This is a simplified version - in practice, we'd need to parse
        # the original text more carefully to preserve formatting
        body_parts = []
        for pred in new_body_preds:
            if pred.is_comparison:
                body_parts.append("<comparison>")
            elif pred.is_negated:
                body_parts.append(f"!{pred.name}(...)")
            else:
                body_parts.append(f"{pred.name}(...)")

        return f"{rule.head} :- {', '.join(body_parts)}."

    def get_report(self, source: str = "<string>", threshold: Optional[RiskLevel] = None,
                   suggest_rewrites: bool = False) -> str:
        """Generate human-readable report."""
        lines = []
        lines.append("=" * 70)
        lines.append("MANGLE PERFORMANCE ANALYSIS")
        lines.append(f"Source: {source}")
        lines.append("=" * 70)

        # Summary
        lines.append(f"\nRules analyzed: {len(self.rules)}")

        risk_counts = {
            RiskLevel.HIGH: sum(1 for i in self.issues if i.risk_level == RiskLevel.HIGH),
            RiskLevel.MEDIUM: sum(1 for i in self.issues if i.risk_level == RiskLevel.MEDIUM),
            RiskLevel.LOW: sum(1 for i in self.issues if i.risk_level == RiskLevel.LOW),
        }

        lines.append(f"\nISSUES FOUND:")
        lines.append(f"  High risk:   {risk_counts[RiskLevel.HIGH]}")
        lines.append(f"  Medium risk: {risk_counts[RiskLevel.MEDIUM]}")
        lines.append(f"  Low risk:    {risk_counts[RiskLevel.LOW]}")

        # Filter by threshold
        filtered_issues = self.issues
        if threshold:
            threshold_order = {RiskLevel.LOW: 0, RiskLevel.MEDIUM: 1, RiskLevel.HIGH: 2}
            min_level = threshold_order[threshold]
            filtered_issues = [
                i for i in self.issues
                if threshold_order[i.risk_level] >= min_level
            ]

        if not filtered_issues:
            lines.append("\nNo issues found at the specified threshold.")
            lines.append("=" * 70)
            return "\n".join(lines)

        # Detailed issues
        lines.append("\n" + "=" * 70)
        lines.append("DETAILED ANALYSIS")
        lines.append("=" * 70)

        for issue in filtered_issues:
            lines.append(f"\n[RISK: {issue.risk_level.value.upper()}] "
                        f"Line {issue.rule.line_start}-{issue.rule.line_end}: "
                        f"{issue.rule.head_predicate}")
            lines.append(f"  Type: {issue.issue_type}")
            lines.append(f"\n  Rule:")
            for line in issue.rule.full_text.split('\n'):
                lines.append(f"    {line}")

            lines.append(f"\n  ISSUE: {issue.description}")
            lines.append(f"  ESTIMATED COST: {issue.estimated_cost}")
            lines.append(f"  SUGGESTION: {issue.suggestion}")

            if suggest_rewrites and issue.rewritten_rule:
                lines.append(f"\n  REWRITTEN:")
                lines.append(f"    {issue.rewritten_rule}")

        # Optimization summary
        lines.append("\n" + "=" * 70)
        lines.append("OPTIMIZATION SUMMARY")
        lines.append("=" * 70)

        if risk_counts[RiskLevel.HIGH] > 0:
            lines.append(f"\n⚠️  {risk_counts[RiskLevel.HIGH]} HIGH-RISK rules need immediate attention")
            lines.append("   These rules may cause severe performance degradation.")

        if risk_counts[RiskLevel.MEDIUM] > 0:
            lines.append(f"\n⚡ {risk_counts[RiskLevel.MEDIUM]} MEDIUM-RISK rules could be optimized")
            lines.append("   Consider reordering predicates or adding indexes.")

        if risk_counts[RiskLevel.LOW] > 0:
            lines.append(f"\n💡 {risk_counts[RiskLevel.LOW]} LOW-RISK optimization opportunities")
            lines.append("   Minor improvements for better performance.")

        lines.append("\n" + "=" * 70)
        return "\n".join(lines)

    def get_json_result(self) -> dict:
        """Return analysis results as JSON."""
        return {
            'summary': {
                'total_rules': len(self.rules),
                'issues': {
                    'high': sum(1 for i in self.issues if i.risk_level == RiskLevel.HIGH),
                    'medium': sum(1 for i in self.issues if i.risk_level == RiskLevel.MEDIUM),
                    'low': sum(1 for i in self.issues if i.risk_level == RiskLevel.LOW),
                },
            },
            'issues': [
                {
                    'rule': {
                        'head': issue.rule.head_predicate,
                        'line_start': issue.rule.line_start,
                        'line_end': issue.rule.line_end,
                        'text': issue.rule.full_text,
                    },
                    'risk_level': issue.risk_level.value,
                    'type': issue.issue_type,
                    'description': issue.description,
                    'estimated_cost': issue.estimated_cost,
                    'suggestion': issue.suggestion,
                    'rewritten_rule': issue.rewritten_rule,
                }
                for issue in self.issues
            ],
        }


def main():
    # Set UTF-8 encoding for stdout on Windows
    if sys.platform == 'win32':
        import io
        sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding='utf-8')
        sys.stderr = io.TextIOWrapper(sys.stderr.buffer, encoding='utf-8')

    parser = argparse.ArgumentParser(
        description="Analyze Mangle programs for performance issues",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog=__doc__
    )
    parser.add_argument('files', nargs='+', help='Mangle files to analyze')
    parser.add_argument('--warn-expensive', action='store_true',
                       help='Exit with error if high-risk rules found')
    parser.add_argument('--threshold', choices=['low', 'medium', 'high'],
                       help='Only show issues at or above this level')
    parser.add_argument('--json', '-j', action='store_true',
                       help='Output results as JSON')
    parser.add_argument('--suggest-rewrites', action='store_true',
                       help='Show rewritten rule suggestions')
    parser.add_argument('--estimate-sizes', type=Path,
                       help='Load predicate size estimates from JSON file')
    parser.add_argument('--verbose', '-v', action='store_true',
                       help='Show detailed analysis for all rules')
    parser.add_argument('--output', '-o', type=Path,
                       help='Write results to file instead of stdout')

    args = parser.parse_args()

    # Load size estimates if provided
    size_estimates = {}
    if args.estimate_sizes:
        try:
            with open(args.estimate_sizes) as f:
                size_estimates = json.load(f)
        except Exception as e:
            print(f"Error loading size estimates: {e}", file=sys.stderr)
            sys.exit(2)

    # Process each file
    all_results = []
    has_high_risk = False

    for filepath in args.files:
        path = Path(filepath)
        if not path.exists():
            print(f"Error: File not found: {filepath}", file=sys.stderr)
            sys.exit(2)

        analyzer = PerformanceAnalyzer(verbose=args.verbose, size_estimates=size_estimates)

        try:
            no_high_risk = analyzer.analyze_file(path)
            if not no_high_risk:
                has_high_risk = True

            all_results.append({
                'file': str(path),
                'analyzer': analyzer,
            })

        except Exception as e:
            print(f"Error analyzing {filepath}: {e}", file=sys.stderr)
            sys.exit(2)

    # Generate output
    if args.json:
        output = {
            'files': [
                {
                    'file': r['file'],
                    'results': r['analyzer'].get_json_result(),
                }
                for r in all_results
            ]
        }
        output_text = json.dumps(output, indent=2)
    else:
        # Text report
        threshold_level = None
        if args.threshold:
            threshold_level = RiskLevel(args.threshold)

        output_parts = []
        for r in all_results:
            report = r['analyzer'].get_report(
                r['file'],
                threshold=threshold_level,
                suggest_rewrites=args.suggest_rewrites
            )
            output_parts.append(report)

        output_text = "\n\n".join(output_parts)

    # Write output
    if args.output:
        with open(args.output, 'w') as f:
            f.write(output_text)
        print(f"Results written to {args.output}")
    else:
        print(output_text)

    # Exit code
    if args.warn_expensive and has_high_risk:
        sys.exit(1)
    else:
        sys.exit(0)


if __name__ == "__main__":
    main()
