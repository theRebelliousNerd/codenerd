#!/usr/bin/env python3
"""
Mangle Query Tracer v1.0

Step-by-step query evaluation debugger for Google Mangle programs.
Simulates query evaluation, showing which rules fire and why.

USAGE
=====
    python trace_query.py <mangle_file> --query "predicate(X, Y)"
    python trace_query.py <mangle_file> --query "next_action(X)" --facts "user_intent(/id, /query, /read, /foo, _)."
    python trace_query.py --check-string "<code>" --query "pred(X)"

PURPOSE
=======
Debug "why doesn't my rule derive anything?" by showing:
- Which rules could potentially match
- Variable bindings attempted
- Which body predicates succeed/fail
- Final results or detailed explanation of failure

OPTIONS
=======
    --query, -q         Query to evaluate (e.g., "next_action(X)" or "?next_action(X)")
    --facts, -f         Seed facts to add (semicolon or period separated)
    --max-steps         Maximum evaluation steps (default: 100)
    --verbose, -v       Show more detailed trace information
    --check-string, -s  Analyze inline Mangle code instead of file

EXIT CODES
==========
    0 - Query succeeded with results
    1 - Query failed (no results)
    2 - Parse error or usage error

EXAMPLES
========
    # Basic query
    python trace_query.py policy.mg --query "next_action(X)"

    # Query with seed facts
    python trace_query.py policy.mg --query "next_action(X)" \\
        --facts "user_intent(/id1, /query, /read, /foo, _). test_state(/failing)."

    # Debug why a rule doesn't fire
    python trace_query.py policy.mg --query "block_commit(X)" -v

Compatible with Mangle v0.4.0 (November 2024)
"""

from __future__ import annotations

import sys
import re
import argparse
from pathlib import Path
from dataclasses import dataclass
from typing import List, Dict, Optional, Tuple, Any
from enum import Enum


class TermType(Enum):
    """Type of term in Mangle."""
    VARIABLE = "variable"      # UPPERCASE: X, Y, Var
    ATOM = "atom"              # /lowercase: /active, /user1
    STRING = "string"          # "quoted"
    NUMBER = "number"          # 42, 3.14
    WILDCARD = "wildcard"      # _
    LIST = "list"              # [X, Y, Z]


@dataclass
class Term:
    """A term in a Mangle predicate."""
    value: Any
    term_type: TermType

    def __repr__(self):
        return f"{self.value}"

    def is_ground(self) -> bool:
        """Check if this term is ground (no variables)."""
        if self.term_type == TermType.VARIABLE:
            return False
        if self.term_type == TermType.LIST:
            return all(t.is_ground() for t in self.value)
        return True

    def matches(self, other: 'Term', bindings: Dict[str, Term]) -> Tuple[bool, Dict[str, Term]]:
        """
        Try to match this term with another, extending bindings.
        Returns (success, new_bindings).
        """
        new_bindings = bindings.copy()

        # Wildcard matches anything
        if self.term_type == TermType.WILDCARD or other.term_type == TermType.WILDCARD:
            return True, new_bindings

        # Dereference variables first to avoid infinite recursion
        self_deref = self._deref(bindings)
        other_deref = other._deref(bindings)

        # Variable matching
        if self_deref.term_type == TermType.VARIABLE:
            var_name = self_deref.value
            if var_name not in new_bindings:
                # Don't create self-referential bindings
                if other_deref.term_type == TermType.VARIABLE and other_deref.value == var_name:
                    return True, new_bindings
                # Bind variable
                new_bindings[var_name] = other_deref
                return True, new_bindings
            # If already bound, fall through to ground term matching

        if other_deref.term_type == TermType.VARIABLE:
            var_name = other_deref.value
            if var_name not in new_bindings:
                # Don't create self-referential bindings
                if self_deref.term_type == TermType.VARIABLE and self_deref.value == var_name:
                    return True, new_bindings
                # Bind variable
                new_bindings[var_name] = self_deref
                return True, new_bindings
            # If already bound, fall through to ground term matching

        # Ground term matching
        if self_deref.term_type != other_deref.term_type:
            return False, bindings

        if self_deref.term_type == TermType.LIST:
            if len(self_deref.value) != len(other_deref.value):
                return False, bindings
            for s, o in zip(self_deref.value, other_deref.value):
                success, new_bindings = s.matches(o, new_bindings)
                if not success:
                    return False, bindings
            return True, new_bindings

        # Direct value comparison
        return self_deref.value == other_deref.value, new_bindings

    def _deref(self, bindings: Dict[str, Term], visited: set = None) -> 'Term':
        """Dereference a term, following variable bindings."""
        if visited is None:
            visited = set()

        if self.term_type == TermType.VARIABLE:
            if self.value in visited:
                # Circular reference detected, return as-is
                return self
            if self.value in bindings:
                visited.add(self.value)
                return bindings[self.value]._deref(bindings, visited)
        return self


@dataclass
class Predicate:
    """A predicate in Mangle (e.g., user_intent(X, /query, Y, _, Z))."""
    name: str
    args: List[Term]
    negated: bool = False

    def __repr__(self):
        neg = "!" if self.negated else ""
        args_str = ", ".join(str(a) for a in self.args)
        return f"{neg}{self.name}({args_str})"

    def arity(self) -> int:
        return len(self.args)

    def matches(self, other: 'Predicate', bindings: Dict[str, Term]) -> Tuple[bool, Dict[str, Term]]:
        """Try to match this predicate with another."""
        if self.name != other.name or len(self.args) != len(other.args):
            return False, bindings

        new_bindings = bindings.copy()
        for self_arg, other_arg in zip(self.args, other.args):
            success, new_bindings = self_arg.matches(other_arg, new_bindings)
            if not success:
                return False, bindings

        return True, new_bindings

    def substitute(self, bindings: Dict[str, Term]) -> 'Predicate':
        """Apply bindings to create a new predicate."""
        new_args = []
        for arg in self.args:
            if arg.term_type == TermType.VARIABLE and arg.value in bindings:
                new_args.append(bindings[arg.value])
            elif arg.term_type == TermType.LIST:
                new_list = []
                for item in arg.value:
                    if item.term_type == TermType.VARIABLE and item.value in bindings:
                        new_list.append(bindings[item.value])
                    else:
                        new_list.append(item)
                new_args.append(Term(new_list, TermType.LIST))
            else:
                new_args.append(arg)
        return Predicate(self.name, new_args, self.negated)

    def is_ground(self) -> bool:
        """Check if predicate is fully ground (no variables)."""
        return all(arg.is_ground() for arg in self.args)


@dataclass
class Comparison:
    """A comparison expression (e.g., X < 3, Y != Z)."""
    left: Term
    operator: str  # <, >, <=, >=, =, !=, ==
    right: Term

    def __repr__(self):
        return f"{self.left} {self.operator} {self.right}"

    def evaluate(self, bindings: Dict[str, Term]) -> bool:
        """Evaluate the comparison with given bindings."""
        # Substitute bindings
        left_val = self._get_value(self.left, bindings)
        right_val = self._get_value(self.right, bindings)

        if left_val is None or right_val is None:
            return False  # Can't evaluate with unbound variables

        try:
            if self.operator in ["<", "<=", ">", ">="]:
                # Numeric comparison
                left_num = self._to_number(left_val)
                right_num = self._to_number(right_val)
                if left_num is None or right_num is None:
                    return False

                if self.operator == "<":
                    return left_num < right_num
                elif self.operator == "<=":
                    return left_num <= right_num
                elif self.operator == ">":
                    return left_num > right_num
                elif self.operator == ">=":
                    return left_num >= right_num

            elif self.operator in ["=", "=="]:
                return left_val == right_val
            elif self.operator == "!=":
                return left_val != right_val

        except (ValueError, TypeError):
            return False

        return False

    def _get_value(self, term: Term, bindings: Dict[str, Term]) -> Any:
        """Get the actual value of a term."""
        if term.term_type == TermType.VARIABLE:
            if term.value in bindings:
                return self._get_value(bindings[term.value], bindings)
            return None  # Unbound variable
        return term.value

    def _to_number(self, val: Any) -> Optional[float]:
        """Convert value to number."""
        if isinstance(val, (int, float)):
            return float(val)
        try:
            return float(val)
        except (ValueError, TypeError):
            return None


@dataclass
class Rule:
    """A Mangle rule: head :- body."""
    head: Predicate
    body: List[Any]  # List of Predicates and Comparisons
    line: int
    source: str

    def __repr__(self):
        body_str = ", ".join(str(b) for b in self.body)
        return f"{self.head} :- {body_str}."


@dataclass
class Fact:
    """A ground fact."""
    predicate: Predicate
    line: int = -1

    def __repr__(self):
        return f"{self.predicate}."


class MangleParser:
    """Parser for Mangle syntax."""

    # Regex patterns
    PATTERNS = {
        'comment': re.compile(r'#.*$'),
        'string': re.compile(r'"(?:[^"\\]|\\.)*"|\'(?:[^\'\\]|\\.)*\'|`[^`]*`'),
        'rule_arrow': re.compile(r':-|<-|⟸'),
        'comparison': re.compile(r'([A-Z][A-Za-z0-9_]*|/[a-z][a-z0-9_]*|\d+(?:\.\d+)?)\s*([<>!=]=?|=)\s*([A-Z][A-Za-z0-9_]*|/[a-z][a-z0-9_]*|\d+(?:\.\d+)?)'),
    }

    def parse_program(self, content: str) -> Tuple[List[Rule], List[Fact]]:
        """Parse a Mangle program into rules and facts."""
        rules = []
        facts = []

        statements = self._split_into_statements(content)

        for stmt, line_num in statements:
            stmt = stmt.strip()
            if not stmt or stmt.startswith('Decl ') or stmt.startswith('Package ') or stmt.startswith('Uses '):
                continue

            # Check for rule
            arrow_match = self.PATTERNS['rule_arrow'].search(stmt)
            if arrow_match:
                head_str = stmt[:arrow_match.start()].strip()
                body_str = stmt[arrow_match.end():].strip()

                # Remove trailing period
                if body_str.endswith('.'):
                    body_str = body_str[:-1]

                head = self._parse_predicate(head_str)
                body = self._parse_body(body_str)

                if head:
                    rules.append(Rule(head, body, line_num, stmt))

            elif stmt.endswith('.') and '(' in stmt:
                # It's a fact
                fact_str = stmt[:-1].strip()
                pred = self._parse_predicate(fact_str)
                if pred and pred.is_ground():
                    facts.append(Fact(pred, line_num))

        return rules, facts

    def _split_into_statements(self, content: str) -> List[Tuple[str, int]]:
        """Split content into statements, handling multi-line rules and multiple statements per line."""
        statements = []

        # First pass: join lines that are part of multi-line statements
        lines = content.split('\n')
        accumulated = []
        start_line = 1

        for i, line in enumerate(lines, 1):
            # Remove comments
            line = self.PATTERNS['comment'].sub('', line)
            stripped = line.strip()

            if not stripped:
                continue

            if not accumulated:
                start_line = i

            accumulated.append(stripped)

            # Check if we have complete statements (ends with . or !)
            if stripped.endswith('.') or stripped.endswith('!'):
                full_line = ' '.join(accumulated)
                # Now split by periods to handle multiple statements on one line
                self._split_single_line(full_line, start_line, statements)
                accumulated = []

        # Handle incomplete statement at end
        if accumulated:
            full_line = ' '.join(accumulated)
            self._split_single_line(full_line, start_line, statements)

        return statements

    def _split_single_line(self, line: str, line_num: int, statements: List[Tuple[str, int]]):
        """Split a single line that may contain multiple statements."""
        # Handle multiple statements on one line separated by periods
        # We need to be careful not to split on periods inside strings or within predicates

        current = ""
        depth = 0  # Track parentheses depth
        in_string = False
        string_char = None

        for i, char in enumerate(line):
            current += char

            # Track string state
            if char in '"\'`' and (i == 0 or line[i-1] != '\\'):
                if not in_string:
                    in_string = True
                    string_char = char
                elif char == string_char:
                    in_string = False
                    string_char = None

            # Track parentheses (but not in strings)
            if not in_string:
                if char == '(':
                    depth += 1
                elif char == ')':
                    depth -= 1
                elif char == '.' and depth == 0:
                    # End of statement
                    stmt = current.strip()
                    if stmt and not stmt.isspace():
                        statements.append((stmt, line_num))
                    current = ""

        # Handle any remaining content
        if current.strip() and not current.strip().isspace():
            statements.append((current.strip(), line_num))

    def _parse_predicate(self, text: str) -> Optional[Predicate]:
        """Parse a predicate from text."""
        text = text.strip()

        # Check for negation
        negated = False
        if text.startswith('!'):
            negated = True
            text = text[1:].strip()
        elif text.startswith('not '):
            negated = True
            text = text[4:].strip()

        # Extract predicate name and args
        match = re.match(r'([a-z][a-z0-9_]*)\s*\((.*)\)\s*$', text)
        if not match:
            return None

        name = match.group(1)
        args_str = match.group(2)

        args = self._parse_args(args_str)

        return Predicate(name, args, negated)

    def _parse_args(self, args_str: str) -> List[Term]:
        """Parse comma-separated arguments."""
        args = []
        current = ""
        depth = 0  # Track parentheses/brackets depth

        for char in args_str:
            if char in '([':
                depth += 1
                current += char
            elif char in ')]':
                depth -= 1
                current += char
            elif char == ',' and depth == 0:
                arg = self._parse_term(current.strip())
                if arg:
                    args.append(arg)
                current = ""
            else:
                current += char

        # Don't forget the last argument
        if current.strip():
            arg = self._parse_term(current.strip())
            if arg:
                args.append(arg)

        return args

    def _parse_term(self, text: str) -> Optional[Term]:
        """Parse a single term."""
        text = text.strip()

        if not text:
            return None

        # Wildcard
        if text == '_':
            return Term('_', TermType.WILDCARD)

        # String
        if text.startswith('"') or text.startswith("'") or text.startswith('`'):
            return Term(text[1:-1], TermType.STRING)

        # Atom (starts with /)
        if text.startswith('/'):
            return Term(text, TermType.ATOM)

        # Number
        if re.match(r'^-?\d+(\.\d+)?$', text):
            if '.' in text:
                return Term(float(text), TermType.NUMBER)
            else:
                return Term(int(text), TermType.NUMBER)

        # List
        if text.startswith('[') and text.endswith(']'):
            inner = text[1:-1].strip()
            if not inner:
                return Term([], TermType.LIST)
            elements = self._parse_args(inner)
            return Term(elements, TermType.LIST)

        # Variable (uppercase or starts with uppercase)
        if text[0].isupper():
            return Term(text, TermType.VARIABLE)

        # Default: treat as atom without /
        return Term(text, TermType.ATOM)

    def _parse_body(self, body_str: str) -> List[Any]:
        """Parse rule body into predicates and comparisons."""
        elements = []

        # Split by comma (but respect parentheses)
        parts = []
        current = ""
        depth = 0

        for char in body_str:
            if char in '([':
                depth += 1
                current += char
            elif char in ')]':
                depth -= 1
                current += char
            elif char == ',' and depth == 0:
                parts.append(current.strip())
                current = ""
            else:
                current += char

        if current.strip():
            parts.append(current.strip())

        # Parse each part
        for part in parts:
            # Check if it's a comparison
            comp_match = self.PATTERNS['comparison'].search(part)
            if comp_match:
                left = self._parse_term(comp_match.group(1))
                operator = comp_match.group(2)
                right = self._parse_term(comp_match.group(3))
                if left and right:
                    elements.append(Comparison(left, operator, right))
                continue

            # Otherwise, try to parse as predicate
            pred = self._parse_predicate(part)
            if pred:
                elements.append(pred)

        return elements

    def parse_query(self, query: str) -> Optional[Predicate]:
        """Parse a query string."""
        query = query.strip()

        # Remove leading ? if present
        if query.startswith('?'):
            query = query[1:].strip()

        # Remove trailing . if present
        if query.endswith('.'):
            query = query[:-1].strip()

        return self._parse_predicate(query)


class QueryTracer:
    """Traces query evaluation in Mangle programs."""

    def __init__(self, verbose: bool = False, max_steps: int = 100):
        self.verbose = verbose
        self.max_steps = max_steps
        self.rules: List[Rule] = []
        self.facts: List[Fact] = []
        self.parser = MangleParser()
        self.step_count = 0
        self.trace_output: List[str] = []

    def load_program(self, content: str):
        """Load a Mangle program."""
        self.rules, self.facts = self.parser.parse_program(content)

    def add_seed_facts(self, facts_str: str):
        """Add seed facts from string."""
        # Split by period or semicolon
        fact_strs = re.split(r'[.;]', facts_str)

        for fact_str in fact_strs:
            fact_str = fact_str.strip()
            if not fact_str:
                continue

            pred = self.parser._parse_predicate(fact_str)
            if pred:
                self.facts.append(Fact(pred, line=-1))

    def trace_query(self, query: Predicate) -> List[Predicate]:
        """
        Trace query evaluation and return results.
        """
        self.step_count = 0
        self.trace_output = []

        self._log(f"QUERY: {query}")
        self._log("")

        # Find all matching facts first
        results = []
        fact_matches = self._match_facts(query)
        if fact_matches:
            self._log(f"FACT MATCHES: Found {len(fact_matches)} direct fact(s)")
            for i, (fact, bindings) in enumerate(fact_matches, 1):
                bound_query = query.substitute(bindings)
                self._log(f"  {i}. {bound_query}")
                results.append(bound_query)
            self._log("")

        # Try to derive from rules
        rule_results = self._derive_from_rules(query, {}, depth=0)
        results.extend(rule_results)

        # Deduplicate results
        unique_results = []
        seen = set()
        for result in results:
            key = str(result)
            if key not in seen:
                seen.add(key)
                unique_results.append(result)

        self._log("=" * 70)
        if unique_results:
            self._log(f"RESULTS: {len(unique_results)} result(s) found")
            for i, result in enumerate(unique_results, 1):
                self._log(f"  {i}. {result}")
        else:
            self._log("RESULTS: No results found")
            self._log("")
            self._log("EXPLANATION: The query did not match any facts and no rules")
            self._log("could derive it. Check:")
            self._log("  1. Are there facts or rules that define this predicate?")
            self._log("  2. Are the arguments compatible?")
            self._log("  3. Do rule bodies have all required facts/predicates?")

        return unique_results

    def _derive_from_rules(self, goal: Predicate, bindings: Dict[str, Term], depth: int) -> List[Predicate]:
        """Try to derive goal from rules."""
        if depth > 10:  # Prevent infinite recursion
            return []

        if self.step_count >= self.max_steps:
            self._log(f"MAX STEPS ({self.max_steps}) REACHED")
            return []

        results = []

        # Find rules that could derive this goal
        matching_rules = self._find_matching_rules(goal)

        if not matching_rules and depth == 0:
            self._log(f"NO RULES FOUND with head matching: {goal.name}/{goal.arity()}")
            self._log("")

        for rule in matching_rules:
            self.step_count += 1

            # Try to unify goal with rule head
            success, new_bindings = goal.matches(rule.head, bindings)

            if not success:
                continue

            indent = "  " * depth
            self._log(f"{indent}STEP {self.step_count}: Trying rule at line {rule.line}")
            self._log(f"{indent}  {rule.head} :- {', '.join(str(b) for b in rule.body)}.")
            self._log(f"{indent}")

            # Try to satisfy rule body
            body_results = self._satisfy_body(rule.body, new_bindings, depth + 1)

            if body_results:
                self._log(f"{indent}  Rule SUCCEEDED with {len(body_results)} solution(s)")
                for result_bindings in body_results:
                    derived = goal.substitute(result_bindings)
                    results.append(derived)
                    if self.verbose:
                        self._log(f"{indent}    Solution: {derived}")
            else:
                self._log(f"{indent}  Rule FAILED: Could not satisfy all body predicates")

            self._log(f"{indent}")

        return results

    def _satisfy_body(self, body: List[Any], bindings: Dict[str, Term], depth: int) -> List[Dict[str, Term]]:
        """Try to satisfy all body elements. Returns list of successful bindings."""
        if not body:
            return [bindings]

        # Process first element
        first = body[0]
        rest = body[1:]

        indent = "  " * depth

        # Handle comparison
        if isinstance(first, Comparison):
            self._log(f"{indent}Checking: {first}")
            if first.evaluate(bindings):
                self._log(f"{indent}  -> TRUE")
                return self._satisfy_body(rest, bindings, depth)
            else:
                self._log(f"{indent}  -> FALSE")
                return []

        # Handle predicate
        if isinstance(first, Predicate):
            if first.negated:
                self._log(f"{indent}Checking: !{first.name}(...)")
                # For negation, check if there are NO matches
                matches = self._match_facts(first)
                if not matches:
                    # Also check if it can be derived
                    derived = self._derive_from_rules(first, bindings, depth)
                    if not derived:
                        self._log(f"{indent}  -> SUCCESS (no matches for negated predicate)")
                        return self._satisfy_body(rest, bindings, depth)

                self._log(f"{indent}  -> FAILED (negated predicate has matches)")
                return []
            else:
                self._log(f"{indent}Checking: {first}")

                # First try facts
                fact_matches = self._match_facts(first, bindings)

                # Then try rules
                rule_results = self._derive_from_rules(first, bindings, depth)

                # Combine results
                all_results = []

                # Process fact matches
                for fact, new_bindings in fact_matches:
                    self._log(f"{indent}  -> MATCH (fact): {fact.predicate}")
                    if self.verbose:
                        self._log(f"{indent}     Bindings: {self._format_bindings(new_bindings)}")
                    # Continue with rest of body
                    rest_results = self._satisfy_body(rest, new_bindings, depth)
                    all_results.extend(rest_results)

                # Process rule results
                for derived in rule_results:
                    # Extract bindings from derived result
                    success, new_bindings = first.matches(derived, bindings)
                    if success:
                        self._log(f"{indent}  -> MATCH (derived): {derived}")
                        if self.verbose:
                            self._log(f"{indent}     Bindings: {self._format_bindings(new_bindings)}")
                        rest_results = self._satisfy_body(rest, new_bindings, depth)
                        all_results.extend(rest_results)

                if not all_results:
                    self._log(f"{indent}  -> NO MATCH")

                return all_results

        return []

    def _match_facts(self, goal: Predicate, bindings: Dict[str, Term] = None) -> List[Tuple[Fact, Dict[str, Term]]]:
        """Find facts that match the goal predicate."""
        if bindings is None:
            bindings = {}

        matches = []

        for fact in self.facts:
            if fact.predicate.name != goal.name:
                continue

            success, new_bindings = goal.matches(fact.predicate, bindings)
            if success:
                matches.append((fact, new_bindings))

        return matches

    def _find_matching_rules(self, goal: Predicate) -> List[Rule]:
        """Find rules whose head could match the goal."""
        matching = []

        for rule in self.rules:
            if rule.head.name == goal.name and len(rule.head.args) == len(goal.args):
                matching.append(rule)

        return matching

    def _format_bindings(self, bindings: Dict[str, Term]) -> str:
        """Format bindings for display."""
        if not bindings:
            return "{}"
        items = [f"{k}={v}" for k, v in bindings.items()]
        return "{" + ", ".join(items) + "}"

    def _log(self, message: str):
        """Add message to trace output."""
        self.trace_output.append(message)

    def get_trace(self) -> str:
        """Get the full trace output."""
        return "\n".join(self.trace_output)

    def is_ground(self, predicate: Predicate) -> bool:
        """Check if predicate is fully ground (no variables)."""
        return all(arg.is_ground() for arg in predicate.args)


def main():
    parser = argparse.ArgumentParser(
        description="Trace Mangle query evaluation step-by-step",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog=__doc__
    )

    parser.add_argument('file', nargs='?', help='Mangle file to analyze')
    parser.add_argument('--check-string', '-s', help='Analyze inline Mangle code')
    parser.add_argument('--query', '-q', required=True, help='Query to evaluate (e.g., "next_action(X)")')
    parser.add_argument('--facts', '-f', help='Seed facts to add (semicolon or period separated)')
    parser.add_argument('--max-steps', type=int, default=100, help='Maximum evaluation steps (default: 100)')
    parser.add_argument('--verbose', '-v', action='store_true', help='Show more detailed trace')

    args = parser.parse_args()

    if not args.file and not args.check_string:
        parser.print_help()
        print("\nError: Must provide either a file or --check-string", file=sys.stderr)
        sys.exit(2)

    # Load program
    tracer = QueryTracer(verbose=args.verbose, max_steps=args.max_steps)

    if args.check_string:
        tracer.load_program(args.check_string)
    else:
        filepath = Path(args.file)
        if not filepath.exists():
            print(f"Error: File not found: {filepath}", file=sys.stderr)
            sys.exit(2)

        with open(filepath, encoding='utf-8') as f:
            content = f.read()
        tracer.load_program(content)

    # Add seed facts if provided
    if args.facts:
        tracer.add_seed_facts(args.facts)

    # Parse and execute query
    query = tracer.parser.parse_query(args.query)
    if not query:
        print(f"Error: Could not parse query: {args.query}", file=sys.stderr)
        sys.exit(2)

    results = tracer.trace_query(query)

    # Output trace
    print(tracer.get_trace())

    # Exit with appropriate code
    sys.exit(0 if results else 1)


if __name__ == "__main__":
    main()
