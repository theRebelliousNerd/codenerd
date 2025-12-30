#!/usr/bin/env python3
"""
Mangle Predicate Lineage/Provenance Tool v1.0

Explains how a fact is (or could be) derived in a Mangle program by building
a proof tree showing which rules and source facts contribute to its derivation.

Answers questions like:
  - "How would X be derived?"
  - "Why is X true?"
  - "What facts are needed for X to hold?"

USAGE
=====
    python3 explain_derivation.py <mangle_file> --explain "predicate(/arg1, /arg2)"
    python3 explain_derivation.py <file1> <file2> --explain "predicate(X, /arg2)"
    python3 explain_derivation.py --check-string "<code>" --explain "pred(/x)"

EXAMPLES
========
    # Explain how delegate_task could be derived
    python3 explain_derivation.py policy.mg schemas.mg --explain "delegate_task(/coder, \"fix bug\", /pending)"

    # Explain with partial grounding (variables)
    python3 explain_derivation.py policy.mg --explain "next_action(X)"

    # Show all derivation paths
    python3 explain_derivation.py policy.mg --explain "next_action(/run_tests)" --all-paths

    # Export to JSON for tooling
    python3 explain_derivation.py policy.mg --explain "permitted(/fs_read)" --json

OPTIONS
=======
    --explain PREDICATE   The fact to explain (required)
    --depth N            Max recursion depth (default: 10)
    --all-paths          Show all derivation paths, not just first
    --json               Output as JSON
    --verbose            Show detailed analysis
    --check-string CODE  Analyze inline Mangle code instead of files

PREDICATE FORMAT
================
Ground (fully specified):
    delegate_task(/coder, "fix bug", /pending)
    user_intent("id1", /mutation, /fix, "app.go", /none)

Partially ground (with variables):
    delegate_task(/coder, Task, /pending)
    next_action(X)
    user_intent(_, Category, /fix, _, _)

Compatible with Mangle v0.4.0 (November 2024)
"""

import sys
import re
import argparse
import json
from pathlib import Path
from dataclasses import dataclass, field
from typing import List, Dict, Set, Optional, Tuple, Any
from collections import defaultdict
from enum import Enum


class FactType(Enum):
    """Type of fact in the derivation."""
    EDB = "edb"           # Extensional (base fact, must exist in fact store)
    IDB = "idb"           # Intensional (derived via rules)
    VIRTUAL = "virtual"   # Virtual predicate (computed by FFI)
    BUILTIN = "builtin"   # Built-in predicate


@dataclass
class ParsedPredicate:
    """A parsed predicate with arguments."""
    name: str
    args: List[str]  # Arguments (may include variables, constants, strings)
    arity: int

    def __str__(self):
        args_str = ", ".join(self.args)
        return f"{self.name}({args_str})"

    def matches(self, other: 'ParsedPredicate', bindings: Dict[str, str] = None) -> Tuple[bool, Dict[str, str]]:
        """
        Check if this predicate matches another, considering variable bindings.
        Returns (matches, updated_bindings).
        """
        if self.name != other.name or self.arity != other.arity:
            return False, {}

        if bindings is None:
            bindings = {}

        new_bindings = bindings.copy()

        for arg1, arg2 in zip(self.args, other.args):
            # Check if either is a variable (uppercase or _)
            is_var1 = self._is_variable(arg1)
            is_var2 = self._is_variable(arg2)

            if is_var1 and is_var2:
                # Both variables - they can unify
                if arg1 in new_bindings:
                    if new_bindings[arg1] != arg2:
                        # Check if arg2 is also bound
                        if arg2 in new_bindings and new_bindings[arg2] != new_bindings[arg1]:
                            return False, {}
                    else:
                        continue
                elif arg2 in new_bindings:
                    new_bindings[arg1] = new_bindings[arg2]
                else:
                    # Neither bound - bind them together
                    new_bindings[arg1] = arg2
            elif is_var1:
                # arg1 is variable, arg2 is constant
                if arg1 in new_bindings:
                    if new_bindings[arg1] != arg2:
                        return False, {}
                else:
                    new_bindings[arg1] = arg2
            elif is_var2:
                # arg2 is variable, arg1 is constant
                if arg2 in new_bindings:
                    if new_bindings[arg2] != arg1:
                        return False, {}
                else:
                    new_bindings[arg2] = arg1
            else:
                # Both constants - must match exactly
                if arg1 != arg2:
                    return False, {}

        return True, new_bindings

    @staticmethod
    def _is_variable(arg: str) -> bool:
        """Check if an argument is a variable (uppercase or underscore)."""
        if arg == "_":
            return True
        if arg and arg[0].isupper():
            return True
        return False


@dataclass
class Rule:
    """A Mangle rule: head :- body."""
    head: ParsedPredicate
    body: List[ParsedPredicate]  # List of predicates in body
    negated: Set[int]  # Indices of negated predicates in body
    line: int
    source_text: str

    def __str__(self):
        body_parts = []
        for i, pred in enumerate(self.body):
            prefix = "!" if i in self.negated else ""
            body_parts.append(f"{prefix}{pred}")
        body_str = ", ".join(body_parts)
        return f"{self.head} :- {body_str}."


@dataclass
class DerivationNode:
    """A node in the proof tree."""
    predicate: ParsedPredicate
    fact_type: FactType
    rule: Optional[Rule] = None
    children: List['DerivationNode'] = field(default_factory=list)
    bindings: Dict[str, str] = field(default_factory=dict)

    def to_dict(self) -> dict:
        """Convert to JSON-serializable dict."""
        return {
            'predicate': str(self.predicate),
            'type': self.fact_type.value,
            'rule_line': self.rule.line if self.rule else None,
            'rule_text': self.rule.source_text if self.rule else None,
            'bindings': self.bindings,
            'children': [c.to_dict() for c in self.children]
        }


class DerivationExplainer:
    """
    Explains how facts are derived in Mangle programs.

    Builds a proof tree showing:
    - Which rules could derive the target fact
    - What facts each rule needs
    - Recursive derivation of dependent facts
    - Missing facts preventing derivation
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
    }

    # Built-in predicates
    BUILTINS = {
        'match_cons', 'match_nil', 'match_field', 'match_entry',
        'list:member', 'list_length', 'time_diff',
    }

    def __init__(self, max_depth: int = 10, all_paths: bool = False, verbose: bool = False):
        self.max_depth = max_depth
        self.all_paths = all_paths
        self.verbose = verbose

        # Parsed program
        self.rules: Dict[str, List[Rule]] = defaultdict(list)  # predicate_name -> rules
        self.edb_predicates: Set[str] = set()
        self.idb_predicates: Set[str] = set()
        self.virtual_predicates: Set[str] = set()

        # Derivation results
        self.derivation_paths: List[DerivationNode] = []
        self.required_edb: List[ParsedPredicate] = []

    def load_files(self, filepaths: List[Path]):
        """Load and parse multiple Mangle files."""
        for filepath in filepaths:
            with open(filepath, encoding='utf-8') as f:
                content = f.read()
            self._parse_program(content)

    def load_string(self, content: str):
        """Load and parse Mangle code from string."""
        self._parse_program(content)

    def _parse_program(self, content: str):
        """Parse Mangle program to extract rules and declarations."""
        statements = self._split_into_statements(content)

        for stmt, line in statements:
            stmt = stmt.strip()
            if not stmt:
                continue

            # Handle declarations (EDB)
            if stmt.startswith('Decl '):
                decl_match = self.PATTERNS['decl'].match(stmt)
                if decl_match:
                    pred_name = decl_match.group(1)
                    self.edb_predicates.add(pred_name)

                    # Check if it's a virtual predicate (common naming: query_*, recall_*)
                    if any(stmt.lower().find(marker) >= 0 for marker in
                           ['virtual', 'ffi', 'query_', 'recall_']):
                        self.virtual_predicates.add(pred_name)
                continue

            # Skip package statements
            if stmt.startswith('Package ') or stmt.startswith('Uses '):
                continue

            # Parse rules
            arrow_match = self.PATTERNS['rule_arrow'].search(stmt)
            if arrow_match:
                head_text = stmt[:arrow_match.start()].strip()
                body_text = stmt[arrow_match.end():].strip()

                # Remove trailing period
                if body_text.endswith('.'):
                    body_text = body_text[:-1].strip()

                rule = self._parse_rule(head_text, body_text, line, stmt)
                if rule:
                    self.rules[rule.head.name].append(rule)
                    self.idb_predicates.add(rule.head.name)

    def _split_into_statements(self, content: str) -> List[Tuple[str, int]]:
        """
        Split content into statements, handling multi-line rules and multiple statements per line.
        Returns list of (statement_text, start_line).
        """
        statements = []

        # First, split by lines to track line numbers
        lines = content.split('\n')

        # Process all content character by character
        current_stmt = []
        start_line = 1
        current_line = 1
        in_string = False
        string_char = None
        paren_depth = 0

        for line_num, line in enumerate(lines, 1):
            # Remove comments
            line = self.PATTERNS['comment'].sub('', line)

            i = 0
            while i < len(line):
                char = line[i]

                # Track string boundaries
                if not in_string:
                    if char in ('"', "'", '`'):
                        in_string = True
                        string_char = char
                        current_stmt.append(char)
                    elif char == '(':
                        paren_depth += 1
                        current_stmt.append(char)
                    elif char == ')':
                        paren_depth -= 1
                        current_stmt.append(char)
                    elif char == '.' and paren_depth == 0:
                        # End of statement
                        current_stmt.append(char)
                        stmt_text = ''.join(current_stmt).strip()
                        if stmt_text and stmt_text != '.':
                            statements.append((stmt_text, start_line))
                        current_stmt = []
                        start_line = line_num
                    else:
                        if char.strip() or current_stmt:  # Skip leading whitespace
                            if not current_stmt and char.strip():
                                start_line = line_num
                            current_stmt.append(char)
                else:
                    # Inside string
                    current_stmt.append(char)
                    if char == string_char and (i == 0 or line[i-1] != '\\'):
                        in_string = False
                        string_char = None

                i += 1

            # Add space between lines if we're in the middle of a statement
            if current_stmt and not in_string:
                current_stmt.append(' ')

        # Handle any incomplete statement at end
        if current_stmt:
            stmt_text = ''.join(current_stmt).strip()
            if stmt_text and stmt_text != '.':
                statements.append((stmt_text, start_line))

        return statements

    def _parse_rule(self, head_text: str, body_text: str, line: int, full_text: str) -> Optional[Rule]:
        """Parse a single rule."""
        head = self._parse_predicate(head_text)
        if not head:
            return None

        # Parse body predicates
        body_preds = []
        negated = set()

        # Split body by comma (simple approach - may need refinement for nested structures)
        body_parts = self._split_body(body_text)

        for part in body_parts:
            part = part.strip()
            if not part:
                continue

            # Check for negation
            is_negated = False
            if part.startswith('!'):
                is_negated = True
                part = part[1:].strip()
            elif part.startswith('not '):
                is_negated = True
                part = part[4:].strip()

            pred = self._parse_predicate(part)
            if pred and pred.name not in self.BUILTINS:
                if is_negated:
                    negated.add(len(body_preds))
                body_preds.append(pred)

        return Rule(
            head=head,
            body=body_preds,
            negated=negated,
            line=line,
            source_text=full_text[:200] + ("..." if len(full_text) > 200 else "")
        )

    def _split_body(self, body_text: str) -> List[str]:
        """Split rule body by commas, respecting parentheses and strings."""
        parts = []
        current = []
        paren_depth = 0
        in_string = False
        string_char = None

        for char in body_text:
            if not in_string:
                if char in ('"', "'", '`'):
                    in_string = True
                    string_char = char
                elif char == '(':
                    paren_depth += 1
                elif char == ')':
                    paren_depth -= 1
                elif char == ',' and paren_depth == 0:
                    parts.append(''.join(current))
                    current = []
                    continue
            else:
                if char == string_char and (len(current) == 0 or current[-1] != '\\'):
                    in_string = False
                    string_char = None

            current.append(char)

        if current:
            parts.append(''.join(current))

        return parts

    def _parse_predicate(self, text: str) -> Optional[ParsedPredicate]:
        """Parse a predicate from text like 'foo(X, /bar, "baz")'."""
        text = text.strip()

        # Match predicate name and opening paren
        match = re.match(r'([a-z][a-z0-9_]*)\s*\(', text)
        if not match:
            return None

        name = match.group(1)
        args_start = match.end()

        # Find matching closing paren
        args_text = text[args_start:]
        paren_depth = 1
        end_pos = 0
        in_string = False
        string_char = None

        for i, char in enumerate(args_text):
            if not in_string:
                if char in ('"', "'", '`'):
                    in_string = True
                    string_char = char
                elif char == '(':
                    paren_depth += 1
                elif char == ')':
                    paren_depth -= 1
                    if paren_depth == 0:
                        end_pos = i
                        break
            else:
                if char == string_char and (i == 0 or args_text[i-1] != '\\'):
                    in_string = False
                    string_char = None

        if paren_depth != 0:
            # Malformed - try to recover
            args_text = args_text.rstrip(')')
        else:
            args_text = args_text[:end_pos]

        # Parse arguments
        args = self._parse_args(args_text)

        return ParsedPredicate(name=name, args=args, arity=len(args))

    def _parse_args(self, args_text: str) -> List[str]:
        """Parse comma-separated arguments, respecting strings and nested structures."""
        if not args_text.strip():
            return []

        args = []
        current = []
        paren_depth = 0
        bracket_depth = 0
        in_string = False
        string_char = None

        for char in args_text:
            if not in_string:
                if char in ('"', "'", '`'):
                    in_string = True
                    string_char = char
                elif char == '(':
                    paren_depth += 1
                elif char == ')':
                    paren_depth -= 1
                elif char == '[':
                    bracket_depth += 1
                elif char == ']':
                    bracket_depth -= 1
                elif char == ',' and paren_depth == 0 and bracket_depth == 0:
                    args.append(''.join(current).strip())
                    current = []
                    continue
            else:
                if char == string_char and (len(current) == 0 or current[-1] != '\\'):
                    in_string = False
                    string_char = None

            current.append(char)

        if current:
            args.append(''.join(current).strip())

        return args

    def explain(self, target_text: str) -> bool:
        """
        Explain how the target predicate could be derived.
        Returns True if derivable.
        """
        target = self._parse_predicate(target_text)
        if not target:
            print(f"Error: Could not parse target predicate: {target_text}", file=sys.stderr)
            return False

        print(f"EXPLAINING: {target}\n")

        # Build derivation tree(s)
        self.derivation_paths = []
        self.required_edb = []
        visited = set()

        paths = self._build_derivation_tree(target, depth=0, visited=visited)

        if paths:
            self.derivation_paths = paths
            return True
        else:
            print(f"No derivation paths found for {target}")
            return False

    def _build_derivation_tree(self, target: ParsedPredicate, depth: int,
                               visited: Set[str], bindings: Dict[str, str] = None) -> List[DerivationNode]:
        """
        Recursively build derivation tree(s) for a target predicate.
        Returns list of possible derivation paths.
        """
        if bindings is None:
            bindings = {}

        # Check depth limit
        if depth > self.max_depth:
            return []

        # Avoid infinite loops (check predicate signature)
        pred_sig = str(target)
        if pred_sig in visited:
            return []

        visited.add(pred_sig)

        # Determine fact type
        fact_type = self._get_fact_type(target.name)

        # Base case: EDB fact
        if fact_type == FactType.EDB:
            self.required_edb.append(target)
            return [DerivationNode(
                predicate=target,
                fact_type=FactType.EDB,
                bindings=bindings.copy()
            )]

        # Base case: Virtual predicate
        if fact_type == FactType.VIRTUAL:
            return [DerivationNode(
                predicate=target,
                fact_type=FactType.VIRTUAL,
                bindings=bindings.copy()
            )]

        # Base case: Builtin
        if fact_type == FactType.BUILTIN:
            return [DerivationNode(
                predicate=target,
                fact_type=FactType.BUILTIN,
                bindings=bindings.copy()
            )]

        # IDB: Find rules that could derive this
        derivation_nodes = []
        rules_for_pred = self.rules.get(target.name, [])

        for rule in rules_for_pred:
            # Try to unify target with rule head
            matches, new_bindings = target.matches(rule.head, bindings)

            if not matches:
                continue

            # Create node for this rule
            node = DerivationNode(
                predicate=target,
                fact_type=FactType.IDB,
                rule=rule,
                bindings=new_bindings.copy()
            )

            # Recursively derive body predicates
            all_body_derivable = True

            for i, body_pred in enumerate(rule.body):
                # Apply current bindings to body predicate
                instantiated = self._apply_bindings(body_pred, new_bindings)

                # Recursively derive
                child_paths = self._build_derivation_tree(
                    instantiated,
                    depth + 1,
                    visited.copy(),
                    new_bindings.copy()
                )

                if not child_paths:
                    all_body_derivable = False
                    # Still continue to show what's needed
                    node.children.append(DerivationNode(
                        predicate=instantiated,
                        fact_type=self._get_fact_type(instantiated.name),
                        bindings=new_bindings.copy()
                    ))
                else:
                    # Add first path (or all if all_paths is True)
                    if self.all_paths:
                        node.children.extend(child_paths)
                    else:
                        node.children.append(child_paths[0])

            derivation_nodes.append(node)

            if not self.all_paths and derivation_nodes:
                break  # Found one path, that's enough

        visited.remove(pred_sig)
        return derivation_nodes

    def _apply_bindings(self, pred: ParsedPredicate, bindings: Dict[str, str]) -> ParsedPredicate:
        """Apply variable bindings to a predicate."""
        new_args = []
        for arg in pred.args:
            if ParsedPredicate._is_variable(arg) and arg in bindings:
                new_args.append(bindings[arg])
            else:
                new_args.append(arg)

        return ParsedPredicate(name=pred.name, args=new_args, arity=pred.arity)

    def _get_fact_type(self, pred_name: str) -> FactType:
        """Determine the type of a predicate."""
        if pred_name in self.BUILTINS:
            return FactType.BUILTIN
        if pred_name in self.virtual_predicates:
            return FactType.VIRTUAL
        if pred_name in self.edb_predicates and pred_name not in self.idb_predicates:
            return FactType.EDB
        if pred_name in self.idb_predicates:
            return FactType.IDB
        # Default to EDB if unknown
        return FactType.EDB

    def print_proof_tree(self):
        """Print the proof tree in human-readable format."""
        if not self.derivation_paths:
            print("No derivation paths found.")
            return

        print("PROOF TREE:")
        print("-" * 70)

        for i, path in enumerate(self.derivation_paths, 1):
            if len(self.derivation_paths) > 1:
                print(f"\n=== PATH {i} ===")
            self._print_node(path, indent=0)

        # Print required EDB facts
        if self.required_edb:
            print("\n" + "=" * 70)
            print("REQUIRED EDB FACTS:")
            print("-" * 70)

            # Group by predicate name
            by_name = defaultdict(list)
            for fact in self.required_edb:
                by_name[fact.name].append(fact)

            for pred_name in sorted(by_name.keys()):
                facts = by_name[pred_name]
                print(f"\n{pred_name}:")
                for fact in facts:
                    print(f"  - {fact}")

        # Status
        print("\n" + "=" * 70)
        if self.required_edb:
            print("STATUS: Derivable if required EDB facts exist in fact store")
        else:
            print("STATUS: Derivable (no EDB facts required)")
        print("=" * 70)

    def _print_node(self, node: DerivationNode, indent: int):
        """Recursively print a derivation node."""
        prefix = "  " * indent

        # Print the predicate
        print(f"{prefix}{node.predicate}")

        # Print rule info if IDB
        if node.fact_type == FactType.IDB and node.rule:
            print(f"{prefix}+-- RULE (line {node.rule.line}):")
            rule_lines = node.rule.source_text.split('\n')
            for line in rule_lines[:3]:  # Show first 3 lines
                print(f"{prefix}|   {line}")
            if len(rule_lines) > 3:
                print(f"{prefix}|   ...")

            if node.children:
                print(f"{prefix}|   NEEDS:")
                for child in node.children:
                    self._print_node(child, indent + 2)
        elif node.fact_type == FactType.EDB:
            print(f"{prefix}+-- EDB FACT (must exist in fact store)")
        elif node.fact_type == FactType.VIRTUAL:
            print(f"{prefix}+-- VIRTUAL PREDICATE (computed by FFI/VirtualStore)")
        elif node.fact_type == FactType.BUILTIN:
            print(f"{prefix}+-- BUILTIN PREDICATE")

        print()

    def get_json_result(self) -> dict:
        """Return explanation results as JSON."""
        return {
            'derivable': len(self.derivation_paths) > 0,
            'paths': [path.to_dict() for path in self.derivation_paths],
            'required_edb': [str(fact) for fact in self.required_edb],
            'num_paths': len(self.derivation_paths)
        }


def main():
    parser = argparse.ArgumentParser(
        description="Explain how predicates are derived in Mangle programs",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog=__doc__
    )

    parser.add_argument('files', nargs='*', help='Mangle files to analyze')
    parser.add_argument('--explain', '-e', required=True,
                       help='Predicate to explain (e.g., "next_action(/run_tests)")')
    parser.add_argument('--check-string', '-s',
                       help='Analyze inline Mangle code instead of files')
    parser.add_argument('--depth', '-d', type=int, default=10,
                       help='Maximum recursion depth (default: 10)')
    parser.add_argument('--all-paths', '-a', action='store_true',
                       help='Show all derivation paths, not just first')
    parser.add_argument('--json', '-j', action='store_true',
                       help='Output as JSON')
    parser.add_argument('--verbose', '-v', action='store_true',
                       help='Show detailed analysis')

    args = parser.parse_args()

    # Validate input
    if not args.files and not args.check_string:
        parser.print_help()
        print("\nError: Must provide either files or --check-string", file=sys.stderr)
        sys.exit(1)

    # Create explainer
    explainer = DerivationExplainer(
        max_depth=args.depth,
        all_paths=args.all_paths,
        verbose=args.verbose
    )

    # Load program
    if args.check_string:
        explainer.load_string(args.check_string)
    else:
        filepaths = [Path(f) for f in args.files]
        for fp in filepaths:
            if not fp.exists():
                print(f"Error: File not found: {fp}", file=sys.stderr)
                sys.exit(2)
        explainer.load_files(filepaths)

    # Explain target
    derivable = explainer.explain(args.explain)

    # Output results
    if args.json:
        print(json.dumps(explainer.get_json_result(), indent=2))
    else:
        explainer.print_proof_tree()

    sys.exit(0 if derivable else 1)


if __name__ == "__main__":
    main()
