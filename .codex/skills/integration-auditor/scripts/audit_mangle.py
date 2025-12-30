#!/usr/bin/env python3
"""
Mangle Schema/Policy Auditor for codeNERD

Deep audit of Mangle kernel integration:
- Schema declarations (Decl statements)
- Policy rules and predicate usage
- Virtual predicate handlers
- Go code fact generation (ToAtom patterns)

Usage:
    python audit_mangle.py [workspace_path] [--verbose] [--json]
"""

import os
import re
import sys
import argparse
import json
from pathlib import Path
from dataclasses import dataclass, field
from typing import List, Dict, Set, Optional, Tuple
from enum import Enum

class Severity(Enum):
    ERROR = "ERROR"
    WARNING = "WARNING"
    INFO = "INFO"
    OK = "OK"

@dataclass
class Finding:
    severity: Severity
    message: str
    file: Optional[str] = None
    line: Optional[int] = None
    suggestion: Optional[str] = None

@dataclass
class PredicateInfo:
    name: str
    arity: Optional[int] = None
    declared: bool = False
    decl_file: Optional[str] = None
    decl_line: Optional[int] = None
    used_in_rules: List[Tuple[str, int]] = field(default_factory=list)  # [(file, line), ...]
    used_in_go: List[Tuple[str, int]] = field(default_factory=list)     # [(file, line), ...]
    is_virtual: bool = False
    has_handler: bool = False

@dataclass
class AuditResult:
    predicates: Dict[str, PredicateInfo] = field(default_factory=dict)
    virtual_handlers: Set[str] = field(default_factory=set)
    findings: List[Finding] = field(default_factory=list)
    stats: Dict[str, int] = field(default_factory=dict)

class MangleAuditor:
    def __init__(self, workspace: str, verbose: bool = False):
        self.workspace = Path(workspace)
        self.verbose = verbose
        self.result = AuditResult()

    def audit(self) -> AuditResult:
        """Run complete Mangle audit."""
        print(f"[*] Auditing Mangle schema/policy in {self.workspace}")
        print()

        # Phase 1: Scan schema files for Decl statements
        self._scan_schemas()

        # Phase 2: Scan policy files for predicate usage
        self._scan_policy()

        # Phase 3: Scan VirtualStore for virtual predicate handlers
        self._scan_virtual_store()

        # Phase 4: Scan Go code for fact generation patterns
        self._scan_go_facts()

        # Phase 5: Cross-reference and find gaps
        self._check_declarations()
        self._check_virtual_predicates()

        # Calculate stats
        self._calculate_stats()

        return self.result

    def _scan_schemas(self):
        """Scan schema files for Decl statements."""
        schema_paths = [
            self.workspace / "internal" / "core" / "defaults" / "schemas.mg",
            self.workspace / "internal" / "core" / "defaults" / "inference.mg",
            self.workspace / "internal" / "mangle" / "schemas.gl",
            self.workspace / ".nerd" / "mangle" / "schemas.mg",
        ]

        # Also scan all .mg files in defaults directory (including subdirectories)
        defaults_dir = self.workspace / "internal" / "core" / "defaults"
        if defaults_dir.exists():
            for mg_file in defaults_dir.glob("**/*.mg"):
                if mg_file not in schema_paths:
                    schema_paths.append(mg_file)

        for schema_file in schema_paths:
            if not schema_file.exists():
                continue

            content = schema_file.read_text(encoding='utf-8')
            lines = content.split('\n')

            # Match Decl statements: Decl predicate_name(Arg1, Arg2, ...).
            decl_pattern = r'^Decl\s+(\w+)\s*\(([^)]*)\)'

            for i, line in enumerate(lines, 1):
                # Skip comments
                stripped = line.strip()
                if stripped.startswith('#') or stripped.startswith('//'):
                    continue

                match = re.search(decl_pattern, stripped)
                if match:
                    pred_name = match.group(1)
                    args = match.group(2)
                    arity = len([a.strip() for a in args.split(',') if a.strip()]) if args.strip() else 0

                    if pred_name not in self.result.predicates:
                        self.result.predicates[pred_name] = PredicateInfo(name=pred_name)

                    self.result.predicates[pred_name].declared = True
                    self.result.predicates[pred_name].arity = arity
                    self.result.predicates[pred_name].decl_file = str(schema_file)
                    self.result.predicates[pred_name].decl_line = i

    def _scan_policy(self):
        """Scan policy files for predicate usage in rules."""
        policy_paths = [
            self.workspace / "internal" / "core" / "defaults" / "policy.mg",
            self.workspace / "internal" / "mangle" / "policy.gl",
            self.workspace / ".nerd" / "mangle" / "policy.mg",
        ]

        for policy_file in policy_paths:
            if not policy_file.exists():
                continue

            content = policy_file.read_text(encoding='utf-8')
            lines = content.split('\n')

            # Find predicates in rule heads and bodies
            # Rule pattern: head(...) :- body1(...), body2(...), ...
            # Also match: predicate(args).

            for i, line in enumerate(lines, 1):
                stripped = line.strip()
                if stripped.startswith('#') or stripped.startswith('//') or not stripped:
                    continue

                # Find all predicate calls: word followed by (
                pred_pattern = r'\b(\w+)\s*\('
                for match in re.finditer(pred_pattern, stripped):
                    pred_name = match.group(1)

                    # Skip Mangle built-ins and function calls
                    builtins = {'fn', 'do', 'let', 'Decl', 'if', 'else', 'not', 'Type'}
                    if pred_name in builtins:
                        continue

                    # Skip fn:xxx function calls (built-in functions like fn:pair, fn:Count, fn:plus)
                    # Check if this is preceded by "fn:" in the line
                    match_pos = match.start()
                    if match_pos >= 3 and stripped[match_pos-3:match_pos] == 'fn:':
                        continue

                    if pred_name not in self.result.predicates:
                        self.result.predicates[pred_name] = PredicateInfo(name=pred_name)

                    self.result.predicates[pred_name].used_in_rules.append(
                        (str(policy_file), i)
                    )

    def _scan_virtual_store(self):
        """Scan VirtualStore for virtual predicate handlers."""
        vs_file = self.workspace / "internal" / "core" / "virtual_store.go"
        if not vs_file.exists():
            self.result.findings.append(Finding(
                severity=Severity.WARNING,
                message="virtual_store.go not found",
                file=str(vs_file)
            ))
            return

        content = vs_file.read_text(encoding='utf-8')
        lines = content.split('\n')

        # Method-based virtual predicate handlers (method name -> predicate name)
        # These are methods on VirtualStore that implement virtual predicates
        method_to_predicate = {
            'QueryLearned': 'query_learned',
            'QuerySession': 'query_session',
            'RecallSimilar': 'recall_similar',
            'QueryKnowledgeGraph': 'query_knowledge_graph',
            'QueryActivations': 'query_activations',
            'HasLearned': 'has_learned',
            'QueryTraces': 'query_traces',
            'QueryTraceStats': 'query_trace_stats',
        }

        # Scan for method implementations
        for i, line in enumerate(lines, 1):
            # Look for method definitions: func (v *VirtualStore) MethodName(
            method_match = re.search(r'func\s+\([^)]+\*VirtualStore\)\s+(\w+)\s*\(', line)
            if method_match:
                method_name = method_match.group(1)
                if method_name in method_to_predicate:
                    pred_name = method_to_predicate[method_name]
                    self.result.virtual_handlers.add(pred_name)

                    if pred_name not in self.result.predicates:
                        self.result.predicates[pred_name] = PredicateInfo(name=pred_name)
                    self.result.predicates[pred_name].is_virtual = True
                    self.result.predicates[pred_name].has_handler = True

        # Also check for case-based handlers in Get() method
        in_get_method = False
        brace_count = 0

        for i, line in enumerate(lines, 1):
            # Look for Get method
            if 'func (vs *VirtualStore) Get(' in line or 'func (vs *VirtualStore)Get(' in line:
                in_get_method = True
                continue

            if in_get_method:
                brace_count += line.count('{') - line.count('}')

                # Look for case statements
                case_match = re.search(r'case\s+"(\w+)":', line)
                if case_match:
                    pred_name = case_match.group(1)
                    self.result.virtual_handlers.add(pred_name)

                    # Mark as virtual and has handler
                    if pred_name not in self.result.predicates:
                        self.result.predicates[pred_name] = PredicateInfo(name=pred_name)
                    self.result.predicates[pred_name].is_virtual = True
                    self.result.predicates[pred_name].has_handler = True

                # Exit when method ends
                if brace_count <= 0 and in_get_method and i > 10:
                    in_get_method = False

    def _scan_go_facts(self):
        """Scan Go code for fact generation patterns."""
        # Patterns that indicate fact creation
        fact_patterns = [
            # Direct fact construction
            (r'core\.Fact\{[^}]*Predicate:\s*"(\w+)"', 'Fact struct'),
            (r'Fact\{[^}]*Predicate:\s*"(\w+)"', 'Fact struct'),
            # ToAtom patterns
            (r'\.ToAtom\(\)', 'ToAtom'),
            # LoadFacts with predicate
            (r'LoadFacts.*"(\w+)"', 'LoadFacts'),
            # kernel.Assert
            (r'\.Assert.*"(\w+)"', 'Assert'),
            # kernel.Query
            (r'\.Query\s*\(\s*"(\w+)"', 'Query'),
        ]

        # Scan all Go files
        for go_file in self.workspace.rglob("*.go"):
            if "vendor" in str(go_file) or "_test.go" in str(go_file):
                continue

            try:
                content = go_file.read_text(encoding='utf-8')
            except:
                continue

            lines = content.split('\n')

            for i, line in enumerate(lines, 1):
                for pattern, pattern_type in fact_patterns:
                    for match in re.finditer(pattern, line):
                        if match.lastindex and match.lastindex >= 1:
                            pred_name = match.group(1)

                            if pred_name not in self.result.predicates:
                                self.result.predicates[pred_name] = PredicateInfo(name=pred_name)

                            self.result.predicates[pred_name].used_in_go.append(
                                (str(go_file), i)
                            )

    def _check_declarations(self):
        """Check that all used predicates are declared."""
        for pred_name, pred in self.result.predicates.items():
            # Skip built-in and special predicates
            if pred_name.startswith('fn') or pred_name in {'do', 'let', 'if'}:
                continue

            if not pred.declared:
                # Used but not declared
                if pred.used_in_rules or pred.used_in_go:
                    locations = []
                    if pred.used_in_rules:
                        file, line = pred.used_in_rules[0]
                        locations.append(f"policy: {Path(file).name}:{line}")
                    if pred.used_in_go:
                        file, line = pred.used_in_go[0]
                        locations.append(f"Go: {Path(file).name}:{line}")

                    self.result.findings.append(Finding(
                        severity=Severity.ERROR,
                        message=f"Predicate '{pred_name}' used but not declared ({', '.join(locations)})",
                        suggestion=f"Add 'Decl {pred_name}(...).' to schemas.mg"
                    ))
            else:
                # Declared but never used (info only)
                if not pred.used_in_rules and not pred.used_in_go:
                    self.result.findings.append(Finding(
                        severity=Severity.INFO,
                        message=f"Predicate '{pred_name}' declared but not used",
                        file=pred.decl_file,
                        line=pred.decl_line
                    ))

    def _check_virtual_predicates(self):
        """Check virtual predicate completeness."""
        # Known virtual predicates (from schema comments)
        known_virtual = {
            'query_learned', 'query_session', 'recall_similar',
            'query_knowledge_graph', 'query_activations', 'has_learned',
            'query_traces', 'query_trace_stats',
        }

        for pred_name, pred in self.result.predicates.items():
            if pred_name in known_virtual:
                pred.is_virtual = True
                if pred_name not in self.result.virtual_handlers:
                    self.result.findings.append(Finding(
                        severity=Severity.WARNING,
                        message=f"Virtual predicate '{pred_name}' declared but no handler in VirtualStore.Get()",
                        suggestion=f'Add case "{pred_name}": handler in virtual_store.go Get()'
                    ))

        # Check for handlers without declarations
        for handler_name in self.result.virtual_handlers:
            if handler_name not in self.result.predicates or not self.result.predicates[handler_name].declared:
                self.result.findings.append(Finding(
                    severity=Severity.WARNING,
                    message=f"VirtualStore has handler for '{handler_name}' but no Decl in schema",
                    suggestion=f"Add 'Decl {handler_name}(...).' to schemas.mg"
                ))

    def _calculate_stats(self):
        """Calculate audit statistics."""
        declared_count = sum(1 for p in self.result.predicates.values() if p.declared)
        used_in_rules = sum(1 for p in self.result.predicates.values() if p.used_in_rules)
        used_in_go = sum(1 for p in self.result.predicates.values() if p.used_in_go)
        virtual_count = len(self.result.virtual_handlers)

        errors = sum(1 for f in self.result.findings if f.severity == Severity.ERROR)
        warnings = sum(1 for f in self.result.findings if f.severity == Severity.WARNING)

        self.result.stats = {
            "total_predicates": len(self.result.predicates),
            "declared": declared_count,
            "used_in_rules": used_in_rules,
            "used_in_go": used_in_go,
            "virtual_handlers": virtual_count,
            "errors": errors,
            "warnings": warnings,
        }

    def print_report(self) -> bool:
        """Print formatted audit report."""
        print("=" * 70)
        print("MANGLE SCHEMA/POLICY AUDIT REPORT")
        print("=" * 70)
        print()

        # Summary
        print(f"Total Predicates Found: {self.result.stats['total_predicates']}")
        print(f"  Declared in Schema:   {self.result.stats['declared']}")
        print(f"  Used in Policy/Rules: {self.result.stats['used_in_rules']}")
        print(f"  Used in Go Code:      {self.result.stats['used_in_go']}")
        print(f"  Virtual Handlers:     {self.result.stats['virtual_handlers']}")
        print()
        print(f"Errors: {self.result.stats['errors']}  Warnings: {self.result.stats['warnings']}")
        print()

        # Errors first
        errors = [f for f in self.result.findings if f.severity == Severity.ERROR]
        if errors:
            print("-" * 70)
            print("ERRORS (Must Fix)")
            print("-" * 70)
            for f in errors:
                print(f"[ERROR] {f.message}")
                if self.verbose and f.suggestion:
                    print(f"        -> {f.suggestion}")
            print()

        # Warnings
        warnings = [f for f in self.result.findings if f.severity == Severity.WARNING]
        if warnings:
            print("-" * 70)
            print("WARNINGS")
            print("-" * 70)
            for f in warnings:
                print(f"[WARNING] {f.message}")
                if self.verbose and f.suggestion:
                    print(f"          -> {f.suggestion}")
            print()

        # Info (only in verbose mode)
        if self.verbose:
            infos = [f for f in self.result.findings if f.severity == Severity.INFO]
            if infos:
                print("-" * 70)
                print("INFO")
                print("-" * 70)
                for f in infos:
                    print(f"[INFO] {f.message}")

        # Virtual predicate details
        if self.verbose and self.result.virtual_handlers:
            print()
            print("-" * 70)
            print("VIRTUAL PREDICATE HANDLERS")
            print("-" * 70)
            for handler in sorted(self.result.virtual_handlers):
                pred = self.result.predicates.get(handler)
                declared = "Y" if pred and pred.declared else "N"
                print(f"  {handler} (Decl: {declared})")

        print()
        print("=" * 70)
        return self.result.stats['errors'] == 0


def find_workspace(start_path: str) -> Path:
    """Find codeNERD workspace root."""
    workspace = Path(start_path).resolve()
    while workspace != workspace.parent:
        if (workspace / ".nerd").exists() or (workspace / "go.mod").exists():
            return workspace
        workspace = workspace.parent
    return Path(start_path).resolve()


def main():
    parser = argparse.ArgumentParser(description="Audit codeNERD Mangle schema/policy")
    parser.add_argument("workspace", nargs="?", default=".", help="Workspace path")
    parser.add_argument("--verbose", "-v", action="store_true", help="Show all findings")
    parser.add_argument("--json", action="store_true", help="Output as JSON")

    args = parser.parse_args()

    workspace = find_workspace(args.workspace)
    auditor = MangleAuditor(str(workspace), verbose=args.verbose)
    result = auditor.audit()

    if args.json:
        output = {
            "predicates": {
                name: {
                    "declared": p.declared,
                    "arity": p.arity,
                    "is_virtual": p.is_virtual,
                    "has_handler": p.has_handler,
                    "rule_usages": len(p.used_in_rules),
                    "go_usages": len(p.used_in_go),
                }
                for name, p in result.predicates.items()
            },
            "virtual_handlers": list(result.virtual_handlers),
            "findings": [
                {"severity": f.severity.value, "message": f.message}
                for f in result.findings
            ],
            "stats": result.stats,
        }
        print(json.dumps(output, indent=2))
    else:
        success = auditor.print_report()
        sys.exit(0 if success else 1)


if __name__ == "__main__":
    main()
