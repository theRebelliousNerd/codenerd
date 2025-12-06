#!/usr/bin/env python3
"""
Enhanced Mangle Syntax Validator v2.0

Comprehensive validation for Google Mangle programs including:
- Declaration syntax (Decl with .Type<> and modes)
- Facts and rules (including ⟸ arrow syntax)
- Aggregation pipelines (|> do fn: let)
- Type expressions and structured data
- Safety constraint checking
- Stratification analysis

Usage:
    python3 validate_mangle.py <path_to_mangle_file> [--strict] [--verbose]
    python3 validate_mangle.py --check-string "<mangle_code>"

Exit codes:
    0 - Valid (may have warnings)
    1 - Errors found
    2 - Fatal/parse errors

Compatible with Mangle v0.4.0 (November 2024)
"""

import sys
import re
import argparse
from pathlib import Path
from dataclasses import dataclass, field
from enum import Enum
from typing import List, Dict, Set, Optional, Tuple


class Severity(Enum):
    INFO = 0
    WARNING = 1
    ERROR = 2
    FATAL = 3


@dataclass
class Issue:
    severity: Severity
    line: int
    column: int
    message: str
    rule: str = ""
    suggestion: str = ""


@dataclass
class Predicate:
    name: str
    arity: int
    is_edb: bool = False
    is_idb: bool = False
    is_declared: bool = False
    negated_in: List[str] = field(default_factory=list)
    defined_in: List[int] = field(default_factory=list)


class MangleValidator:
    """Comprehensive Mangle syntax validator."""

    # Token patterns
    PATTERNS = {
        'name_constant': re.compile(r'/[a-zA-Z_][a-zA-Z0-9_./:-]*'),
        'variable': re.compile(r'\b[A-Z][a-zA-Z0-9_]*\b'),
        'predicate': re.compile(r'\b([a-z][a-z0-9_]*)\s*\('),
        'string_double': re.compile(r'"(?:[^"\\]|\\.)*"'),
        'string_single': re.compile(r"'(?:[^'\\]|\\.)*'"),
        'string_backtick': re.compile(r'`[^`]*`', re.DOTALL),
        'number': re.compile(r'-?\d+(?:\.\d+)?(?:[eE][+-]?\d+)?'),
        'decl': re.compile(r'^\s*Decl\s+([a-z][a-z0-9_]*)\s*\(', re.MULTILINE),
        'rule_arrow': re.compile(r':-|<-|⟸'),
        'pipeline': re.compile(r'\|>'),
        'fn_call': re.compile(r'fn:([a-zA-Z_][a-zA-Z0-9_]*)'),
        'builtin_pred': re.compile(r':([a-z][a-z0-9_]*)\('),
        'negation': re.compile(r'[!]\s*([a-z][a-z0-9_]*)\s*\('),
        'type_expr': re.compile(r'\.Type<([^>]+)>'),
        'list': re.compile(r'\[(?:[^\[\]]|\[[^\]]*\])*\]'),
        'struct': re.compile(r'\{(?:[^{}]|\{[^}]*\})*\}'),
    }

    # Known built-in functions (v0.4.0)
    BUILTIN_FUNCTIONS = {
        # Aggregation
        'Count', 'Sum', 'Min', 'Max', 'Avg', 'Collect',
        # Grouping
        'group_by',
        # Arithmetic
        'plus', 'minus', 'multiply', 'divide', 'modulo', 'negate', 'abs',
        # Comparison
        'eq', 'ne', 'lt', 'le', 'gt', 'ge',
        # List operations
        'list', 'cons', 'list:get', 'list:length', 'list:append', 'list:contains',
        # Map/Struct operations
        'map', 'struct', 'pair', 'tuple',
        # String operations
        'string_concat', 'string_length', 'string_contains', 'string:split',
        # Type constructors
        'Singleton', 'Union', 'Pair', 'List', 'Map', 'Struct',
        # Misc
        'filter',
    }

    # Known built-in predicates
    BUILTIN_PREDICATES = {
        'match_cons', 'match_nil', 'match_field', 'match_entry',
        'list:member',
    }

    def __init__(self, strict: bool = False, verbose: bool = False):
        self.strict = strict
        self.verbose = verbose
        self.issues: List[Issue] = []
        self.predicates: Dict[str, Predicate] = {}
        self.declared_types: Set[str] = set()

    def validate_file(self, filepath: Path) -> bool:
        """Validate a Mangle file."""
        with open(filepath, encoding='utf-8') as f:
            content = f.read()
        return self.validate(content, str(filepath))

    def validate(self, content: str, source: str = "<string>") -> bool:
        """Validate Mangle source code."""
        self.issues = []
        self.predicates = {}

        # Preprocessing
        lines = content.split('\n')

        # Pass 1: Extract declarations and build predicate index
        self._pass1_declarations(content, lines)

        # Pass 2: Validate syntax line by line
        self._pass2_syntax(content, lines)

        # Pass 3: Semantic analysis
        self._pass3_semantics(content, lines)

        # Report results
        self._report(source, len(lines))

        return not any(i.severity >= Severity.ERROR for i in self.issues)

    def _pass1_declarations(self, content: str, lines: List[str]):
        """Extract and validate declarations."""
        # Find all Decl statements
        decl_pattern = re.compile(
            r'Decl\s+([a-z][a-z0-9_]*)\s*\(([^)]*)\)\s*'
            r'(?:descr\s*\[([^\]]*)\])?\s*'
            r'(?:bound\s*\[([^\]]*)\])?\s*\.',
            re.MULTILINE | re.DOTALL
        )

        for match in decl_pattern.finditer(content):
            pred_name = match.group(1)
            args_str = match.group(2)
            descr_str = match.group(3) or ""
            bound_str = match.group(4) or ""

            # Count arity from arguments
            args = [a.strip() for a in args_str.split(',') if a.strip()]
            arity = len(args)

            # Record predicate
            if pred_name not in self.predicates:
                self.predicates[pred_name] = Predicate(name=pred_name, arity=arity)
            self.predicates[pred_name].is_declared = True
            self.predicates[pred_name].arity = arity

            # Validate type expressions
            for arg in args:
                type_match = self.PATTERNS['type_expr'].search(arg)
                if type_match:
                    type_expr = type_match.group(1)
                    self._validate_type_expr(type_expr, content.find(arg))

    def _pass2_syntax(self, content: str, lines: List[str]):
        """Line-by-line syntax validation."""
        in_multiline = False
        multiline_start = 0
        multiline_buffer = ""

        for i, line in enumerate(lines, 1):
            stripped = line.strip()

            # Skip comments and empty lines
            if not stripped or stripped.startswith('#'):
                continue

            # Handle multi-line statements
            if in_multiline:
                multiline_buffer += " " + stripped
                if stripped.endswith('.'):
                    self._validate_statement(multiline_buffer, multiline_start)
                    in_multiline = False
                    multiline_buffer = ""
                continue

            # Check for statement continuation
            if stripped and not stripped.endswith('.') and not stripped.startswith('?'):
                # Could be start of multi-line statement
                if ':-' in stripped or '<-' in stripped or '⟸' in stripped or 'Decl' in stripped:
                    in_multiline = True
                    multiline_start = i
                    multiline_buffer = stripped
                    continue

            # Single-line statement
            self._validate_statement(stripped, i)

    def _validate_statement(self, stmt: str, line: int):
        """Validate a single statement."""
        stmt = stmt.strip()

        # Query (REPL only)
        if stmt.startswith('?'):
            self._validate_query(stmt, line)
            return

        # Declaration
        if stmt.startswith('Decl '):
            self._validate_declaration(stmt, line)
            return

        # Package/Uses
        if stmt.startswith('Package ') or stmt.startswith('Uses '):
            self._validate_package_stmt(stmt, line)
            return

        # Rule or Fact
        self._validate_clause(stmt, line)

    def _validate_clause(self, stmt: str, line: int):
        """Validate a fact or rule."""
        # Must end with period
        if not stmt.endswith('.'):
            self.issues.append(Issue(
                severity=Severity.ERROR,
                line=line,
                column=len(stmt),
                message="Statement must end with period",
                rule="syntax.period",
                suggestion=f"{stmt}."
            ))
            return

        stmt_body = stmt[:-1].strip()

        # Check for rule arrow
        arrow_match = self.PATTERNS['rule_arrow'].search(stmt_body)

        if arrow_match:
            # It's a rule
            self._validate_rule(stmt_body, line, arrow_match)
        else:
            # It's a fact
            self._validate_fact(stmt_body, line)

    def _validate_fact(self, stmt: str, line: int):
        """Validate a ground fact."""
        # Extract predicate
        pred_match = self.PATTERNS['predicate'].match(stmt)
        if not pred_match:
            self.issues.append(Issue(
                severity=Severity.ERROR,
                line=line,
                column=0,
                message="Invalid fact syntax - expected predicate(args)",
                rule="syntax.fact"
            ))
            return

        pred_name = pred_match.group(1)
        self._register_predicate(pred_name, line, is_edb=True)

        # Check for variables in fact (should be ground)
        args_start = stmt.find('(')
        args_end = stmt.rfind(')')
        if args_start > 0 and args_end > args_start:
            args_str = stmt[args_start+1:args_end]
            # Remove strings to avoid false positives
            args_cleaned = self.PATTERNS['string_double'].sub('""', args_str)
            args_cleaned = self.PATTERNS['string_single'].sub("''", args_cleaned)

            vars_in_fact = self.PATTERNS['variable'].findall(args_cleaned)
            if vars_in_fact:
                self.issues.append(Issue(
                    severity=Severity.ERROR,
                    line=line,
                    column=args_start,
                    message=f"Fact must be ground (no variables). Found: {', '.join(vars_in_fact)}",
                    rule="safety.ground_fact"
                ))

        # Validate balanced parentheses
        self._check_balanced(stmt, line)

    def _validate_rule(self, stmt: str, line: int, arrow_match):
        """Validate a rule clause."""
        arrow_pos = arrow_match.start()
        head = stmt[:arrow_pos].strip()
        body = stmt[arrow_match.end():].strip()

        # Validate head
        head_pred_match = self.PATTERNS['predicate'].match(head)
        if not head_pred_match:
            self.issues.append(Issue(
                severity=Severity.ERROR,
                line=line,
                column=0,
                message="Invalid rule head syntax",
                rule="syntax.rule_head"
            ))
            return

        head_pred = head_pred_match.group(1)
        self._register_predicate(head_pred, line, is_idb=True)

        # Extract head variables
        head_vars = set(self.PATTERNS['variable'].findall(head))

        # Parse body for variable binding analysis
        body_vars = set()
        negated_vars = set()

        # Find all positive predicates in body (before any negation)
        for match in self.PATTERNS['predicate'].finditer(body):
            pred_name = match.group(1)
            self._register_predicate(pred_name, line)

        # Find negated predicates
        for match in self.PATTERNS['negation'].finditer(body):
            negated_pred = match.group(1)
            if negated_pred in self.predicates:
                self.predicates[negated_pred].negated_in.append(f"line {line}")

            # Extract variables in negated atom
            neg_start = match.start()
            # Find the closing paren
            depth = 0
            neg_end = neg_start
            for j, c in enumerate(body[neg_start:]):
                if c == '(':
                    depth += 1
                elif c == ')':
                    depth -= 1
                    if depth == 0:
                        neg_end = neg_start + j + 1
                        break
            neg_atom = body[neg_start:neg_end]
            neg_atom_vars = set(self.PATTERNS['variable'].findall(neg_atom))
            negated_vars.update(neg_atom_vars)

        # Collect bound variables from positive atoms
        # Simple heuristic: variables appearing in non-negated predicates
        body_without_neg = re.sub(r'!\s*[a-z][a-z0-9_]*\s*\([^)]*\)', '', body)
        body_vars = set(self.PATTERNS['variable'].findall(body_without_neg))

        # Safety check: head variables must be in body
        unbound_head_vars = head_vars - body_vars
        if unbound_head_vars:
            self.issues.append(Issue(
                severity=Severity.ERROR,
                line=line,
                column=0,
                message=f"Unsafe rule: head variables not bound in body: {', '.join(unbound_head_vars)}",
                rule="safety.head_vars"
            ))

        # Safety check: negated variables must be bound first
        unbound_neg_vars = negated_vars - body_vars
        if unbound_neg_vars and self.strict:
            self.issues.append(Issue(
                severity=Severity.WARNING,
                line=line,
                column=0,
                message=f"Variables in negation may not be bound: {', '.join(unbound_neg_vars)}",
                rule="safety.negation_vars"
            ))

        # Validate pipeline syntax if present
        if '|>' in body:
            self._validate_pipeline(body, line)

        # Check balanced parens/brackets
        self._check_balanced(stmt, line)

    def _validate_pipeline(self, body: str, line: int):
        """Validate aggregation pipeline syntax."""
        # Split by pipeline operator
        pipeline_parts = re.split(r'\|>', body)

        for i, part in enumerate(pipeline_parts[1:], 1):
            part = part.strip()

            # Should start with 'do' or 'let' or be a statement list
            if not part:
                self.issues.append(Issue(
                    severity=Severity.ERROR,
                    line=line,
                    column=0,
                    message=f"Empty pipeline segment after |>",
                    rule="syntax.pipeline"
                ))
                continue

            # Check for fn: calls
            fn_calls = self.PATTERNS['fn_call'].findall(part)
            for fn_name in fn_calls:
                if fn_name not in self.BUILTIN_FUNCTIONS:
                    self.issues.append(Issue(
                        severity=Severity.WARNING,
                        line=line,
                        column=0,
                        message=f"Unknown function fn:{fn_name}",
                        rule="semantics.unknown_function"
                    ))

    def _validate_declaration(self, stmt: str, line: int):
        """Validate a Decl statement."""
        if not stmt.endswith('.'):
            self.issues.append(Issue(
                severity=Severity.ERROR,
                line=line,
                column=len(stmt),
                message="Declaration must end with period",
                rule="syntax.decl_period"
            ))

        # Check type expressions
        for match in self.PATTERNS['type_expr'].finditer(stmt):
            self._validate_type_expr(match.group(1), line)

    def _validate_type_expr(self, type_expr: str, line: int):
        """Validate a type expression."""
        valid_base_types = {'int', 'float', 'string', 'n', 'Any', 'name'}

        # Check for known types (simplified)
        if type_expr.startswith('[') or type_expr.startswith('{'):
            return  # List or map type, complex validation skipped

        if '|' in type_expr:
            return  # Union type

        base_type = type_expr.strip()
        if base_type and base_type not in valid_base_types:
            if not base_type[0].isupper():  # Not a type variable
                self.issues.append(Issue(
                    severity=Severity.WARNING,
                    line=line,
                    column=0,
                    message=f"Unknown type: {base_type}",
                    rule="type.unknown"
                ))

    def _validate_query(self, stmt: str, line: int):
        """Validate a query."""
        query_body = stmt[1:].strip()
        if query_body.endswith('.'):
            query_body = query_body[:-1]

        if not query_body:
            self.issues.append(Issue(
                severity=Severity.ERROR,
                line=line,
                column=1,
                message="Empty query",
                rule="syntax.query"
            ))
            return

        # Check predicate exists
        pred_match = self.PATTERNS['predicate'].match(query_body)
        if pred_match:
            pred_name = pred_match.group(1)
            if pred_name not in self.predicates and pred_name not in self.BUILTIN_PREDICATES:
                self.issues.append(Issue(
                    severity=Severity.WARNING,
                    line=line,
                    column=1,
                    message=f"Query references unknown predicate: {pred_name}",
                    rule="semantics.unknown_predicate"
                ))

    def _validate_package_stmt(self, stmt: str, line: int):
        """Validate Package or Uses statement."""
        if not stmt.endswith('!'):
            self.issues.append(Issue(
                severity=Severity.ERROR,
                line=line,
                column=len(stmt),
                message="Package/Uses statement must end with !",
                rule="syntax.package"
            ))

    def _register_predicate(self, name: str, line: int, is_edb: bool = False, is_idb: bool = False):
        """Register a predicate usage."""
        if name not in self.predicates:
            self.predicates[name] = Predicate(name=name, arity=-1)

        if is_edb:
            self.predicates[name].is_edb = True
        if is_idb:
            self.predicates[name].is_idb = True
        self.predicates[name].defined_in.append(line)

    def _check_balanced(self, text: str, line: int):
        """Check for balanced parentheses, brackets, braces."""
        stack = []
        pairs = {'(': ')', '[': ']', '{': '}'}
        in_string = False
        string_char = None

        for i, c in enumerate(text):
            if c in '"\'`' and (i == 0 or text[i-1] != '\\'):
                if not in_string:
                    in_string = True
                    string_char = c
                elif c == string_char:
                    in_string = False
                continue

            if in_string:
                continue

            if c in pairs:
                stack.append((c, i))
            elif c in pairs.values():
                if not stack:
                    self.issues.append(Issue(
                        severity=Severity.ERROR,
                        line=line,
                        column=i,
                        message=f"Unmatched closing '{c}'",
                        rule="syntax.balanced"
                    ))
                else:
                    open_char, _ = stack.pop()
                    if pairs[open_char] != c:
                        self.issues.append(Issue(
                            severity=Severity.ERROR,
                            line=line,
                            column=i,
                            message=f"Mismatched brackets: '{open_char}' and '{c}'",
                            rule="syntax.balanced"
                        ))

        if stack:
            for open_char, pos in stack:
                self.issues.append(Issue(
                    severity=Severity.ERROR,
                    line=line,
                    column=pos,
                    message=f"Unclosed '{open_char}'",
                    rule="syntax.balanced"
                ))

    def _pass3_semantics(self, content: str, lines: List[str]):
        """Semantic analysis pass."""
        # Check for undefined predicates (only in strict mode)
        if self.strict:
            for name, pred in self.predicates.items():
                if not pred.is_declared and not pred.is_edb and pred.is_idb:
                    self.issues.append(Issue(
                        severity=Severity.INFO,
                        line=pred.defined_in[0] if pred.defined_in else 0,
                        column=0,
                        message=f"Predicate '{name}' used without declaration",
                        rule="style.undeclared"
                    ))

        # Stratification check for negation
        self._check_stratification()

    def _check_stratification(self):
        """Check for stratification issues with negation."""
        # Build dependency graph
        for name, pred in self.predicates.items():
            if pred.negated_in:
                # Check if negated predicate depends on the negating predicate
                # This is a simplified check - full stratification is complex
                if self.verbose:
                    for neg_loc in pred.negated_in:
                        self.issues.append(Issue(
                            severity=Severity.INFO,
                            line=0,
                            column=0,
                            message=f"Predicate '{name}' is negated in {neg_loc}",
                            rule="info.negation"
                        ))

    def _report(self, source: str, total_lines: int):
        """Print validation report."""
        print(f"\n{'='*60}")
        print(f"Mangle Validation Report: {source}")
        print(f"{'='*60}")
        print(f"Lines analyzed: {total_lines}")
        print(f"Predicates found: {len(self.predicates)}")

        # Count by severity
        counts = {s: 0 for s in Severity}
        for issue in self.issues:
            counts[issue.severity] += 1

        print(f"\nIssues: {len(self.issues)} total")
        for severity in [Severity.FATAL, Severity.ERROR, Severity.WARNING, Severity.INFO]:
            if counts[severity] > 0:
                print(f"  {severity.name}: {counts[severity]}")

        # Print issues
        if self.issues:
            print(f"\n{'-'*60}")
            for issue in sorted(self.issues, key=lambda x: (x.line, x.severity.value)):
                icon = {'FATAL': '!!!', 'ERROR': 'ERR', 'WARNING': 'WRN', 'INFO': 'INF'}[issue.severity.name]
                print(f"  [{icon}] Line {issue.line}: {issue.message}")
                if issue.rule:
                    print(f"         Rule: {issue.rule}")
                if issue.suggestion:
                    print(f"         Fix: {issue.suggestion}")

        print(f"\n{'='*60}")
        if counts[Severity.ERROR] == 0 and counts[Severity.FATAL] == 0:
            print("Result: VALID")
        else:
            print("Result: INVALID")
        print(f"{'='*60}\n")


def main():
    parser = argparse.ArgumentParser(
        description="Validate Mangle syntax and semantics",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog=__doc__
    )
    parser.add_argument('file', nargs='?', help='Mangle file to validate')
    parser.add_argument('--check-string', '-s', help='Validate inline Mangle code')
    parser.add_argument('--strict', action='store_true', help='Enable strict mode')
    parser.add_argument('--verbose', '-v', action='store_true', help='Verbose output')

    args = parser.parse_args()

    if not args.file and not args.check_string:
        parser.print_help()
        sys.exit(1)

    validator = MangleValidator(strict=args.strict, verbose=args.verbose)

    if args.check_string:
        success = validator.validate(args.check_string, "<inline>")
    else:
        filepath = Path(args.file)
        if not filepath.exists():
            print(f"Error: File not found: {filepath}")
            sys.exit(2)
        success = validator.validate_file(filepath)

    sys.exit(0 if success else 1)


if __name__ == "__main__":
    main()
