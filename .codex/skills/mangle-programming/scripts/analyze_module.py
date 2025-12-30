#!/usr/bin/env python3
"""
Mangle Cross-File Module Analysis Tool v1.0

Analyzes multiple .mg files together to detect:
- Cross-file dependencies and imports
- Missing definitions (predicates used but never defined)
- Duplicate definitions (conflicts across files)
- Arity mismatches (same predicate, different arities)
- Unused exports (defined but never imported)
- Module dependency graph

USAGE
=====
    python analyze_module.py <file1.mg> [file2.mg ...] [options]
    python analyze_module.py internal/core/defaults/*.mg --check-completeness
    python analyze_module.py *.mg --graph > module_deps.dot
    python analyze_module.py *.mg --json
    python analyze_module.py *.mg --virtual "virtual_store_query,external_api"

OPTIONS
=======
    --check-completeness  Fail if any predicate is undefined
    --graph              Output DOT format dependency graph
    --json               JSON output
    --virtual LIST       Comma-separated predicates implemented in Go (don't flag as missing)
    --strict             Treat warnings as errors
    -v, --verbose        Show detailed analysis

EXIT CODES
==========
    0 - No issues found
    1 - Conflicts, missing definitions, or errors found
    2 - Parse error or fatal error

EXAMPLES
========
    # Analyze all core policy files
    python analyze_module.py internal/core/defaults/*.mg

    # Check completeness with known virtual predicates
    python analyze_module.py *.mg --check-completeness --virtual "file_content,symbol_at"

    # Generate dependency graph
    python analyze_module.py *.mg --graph | dot -Tpng > deps.png

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


@dataclass
class PredicateDeclaration:
    """A predicate declaration (Decl statement)."""
    name: str
    arity: int
    file: str
    line: int
    full_decl: str


@dataclass
class PredicateDefinition:
    """A predicate definition (rule with this predicate in head)."""
    name: str
    arity: int  # Extracted from actual usage
    file: str
    line: int
    rule_head: str


@dataclass
class PredicateUsage:
    """A predicate usage in a rule body."""
    name: str
    file: str
    line: int
    context: str  # The rule where it appears


@dataclass
class Conflict:
    """A detected conflict between files."""
    conflict_type: str  # "duplicate_definition", "arity_mismatch", "decl_conflict"
    predicate: str
    locations: List[Tuple[str, int, str]]  # [(file, line, context), ...]
    message: str
    severity: str  # "error" or "warning"


@dataclass
class ModuleDependency:
    """Dependency between two files."""
    source_file: str
    target_file: str
    predicates: Set[str]  # Predicates imported from target


class ModuleAnalyzer:
    """
    Analyzes multiple Mangle files for cross-file coherence.

    Algorithm:
    1. Parse all files to extract declarations, definitions, and usages
    2. Build cross-reference maps
    3. Detect conflicts (duplicates, arity mismatches)
    4. Find missing definitions
    5. Build dependency graph
    6. Generate report
    """

    # Regex patterns (reuse from stratification analyzer)
    PATTERNS = {
        'predicate': re.compile(r'\b([a-z][a-z0-9_]*)\s*\('),
        'predicate_with_args': re.compile(r'\b([a-z][a-z0-9_]*)\s*\(((?:[^()]|\([^)]*\))*)\)'),
        'negation_bang': re.compile(r'!\s*([a-z][a-z0-9_]*)\s*\('),
        'negation_not': re.compile(r'\bnot\s+([a-z][a-z0-9_]*)\s*\('),
        'rule_arrow': re.compile(r':-|<-|⟸'),
        'decl': re.compile(r'^\s*Decl\s+([a-z][a-z0-9_]*)\s*\(((?:[^()]|\([^)]*\))*)\)'),
        'comment': re.compile(r'#.*$'),
        'string': re.compile(r'"(?:[^"\\]|\\.)*"|\'(?:[^\'\\]|\\.)*\'|`[^`]*`'),
    }

    # Built-in predicates
    BUILTINS = {
        'match_cons', 'match_nil', 'match_field', 'match_entry',
        'list:member', 'list_length', 'time_diff', 'string:contains',
        'int:add', 'int:sub', 'int:mul', 'int:div',
    }

    def __init__(self, verbose: bool = False, virtual_predicates: Set[str] = None):
        self.verbose = verbose
        self.virtual_predicates = virtual_predicates or set()

        # Data structures
        self.declarations: Dict[str, List[PredicateDeclaration]] = defaultdict(list)
        self.definitions: Dict[str, List[PredicateDefinition]] = defaultdict(list)
        self.usages: Dict[str, List[PredicateUsage]] = defaultdict(list)

        # Analysis results
        self.conflicts: List[Conflict] = []
        self.missing: Set[str] = set()
        self.unused: Set[str] = set()
        self.dependencies: List[ModuleDependency] = []

        # File tracking
        self.files_analyzed: List[str] = []

    def analyze_files(self, filepaths: List[Path]) -> bool:
        """
        Analyze multiple Mangle files.
        Returns True if no critical issues found.
        """
        # Phase 1: Parse all files
        for filepath in filepaths:
            self._parse_file(filepath)

        # Phase 2: Cross-reference analysis
        self._detect_conflicts()
        self._find_missing_definitions()
        self._find_unused_exports()
        self._build_dependency_graph()

        # Check for critical issues
        has_errors = any(c.severity == "error" for c in self.conflicts) or bool(self.missing)
        return not has_errors

    def _parse_file(self, filepath: Path):
        """Parse a single Mangle file."""
        if not filepath.exists():
            print(f"Warning: File not found: {filepath}", file=sys.stderr)
            return

        filename = str(filepath)
        self.files_analyzed.append(filename)

        with open(filepath, encoding='utf-8') as f:
            content = f.read()

        statements = self._split_into_statements(content)

        for stmt, start_line in statements:
            stmt = stmt.strip()
            if not stmt:
                continue

            # Check for declarations
            decl_match = self.PATTERNS['decl'].match(stmt)
            if decl_match:
                pred_name = decl_match.group(1)
                args_str = decl_match.group(2)
                arity = self._count_arity(args_str)

                self.declarations[pred_name].append(PredicateDeclaration(
                    name=pred_name,
                    arity=arity,
                    file=filename,
                    line=start_line,
                    full_decl=stmt[:100]
                ))
                continue

            # Skip package statements
            if stmt.startswith('Package ') or stmt.startswith('Uses '):
                continue

            # Check for rules
            arrow_match = self.PATTERNS['rule_arrow'].search(stmt)
            if arrow_match:
                head = stmt[:arrow_match.start()]
                body = stmt[arrow_match.end():]

                # Extract head predicate
                head_match = self.PATTERNS['predicate_with_args'].match(head.strip())
                if head_match:
                    pred_name = head_match.group(1)
                    args_str = head_match.group(2)
                    arity = self._count_arity(args_str)

                    self.definitions[pred_name].append(PredicateDefinition(
                        name=pred_name,
                        arity=arity,
                        file=filename,
                        line=start_line,
                        rule_head=head.strip()[:80]
                    ))

                # Extract body predicates
                self._extract_body_usages(body, filename, start_line, stmt)

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

            # Check if statement is complete
            if stripped.endswith('.') or stripped.endswith('!'):
                full_stmt = ' '.join(current_stmt)
                statements.append((full_stmt, start_line))
                current_stmt = []
                in_statement = False

        # Handle incomplete statement
        if current_stmt:
            full_stmt = ' '.join(current_stmt)
            statements.append((full_stmt, start_line))

        return statements

    def _extract_body_usages(self, body: str, filename: str, line: int, full_rule: str):
        """Extract all predicate usages from a rule body."""
        # Remove strings to avoid false matches
        body_clean = self.PATTERNS['string'].sub('""', body)

        # Find all predicates
        for match in self.PATTERNS['predicate'].finditer(body_clean):
            pred_name = match.group(1)

            if pred_name in self.BUILTINS:
                continue

            self.usages[pred_name].append(PredicateUsage(
                name=pred_name,
                file=filename,
                line=line,
                context=full_rule[:100] + ("..." if len(full_rule) > 100 else "")
            ))

    def _count_arity(self, args_str: str) -> int:
        """Count arity from argument string."""
        if not args_str.strip():
            return 0

        # Simple comma counting (not perfect for nested structures, but good enough)
        # Remove nested parens first
        cleaned = args_str
        while '(' in cleaned:
            cleaned = re.sub(r'\([^()]*\)', '', cleaned)

        args = [a.strip() for a in cleaned.split(',') if a.strip()]
        return len(args)

    def _detect_conflicts(self):
        """Detect conflicts: duplicate definitions, arity mismatches, declaration conflicts."""

        # Check for duplicate definitions
        for pred_name, defs in self.definitions.items():
            if len(defs) > 1:
                # Group by arity
                by_arity: Dict[int, List[PredicateDefinition]] = defaultdict(list)
                for defn in defs:
                    by_arity[defn.arity].append(defn)

                if len(by_arity) > 1:
                    # Arity mismatch - critical error
                    locations = [(d.file, d.line, d.rule_head) for d in defs]
                    arities = sorted(by_arity.keys())

                    self.conflicts.append(Conflict(
                        conflict_type="arity_mismatch",
                        predicate=pred_name,
                        locations=locations,
                        message=f"Predicate '{pred_name}' defined with different arities: {arities}",
                        severity="error"
                    ))
                else:
                    # Same arity, multiple files - warning
                    # This is OK in Mangle (multiple rules for same predicate)
                    # But flag if in different files
                    files_involved = set(d.file for d in defs)
                    if len(files_involved) > 1:
                        locations = [(d.file, d.line, d.rule_head) for d in defs]
                        self.conflicts.append(Conflict(
                            conflict_type="duplicate_definition",
                            predicate=pred_name,
                            locations=locations,
                            message=f"Predicate '{pred_name}' defined in multiple files: {', '.join(Path(f).name for f in files_involved)}",
                            severity="warning"
                        ))

        # Check for declaration conflicts
        for pred_name, decls in self.declarations.items():
            if len(decls) > 1:
                arities = set(d.arity for d in decls)
                if len(arities) > 1:
                    locations = [(d.file, d.line, d.full_decl) for d in decls]
                    self.conflicts.append(Conflict(
                        conflict_type="decl_conflict",
                        predicate=pred_name,
                        locations=locations,
                        message=f"Predicate '{pred_name}' declared with different arities: {sorted(arities)}",
                        severity="error"
                    ))
                else:
                    # Same arity but multiple declarations - warning
                    locations = [(d.file, d.line, d.full_decl) for d in decls]
                    files = set(d.file for d in decls)
                    if len(files) > 1:
                        self.conflicts.append(Conflict(
                            conflict_type="duplicate_decl",
                            predicate=pred_name,
                            locations=locations,
                            message=f"Predicate '{pred_name}' declared in multiple files",
                            severity="warning"
                        ))

    def _find_missing_definitions(self):
        """Find predicates used but never defined."""
        all_defined = set(self.definitions.keys()) | set(self.declarations.keys())
        all_used = set(self.usages.keys())

        # Missing = used but not defined (excluding builtins and virtuals)
        self.missing = all_used - all_defined - self.BUILTINS - self.virtual_predicates

    def _find_unused_exports(self):
        """Find predicates defined but never used."""
        all_defined = set(self.definitions.keys())
        all_used = set(self.usages.keys())

        # Unused = defined but not used anywhere
        self.unused = all_defined - all_used

    def _build_dependency_graph(self):
        """Build module dependency graph (which files depend on which)."""
        # Map: predicate -> files where it's defined
        pred_to_def_files: Dict[str, Set[str]] = defaultdict(set)
        for pred_name, defs in self.definitions.items():
            for defn in defs:
                pred_to_def_files[pred_name].add(defn.file)

        # Also include declarations
        for pred_name, decls in self.declarations.items():
            for decl in decls:
                pred_to_def_files[pred_name].add(decl.file)

        # Map: (source_file, target_file) -> set of predicates imported
        file_deps: Dict[Tuple[str, str], Set[str]] = defaultdict(set)

        for pred_name, usages in self.usages.items():
            if pred_name in pred_to_def_files:
                def_files = pred_to_def_files[pred_name]
                for usage in usages:
                    for def_file in def_files:
                        if def_file != usage.file:
                            # usage.file imports from def_file
                            file_deps[(usage.file, def_file)].add(pred_name)

        # Convert to ModuleDependency objects
        for (source, target), preds in file_deps.items():
            self.dependencies.append(ModuleDependency(
                source_file=source,
                target_file=target,
                predicates=preds
            ))

    def get_report(self) -> str:
        """Generate human-readable report."""
        lines = []
        lines.append("=" * 70)
        lines.append("CROSS-FILE ANALYSIS")
        lines.append("=" * 70)

        # Files analyzed
        lines.append(f"\nFiles analyzed: {len(self.files_analyzed)}")
        for f in self.files_analyzed:
            lines.append(f"  - {Path(f).name}")

        # Summary statistics
        total_predicates = len(set(self.definitions.keys()) | set(self.declarations.keys()))
        lines.append(f"\nTotal predicates: {total_predicates}")
        lines.append(f"Cross-file dependencies: {len(self.dependencies)}")
        lines.append(f"Conflicts: {len(self.conflicts)}")
        lines.append(f"Missing definitions: {len(self.missing)}")
        lines.append(f"Unused exports: {len(self.unused)}")

        # Module dependencies
        if self.dependencies:
            lines.append("\n" + "-" * 70)
            lines.append("MODULE DEPENDENCIES")
            lines.append("-" * 70)

            # Group by source file
            by_source: Dict[str, List[ModuleDependency]] = defaultdict(list)
            for dep in self.dependencies:
                by_source[dep.source_file].append(dep)

            for source_file in sorted(by_source.keys()):
                lines.append(f"\n{Path(source_file).name}")
                deps = by_source[source_file]
                for dep in sorted(deps, key=lambda d: d.target_file):
                    count = len(dep.predicates)
                    lines.append(f"  ├── imports from: {Path(dep.target_file).name} ({count} predicates)")
                    if self.verbose:
                        pred_list = sorted(dep.predicates)
                        if len(pred_list) <= 5:
                            lines.append(f"      {', '.join(pred_list)}")
                        else:
                            lines.append(f"      {', '.join(pred_list[:5])} ... and {len(pred_list)-5} more")

        # Conflicts
        if self.conflicts:
            lines.append("\n" + "!" * 70)
            lines.append("CONFLICTS")
            lines.append("!" * 70)

            for i, conflict in enumerate(self.conflicts, 1):
                lines.append(f"\n--- Conflict #{i} ({conflict.severity.upper()}) ---")
                lines.append(f"Type: {conflict.conflict_type}")
                lines.append(f"Predicate: {conflict.predicate}")
                lines.append(f"Message: {conflict.message}")
                lines.append(f"Locations:")
                for file, line, context in conflict.locations:
                    lines.append(f"  {Path(file).name}:{line}")
                    lines.append(f"    {context}")

        # Missing definitions
        if self.missing:
            lines.append("\n" + "!" * 70)
            lines.append("MISSING DEFINITIONS")
            lines.append("!" * 70)

            for pred_name in sorted(self.missing):
                usages = self.usages[pred_name]
                lines.append(f"\n'{pred_name}' - used but never defined")
                lines.append(f"  Used in:")
                for usage in usages[:3]:  # Show first 3 usages
                    lines.append(f"    {Path(usage.file).name}:{usage.line}")
                if len(usages) > 3:
                    lines.append(f"    ... and {len(usages)-3} more locations")
                lines.append(f"  (May be a Go virtual predicate - add to --virtual list if intentional)")

        # Unused exports
        if self.unused and self.verbose:
            lines.append("\n" + "-" * 70)
            lines.append("UNUSED EXPORTS")
            lines.append("-" * 70)

            for pred_name in sorted(self.unused):
                defs = self.definitions[pred_name]
                lines.append(f"\n'{pred_name}' - defined but never used")
                for defn in defs:
                    lines.append(f"  {Path(defn.file).name}:{defn.line}")

        # Summary
        lines.append("\n" + "=" * 70)
        lines.append("SUMMARY")
        lines.append("=" * 70)

        if self.conflicts or self.missing:
            lines.append("\nISSUES FOUND:")
            if any(c.severity == "error" for c in self.conflicts):
                lines.append("  - Critical conflicts detected")
            if self.missing:
                lines.append(f"  - {len(self.missing)} predicates used but not defined")
            lines.append("\nStatus: FAILED")
        else:
            lines.append("\nAll checks passed!")
            lines.append("Status: OK")

        lines.append("=" * 70)
        return "\n".join(lines)

    def get_dot_graph(self) -> str:
        """Generate DOT format dependency graph."""
        lines = [
            'digraph module_dependencies {',
            '  rankdir=LR;',
            '  node [shape=box, style=filled, fillcolor=lightblue];',
            ''
        ]

        # Nodes (files)
        for filepath in self.files_analyzed:
            name = Path(filepath).name
            lines.append(f'  "{name}";')

        lines.append('')

        # Edges (dependencies)
        for dep in self.dependencies:
            source_name = Path(dep.source_file).name
            target_name = Path(dep.target_file).name
            count = len(dep.predicates)
            lines.append(f'  "{source_name}" -> "{target_name}" [label="{count}"];')

        lines.append('}')
        return '\n'.join(lines)

    def get_json_result(self) -> dict:
        """Return analysis results as JSON."""
        return {
            'files_analyzed': self.files_analyzed,
            'statistics': {
                'total_predicates': len(set(self.definitions.keys()) | set(self.declarations.keys())),
                'total_definitions': sum(len(defs) for defs in self.definitions.values()),
                'total_declarations': sum(len(decls) for decls in self.declarations.values()),
                'total_usages': sum(len(usages) for usages in self.usages.values()),
                'cross_file_dependencies': len(self.dependencies),
            },
            'conflicts': [
                {
                    'type': c.conflict_type,
                    'predicate': c.predicate,
                    'severity': c.severity,
                    'message': c.message,
                    'locations': [
                        {'file': f, 'line': ln, 'context': ctx}
                        for f, ln, ctx in c.locations
                    ]
                }
                for c in self.conflicts
            ],
            'missing_definitions': [
                {
                    'predicate': pred,
                    'usages': [
                        {'file': u.file, 'line': u.line}
                        for u in self.usages[pred]
                    ]
                }
                for pred in sorted(self.missing)
            ],
            'unused_exports': list(sorted(self.unused)),
            'dependencies': [
                {
                    'source': dep.source_file,
                    'target': dep.target_file,
                    'predicates': sorted(dep.predicates)
                }
                for dep in self.dependencies
            ],
            'status': 'ok' if not (self.conflicts or self.missing) else 'failed'
        }


def main():
    # Ensure UTF-8 output on Windows
    if sys.platform == 'win32':
        import io
        sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding='utf-8', errors='replace')
        sys.stderr = io.TextIOWrapper(sys.stderr.buffer, encoding='utf-8', errors='replace')

    parser = argparse.ArgumentParser(
        description="Analyze multiple Mangle files for cross-file coherence",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog=__doc__
    )
    parser.add_argument('files', nargs='+', help='Mangle files to analyze')
    parser.add_argument('--check-completeness', action='store_true',
                       help='Fail if any predicate is undefined')
    parser.add_argument('--graph', '-g', action='store_true',
                       help='Output DOT format dependency graph')
    parser.add_argument('--json', '-j', action='store_true',
                       help='Output as JSON')
    parser.add_argument('--virtual', help='Comma-separated list of virtual predicates (implemented in Go)')
    parser.add_argument('--strict', action='store_true',
                       help='Treat warnings as errors')
    parser.add_argument('--verbose', '-v', action='store_true',
                       help='Show detailed analysis')

    args = parser.parse_args()

    # Parse virtual predicates
    virtual_preds = set()
    if args.virtual:
        virtual_preds = set(p.strip() for p in args.virtual.split(','))

    # Create analyzer
    analyzer = ModuleAnalyzer(verbose=args.verbose, virtual_predicates=virtual_preds)

    # Convert file arguments to Path objects
    filepaths = [Path(f) for f in args.files]

    # Analyze
    is_ok = analyzer.analyze_files(filepaths)

    # Apply strict mode
    if args.strict and any(c.severity == "warning" for c in analyzer.conflicts):
        is_ok = False

    # Apply completeness check
    if args.check_completeness and analyzer.missing:
        is_ok = False

    # Output results
    if args.json:
        print(json.dumps(analyzer.get_json_result(), indent=2))
    elif args.graph:
        print(analyzer.get_dot_graph())
    else:
        print(analyzer.get_report())

    sys.exit(0 if is_ok else 1)


if __name__ == "__main__":
    main()
