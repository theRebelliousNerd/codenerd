#!/usr/bin/env python3
"""
Mangle Dead Code Detection Tool v1.0

Analyzes Google Mangle programs for dead/unreachable code - rules that can never
fire given the current EDB declarations and rule dependencies.

WHAT IS DEAD CODE?
==================
Dead code in Mangle includes:
  - Rules with unreachable body predicates (depend on undefined predicates)
  - Predicates defined but never used
  - Predicates used but never defined
  - Rules shadowed by earlier rules (may be unreachable)

This tool helps maintain clean Mangle codebases by identifying code that cannot
contribute to query results.

USAGE
=====
    python3 dead_code.py <file1.mg> [file2.mg ...] [options]
    python3 dead_code.py policy.mg schemas.mg --report
    python3 dead_code.py internal/core/defaults/*.mg --json
    python3 dead_code.py policy.mg --unused-only
    python3 dead_code.py policy.mg --undefined-only

OPTIONS
=======
    --report            Full human-readable report (default)
    --json              Output as JSON
    --unused-only       Only show unused predicates
    --undefined-only    Only show undefined predicates
    --ignore PRED       Ignore specific predicates (can be repeated)
    --verbose, -v       Show detailed analysis

EXIT CODES
==========
    0 - No dead code found
    1 - Dead code detected
    2 - Parse error or fatal error

EXAMPLES
========
    # Check policy and schema files
    python3 dead_code.py policy.mg schemas.mg

    # Ignore virtual predicates
    python3 dead_code.py policy.mg --ignore query_learned --ignore recall_similar

    # Get JSON output for tooling
    python3 dead_code.py *.mg --json > dead_code.json

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


@dataclass
class PredicateInfo:
    """Information about a predicate."""
    name: str
    arity: int = -1
    defined_at: List[Tuple[str, int]] = field(default_factory=list)  # (file, line)
    used_at: List[Tuple[str, int]] = field(default_factory=list)     # (file, line)
    is_edb: bool = False  # Declared with Decl (base fact)
    is_idb: bool = False  # Defined by rules (derived)
    is_virtual: bool = False  # Virtual predicate (Go FFI)
    is_builtin: bool = False  # Built-in predicate


@dataclass
class RuleInfo:
    """Information about a rule."""
    head_predicate: str
    body_predicates: List[str]
    file: str
    line: int
    text: str


@dataclass
class DeadCodeIssue:
    """A detected dead code issue."""
    issue_type: str  # "unreachable", "unused", "undefined", "shadowed"
    severity: str    # "error", "warning", "info"
    message: str
    predicate: Optional[str] = None
    file: Optional[str] = None
    line: Optional[int] = None
    lines: List[Tuple[str, int]] = field(default_factory=list)  # Related locations
    suggestion: str = ""


class DeadCodeAnalyzer:
    """
    Analyzes Mangle programs for dead/unreachable code.

    Algorithm:
    1. Parse all files to extract predicates and rules
    2. Build reachability graph (EDB predicates are "grounded")
    3. Find unreachable rules (depend on undefined predicates)
    4. Find unused predicates (defined but never referenced)
    5. Find undefined predicates (used but never defined)
    6. Optionally detect shadowed rules (earlier rule always matches first)
    """

    # Regex patterns for parsing (from stratification tool)
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

    # Common virtual predicates (Go FFI) - can be extended via --ignore
    VIRTUALS = {
        'query_learned', 'query_session', 'recall_similar',
        'query_knowledge', 'vector_recall',
    }

    def __init__(self, ignored_predicates: Set[str] = None, verbose: bool = False):
        self.verbose = verbose
        self.ignored = ignored_predicates or set()
        self.ignored.update(self.BUILTINS)
        self.ignored.update(self.VIRTUALS)

        self.predicates: Dict[str, PredicateInfo] = {}
        self.rules: List[RuleInfo] = []
        self.issues: List[DeadCodeIssue] = []
        self.files_analyzed: List[str] = []

    def analyze_files(self, filepaths: List[Path]) -> bool:
        """Analyze multiple Mangle files. Returns True if no dead code found."""
        self.predicates = {}
        self.rules = []
        self.issues = []
        self.files_analyzed = []

        # Phase 1: Parse all files
        for filepath in filepaths:
            if not filepath.exists():
                print(f"Warning: File not found: {filepath}", file=sys.stderr)
                continue

            with open(filepath, encoding='utf-8') as f:
                content = f.read()

            self._parse_file(content, str(filepath))
            self.files_analyzed.append(str(filepath))

        # Mark virtual predicates
        for pred_name in self.ignored:
            if pred_name in self.predicates:
                self.predicates[pred_name].is_virtual = True

        # Phase 2: Detect dead code issues
        self._detect_unreachable_rules()
        self._detect_unused_predicates()
        self._detect_undefined_predicates()
        # self._detect_shadowed_rules()  # Optional: can be expensive

        return len(self.issues) == 0

    def _parse_file(self, content: str, filepath: str):
        """Parse a Mangle file and extract predicates and rules."""
        statements = self._split_into_statements(content)

        for stmt, start_line in statements:
            stmt = stmt.strip()
            if not stmt:
                continue

            # Handle declarations (EDB predicates)
            if stmt.startswith('Decl '):
                decl_match = self.PATTERNS['decl'].match(stmt)
                if decl_match:
                    pred_name = decl_match.group(1)
                    if pred_name not in self.ignored:
                        self._register_predicate(
                            pred_name, filepath, start_line, is_edb=True
                        )
                continue

            # Skip package statements
            if stmt.startswith('Package ') or stmt.startswith('Uses '):
                continue

            # Handle rules
            arrow_match = self.PATTERNS['rule_arrow'].search(stmt)
            if arrow_match:
                head = stmt[:arrow_match.start()]
                body = stmt[arrow_match.end():]
                self._parse_rule(head, body, filepath, start_line, stmt)

            # Handle facts (ground EDB facts)
            elif stmt.endswith('.') and '(' in stmt:
                pred_match = self.PATTERNS['predicate'].match(stmt)
                if pred_match:
                    pred_name = pred_match.group(1)
                    if pred_name not in self.ignored:
                        self._register_predicate(
                            pred_name, filepath, start_line, is_edb=True
                        )

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

    def _parse_rule(self, head: str, body: str, filepath: str, line: int, full_rule: str):
        """Parse a single rule and extract predicates."""
        # Remove strings to avoid false matches
        body_clean = self.PATTERNS['string'].sub('""', body)

        # Extract head predicate
        head_match = self.PATTERNS['predicate'].match(head.strip())
        if not head_match:
            return

        head_pred = head_match.group(1)
        if head_pred not in self.ignored:
            self._register_predicate(head_pred, filepath, line, is_idb=True)

        # Extract all predicates from body (including negated ones)
        body_predicates = []

        # Find all positive predicates
        for match in self.PATTERNS['predicate'].finditer(body_clean):
            pred_name = match.group(1)
            if pred_name not in self.ignored and pred_name not in body_predicates:
                body_predicates.append(pred_name)
                self._register_predicate(pred_name, filepath, line, used=True)

        # Find negated predicates
        for pattern in [self.PATTERNS['negation_bang'], self.PATTERNS['negation_not']]:
            for match in pattern.finditer(body_clean):
                neg_pred = match.group(1)
                if neg_pred not in self.ignored and neg_pred not in body_predicates:
                    body_predicates.append(neg_pred)
                    self._register_predicate(neg_pred, filepath, line, used=True)

        # Store rule info
        self.rules.append(RuleInfo(
            head_predicate=head_pred,
            body_predicates=body_predicates,
            file=filepath,
            line=line,
            text=full_rule[:100] + ("..." if len(full_rule) > 100 else "")
        ))

    def _register_predicate(self, name: str, filepath: str, line: int,
                           is_edb: bool = False, is_idb: bool = False,
                           used: bool = False):
        """Register a predicate occurrence."""
        if name not in self.predicates:
            self.predicates[name] = PredicateInfo(name=name)

        pred = self.predicates[name]

        if is_edb:
            pred.is_edb = True
            if (filepath, line) not in pred.defined_at:
                pred.defined_at.append((filepath, line))

        if is_idb:
            pred.is_idb = True
            if (filepath, line) not in pred.defined_at:
                pred.defined_at.append((filepath, line))

        if used or is_idb or is_edb:
            if (filepath, line) not in pred.used_at:
                pred.used_at.append((filepath, line))

    def _detect_unreachable_rules(self):
        """Find rules with undefined body predicates (unreachable)."""
        for rule in self.rules:
            undefined_preds = []

            for body_pred in rule.body_predicates:
                if body_pred not in self.predicates:
                    undefined_preds.append(body_pred)
                    continue

                pred_info = self.predicates[body_pred]
                # Check if predicate is defined (either EDB or IDB)
                if not pred_info.is_edb and not pred_info.is_idb and not pred_info.is_virtual:
                    undefined_preds.append(body_pred)

            if undefined_preds:
                self.issues.append(DeadCodeIssue(
                    issue_type="unreachable",
                    severity="error",
                    message=f"Rule depends on undefined predicate(s): {', '.join(undefined_preds)}",
                    predicate=rule.head_predicate,
                    file=rule.file,
                    line=rule.line,
                    suggestion=f"Define missing predicate(s) or remove this rule:\n  {rule.text}"
                ))

    def _detect_unused_predicates(self):
        """Find predicates defined but never used."""
        for pred_name, pred_info in self.predicates.items():
            # Skip if not defined
            if not pred_info.is_idb and not pred_info.is_edb:
                continue

            # Skip if virtual or builtin
            if pred_info.is_virtual or pred_info.is_builtin:
                continue

            # Check if it's only "used" at definition sites
            used_outside_def = False
            for use_loc in pred_info.used_at:
                if use_loc not in pred_info.defined_at:
                    used_outside_def = True
                    break

            # Also check if it appears in any rule body
            used_in_body = any(
                pred_name in rule.body_predicates
                for rule in self.rules
            )

            if not used_outside_def and not used_in_body:
                # Get definition locations
                def_locs = pred_info.defined_at

                self.issues.append(DeadCodeIssue(
                    issue_type="unused",
                    severity="warning",
                    message=f"Predicate '{pred_name}' is defined but never used",
                    predicate=pred_name,
                    file=def_locs[0][0] if def_locs else None,
                    line=def_locs[0][1] if def_locs else None,
                    lines=def_locs,
                    suggestion=f"Remove unused predicate or add usage"
                ))

    def _detect_undefined_predicates(self):
        """Find predicates used but never defined."""
        for pred_name, pred_info in self.predicates.items():
            # Skip if defined (EDB or IDB)
            if pred_info.is_edb or pred_info.is_idb:
                continue

            # Skip if virtual or builtin
            if pred_info.is_virtual or pred_info.is_builtin:
                continue

            # This predicate is used but not defined
            self.issues.append(DeadCodeIssue(
                issue_type="undefined",
                severity="error",
                message=f"Predicate '{pred_name}' is used but never defined",
                predicate=pred_name,
                lines=pred_info.used_at,
                suggestion=f"Add a Decl or rule defining '{pred_name}', or mark as virtual"
            ))

    def _detect_shadowed_rules(self):
        """
        Find rules that may be shadowed by earlier rules.
        This is a heuristic check - not always accurate.
        """
        # Group rules by head predicate
        rules_by_head: Dict[str, List[RuleInfo]] = defaultdict(list)
        for rule in self.rules:
            rules_by_head[rule.head_predicate].append(rule)

        # Check each predicate with multiple rules
        for pred_name, pred_rules in rules_by_head.items():
            if len(pred_rules) < 2:
                continue

            # Sort by file and line number
            sorted_rules = sorted(pred_rules, key=lambda r: (r.file, r.line))

            # Check for potential shadowing (simplified heuristic)
            for i in range(1, len(sorted_rules)):
                current = sorted_rules[i]
                previous = sorted_rules[i-1]

                # If previous rule has subset of body predicates, might shadow
                if set(previous.body_predicates).issubset(set(current.body_predicates)):
                    self.issues.append(DeadCodeIssue(
                        issue_type="shadowed",
                        severity="info",
                        message=f"Rule at {current.file}:{current.line} may be shadowed by earlier rule",
                        predicate=pred_name,
                        file=current.file,
                        line=current.line,
                        lines=[(previous.file, previous.line)],
                        suggestion=f"Earlier rule at {previous.file}:{previous.line} may match first"
                    ))

    def get_report(self) -> str:
        """Generate a human-readable report."""
        lines = []
        lines.append("=" * 70)
        lines.append("MANGLE DEAD CODE ANALYSIS")
        lines.append("=" * 70)

        # Files analyzed
        lines.append(f"\nFiles analyzed: {len(self.files_analyzed)}")
        for f in self.files_analyzed:
            lines.append(f"  - {f}")

        # Summary statistics
        lines.append(f"\nRules: {len(self.rules)}")
        lines.append(f"Predicates: {len(self.predicates)}")

        edb_count = sum(1 for p in self.predicates.values() if p.is_edb)
        idb_count = sum(1 for p in self.predicates.values() if p.is_idb)
        virtual_count = sum(1 for p in self.predicates.values() if p.is_virtual)

        lines.append(f"  - EDB (declared): {edb_count}")
        lines.append(f"  - IDB (derived): {idb_count}")
        lines.append(f"  - Virtual/Builtin: {virtual_count}")

        # Issues by type
        issues_by_type = defaultdict(list)
        for issue in self.issues:
            issues_by_type[issue.issue_type].append(issue)

        if not self.issues:
            lines.append("\n" + "=" * 70)
            lines.append("RESULT: No dead code detected!")
            lines.append("=" * 70)
        else:
            lines.append(f"\nISSUES FOUND: {len(self.issues)}")

            # Unreachable rules
            if 'unreachable' in issues_by_type:
                lines.append("\n" + "-" * 70)
                lines.append(f"UNREACHABLE RULES ({len(issues_by_type['unreachable'])})")
                lines.append("(Rules with undefined body predicates)")
                lines.append("-" * 70)

                for issue in issues_by_type['unreachable']:
                    lines.append(f"\n  {issue.file}:{issue.line}")
                    lines.append(f"  {issue.message}")
                    if self.verbose and issue.suggestion:
                        lines.append(f"  Suggestion: {issue.suggestion}")

            # Unused predicates
            if 'unused' in issues_by_type:
                lines.append("\n" + "-" * 70)
                lines.append(f"UNUSED PREDICATES ({len(issues_by_type['unused'])})")
                lines.append("(Defined but never referenced)")
                lines.append("-" * 70)

                for issue in issues_by_type['unused']:
                    locations = ", ".join(f"{f}:{l}" for f, l in issue.lines[:3])
                    if len(issue.lines) > 3:
                        locations += f" ... ({len(issue.lines)} total)"
                    lines.append(f"\n  - {issue.predicate}")
                    lines.append(f"    Defined at: {locations}")

            # Undefined predicates
            if 'undefined' in issues_by_type:
                lines.append("\n" + "-" * 70)
                lines.append(f"UNDEFINED PREDICATES ({len(issues_by_type['undefined'])})")
                lines.append("(Used but never defined)")
                lines.append("-" * 70)

                for issue in issues_by_type['undefined']:
                    locations = ", ".join(f"{f}:{l}" for f, l in issue.lines[:5])
                    if len(issue.lines) > 5:
                        locations += f" ... ({len(issue.lines)} total)"
                    lines.append(f"\n  - {issue.predicate}")
                    lines.append(f"    Used at: {locations}")
                    if self.verbose and issue.suggestion:
                        lines.append(f"    Suggestion: {issue.suggestion}")

            # Shadowed rules
            if 'shadowed' in issues_by_type:
                lines.append("\n" + "-" * 70)
                lines.append(f"POTENTIALLY SHADOWED RULES ({len(issues_by_type['shadowed'])})")
                lines.append("(May be unreachable due to earlier rules)")
                lines.append("-" * 70)

                for issue in issues_by_type['shadowed']:
                    lines.append(f"\n  {issue.file}:{issue.line}")
                    lines.append(f"  {issue.message}")
                    if issue.lines:
                        shadower = issue.lines[0]
                        lines.append(f"  Shadowed by: {shadower[0]}:{shadower[1]}")

            # Summary
            lines.append("\n" + "=" * 70)
            lines.append("SUMMARY")
            lines.append("=" * 70)
            lines.append(f"  Unreachable rules: {len(issues_by_type.get('unreachable', []))}")
            lines.append(f"  Unused predicates: {len(issues_by_type.get('unused', []))}")
            lines.append(f"  Undefined predicates: {len(issues_by_type.get('undefined', []))}")
            lines.append(f"  Shadowed rules: {len(issues_by_type.get('shadowed', []))}")
            lines.append(f"  Total issues: {len(self.issues)}")

        lines.append("=" * 70)
        return "\n".join(lines)

    def get_json_result(self) -> dict:
        """Return analysis results as JSON-serializable dict."""
        return {
            'files_analyzed': self.files_analyzed,
            'statistics': {
                'total_rules': len(self.rules),
                'total_predicates': len(self.predicates),
                'edb_predicates': sum(1 for p in self.predicates.values() if p.is_edb),
                'idb_predicates': sum(1 for p in self.predicates.values() if p.is_idb),
                'virtual_predicates': sum(1 for p in self.predicates.values() if p.is_virtual),
            },
            'issues': [
                {
                    'type': issue.issue_type,
                    'severity': issue.severity,
                    'message': issue.message,
                    'predicate': issue.predicate,
                    'file': issue.file,
                    'line': issue.line,
                    'related_locations': [{'file': f, 'line': l} for f, l in issue.lines],
                    'suggestion': issue.suggestion,
                }
                for issue in self.issues
            ],
            'predicates': {
                name: {
                    'is_edb': p.is_edb,
                    'is_idb': p.is_idb,
                    'is_virtual': p.is_virtual,
                    'defined_at': [{'file': f, 'line': l} for f, l in p.defined_at],
                    'used_at': [{'file': f, 'line': l} for f, l in p.used_at],
                }
                for name, p in self.predicates.items()
            }
        }


def main():
    parser = argparse.ArgumentParser(
        description="Detect dead/unreachable code in Mangle programs",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog=__doc__
    )
    parser.add_argument('files', nargs='+', help='Mangle files to analyze')
    parser.add_argument('--report', action='store_true', help='Full report (default)')
    parser.add_argument('--json', '-j', action='store_true', help='Output as JSON')
    parser.add_argument('--unused-only', action='store_true', help='Only show unused predicates')
    parser.add_argument('--undefined-only', action='store_true', help='Only show undefined predicates')
    parser.add_argument('--ignore', action='append', default=[], help='Ignore specific predicates')
    parser.add_argument('--verbose', '-v', action='store_true', help='Show detailed analysis')

    args = parser.parse_args()

    # Collect file paths
    filepaths = [Path(f) for f in args.files]

    # Create analyzer with ignored predicates
    ignored = set(args.ignore)
    analyzer = DeadCodeAnalyzer(ignored_predicates=ignored, verbose=args.verbose)

    # Analyze files
    is_clean = analyzer.analyze_files(filepaths)

    # Filter issues if requested
    if args.unused_only:
        analyzer.issues = [i for i in analyzer.issues if i.issue_type == 'unused']
    elif args.undefined_only:
        analyzer.issues = [i for i in analyzer.issues if i.issue_type == 'undefined']

    # Output results
    if args.json:
        print(json.dumps(analyzer.get_json_result(), indent=2))
    else:
        print(analyzer.get_report())

    sys.exit(0 if is_clean else 1)


if __name__ == "__main__":
    main()
