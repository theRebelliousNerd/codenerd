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
    # Type inference: track which arguments are compared against integers
    int_compared_args: "Set[int]" = field(default_factory=set)
    # Type inference: track which arguments are compared against floats
    float_compared_args: "Set[int]" = field(default_factory=set)
    # Track argument types from declarations
    declared_arg_types: "List[str]" = field(default_factory=list)


@dataclass
class TypeComparison:
    """Tracks a variable comparison for type inference."""
    variable: str
    compared_to: str  # The literal or expression being compared
    comparison_op: str  # >, <, >=, <=, =, !=
    is_integer: bool  # True if compared to integer literal
    is_float: bool  # True if compared to float literal
    line: int
    predicate_source: str = ""  # Which predicate this variable came from
    arg_index: int = -1  # Which argument position in the predicate


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
        'list:member', 'string:contains',
    }

    # Patterns for type inference
    TYPE_PATTERNS = {
        # Variable compared to integer literal: Var > 80, Var >= 100, etc.
        'var_cmp_int': re.compile(
            r'\b([A-Z][a-zA-Z0-9_]*)\s*([><=!]+)\s*(-?\d+)\b(?!\.)'
        ),
        # Integer literal compared to variable: 80 < Var
        'int_cmp_var': re.compile(
            r'\b(-?\d+)\b(?!\.)\s*([><=!]+)\s*([A-Z][a-zA-Z0-9_]*)\b'
        ),
        # Variable compared to float literal: Var > 0.8, Var >= 0.95
        'var_cmp_float': re.compile(
            r'\b([A-Z][a-zA-Z0-9_]*)\s*([><=!]+)\s*(-?\d+\.\d+)\b'
        ),
        # Float literal compared to variable: 0.8 < Var
        'float_cmp_var': re.compile(
            r'\b(-?\d+\.\d+)\s*([><=!]+)\s*([A-Z][a-zA-Z0-9_]*)\b'
        ),
        # Predicate with variable at position: pred(X, Y, Z) -> extract positions
        'pred_args': re.compile(
            r'\b([a-z][a-z0-9_]*)\s*\(([^)]+)\)'
        ),
    }

    # Known predicates where Go code might produce floats but rules compare integers
    # These should trigger warnings when integer comparisons are detected
    FLOAT_RISK_PREDICATES = {
        # predicate_name: list of argument indices (0-based) that might be floats
        'learned_exemplar': [4],      # confidence: Go might use 0.0-1.0
        'score': [0],                 # scores often 0.0-1.0 in Go
        'confidence': [0],            # confidence values
        'similarity': [0, 1],         # similarity scores
        'quality_score': [0],         # quality metrics
    }

    # Predicates known to use integers (timestamps, counts, etc.)
    INTEGER_PREDICATES = {
        'current_time': [0],          # Unix timestamp
        'system_heartbeat': [1],      # timestamp
        'temporary_override': [1],    # expiration timestamp
        'rejection_count': [1],       # count
        'review_accuracy': [1, 2, 3, 4],  # all counts
        'iteration': [1],             # iteration number
        'max_iterations': [0],        # limit
        'max_retries': [0],           # limit
        'retry_attempt': [1],         # attempt number
    }

    def __init__(self, strict: bool = False, verbose: bool = False):
        self.strict = strict
        self.verbose = verbose
        self.issues: List[Issue] = []
        self.predicates: Dict[str, Predicate] = {}
        self.declared_types: Set[str] = set()
        self.type_comparisons: List[TypeComparison] = []
        self.var_to_predicate: Dict[str, List[Tuple[str, int]]] = {}  # var -> [(pred, arg_idx)]

    def validate_file(self, filepath: Path) -> bool:
        """Validate a Mangle file."""
        with open(filepath, encoding='utf-8') as f:
            content = f.read()
        return self.validate(content, str(filepath))

    def validate(self, content: str, source: str = "<string>") -> bool:
        """Validate Mangle source code."""
        self.issues = []
        self.predicates = {}
        self.type_comparisons = []
        self.var_to_predicate = {}

        # Preprocessing
        lines = content.split('\n')

        # Pass 1: Extract declarations and build predicate index
        self._pass1_declarations(content, lines)

        # Pass 2: Validate syntax line by line
        self._pass2_syntax(content, lines)

        # Pass 3: Semantic analysis
        self._pass3_semantics(content, lines)

        # Pass 4: Type consistency analysis (float/int mismatches)
        self._pass4_type_consistency(content, lines)

        # Report results
        self._report(source, len(lines))

        return not any(i.severity.value >= Severity.ERROR.value for i in self.issues)

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

    def _pass4_type_consistency(self, _content: str, lines: List[str]) -> None:
        """
        Type consistency analysis pass.

        Detects potential float/int type mismatches by:
        1. Tracking which variables are compared against integer literals
        2. Mapping variables back to their source predicates
        3. Warning when "float-risk" predicates have integer comparisons

        This catches bugs like:
            learned_exemplar(Pattern, Verb, Target, Constraint, Conf),
            Conf > 80.  # BUG: Go code might store 0.8 (float), not 80 (int)
        """
        # Process rules, handling multi-line statements
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
                    # Complete multi-line rule
                    if ':-' in multiline_buffer or '<-' in multiline_buffer:
                        self._analyze_rule_types(multiline_buffer, multiline_start)
                    in_multiline = False
                    multiline_buffer = ""
                continue

            # Skip declarations
            if stripped.startswith('Decl ') or stripped.startswith('Package '):
                continue

            # Check for start of multi-line statement
            if not stripped.endswith('.'):
                if ':-' in stripped or '<-' in stripped:
                    in_multiline = True
                    multiline_start = i
                    multiline_buffer = stripped
                continue

            # Single-line rule
            if ':-' in stripped or '<-' in stripped:
                self._analyze_rule_types(stripped, i)

        # Check for type mismatches
        self._check_type_mismatches()

    def _analyze_rule_types(self, rule: str, line: int) -> None:
        """Analyze a rule for variable-to-predicate mappings and comparisons."""
        # Split into head and body
        arrow_match = self.PATTERNS['rule_arrow'].search(rule)
        if not arrow_match:
            return

        body = rule[arrow_match.end():].strip()
        if body.endswith('.'):
            body = body[:-1]

        # Build variable-to-predicate mapping from body predicates
        var_mapping: Dict[str, List[Tuple[str, int]]] = {}

        for pred_match in self.TYPE_PATTERNS['pred_args'].finditer(body):
            pred_name = pred_match.group(1)
            args_str = pred_match.group(2)

            # Parse arguments
            args = self._parse_predicate_args(args_str)
            for idx, arg in enumerate(args):
                arg = arg.strip()
                # Check if argument is a variable
                if self.PATTERNS['variable'].fullmatch(arg):
                    if arg not in var_mapping:
                        var_mapping[arg] = []
                    var_mapping[arg].append((pred_name, idx))

        # Update global mapping
        for var, preds in var_mapping.items():
            if var not in self.var_to_predicate:
                self.var_to_predicate[var] = []
            self.var_to_predicate[var].extend(preds)

        # Find integer comparisons: Var > 80, Var >= 100, etc.
        for match in self.TYPE_PATTERNS['var_cmp_int'].finditer(body):
            var_name = match.group(1)
            op = match.group(2)
            int_val = match.group(3)

            # Skip if this looks like a float (shouldn't match due to negative lookahead)
            if '.' in int_val:
                continue

            comp = TypeComparison(
                variable=var_name,
                compared_to=int_val,
                comparison_op=op,
                is_integer=True,
                is_float=False,
                line=line
            )

            # Link to source predicate if known
            if var_name in var_mapping:
                for pred_name, arg_idx in var_mapping[var_name]:
                    comp.predicate_source = pred_name
                    comp.arg_index = arg_idx
                    self.type_comparisons.append(comp)

                    # Track in predicate's int_compared_args
                    if pred_name in self.predicates:
                        self.predicates[pred_name].int_compared_args.add(arg_idx)
            else:
                self.type_comparisons.append(comp)

        # Also check int < Var pattern
        for match in self.TYPE_PATTERNS['int_cmp_var'].finditer(body):
            int_val = match.group(1)
            op = match.group(2)
            var_name = match.group(3)

            if '.' in int_val:
                continue

            comp = TypeComparison(
                variable=var_name,
                compared_to=int_val,
                comparison_op=op,
                is_integer=True,
                is_float=False,
                line=line
            )

            if var_name in var_mapping:
                for pred_name, arg_idx in var_mapping[var_name]:
                    comp.predicate_source = pred_name
                    comp.arg_index = arg_idx
                    self.type_comparisons.append(comp)

                    if pred_name in self.predicates:
                        self.predicates[pred_name].int_compared_args.add(arg_idx)
            else:
                self.type_comparisons.append(comp)

        # Find float comparisons for tracking
        for match in self.TYPE_PATTERNS['var_cmp_float'].finditer(body):
            var_name = match.group(1)
            op = match.group(2)
            float_val = match.group(3)

            comp = TypeComparison(
                variable=var_name,
                compared_to=float_val,
                comparison_op=op,
                is_integer=False,
                is_float=True,
                line=line
            )

            if var_name in var_mapping:
                for pred_name, arg_idx in var_mapping[var_name]:
                    comp.predicate_source = pred_name
                    comp.arg_index = arg_idx
                    self.type_comparisons.append(comp)

                    if pred_name in self.predicates:
                        self.predicates[pred_name].float_compared_args.add(arg_idx)
            else:
                self.type_comparisons.append(comp)

    def _parse_predicate_args(self, args_str: str) -> List[str]:
        """Parse predicate arguments, handling nested structures."""
        args = []
        current = ""
        depth = 0

        for char in args_str:
            if char in '([{':
                depth += 1
                current += char
            elif char in ')]}':
                depth -= 1
                current += char
            elif char == ',' and depth == 0:
                args.append(current.strip())
                current = ""
            else:
                current += char

        if current.strip():
            args.append(current.strip())

        return args

    def _check_type_mismatches(self) -> None:
        """Check for potential type mismatches between Go code and Mangle rules."""
        # Check float-risk predicates that have integer comparisons
        for pred_name, risk_args in self.FLOAT_RISK_PREDICATES.items():
            if pred_name not in self.predicates:
                continue

            pred = self.predicates[pred_name]

            for arg_idx in risk_args:
                if arg_idx in pred.int_compared_args:
                    # Find the comparison for line number
                    comp_line = 0
                    comp_val = ""
                    for comp in self.type_comparisons:
                        if (comp.predicate_source == pred_name and
                            comp.arg_index == arg_idx and
                            comp.is_integer):
                            comp_line = comp.line
                            comp_val = comp.compared_to
                            break

                    self.issues.append(Issue(
                        severity=Severity.WARNING,
                        line=comp_line,
                        column=0,
                        message=(
                            f"Integer comparison on '{pred_name}' arg {arg_idx} "
                            f"(compared to {comp_val}) - Go code may produce floats"
                        ),
                        rule="type.float_risk_int_comparison",
                        suggestion=(
                            f"Verify Go code uses int64 not float64 for this value, "
                            f"or use float comparison (e.g., > 0.8 instead of > 80)"
                        )
                    ))

        # Check for mixed int/float comparisons on same predicate argument
        for pred_name, pred in self.predicates.items():
            mixed_args = pred.int_compared_args & pred.float_compared_args
            for arg_idx in mixed_args:
                self.issues.append(Issue(
                    severity=Severity.WARNING,
                    line=pred.defined_in[0] if pred.defined_in else 0,
                    column=0,
                    message=(
                        f"Mixed int/float comparisons on '{pred_name}' arg {arg_idx} "
                        f"- potential type inconsistency"
                    ),
                    rule="type.mixed_numeric_comparisons",
                    suggestion="Use consistent numeric types for the same predicate argument"
                ))

        # Info: Report predicates with integer comparisons for Go code review
        if self.verbose:
            for comp in self.type_comparisons:
                if comp.is_integer and comp.predicate_source:
                    self.issues.append(Issue(
                        severity=Severity.INFO,
                        line=comp.line,
                        column=0,
                        message=(
                            f"Integer comparison: {comp.variable} {comp.comparison_op} "
                            f"{comp.compared_to} (from {comp.predicate_source}[{comp.arg_index}])"
                        ),
                        rule="info.int_comparison"
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
