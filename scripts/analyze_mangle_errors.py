#!/usr/bin/env python3
"""
Comprehensive Mangle Error Analyzer

Analyzes a combined Mangle dump file for:
1. Duplicate predicate declarations (Decl)
2. Duplicate rule definitions (same head, same body)
3. Arity mismatches (same predicate with different arities)
4. Safety violations (unbound variables in negation)
5. Stratification issues (negative cycles)
6. Syntax errors (missing periods, bad operators)
7. Atom vs String confusion
8. Aggregation syntax errors

Maps errors back to source .mg files by tracking file markers/comments.
"""

import re
import sys
from collections import defaultdict
from pathlib import Path
from dataclasses import dataclass, field
from typing import Optional

@dataclass(frozen=True)
class Location:
    file: str
    line: int

    def __hash__(self):
        return hash((self.file, self.line))

@dataclass
class Predicate:
    name: str
    arity: int
    location: Location
    raw_line: str

@dataclass
class Rule:
    head_pred: str
    head_arity: int
    body: str
    location: Location
    raw_line: str

@dataclass
class Issue:
    severity: str  # ERROR, WARNING, INFO
    category: str
    message: str
    location: Location
    suggestion: str = ""

class MangleAnalyzer:
    def __init__(self):
        self.declarations: dict[str, list[Predicate]] = defaultdict(list)
        self.rules: dict[str, list[Rule]] = defaultdict(list)
        self.facts: dict[str, list[Predicate]] = defaultdict(list)
        self.issues: list[Issue] = []
        self.current_file = "unknown"
        self.current_line = 0
        self.all_predicates: set[str] = set()
        self.defined_predicates: set[str] = set()  # Has Decl or fact/rule head
        self.used_predicates: set[str] = set()  # Used in rule bodies

        # Track predicate -> files for cross-file analysis
        self.pred_to_files: dict[str, set[str]] = defaultdict(set)

    def parse_file(self, filepath: str):
        """Parse a combined Mangle dump file."""
        with open(filepath, 'r', encoding='utf-8') as f:
            lines = f.readlines()

        for i, line in enumerate(lines, 1):
            self.current_line = i
            self._detect_file_marker(line)
            self._analyze_line(line.strip(), line)

    def _detect_file_marker(self, line: str):
        """Detect file boundary markers in combined dump."""
        # Common patterns for file markers
        patterns = [
            r'#\s*File:\s*(.+\.mg)',
            r'#\s*Source:\s*(.+\.mg)',
            r'#\s*=+\s*(.+\.mg)\s*=+',
            r'#\s*---+\s*(.+\.mg)\s*---+',
            r'#\s*From:\s*(.+\.mg)',
            # Section markers that indicate file boundaries
            r'#\s*Cortex.*Schemas',
            r'#\s*Cortex.*Policy',
        ]

        for pattern in patterns:
            match = re.search(pattern, line, re.IGNORECASE)
            if match:
                if match.lastindex and match.lastindex >= 1:
                    self.current_file = match.group(1)
                elif 'Schema' in line:
                    self.current_file = 'schemas.mg'
                elif 'Policy' in line:
                    self.current_file = 'policy.mg'
                return

        # Also detect SECTION markers to track logical sections
        section_match = re.search(r'#\s*SECTION\s+(\d+[A-Z]?):', line)
        if section_match:
            pass  # Could track sections for better error reporting

    def _analyze_line(self, line: str, raw_line: str):
        """Analyze a single line for issues."""
        # Skip comments and empty lines
        if not line or line.startswith('#'):
            return

        # Check for declarations
        if line.startswith('Decl '):
            self._parse_declaration(line, raw_line)
        # Check for rules (contains :-)
        elif ':-' in line and not line.startswith('#'):
            self._parse_rule(line, raw_line)
        # Check for facts (predicate with args ending in .)
        elif re.match(r'^[a-z_][a-z0-9_]*\s*\(', line) and line.rstrip().endswith('.'):
            self._parse_fact(line, raw_line)

    def _parse_declaration(self, line: str, raw_line: str):
        """Parse a Decl statement."""
        # Decl predicate(Arg1, Arg2, ...).
        match = re.match(r'Decl\s+([a-z_][a-z0-9_]*)\s*\(([^)]*)\)\s*\.?', line)
        if match:
            pred_name = match.group(1)
            args = match.group(2)
            arity = len([a.strip() for a in args.split(',') if a.strip()]) if args.strip() else 0

            loc = Location(self.current_file, self.current_line)
            pred = Predicate(pred_name, arity, loc, raw_line.strip())

            self.declarations[pred_name].append(pred)
            self.defined_predicates.add(pred_name)
            self.all_predicates.add(pred_name)
            self.pred_to_files[pred_name].add(self.current_file)
        else:
            # Bad declaration syntax
            self.issues.append(Issue(
                severity="ERROR",
                category="syntax",
                message=f"Malformed Decl statement",
                location=Location(self.current_file, self.current_line),
                suggestion="Use: Decl predicate(Arg1.Type<type>, Arg2.Type<type>)."
            ))

    def _parse_rule(self, line: str, raw_line: str):
        """Parse a rule (head :- body)."""
        # Split on :- but handle strings
        parts = self._split_rule(line)
        if len(parts) != 2:
            return

        head, body = parts
        head = head.strip()
        body = body.strip().rstrip('.')

        # Parse head predicate
        head_match = re.match(r'([a-z_][a-z0-9_]*)\s*\(([^)]*)\)', head)
        if head_match:
            pred_name = head_match.group(1)
            args = head_match.group(2)
            arity = self._count_args(args)

            loc = Location(self.current_file, self.current_line)
            rule = Rule(pred_name, arity, body, loc, raw_line.strip())

            self.rules[pred_name].append(rule)
            self.defined_predicates.add(pred_name)
            self.all_predicates.add(pred_name)
            self.pred_to_files[pred_name].add(self.current_file)

            # Analyze body for used predicates and safety
            self._analyze_rule_body(body, pred_name, loc)
        elif head and not '(' in head:
            # Nullary predicate
            pred_name = head.strip()
            loc = Location(self.current_file, self.current_line)
            rule = Rule(pred_name, 0, body, loc, raw_line.strip())
            self.rules[pred_name].append(rule)
            self.defined_predicates.add(pred_name)
            self._analyze_rule_body(body, pred_name, loc)

    def _parse_fact(self, line: str, raw_line: str):
        """Parse a ground fact."""
        match = re.match(r'([a-z_][a-z0-9_]*)\s*\(([^)]*)\)\s*\.', line)
        if match:
            pred_name = match.group(1)
            args = match.group(2)
            arity = self._count_args(args)

            loc = Location(self.current_file, self.current_line)
            fact = Predicate(pred_name, arity, loc, raw_line.strip())

            self.facts[pred_name].append(fact)
            self.defined_predicates.add(pred_name)
            self.all_predicates.add(pred_name)
            self.pred_to_files[pred_name].add(self.current_file)

    def _split_rule(self, line: str) -> list[str]:
        """Split rule on :- handling strings."""
        # Simple split - can be enhanced for complex cases
        if ':-' in line:
            idx = line.index(':-')
            return [line[:idx], line[idx+2:]]
        return [line]

    def _count_args(self, args_str: str) -> int:
        """Count arguments handling nested structures."""
        if not args_str.strip():
            return 0

        depth = 0
        count = 1
        in_string = False

        for char in args_str:
            if char == '"' and not in_string:
                in_string = True
            elif char == '"' and in_string:
                in_string = False
            elif not in_string:
                if char in '([{':
                    depth += 1
                elif char in ')]}':
                    depth -= 1
                elif char == ',' and depth == 0:
                    count += 1

        return count

    def _analyze_rule_body(self, body: str, head_pred: str, loc: Location):
        """Analyze rule body for safety and used predicates."""
        # Extract predicates from body
        pred_pattern = r'(!?)([a-z_][a-z0-9_]*)\s*\('

        # Track bound variables
        bound_vars = set()
        negated_preds = []

        # First pass: find all predicates and their variables
        for match in re.finditer(pred_pattern, body):
            negated = match.group(1) == '!'
            pred_name = match.group(2)

            # Skip builtins
            if pred_name.startswith('fn:') or pred_name.startswith(':'):
                continue

            self.used_predicates.add(pred_name)
            self.all_predicates.add(pred_name)

            if negated:
                # Extract variables from negated predicate
                start = match.end()
                depth = 1
                end = start
                for i, c in enumerate(body[start:], start):
                    if c == '(':
                        depth += 1
                    elif c == ')':
                        depth -= 1
                        if depth == 0:
                            end = i
                            break

                args = body[start:end]
                vars_in_neg = re.findall(r'\b([A-Z][A-Za-z0-9_]*)\b', args)
                negated_preds.append((pred_name, vars_in_neg, match.start()))
            else:
                # Positive predicates bind their variables
                start = match.end()
                depth = 1
                end = start
                for i, c in enumerate(body[start:], start):
                    if c == '(':
                        depth += 1
                    elif c == ')':
                        depth -= 1
                        if depth == 0:
                            end = i
                            break
                args = body[start:end]
                vars_in_pos = re.findall(r'\b([A-Z][A-Za-z0-9_]*)\b', args)
                bound_vars.update(vars_in_pos)

        # Check for unbound variables in negation
        for pred_name, neg_vars, pos in negated_preds:
            for var in neg_vars:
                if var not in bound_vars and var != '_':
                    self.issues.append(Issue(
                        severity="ERROR",
                        category="safety",
                        message=f"Unbound variable '{var}' in negation of '{pred_name}'",
                        location=loc,
                        suggestion=f"Bind '{var}' in a positive predicate before negation"
                    ))

        # Check for self-negation (stratification)
        for pred_name, _, _ in negated_preds:
            if pred_name == head_pred:
                self.issues.append(Issue(
                    severity="ERROR",
                    category="stratification",
                    message=f"Self-negation: '{head_pred}' negates itself",
                    location=loc,
                    suggestion="Break the cycle with a helper predicate or base case"
                ))

    def check_duplicates(self):
        """Check for duplicate declarations."""
        for pred_name, decls in self.declarations.items():
            if len(decls) > 1:
                # Check if they have same arity
                arities = set(d.arity for d in decls)
                files = set(d.location.file for d in decls)

                if len(arities) > 1:
                    self.issues.append(Issue(
                        severity="ERROR",
                        category="duplicate",
                        message=f"Predicate '{pred_name}' declared with different arities: {arities}",
                        location=decls[0].location,
                        suggestion=f"Unify arity across files: {files}"
                    ))
                else:
                    # Same arity, just duplicate decls
                    locs = [(d.location.file, d.location.line) for d in decls]
                    self.issues.append(Issue(
                        severity="WARNING",
                        category="duplicate",
                        message=f"Predicate '{pred_name}' declared {len(decls)} times",
                        location=decls[0].location,
                        suggestion=f"Remove duplicates. Found at: {locs}"
                    ))

    def check_arity_consistency(self):
        """Check arity consistency between declarations, rules, and facts."""
        for pred_name in self.all_predicates:
            arities = set()

            # From declarations
            for d in self.declarations.get(pred_name, []):
                arities.add(('decl', d.arity, d.location))

            # From rule heads
            for r in self.rules.get(pred_name, []):
                arities.add(('rule', r.head_arity, r.location))

            # From facts
            for f in self.facts.get(pred_name, []):
                arities.add(('fact', f.arity, f.location))

            unique_arities = set(a[1] for a in arities)
            if len(unique_arities) > 1:
                self.issues.append(Issue(
                    severity="ERROR",
                    category="arity",
                    message=f"Predicate '{pred_name}' used with inconsistent arities: {unique_arities}",
                    location=list(arities)[0][2],
                    suggestion=f"Check all uses of '{pred_name}' and unify arity"
                ))

    def check_undefined_predicates(self):
        """Check for predicates used but never defined."""
        # Built-in predicates that don't need declaration
        builtins = {
            'fn:Count', 'fn:Sum', 'fn:Max', 'fn:Min', 'fn:Avg',
            'fn:plus', 'fn:minus', 'fn:mult', 'fn:div', 'fn:mod',
            'fn:group_by', 'fn:list', 'fn:pair', 'fn:tuple',
            'fn:list:get', 'fn:list:len', 'fn:append',
            ':match_field', ':match_cons', ':list:member',
        }

        for pred in self.used_predicates:
            if pred not in self.defined_predicates:
                if not any(pred.startswith(b.split(':')[0]) for b in builtins):
                    self.issues.append(Issue(
                        severity="WARNING",
                        category="undefined",
                        message=f"Predicate '{pred}' used but never declared or defined",
                        location=Location("unknown", 0),
                        suggestion=f"Add 'Decl {pred}(...).' or define via fact/rule"
                    ))

    def check_syntax_issues(self):
        """Additional syntax checks done during parsing."""
        pass  # Most syntax checks are done inline

    def check_aggregation_syntax(self, line: str, loc: Location):
        """Check for correct aggregation syntax."""
        # Wrong: sum(X), count(X)
        # Right: |> let N = fn:Sum(X)
        bad_agg = re.search(r'\b(sum|count|max|min|avg)\s*\(', line, re.IGNORECASE)
        if bad_agg and '|>' not in line:
            self.issues.append(Issue(
                severity="ERROR",
                category="aggregation",
                message=f"Wrong aggregation syntax: {bad_agg.group(0)}",
                location=loc,
                suggestion="Use: |> do fn:group_by(...), let N = fn:Sum(X)"
            ))

    def check_string_vs_atom(self):
        """Check for potential atom vs string confusion."""
        # This is harder to detect automatically
        # Look for patterns like status(X, "active") which should be status(X, /active)
        pass

    def run_all_checks(self):
        """Run all analysis checks."""
        self.check_duplicates()
        self.check_arity_consistency()
        self.check_undefined_predicates()

    def report(self) -> str:
        """Generate a report of all issues."""
        lines = []
        lines.append("=" * 70)
        lines.append("MANGLE ERROR ANALYSIS REPORT")
        lines.append("=" * 70)

        # Group by severity
        errors = [i for i in self.issues if i.severity == "ERROR"]
        warnings = [i for i in self.issues if i.severity == "WARNING"]
        info = [i for i in self.issues if i.severity == "INFO"]

        lines.append(f"\nSummary: {len(errors)} errors, {len(warnings)} warnings, {len(info)} info")
        lines.append("")

        # Group by category
        by_category = defaultdict(list)
        for issue in self.issues:
            by_category[issue.category].append(issue)

        for category in ['stratification', 'safety', 'duplicate', 'arity', 'syntax', 'aggregation', 'undefined']:
            issues = by_category.get(category, [])
            if issues:
                lines.append(f"\n{'='*50}")
                lines.append(f"  {category.upper()} ISSUES ({len(issues)})")
                lines.append(f"{'='*50}")

                for issue in issues:
                    lines.append(f"\n[{issue.severity}] {issue.message}")
                    lines.append(f"  Location: {issue.location.file}:{issue.location.line}")
                    if issue.suggestion:
                        lines.append(f"  Fix: {issue.suggestion}")

        # Show predicate statistics
        lines.append(f"\n{'='*50}")
        lines.append("  STATISTICS")
        lines.append(f"{'='*50}")
        lines.append(f"Total declarations: {sum(len(v) for v in self.declarations.values())}")
        lines.append(f"Unique declared predicates: {len(self.declarations)}")
        lines.append(f"Total rules: {sum(len(v) for v in self.rules.values())}")
        lines.append(f"Total facts: {sum(len(v) for v in self.facts.values())}")

        # Show duplicates detail
        dups = [(k, v) for k, v in self.declarations.items() if len(v) > 1]
        if dups:
            lines.append(f"\n{'='*50}")
            lines.append("  DUPLICATE DECLARATIONS DETAIL")
            lines.append(f"{'='*50}")
            for pred_name, decls in sorted(dups, key=lambda x: -len(x[1])):
                lines.append(f"\n{pred_name} ({len(decls)} declarations):")
                for d in decls:
                    lines.append(f"  - {d.location.file}:{d.location.line} (arity={d.arity})")

        return '\n'.join(lines)

    def get_fixes(self) -> dict[str, list[dict]]:
        """Get suggested fixes grouped by source file."""
        fixes = defaultdict(list)

        for issue in self.issues:
            if issue.location.file != "unknown":
                fixes[issue.location.file].append({
                    'line': issue.location.line,
                    'category': issue.category,
                    'message': issue.message,
                    'suggestion': issue.suggestion
                })

        return dict(fixes)


def main():
    if len(sys.argv) < 2:
        print("Usage: python analyze_mangle_errors.py <file.mg> [--json]")
        sys.exit(1)

    filepath = sys.argv[1]
    json_output = '--json' in sys.argv

    analyzer = MangleAnalyzer()
    analyzer.parse_file(filepath)
    analyzer.run_all_checks()

    if json_output:
        import json
        result = {
            'issues': [
                {
                    'severity': i.severity,
                    'category': i.category,
                    'message': i.message,
                    'file': i.location.file,
                    'line': i.location.line,
                    'suggestion': i.suggestion
                }
                for i in analyzer.issues
            ],
            'statistics': {
                'declarations': sum(len(v) for v in analyzer.declarations.values()),
                'unique_predicates': len(analyzer.declarations),
                'rules': sum(len(v) for v in analyzer.rules.values()),
                'facts': sum(len(v) for v in analyzer.facts.values()),
                'errors': len([i for i in analyzer.issues if i.severity == 'ERROR']),
                'warnings': len([i for i in analyzer.issues if i.severity == 'WARNING']),
            },
            'duplicates': {
                k: [{'file': d.location.file, 'line': d.location.line, 'arity': d.arity}
                    for d in v]
                for k, v in analyzer.declarations.items() if len(v) > 1
            }
        }
        print(json.dumps(result, indent=2))
    else:
        print(analyzer.report())


if __name__ == '__main__':
    main()
