#!/usr/bin/env python3
"""
Mangle-to-Go Virtual Predicate Stub Generator v1.0

Parses Mangle schema files and generates idiomatic Go stub implementations
for virtual predicates, accelerating VirtualStore development.

USAGE
=====
    python generate_stubs.py <schema_file.mg> [options]
    python generate_stubs.py schemas.mg --output internal/mangle/stubs.go
    python generate_stubs.py schemas.mg --package mangle
    python generate_stubs.py schemas.mg --predicates "user_intent,file_topology"
    python generate_stubs.py schemas.mg --list

OPTIONS
=======
    --output FILE           Output Go file path (default: stdout)
    --package NAME          Go package name (default: mangle)
    --predicates LIST       Comma-separated list of predicates to generate
    --list                  Just list declared predicates with arities
    --interface-only        Generate interface definitions only, no stubs
    --virtual-only          Only generate stubs for virtual predicates (Section 7B)

TYPE MAPPINGS
=============
    Type<n>, Type<name>    -> engine.Atom (name constant like /foo)
    Type<string>           -> engine.String
    Type<int>              -> engine.Int64
    Type<float>            -> engine.Float64
    Type<[T]>              -> engine.List
    Type<{/k: v}>          -> engine.Map
    Type<Any>              -> engine.Value

EXAMPLES
========
    # Generate all predicates
    python generate_stubs.py internal/core/defaults/schemas.mg --output stubs.go

    # List all predicates
    python generate_stubs.py internal/core/defaults/schemas.mg --list

    # Generate specific predicates
    python generate_stubs.py schemas.mg --predicates "user_intent,recall_similar"

    # Generate only virtual predicates
    python generate_stubs.py schemas.mg --virtual-only --output virtual_preds.go
"""

import sys
import re
import argparse
from pathlib import Path
from dataclasses import dataclass, field
from typing import List, Dict, Optional, Set
from collections import defaultdict


@dataclass
class Argument:
    """Represents a predicate argument."""
    name: str
    mangle_type: str  # e.g., "Type<string>", "Type<n>", "Type<int>"
    go_type: str      # e.g., "engine.String", "engine.Atom"
    description: str = ""


@dataclass
class PredicateDecl:
    """Represents a Mangle predicate declaration."""
    name: str
    arity: int
    args: List[Argument] = field(default_factory=list)
    line_number: int = 0
    comment: str = ""
    section: str = ""
    is_virtual: bool = False  # True if in Section 7B (Virtual Predicates)


class MangleSchemaParser:
    """Parses Mangle schema files to extract predicate declarations."""

    # Type mapping from Mangle to Go
    TYPE_MAPPING = {
        'string': 'engine.String',
        'int': 'engine.Int64',
        'float': 'engine.Float64',
        'n': 'engine.Atom',      # name constant
        'name': 'engine.Atom',   # name constant
        'Any': 'engine.Value',
    }

    # Regex patterns
    PATTERNS = {
        'decl': re.compile(r'^\s*Decl\s+([a-z][a-z0-9_]*)\s*\((.*?)\)\s*\.', re.MULTILINE),
        'comment': re.compile(r'#(.*)$'),
        'section': re.compile(r'^#\s*={5,}\s*$\n#\s*SECTION\s+(\d+[A-Z]*):\s*(.+?)\s*$', re.MULTILINE),
        'arg': re.compile(r'([A-Z][a-zA-Z0-9_]*)\s*\.\s*Type<([^>]+)>'),
    }

    def __init__(self):
        self.predicates: Dict[str, PredicateDecl] = {}
        self.sections: Dict[int, str] = {}  # line_number -> section_name
        self.virtual_section_start = None
        self.virtual_section_end = None

    def parse_file(self, filepath: Path) -> Dict[str, PredicateDecl]:
        """Parse a Mangle schema file and extract predicates."""
        with open(filepath, encoding='utf-8') as f:
            content = f.read()

        # First pass: identify sections
        self._extract_sections(content)

        # Second pass: extract declarations
        self._extract_declarations(content)

        return self.predicates

    def _extract_sections(self, content: str):
        """Extract section headers to annotate predicates."""
        lines = content.split('\n')
        current_section = ""

        for i, line in enumerate(lines, 1):
            # Match section headers
            if line.strip().startswith('# ===') and i + 1 < len(lines):
                next_line = lines[i]  # 0-indexed
                section_match = re.match(r'^#\s*SECTION\s+(\d+[A-Z]*):\s*(.+?)\s*(?:\(|$)', next_line)
                if section_match:
                    section_num = section_match.group(1)
                    section_name = section_match.group(2)
                    current_section = f"§{section_num} {section_name}"
                    self.sections[i] = current_section

                    # Track virtual predicate section
                    if "VIRTUAL PREDICATES" in section_name.upper() or section_num == "7B":
                        self.virtual_section_start = i
                    elif self.virtual_section_start and not self.virtual_section_end:
                        # Next section after virtual predicates
                        if section_num not in ["7B", "7C", "7D"]:
                            self.virtual_section_end = i

    def _extract_declarations(self, content: str):
        """Extract Decl statements from content."""
        lines = content.split('\n')

        for i, line in enumerate(lines, 1):
            # Skip comments and empty lines
            stripped = line.strip()
            if not stripped or stripped.startswith('#'):
                continue

            # Match Decl statements
            decl_match = self.PATTERNS['decl'].match(line)
            if decl_match:
                pred_name = decl_match.group(1)
                args_text = decl_match.group(2)

                # Parse arguments
                args = self._parse_arguments(args_text)

                # Extract comment from preceding lines
                comment = self._extract_comment(lines, i)

                # Find section
                section = self._find_section(i)

                # Check if virtual
                is_virtual = self._is_virtual_predicate(i)

                predicate = PredicateDecl(
                    name=pred_name,
                    arity=len(args),
                    args=args,
                    line_number=i,
                    comment=comment,
                    section=section,
                    is_virtual=is_virtual
                )

                self.predicates[pred_name] = predicate

    def _parse_arguments(self, args_text: str) -> List[Argument]:
        """Parse argument list from Decl statement."""
        args = []

        # Handle empty argument list
        if not args_text.strip():
            return args

        # Find all arguments with Type<...> pattern
        for match in self.PATTERNS['arg'].finditer(args_text):
            arg_name = match.group(1)
            type_param = match.group(2)

            # Map Mangle type to Go type
            go_type = self._map_type(type_param)

            arg = Argument(
                name=arg_name,
                mangle_type=f"Type<{type_param}>",
                go_type=go_type
            )
            args.append(arg)

        # Fallback: handle simple comma-separated args without Type annotations
        if not args:
            simple_args = [a.strip() for a in args_text.split(',') if a.strip()]
            for arg_name in simple_args:
                arg = Argument(
                    name=arg_name,
                    mangle_type="Type<Any>",
                    go_type="engine.Value"
                )
                args.append(arg)

        return args

    def _map_type(self, type_param: str) -> str:
        """Map Mangle type parameter to Go engine type."""
        # Handle list types: [T]
        if type_param.startswith('[') and type_param.endswith(']'):
            return 'engine.List'

        # Handle map types: {/k: v}
        if type_param.startswith('{'):
            return 'engine.Map'

        # Direct mapping
        return self.TYPE_MAPPING.get(type_param, 'engine.Value')

    def _extract_comment(self, lines: List[str], decl_line: int) -> str:
        """Extract comment from lines preceding the declaration."""
        comments = []

        # Look backwards for comment lines
        for i in range(decl_line - 2, max(0, decl_line - 10), -1):
            line = lines[i].strip()

            if not line:
                continue

            if line.startswith('#'):
                # Remove comment marker and leading/trailing whitespace
                comment_text = line.lstrip('#').strip()
                if comment_text and not comment_text.startswith('==='):
                    comments.insert(0, comment_text)
            else:
                # Stop at first non-comment, non-empty line
                break

        return ' '.join(comments)

    def _find_section(self, line_number: int) -> str:
        """Find the section name for a given line number."""
        current_section = ""

        for section_line in sorted(self.sections.keys()):
            if section_line <= line_number:
                current_section = self.sections[section_line]
            else:
                break

        return current_section

    def _is_virtual_predicate(self, line_number: int) -> bool:
        """Check if predicate is in virtual predicates section."""
        if self.virtual_section_start:
            end = self.virtual_section_end or float('inf')
            return self.virtual_section_start <= line_number < end
        return False


class GoStubGenerator:
    """Generates Go stub code for Mangle virtual predicates."""

    def __init__(self, package_name: str = "mangle", interface_only: bool = False):
        self.package_name = package_name
        self.interface_only = interface_only

    def generate(self, predicates: List[PredicateDecl], source_file: str) -> str:
        """Generate complete Go file with stubs."""
        lines = []

        # File header
        lines.append(f"// Code generated by generate_stubs.py from {source_file}")
        lines.append("// DO NOT EDIT.")
        lines.append("")
        lines.append(f"package {self.package_name}")
        lines.append("")
        lines.append('import "github.com/google/mangle/engine"')
        lines.append("")

        # Generate stubs for each predicate
        for pred in sorted(predicates, key=lambda p: p.name):
            stub = self._generate_stub(pred)
            lines.append(stub)
            lines.append("")

        # Generate registration function
        if not self.interface_only:
            lines.append(self._generate_registration(predicates))

        return '\n'.join(lines)

    def _generate_stub(self, pred: PredicateDecl) -> str:
        """Generate stub code for a single predicate."""
        lines = []

        # Struct type name (PascalCase)
        struct_name = self._to_pascal_case(pred.name) + "Predicate"

        # Documentation comment
        lines.append(f"// {struct_name} implements the {pred.name}/{pred.arity} virtual predicate.")

        if pred.comment:
            lines.append(f"// {pred.comment}")

        if pred.section:
            lines.append(f"// Section: {pred.section}")

        # Add declaration signature
        decl_sig = self._format_decl(pred)
        lines.append(f"// {decl_sig}")

        # Struct definition
        lines.append(f"type {struct_name} struct {{}}")
        lines.append("")

        # Name method
        lines.append(f"func (p *{struct_name}) Name() string {{ return \"{pred.name}\" }}")

        # Arity method
        lines.append(f"func (p *{struct_name}) Arity() int {{ return {pred.arity} }}")

        lines.append("")

        # Query method (stub implementation)
        if not self.interface_only:
            lines.append(self._generate_query_method(struct_name, pred))

        return '\n'.join(lines)

    def _generate_query_method(self, struct_name: str, pred: PredicateDecl) -> str:
        """Generate the Query method stub."""
        lines = []

        lines.append(f"func (p *{struct_name}) Query(query engine.Query, callback func(engine.Fact) error) error {{")

        # Add argument documentation
        if pred.args:
            lines.append("\t// Arguments:")
            for i, arg in enumerate(pred.args):
                lines.append(f"\t//   query.Args[{i}] - {arg.name}: {arg.go_type}")

        lines.append("\t// TODO: Implement query logic")
        lines.append("\t//")
        lines.append("\t// Check binding patterns:")

        # Generate binding check examples
        for i, arg in enumerate(pred.args):
            if arg.go_type == 'engine.Atom':
                lines.append(f"\t//   if {arg.name.lower()}, ok := query.Args[{i}].(engine.Atom); ok {{")
                lines.append(f"\t//       // {arg.name} is bound to specific atom constant")
                lines.append("\t//   }")
            elif arg.go_type == 'engine.String':
                lines.append(f"\t//   if {arg.name.lower()}, ok := query.Args[{i}].(engine.String); ok {{")
                lines.append(f"\t//       // {arg.name} is bound to specific string")
                lines.append("\t//   }")
            elif arg.go_type in ['engine.Int64', 'engine.Float64']:
                lines.append(f"\t//   if {arg.name.lower()}, ok := query.Args[{i}].({arg.go_type}); ok {{")
                lines.append(f"\t//       // {arg.name} is bound to specific number")
                lines.append("\t//   }")
            else:
                lines.append(f"\t//   // query.Args[{i}] - {arg.name} ({arg.go_type})")

        lines.append("\t//")
        lines.append("\t// Example implementation:")
        lines.append("\t// return callback(engine.Fact{")
        lines.append(f"\t//     Predicate: \"{pred.name}\",")
        lines.append("\t//     Args: []engine.Value{")

        for arg in pred.args:
            if arg.go_type == 'engine.Atom':
                lines.append(f"\t//         engine.Atom(\"/example_{arg.name.lower()}\"),")
            elif arg.go_type == 'engine.String':
                lines.append(f"\t//         engine.String(\"example_{arg.name.lower()}\"),")
            elif arg.go_type == 'engine.Int64':
                lines.append(f"\t//         engine.Int64(42),")
            elif arg.go_type == 'engine.Float64':
                lines.append(f"\t//         engine.Float64(3.14),")
            else:
                lines.append(f"\t//         nil, // {arg.name}")

        lines.append("\t//     },")
        lines.append("\t// })")

        lines.append("\t")
        lines.append("\treturn nil")
        lines.append("}")

        return '\n'.join(lines)

    def _generate_registration(self, predicates: List[PredicateDecl]) -> str:
        """Generate the RegisterPredicates function."""
        lines = []

        lines.append("// RegisterPredicates registers all virtual predicates with the store.")
        lines.append("func RegisterPredicates(store *engine.FactStore) {")

        for pred in sorted(predicates, key=lambda p: p.name):
            struct_name = self._to_pascal_case(pred.name) + "Predicate"
            lines.append(f"\tstore.RegisterVirtual(&{struct_name}{{}})")

        lines.append("}")

        return '\n'.join(lines)

    def _to_pascal_case(self, snake_case: str) -> str:
        """Convert snake_case to PascalCase."""
        parts = snake_case.split('_')
        return ''.join(word.capitalize() for word in parts)

    def _format_decl(self, pred: PredicateDecl) -> str:
        """Format the Decl statement for documentation."""
        if not pred.args:
            return f"Decl {pred.name}()."

        arg_strs = []
        for arg in pred.args:
            arg_strs.append(f"{arg.name}.{arg.mangle_type}")

        return f"Decl {pred.name}({', '.join(arg_strs)})."


def main():
    parser = argparse.ArgumentParser(
        description="Generate Go stubs for Mangle virtual predicates",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog=__doc__
    )

    parser.add_argument('schema_file', nargs='?', help='Mangle schema file to parse')
    parser.add_argument('--output', '-o', help='Output Go file (default: stdout)')
    parser.add_argument('--package', '-p', default='mangle', help='Go package name (default: mangle)')
    parser.add_argument('--predicates', help='Comma-separated list of predicates to generate')
    parser.add_argument('--list', '-l', action='store_true', help='List predicates and exit')
    parser.add_argument('--interface-only', action='store_true', help='Generate interface definitions only')
    parser.add_argument('--virtual-only', action='store_true', help='Only generate virtual predicates (Section 7B)')

    args = parser.parse_args()

    # Check schema file is provided
    if not args.schema_file:
        parser.print_help()
        sys.exit(1)

    # Check file exists
    schema_path = Path(args.schema_file)
    if not schema_path.exists():
        print(f"Error: Schema file not found: {schema_path}", file=sys.stderr)
        sys.exit(1)

    # Parse schema file
    parser_obj = MangleSchemaParser()
    predicates = parser_obj.parse_file(schema_path)

    if not predicates:
        print(f"Warning: No predicates found in {schema_path}", file=sys.stderr)
        sys.exit(0)

    # Filter by predicate names if specified
    if args.predicates:
        pred_names = set(p.strip() for p in args.predicates.split(','))
        predicates = {name: pred for name, pred in predicates.items() if name in pred_names}

        # Check for missing predicates
        missing = pred_names - set(predicates.keys())
        if missing:
            print(f"Warning: Predicates not found: {', '.join(sorted(missing))}", file=sys.stderr)

    # Filter virtual predicates only if requested
    if args.virtual_only:
        predicates = {name: pred for name, pred in predicates.items() if pred.is_virtual}
        if not predicates:
            print("Warning: No virtual predicates found in schema", file=sys.stderr)
            sys.exit(0)

    # List mode
    if args.list:
        # Use UTF-8 encoding for output to handle Unicode characters
        import io
        sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding='utf-8', errors='replace')

        print(f"Predicates found in {schema_path}:\n")
        for name, pred in sorted(predicates.items()):
            virtual_marker = " [VIRTUAL]" if pred.is_virtual else ""
            print(f"  {name}/{pred.arity}{virtual_marker}")
            if pred.comment:
                # Replace problematic Unicode characters for console display
                comment_safe = pred.comment.replace('\u2192', '->').replace('\u00a7', 'Section')
                print(f"    {comment_safe}")
            if pred.section:
                # Replace section symbol
                section_safe = pred.section.replace('\u00a7', 'Section')
                print(f"    Section: {section_safe}")
            print()
        print(f"Total: {len(predicates)} predicates")
        sys.exit(0)

    # Generate Go code
    generator = GoStubGenerator(package_name=args.package, interface_only=args.interface_only)
    code = generator.generate(list(predicates.values()), schema_path.name)

    # Output
    if args.output:
        output_path = Path(args.output)
        output_path.parent.mkdir(parents=True, exist_ok=True)
        with open(output_path, 'w', encoding='utf-8') as f:
            f.write(code)
        print(f"Generated {len(predicates)} predicate stubs -> {output_path}")
    else:
        print(code)


if __name__ == "__main__":
    main()
