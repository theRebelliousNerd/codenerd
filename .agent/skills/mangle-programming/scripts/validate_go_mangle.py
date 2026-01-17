#!/usr/bin/env python3
"""
Go Mangle Integration Validator

Validates Go source code that integrates with the Google Mangle library.
Checks for correct API usage patterns, imports, type handling, and common pitfalls.

Usage:
    python3 validate_go_mangle.py <go_file_or_directory>
    python3 validate_go_mangle.py --codebase <project_root>

Checks:
    - Correct github.com/google/mangle/* imports
    - Proper AST type handling (ast.Atom, ast.Constant, etc.)
    - Engine API usage patterns
    - Error handling for Mangle operations
    - Fact/Rule construction best practices
    - Type conversion correctness

Compatible with:
    - Mangle v0.4.0+
    - codeNERD architecture patterns

Exit codes:
    0 - All checks passed
    1 - Issues found
    2 - Fatal errors
"""

import sys
import re
import os
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
    file: str
    line: int
    message: str
    rule: str = ""
    suggestion: str = ""


class GoMangleValidator:
    """Validates Go code that integrates with Mangle."""

    # Required/recommended imports
    MANGLE_IMPORTS = {
        'github.com/google/mangle/analysis': 'Program analysis',
        'github.com/google/mangle/ast': 'AST types (Atom, Constant, etc.)',
        'github.com/google/mangle/builtin': 'Built-in predicates',
        'github.com/google/mangle/engine': 'Evaluation engine',
        'github.com/google/mangle/factstore': 'Fact storage',
        'github.com/google/mangle/functional': 'Function evaluation',
        'github.com/google/mangle/parse': 'Parsing utilities',
        'github.com/google/mangle/packages': 'Package support',
        'github.com/google/mangle/rewrite': 'Rule rewriting',
        'github.com/google/mangle/symbols': 'Built-in symbols',
        'github.com/google/mangle/unionfind': 'Unification',
    }

    # Common API patterns to check
    PATTERNS = {
        # Good patterns
        'parse_unit': re.compile(r'parse\.Unit\s*\('),
        'analyze_one_unit': re.compile(r'analysis\.AnalyzeOneUnit\s*\('),
        'eval_program': re.compile(r'engine\.EvalProgram\w*\s*\('),
        'factstore_add': re.compile(r'\.Add\s*\(\s*\w+\s*\)'),
        'get_facts': re.compile(r'\.GetFacts\s*\('),
        'new_atom': re.compile(r'ast\.NewAtom\s*\('),
        'ast_string': re.compile(r'ast\.String\s*\('),
        'ast_number': re.compile(r'ast\.Number\s*\('),
        'ast_name': re.compile(r'ast\.Name\s*\('),

        # Potential issues
        'raw_string_in_atom': re.compile(r'ast\.NewAtom\s*\([^)]*"[^"]*"[^)]*\)'),
        'unchecked_parse': re.compile(r'parse\.\w+\s*\([^)]*\)\s*$'),
        'missing_error_check': re.compile(r',\s*_\s*:?=\s*parse\.|,\s*_\s*:?=\s*analysis\.'),

        # Import detection
        'mangle_import': re.compile(r'"github\.com/google/mangle/(\w+)"'),
        'import_alias': re.compile(r'(\w+)\s+"github\.com/google/mangle/(\w+)"'),

        # Type conversions
        'base_term_switch': re.compile(r'switch\s+\w+\s*:=\s*\w+\.\(type\)'),
        'constant_type_check': re.compile(r'\.Type\s*==\s*ast\.\w+Type'),
    }

    # AST types that need proper handling
    AST_TYPES = {
        'ast.Atom': 'Represents a predicate application',
        'ast.Clause': 'Represents a fact or rule',
        'ast.Constant': 'Base constant type (Name, String, Number)',
        'ast.Variable': 'Logic variable',
        'ast.BaseTerm': 'Interface for all terms',
        'ast.PredicateSym': 'Predicate symbol with arity',
        'ast.Decl': 'Predicate declaration',
    }

    # Constant type checks
    CONSTANT_TYPES = {
        'ast.NameType': '/name constants',
        'ast.StringType': '"string" constants',
        'ast.NumberType': 'integer constants',
        'ast.Float64Type': 'float64 constants',
        'ast.BytesType': 'byte string constants',
    }

    # Patterns for detecting float/int type mismatches in fact construction
    FACT_PATTERNS = {
        # Detect float literals in Args slices (e.g., Args: []interface{}{..., 0.8, ...})
        'float_in_args': re.compile(
            r'Args\s*:\s*\[\]interface\{\}\s*\{[^}]*\b(\d+\.\d+)\b[^}]*\}',
            re.MULTILINE
        ),
        # Detect float64() cast in Args (e.g., float64(time.Now().Unix()))
        'float64_cast_in_args': re.compile(
            r'Args\s*:\s*\[\]interface\{\}\s*\{[^}]*float64\s*\([^)]+\)[^}]*\}',
            re.MULTILINE
        ),
        # Detect Fact struct with Args containing floats
        'fact_struct_float': re.compile(
            r'\{Predicate:\s*"(\w+)"[^}]*Args:\s*\[\]interface\{\}\s*\{([^}]+)\}\}',
            re.MULTILINE
        ),
        # Detect mangle.Fact with float args
        'mangle_fact_float': re.compile(
            r'mangle\.Fact\s*\{[^}]*Args:\s*\[\]interface\{\}\s*\{([^}]+)\}',
            re.MULTILINE
        ),
    }

    # Known predicates that expect integer values (common patterns)
    # Format: predicate_name -> list of argument indices (0-based) that should be integers
    INTEGER_PREDICATES = {
        'learned_exemplar': [4],      # confidence should be 0-100, not 0.0-1.0
        'current_time': [0],          # Unix timestamp should be int64
        'system_heartbeat': [1],      # timestamp
        'temporary_override': [1],    # expiration timestamp
        'rejection_count': [1],       # count
        'review_accuracy': [1, 2, 3, 4],  # all counts
    }

    # Known predicates that intentionally use floats
    FLOAT_PREDICATES = {
        'state': [1],                 # stability (0.0-1.0)
        'base_stability': [1],        # stability score
        'effective_stability': [1],   # stability score
        'cumulative_penalty': [1],    # penalty (0.0-0.3)
        'context_compressing': [1],   # ratio (0.0-1.0)
    }

    def __init__(self, verbose: bool = False):
        self.verbose = verbose
        self.issues: List[Issue] = []
        self.files_checked: int = 0
        self.mangle_files: List[str] = []

    def validate_directory(self, path: Path) -> bool:
        """Validate all Go files in a directory."""
        for go_file in path.rglob('*.go'):
            # Skip vendor and test files optionally
            if 'vendor' in str(go_file):
                continue
            self.validate_file(go_file)

        return self._summarize()

    def validate_file(self, filepath: Path) -> bool:
        """Validate a single Go file."""
        self.files_checked += 1

        try:
            with open(filepath, encoding='utf-8') as f:
                content = f.read()
        except Exception as e:
            self.issues.append(Issue(
                severity=Severity.FATAL,
                file=str(filepath),
                line=0,
                message=f"Could not read file: {e}",
                rule="io.read_error"
            ))
            return False

        # Check if this file uses Mangle
        if not self._uses_mangle(content):
            return True

        self.mangle_files.append(str(filepath))
        lines = content.split('\n')

        # Run all checks
        self._check_imports(content, filepath, lines)
        self._check_parsing_patterns(content, filepath, lines)
        self._check_engine_usage(content, filepath, lines)
        self._check_type_handling(content, filepath, lines)
        self._check_error_handling(content, filepath, lines)
        self._check_fact_construction(content, filepath, lines)
        self._check_float_int_mismatches(content, filepath, lines)
        self._check_codeNERD_patterns(content, filepath, lines)

        return True

    def _uses_mangle(self, content: str) -> bool:
        """Check if file imports any Mangle packages or uses Mangle types."""
        # Direct Google Mangle import
        if 'github.com/google/mangle' in content:
            return True
        # codeNERD internal mangle wrapper
        if 'codeNERD/internal/mangle' in content:
            return True
        # Uses mangle types (internal wrapper or direct)
        if 'mangle.Fact' in content or 'mangle.Engine' in content:
            return True
        # Uses Fact struct with Args (codeNERD pattern)
        if 'Predicate:' in content and 'Args:' in content and '[]interface{}' in content:
            return True
        return False

    def _check_imports(self, content: str, filepath: Path, lines: List[str]):
        """Check Mangle import patterns."""
        imports_found = set()

        for match in self.PATTERNS['mangle_import'].finditer(content):
            imports_found.add(match.group(1))

        # Check for alias imports (e.g., mengine "github.com/google/mangle/engine")
        for match in self.PATTERNS['import_alias'].finditer(content):
            alias = match.group(1)
            pkg = match.group(2)
            imports_found.add(pkg)

            if self.verbose:
                self.issues.append(Issue(
                    severity=Severity.INFO,
                    file=str(filepath),
                    line=0,
                    message=f"Import alias '{alias}' for mangle/{pkg}",
                    rule="info.import_alias"
                ))

        # Check for missing common imports
        if 'ast' not in imports_found and self._uses_ast_types(content):
            self.issues.append(Issue(
                severity=Severity.WARNING,
                file=str(filepath),
                line=0,
                message="Uses AST types but missing 'github.com/google/mangle/ast' import",
                rule="import.missing_ast"
            ))

        if 'parse' not in imports_found and 'parse.' in content:
            self.issues.append(Issue(
                severity=Severity.WARNING,
                file=str(filepath),
                line=0,
                message="Uses parse functions but missing 'github.com/google/mangle/parse' import",
                rule="import.missing_parse"
            ))

    def _uses_ast_types(self, content: str) -> bool:
        """Check if content uses AST types."""
        return any(t in content for t in ['ast.Atom', 'ast.Constant', 'ast.Variable', 'ast.Clause'])

    def _check_parsing_patterns(self, content: str, filepath: Path, lines: List[str]):
        """Check parsing API usage."""
        for i, line in enumerate(lines, 1):
            # Check for parse.Unit usage
            if 'parse.Unit' in line:
                # Verify it's passed a reader
                if 'strings.NewReader' not in line and 'bytes.NewReader' not in line:
                    # Check surrounding lines
                    context = '\n'.join(lines[max(0, i-3):min(len(lines), i+2)])
                    if 'Reader' not in context:
                        self.issues.append(Issue(
                            severity=Severity.WARNING,
                            file=str(filepath),
                            line=i,
                            message="parse.Unit requires an io.Reader argument",
                            rule="api.parse_unit_reader",
                            suggestion="Use strings.NewReader(content) or bytes.NewReader(data)"
                        ))

            # Check for parse.Atom usage without error handling
            if 'parse.Atom' in line and ', _' not in line and 'err' not in line:
                self.issues.append(Issue(
                    severity=Severity.WARNING,
                    file=str(filepath),
                    line=i,
                    message="parse.Atom can return error - ensure it's handled",
                    rule="api.parse_atom_error"
                ))

    def _check_engine_usage(self, content: str, filepath: Path, lines: List[str]):
        """Check engine API usage patterns."""
        for i, line in enumerate(lines, 1):
            # Check for EvalProgram variants
            if 'engine.Eval' in line:
                # Verify program info is passed
                if 'programInfo' not in line and 'ProgramInfo' not in line and 'info' not in line.lower():
                    context = '\n'.join(lines[max(0, i-5):min(len(lines), i+1)])
                    if 'analysis.Analyze' not in context:
                        self.issues.append(Issue(
                            severity=Severity.WARNING,
                            file=str(filepath),
                            line=i,
                            message="EvalProgram requires ProgramInfo from analysis.AnalyzeOneUnit",
                            rule="api.eval_requires_analysis"
                        ))

            # Check for deprecated or incorrect API usage
            if 'engine.Query' in line:
                self.issues.append(Issue(
                    severity=Severity.INFO,
                    file=str(filepath),
                    line=i,
                    message="Consider using QueryContext for better control over query evaluation",
                    rule="api.query_context"
                ))

    def _check_type_handling(self, content: str, filepath: Path, lines: List[str]):
        """Check AST type handling patterns."""
        has_base_term_handling = False

        for i, line in enumerate(lines, 1):
            # Check for proper type switch on BaseTerm
            if 'ast.BaseTerm' in line or 'BaseTerm' in line:
                context = '\n'.join(lines[max(0, i-1):min(len(lines), i+10)])
                if 'switch' in context and '.(type)' in context:
                    has_base_term_handling = True

            # Check for Constant type access without type check
            if '.Symbol' in line and 'Constant' in line:
                # Check if there's a type check nearby
                context = '\n'.join(lines[max(0, i-5):i])
                if '.Type ==' not in context and 'case ast.' not in context:
                    self.issues.append(Issue(
                        severity=Severity.WARNING,
                        file=str(filepath),
                        line=i,
                        message="Accessing Constant.Symbol without checking Type field",
                        rule="type.constant_type_check",
                        suggestion="Check constant.Type before accessing Symbol/NumValue"
                    ))

            # Check for NumValue access
            if '.NumValue' in line:
                context = '\n'.join(lines[max(0, i-5):i])
                if 'NumberType' not in context and 'Float64Type' not in context:
                    self.issues.append(Issue(
                        severity=Severity.WARNING,
                        file=str(filepath),
                        line=i,
                        message="Accessing NumValue without verifying NumberType or Float64Type",
                        rule="type.numvalue_check"
                    ))

    def _check_error_handling(self, content: str, filepath: Path, lines: List[str]):
        """Check error handling patterns for Mangle operations."""
        for i, line in enumerate(lines, 1):
            # Check for ignored errors
            if ', _' in line:
                if 'parse.' in line or 'analysis.' in line:
                    self.issues.append(Issue(
                        severity=Severity.ERROR,
                        file=str(filepath),
                        line=i,
                        message="Mangle parse/analysis errors should not be ignored",
                        rule="error.ignored_parse_error"
                    ))

                if 'ast.Name(' in line:
                    self.issues.append(Issue(
                        severity=Severity.WARNING,
                        file=str(filepath),
                        line=i,
                        message="ast.Name can return error for invalid names",
                        rule="error.ignored_name_error"
                    ))

    def _check_fact_construction(self, content: str, filepath: Path, lines: List[str]):
        """Check fact/atom construction patterns."""
        for i, line in enumerate(lines, 1):
            # Check ast.NewAtom usage
            if 'ast.NewAtom' in line:
                # Verify predicate name is first arg
                match = re.search(r'ast\.NewAtom\s*\(\s*([^,)]+)', line)
                if match:
                    first_arg = match.group(1).strip()
                    # Should be a string or variable containing predicate name
                    if first_arg.startswith('/'):
                        self.issues.append(Issue(
                            severity=Severity.ERROR,
                            file=str(filepath),
                            line=i,
                            message="ast.NewAtom first argument should be predicate name string, not a name constant",
                            rule="api.newatom_predicate",
                            suggestion='Use ast.NewAtom("predicate_name", args...) not ast.NewAtom("/name", ...)'
                        ))

            # Check for proper term construction
            if 'ast.String(' in line:
                # Verify it's not being used for name constants
                match = re.search(r'ast\.String\s*\(\s*([^)]+)\s*\)', line)
                if match:
                    arg = match.group(1)
                    if '"/' in arg or "'/":
                        self.issues.append(Issue(
                            severity=Severity.WARNING,
                            file=str(filepath),
                            line=i,
                            message="Name constants (starting with /) should use ast.Name, not ast.String",
                            rule="api.name_vs_string",
                            suggestion='Use ast.Name("/constant") instead of ast.String("/constant")'
                        ))

    def _check_float_int_mismatches(self, content: str, filepath: Path, lines: List[str]):
        """
        Check for float/int type mismatches in Mangle fact construction.

        This catches bugs like:
        - Using float64(time.Now().Unix()) instead of int64 for timestamps
        - Using 0.8 instead of 80 for confidence values compared against integers
        - Float literals in Args that should be integers based on Mangle schema comparisons

        The issue: Mangle compares types strictly. If a rule has `Conf > 80` (integer comparison)
        but the Go code stores `0.8` (float), the comparison fails with "value X is not a number".
        """
        for i, line in enumerate(lines, 1):
            # Check for float64() cast in Args - often wrong for timestamps
            if 'float64(' in line and 'Args' in line:
                # Check if it's wrapping a timestamp function
                if 'time.Now()' in line or 'Unix()' in line:
                    self.issues.append(Issue(
                        severity=Severity.ERROR,
                        file=str(filepath),
                        line=i,
                        message="float64() cast on timestamp - Mangle timestamp comparisons expect int64",
                        rule="type.float_timestamp",
                        suggestion="Use int64(time.Now().Unix()) instead of float64(...)"
                    ))
                elif 'Args' in line and '[]interface{}' in line:
                    self.issues.append(Issue(
                        severity=Severity.WARNING,
                        file=str(filepath),
                        line=i,
                        message="float64() cast in Mangle Args - verify Mangle rules don't compare against integers",
                        rule="type.float_cast_in_args"
                    ))

            # Check for float literals in Args (e.g., 0.8, 0.25)
            if 'Args' in line and '[]interface{}' in line:
                # Find float literals
                float_pattern = re.compile(r'\b(\d+\.\d+)\b')
                floats_found = float_pattern.findall(line)

                for float_val in floats_found:
                    float_num = float(float_val)

                    # Check if this looks like a 0.0-1.0 value that should be 0-100
                    if 0.0 <= float_num <= 1.0 and float_num not in (0.0, 1.0):
                        # Try to extract predicate name to check if it's intentionally float
                        pred_match = re.search(r'Predicate:\s*"(\w+)"', line)
                        if not pred_match:
                            # Check previous lines for predicate
                            for j in range(max(0, i-5), i):
                                pred_match = re.search(r'Predicate:\s*"(\w+)"', lines[j-1] if j > 0 else '')
                                if pred_match:
                                    break

                        pred_name = pred_match.group(1) if pred_match else None

                        # Skip known float predicates
                        if pred_name and pred_name in self.FLOAT_PREDICATES:
                            continue

                        # Check known integer predicates
                        if pred_name and pred_name in self.INTEGER_PREDICATES:
                            self.issues.append(Issue(
                                severity=Severity.ERROR,
                                file=str(filepath),
                                line=i,
                                message=f"Float {float_val} in predicate '{pred_name}' - this predicate expects integers",
                                rule="type.float_should_be_int",
                                suggestion=f"Convert to integer: int64({float_val} * 100) for 0-100 scale, or int64({float_val}) for other cases"
                            ))
                        else:
                            # Generic warning for unknown predicates
                            self.issues.append(Issue(
                                severity=Severity.WARNING,
                                file=str(filepath),
                                line=i,
                                message=f"Float literal {float_val} in Mangle Args - verify Mangle rules use float comparisons",
                                rule="type.float_in_args",
                                suggestion=f"If Mangle rules compare against integers (e.g., '> 80'), use int64({float_val} * 100)"
                            ))

        # Multi-line analysis: Check Fact struct blocks
        self._check_fact_struct_blocks(content, filepath, lines)

    def _check_fact_struct_blocks(self, content: str, filepath: Path, lines: List[str]):
        """
        Analyze multi-line Fact struct definitions for type issues.

        Looks for patterns like:
            mangle.Fact{
                Predicate: "learned_exemplar",
                Args: []interface{}{pattern, verb, target, constraint, 0.8},  // 0.8 should be int
            }
        """
        # Find all mangle.Fact or Fact struct blocks
        fact_block_pattern = re.compile(
            r'(?:mangle\.)?Fact\s*\{\s*'
            r'Predicate:\s*"(\w+)"[^}]*'
            r'Args:\s*\[\]interface\{\}\s*\{([^}]+)\}',
            re.MULTILINE | re.DOTALL
        )

        for match in fact_block_pattern.finditer(content):
            pred_name = match.group(1)
            args_content = match.group(2)

            # Find line number
            line_num = content[:match.start()].count('\n') + 1

            # Skip known float predicates
            if pred_name in self.FLOAT_PREDICATES:
                continue

            # Check for float literals in args
            float_pattern = re.compile(r'\b(\d+\.\d+)\b')
            floats_found = float_pattern.findall(args_content)

            for float_val in floats_found:
                float_num = float(float_val)

                if pred_name in self.INTEGER_PREDICATES:
                    self.issues.append(Issue(
                        severity=Severity.ERROR,
                        file=str(filepath),
                        line=line_num,
                        message=f"Float {float_val} in '{pred_name}' fact - Mangle schema expects integer",
                        rule="type.fact_float_mismatch",
                        suggestion=f"Convert confidence/count values: int64({float_val} * 100) for percentages"
                    ))
                elif 0.0 < float_num < 1.0:
                    # Suspicious 0.0-1.0 range value
                    self.issues.append(Issue(
                        severity=Severity.WARNING,
                        file=str(filepath),
                        line=line_num,
                        message=f"Float {float_val} in '{pred_name}' fact - verify Mangle rules use float comparisons",
                        rule="type.suspicious_float_range",
                        suggestion="If compared against integers in .mg rules (e.g., '> 80'), multiply by 100"
                    ))

    def _check_codeNERD_patterns(self, content: str, filepath: Path, lines: List[str]):
        """Check codeNERD-specific Mangle integration patterns."""
        # Check for Fact struct with ToAtom
        if 'type Fact struct' in content:
            if 'ToAtom()' not in content and 'ToAtom(' not in content:
                self.issues.append(Issue(
                    severity=Severity.INFO,
                    file=str(filepath),
                    line=0,
                    message="Fact struct should implement ToAtom() for Mangle AST conversion",
                    rule="codenerd.fact_to_atom"
                ))

        # Check for proper predicate registration
        if 'predicateIndex' in content or 'PredicateIndex' in content:
            if 'PredicateSym' not in content:
                self.issues.append(Issue(
                    severity=Severity.WARNING,
                    file=str(filepath),
                    line=0,
                    message="Predicate index should use ast.PredicateSym for proper arity tracking",
                    rule="codenerd.predicate_sym"
                ))

        # Check for VirtualStore pattern
        if 'VirtualStore' in content or 'virtualStore' in content:
            # Verify it implements required interface
            for i, line in enumerate(lines, 1):
                if 'func' in line and 'VirtualStore' in line:
                    if 'Query' not in content or 'AddFact' not in content:
                        self.issues.append(Issue(
                            severity=Severity.INFO,
                            file=str(filepath),
                            line=i,
                            message="VirtualStore should implement Query and AddFact methods",
                            rule="codenerd.virtual_store_interface"
                        ))
                    break

    def _summarize(self) -> bool:
        """Print summary and return success status."""
        print(f"\n{'='*70}")
        print("Go Mangle Integration Validation Report")
        print(f"{'='*70}")
        print(f"Files checked: {self.files_checked}")
        print(f"Files using Mangle: {len(self.mangle_files)}")

        if self.mangle_files and self.verbose:
            print("\nMangle integration files:")
            for f in self.mangle_files:
                print(f"  - {f}")

        # Count by severity
        counts = {s: 0 for s in Severity}
        for issue in self.issues:
            counts[issue.severity] += 1

        print(f"\nIssues found: {len(self.issues)}")
        for severity in [Severity.FATAL, Severity.ERROR, Severity.WARNING, Severity.INFO]:
            if counts[severity] > 0:
                print(f"  {severity.name}: {counts[severity]}")

        # Print issues grouped by file
        if self.issues:
            print(f"\n{'-'*70}")
            issues_by_file: Dict[str, List[Issue]] = {}
            for issue in self.issues:
                if issue.file not in issues_by_file:
                    issues_by_file[issue.file] = []
                issues_by_file[issue.file].append(issue)

            for file, file_issues in sorted(issues_by_file.items()):
                print(f"\n{file}:")
                for issue in sorted(file_issues, key=lambda x: (x.line, x.severity.value)):
                    icon = {'FATAL': '!!!', 'ERROR': 'ERR', 'WARNING': 'WRN', 'INFO': 'INF'}[issue.severity.name]
                    if issue.line > 0:
                        print(f"  [{icon}] Line {issue.line}: {issue.message}")
                    else:
                        print(f"  [{icon}] {issue.message}")
                    if issue.suggestion:
                        print(f"         Suggestion: {issue.suggestion}")

        print(f"\n{'='*70}")
        has_errors = counts[Severity.ERROR] > 0 or counts[Severity.FATAL] > 0
        if not has_errors:
            print("Result: PASSED")
        else:
            print("Result: FAILED")
        print(f"{'='*70}\n")

        return not has_errors


def main():
    parser = argparse.ArgumentParser(
        description="Validate Go code that integrates with Google Mangle",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog=__doc__
    )
    parser.add_argument('path', nargs='?', help='Go file or directory to validate')
    parser.add_argument('--codebase', '-c', help='Validate entire codebase from project root')
    parser.add_argument('--verbose', '-v', action='store_true', help='Verbose output')

    args = parser.parse_args()

    if not args.path and not args.codebase:
        parser.print_help()
        sys.exit(1)

    validator = GoMangleValidator(verbose=args.verbose)

    target = Path(args.codebase or args.path)
    if not target.exists():
        print(f"Error: Path not found: {target}")
        sys.exit(2)

    if target.is_file():
        success = validator.validate_file(target)
    else:
        success = validator.validate_directory(target)

    # Also summarize if single file
    if target.is_file():
        success = validator._summarize()

    sys.exit(0 if success else 1)


if __name__ == "__main__":
    main()
