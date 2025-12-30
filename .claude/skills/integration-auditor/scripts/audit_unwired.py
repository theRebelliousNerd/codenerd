#!/usr/bin/env python3
"""
codeNERD Unwired Code Auditor

Detects code that exists but isn't properly wired - the most insidious bugs.
This auditor follows the "Wire, Don't Remove" principle: unused code typically
represents planned functionality that needs to be connected, not deleted.

Detection Categories:
1. Unused Parameters - Function params that are never used in the body
2. Unused Struct Fields - Fields declared but never assigned or read
3. Unimplemented Interfaces - Interface methods not implemented by types
4. Orphan Channels - Channels created but never consumed
5. Dead Callbacks - Callbacks/hooks declared but never registered
6. Missing Injections - Dependency injection fields never set
7. Unrouted Handlers - Handler functions that aren't connected to any router
8. Unused Return Values - Functions that return values never captured
9. Declared But Unused Factories - Factory functions never called
10. Incomplete Builders - Builder patterns missing terminal operations

Usage:
    python audit_unwired.py [workspace_path] [--verbose] [--json] [--fix-suggestions]

Examples:
    python audit_unwired.py                      # Full audit
    python audit_unwired.py --verbose            # With detailed context
    python audit_unwired.py --fix-suggestions    # Include wiring suggestions
"""

import os
import re
import sys
import argparse
import json
from pathlib import Path
from dataclasses import dataclass, field
from typing import List, Dict, Set, Optional, Tuple, NamedTuple
from enum import Enum
from datetime import datetime
from collections import defaultdict


class Severity(Enum):
    ERROR = "ERROR"
    WARNING = "WARNING"
    INFO = "INFO"
    OK = "OK"


class WiringCategory(Enum):
    UNUSED_PARAM = "unused_param"
    UNUSED_FIELD = "unused_field"
    UNIMPLEMENTED_INTERFACE = "unimplemented_interface"
    ORPHAN_CHANNEL = "orphan_channel"
    DEAD_CALLBACK = "dead_callback"
    MISSING_INJECTION = "missing_injection"
    UNROUTED_HANDLER = "unrouted_handler"
    UNUSED_RETURN = "unused_return"
    UNUSED_FACTORY = "unused_factory"
    INCOMPLETE_BUILDER = "incomplete_builder"


@dataclass
class Finding:
    severity: Severity
    category: WiringCategory
    message: str
    file: Optional[str] = None
    line: Optional[int] = None
    code_snippet: Optional[str] = None
    suggestion: Optional[str] = None
    wire_to: Optional[str] = None  # Where this code should be wired


@dataclass
class UnwiredAuditResult:
    timestamp: str = ""
    workspace: str = ""
    findings: List[Finding] = field(default_factory=list)
    stats: Dict[str, int] = field(default_factory=dict)


class StructField(NamedTuple):
    name: str
    type: str
    file: str
    line: int
    struct_name: str


class FunctionParam(NamedTuple):
    name: str
    type: str
    file: str
    line: int
    func_name: str


class InterfaceMethod(NamedTuple):
    name: str
    signature: str
    interface_name: str
    file: str
    line: int


class UnwiredAuditor:
    """Audits Go code for unwired code that should be connected."""

    # Common injection field patterns (fields that should be set via Set* methods)
    INJECTION_FIELDS = [
        'kernel', 'store', 'logger', 'client', 'manager', 'service',
        'handler', 'router', 'config', 'cache', 'db', 'conn', 'session',
        'virtualStore', 'shardManager', 'learningStore', 'factStore'
    ]

    # Handler function name patterns
    HANDLER_PATTERNS = [
        r'handle\w+',
        r'on\w+',
        r'process\w+',
        r'\w+Handler',
        r'\w+Callback',
    ]

    # Builder method patterns that need terminal operations
    # Maps builder type suffix -> (creation pattern, terminal methods)
    BUILDER_TERMINALS = {
        'Builder': (r'New\w*Builder\s*\(', ['Build', 'Create', 'Make']),
        'Request': (r'\.NewRequest\s*\(', ['Do', 'Execute', 'Send']),
        'Transaction': (r'\.Begin\w*\(', ['Commit', 'Rollback']),
    }

    def __init__(self, workspace: str, verbose: bool = False,
                 fix_suggestions: bool = False, component: Optional[str] = None):
        self.workspace = Path(workspace)
        self.verbose = verbose
        self.fix_suggestions = fix_suggestions
        self.component = component
        self.result = UnwiredAuditResult(
            timestamp=datetime.now().isoformat(),
            workspace=str(self.workspace)
        )

        # Collected data for cross-file analysis
        self.struct_fields: Dict[str, List[StructField]] = defaultdict(list)
        self.function_params: List[FunctionParam] = []
        self.interface_methods: Dict[str, List[InterfaceMethod]] = defaultdict(list)
        self.type_methods: Dict[str, Set[str]] = defaultdict(set)  # type -> method names
        self.channels: Dict[str, Tuple[str, int, str]] = {}  # chan_name -> (file, line, type)
        self.channel_consumers: Set[str] = set()
        self.factories: Dict[str, Tuple[str, int]] = {}  # New* func -> (file, line)
        self.factory_calls: Set[str] = set()
        self.handlers: Dict[str, Tuple[str, int]] = {}  # handler func -> (file, line)
        self.registered_handlers: Set[str] = set()

    def audit(self) -> UnwiredAuditResult:
        """Run all unwired code audits."""
        print("=" * 70)
        print("UNWIRED CODE AUDIT")
        print("=" * 70)
        print(f"Workspace: {self.workspace}")
        print("Philosophy: Wire, Don't Remove")
        print()

        # Collect all Go files
        go_files = self._collect_go_files()
        print(f"Found {len(go_files)} Go files to audit")
        print()

        # Phase 1: Collect all definitions
        print("[Phase 1] Collecting definitions...")
        for filepath in go_files:
            self._collect_definitions(filepath)

        # Phase 2: Collect all usages
        print("[Phase 2] Collecting usages...")
        for filepath in go_files:
            self._collect_usages(filepath)

        # Phase 3: Run audits
        print("[Phase 3] Running audits...")

        print("  [1/10] Checking unused parameters...")
        self._audit_unused_params(go_files)

        print("  [2/10] Checking unused struct fields...")
        self._audit_unused_fields()

        print("  [3/10] Checking unimplemented interfaces...")
        self._audit_unimplemented_interfaces()

        print("  [4/10] Checking orphan channels...")
        self._audit_orphan_channels()

        print("  [5/10] Checking dead callbacks...")
        self._audit_dead_callbacks()

        print("  [6/10] Checking missing injections...")
        self._audit_missing_injections(go_files)

        print("  [7/10] Checking unrouted handlers...")
        self._audit_unrouted_handlers()

        print("  [8/10] Checking unused return values...")
        self._audit_unused_returns(go_files)

        print("  [9/10] Checking unused factories...")
        self._audit_unused_factories()

        print("  [10/10] Checking incomplete builders...")
        self._audit_incomplete_builders(go_files)

        # Calculate stats
        self._calculate_stats()

        return self.result

    def _collect_go_files(self) -> List[Path]:
        """Collect all Go files in the workspace."""
        go_files = []
        exclude_dirs = {'.git', 'vendor', 'node_modules', '.nerd', 'testdata'}

        for root, dirs, files in os.walk(self.workspace):
            dirs[:] = [d for d in dirs if d not in exclude_dirs]

            for file in files:
                if file.endswith('.go') and not file.endswith('_test.go'):
                    filepath = Path(root) / file
                    if self.component:
                        if self.component.lower() in str(filepath).lower():
                            go_files.append(filepath)
                    else:
                        go_files.append(filepath)

        return go_files

    def _collect_definitions(self, filepath: Path):
        """Collect struct fields, interfaces, channels, factories, handlers."""
        try:
            content = filepath.read_text(encoding='utf-8')
            rel_path = str(filepath.relative_to(self.workspace))

            # Collect struct fields
            struct_pattern = re.compile(
                r'type\s+(\w+)\s+struct\s*\{([^}]*)\}',
                re.DOTALL
            )
            for match in struct_pattern.finditer(content):
                struct_name = match.group(1)
                fields_block = match.group(2)
                struct_line = content[:match.start()].count('\n') + 1

                # Parse fields
                field_pattern = re.compile(r'^\s*(\w+)\s+(\S+)', re.MULTILINE)
                for field_match in field_pattern.finditer(fields_block):
                    field_name = field_match.group(1)
                    field_type = field_match.group(2)

                    # Skip embedded types and common false positives
                    if field_name[0].isupper() and field_type == '':
                        continue
                    if field_name in ['_', 'sync', 'context']:
                        continue

                    field_line = struct_line + fields_block[:field_match.start()].count('\n')
                    self.struct_fields[struct_name].append(
                        StructField(field_name, field_type, rel_path, field_line, struct_name)
                    )

            # Collect interface methods
            interface_pattern = re.compile(
                r'type\s+(\w+)\s+interface\s*\{([^}]*)\}',
                re.DOTALL
            )
            for match in interface_pattern.finditer(content):
                interface_name = match.group(1)
                methods_block = match.group(2)
                iface_line = content[:match.start()].count('\n') + 1

                method_pattern = re.compile(r'^\s*(\w+)\s*\(([^)]*)\)', re.MULTILINE)
                for method_match in method_pattern.finditer(methods_block):
                    method_name = method_match.group(1)
                    method_sig = method_match.group(0).strip()
                    method_line = iface_line + methods_block[:method_match.start()].count('\n')

                    self.interface_methods[interface_name].append(
                        InterfaceMethod(method_name, method_sig, interface_name, rel_path, method_line)
                    )

            # Collect type methods (for interface checking)
            method_pattern = re.compile(r'func\s+\(\s*\w+\s+\*?(\w+)\s*\)\s+(\w+)\s*\(')
            for match in method_pattern.finditer(content):
                type_name = match.group(1)
                method_name = match.group(2)
                self.type_methods[type_name].add(method_name)

            # Collect channel creations
            chan_pattern = re.compile(r'(\w+)\s*:?=\s*make\s*\(\s*chan\s+([^,\)]+)')
            for match in chan_pattern.finditer(content):
                chan_name = match.group(1)
                chan_type = match.group(2).strip()
                line = content[:match.start()].count('\n') + 1

                # Skip common names that are typically consumed
                if chan_name not in ['done', 'quit', 'stop', 'ctx', 'ch', 'c']:
                    self.channels[f"{rel_path}:{chan_name}"] = (rel_path, line, chan_type)

            # Collect factory functions (New* functions)
            factory_pattern = re.compile(r'func\s+(New\w+)\s*\(')
            for match in factory_pattern.finditer(content):
                func_name = match.group(1)
                line = content[:match.start()].count('\n') + 1
                self.factories[func_name] = (rel_path, line)

            # Collect handler functions
            for pattern in self.HANDLER_PATTERNS:
                handler_re = re.compile(rf'func\s+(?:\([^)]+\)\s+)?({pattern})\s*\(', re.IGNORECASE)
                for match in handler_re.finditer(content):
                    func_name = match.group(1)
                    line = content[:match.start()].count('\n') + 1
                    self.handlers[func_name] = (rel_path, line)

        except Exception as e:
            if self.verbose:
                print(f"  Warning: Could not parse {filepath}: {e}")

    def _collect_usages(self, filepath: Path):
        """Collect usages of channels, factories, handlers."""
        try:
            content = filepath.read_text(encoding='utf-8')
            rel_path = str(filepath.relative_to(self.workspace))

            # Collect channel consumers
            # Patterns: <-chan, range chan, case <-chan
            consume_patterns = [
                r'<-\s*(\w+)',
                r'range\s+(\w+)',
                r'case\s+[^:]*<-\s*(\w+)',
            ]
            for pattern in consume_patterns:
                for match in re.finditer(pattern, content):
                    chan_name = match.group(1)
                    self.channel_consumers.add(f"{rel_path}:{chan_name}")
                    # Also add without file prefix for cross-file consumption
                    self.channel_consumers.add(chan_name)

            # Collect factory calls
            for factory_name in self.factories.keys():
                if re.search(rf'\b{factory_name}\s*\(', content):
                    self.factory_calls.add(factory_name)

            # Collect handler registrations
            # Patterns: .Handle*, .On*, HandleFunc, routes
            register_patterns = [
                r'\.Handle(?:Func)?\s*\(\s*["\']?[^"\']*["\']?\s*,\s*(\w+)',
                r'\.On\s*\(\s*["\']?[^"\']*["\']?\s*,\s*(\w+)',
                r'\.Get|\.Post|\.Put|\.Delete\s*\([^,]+,\s*(\w+)',
                r'case\s+["\'][^"\']+["\']\s*:\s*(\w+)',  # switch on command
            ]
            for pattern in register_patterns:
                for match in re.finditer(pattern, content):
                    handler_name = match.group(1)
                    self.registered_handlers.add(handler_name)

        except Exception:
            pass

    def _audit_unused_params(self, go_files: List[Path]):
        """Check for function parameters that are never used in the body."""
        for filepath in go_files:
            try:
                content = filepath.read_text(encoding='utf-8')
                rel_path = str(filepath.relative_to(self.workspace))

                # Find function definitions with parameters
                func_pattern = re.compile(
                    r'func\s+(?:\([^)]+\)\s+)?(\w+)\s*\(([^)]+)\)\s*(?:[^{]*)\{',
                    re.DOTALL
                )

                for match in func_pattern.finditer(content):
                    func_name = match.group(1)
                    params_str = match.group(2)
                    func_start = match.end()
                    line = content[:match.start()].count('\n') + 1

                    # Find function body (balanced braces)
                    brace_count = 1
                    func_end = func_start
                    while brace_count > 0 and func_end < len(content):
                        if content[func_end] == '{':
                            brace_count += 1
                        elif content[func_end] == '}':
                            brace_count -= 1
                        func_end += 1

                    func_body = content[func_start:func_end]

                    # Parse parameters
                    param_pattern = re.compile(r'(\w+)\s+\S+')
                    for param_match in param_pattern.finditer(params_str):
                        param_name = param_match.group(1)

                        # Skip underscore and ctx (often used for interface compliance)
                        if param_name in ['_', 'ctx', 'context']:
                            continue

                        # Check if param is used in body
                        # Use word boundary to avoid false positives
                        if not re.search(rf'\b{param_name}\b', func_body):
                            self.result.findings.append(Finding(
                                severity=Severity.WARNING,
                                category=WiringCategory.UNUSED_PARAM,
                                message=f"Parameter '{param_name}' in {func_name}() is never used",
                                file=rel_path,
                                line=line,
                                suggestion=f"Wire '{param_name}' into the function logic, or rename to '_' if intentionally unused",
                                wire_to=f"Use in {func_name}() body"
                            ))

            except Exception:
                pass

    def _audit_unused_fields(self):
        """Check for struct fields that are never assigned or read."""
        # This needs cross-file analysis - check all files for field access
        all_content = ""
        for file_path in self._collect_go_files():
            try:
                all_content += file_path.read_text(encoding='utf-8') + "\n"
            except Exception:
                pass

        for struct_name, fields in self.struct_fields.items():
            for fld in fields:
                # Skip private fields starting with lowercase in other packages
                if fld.name[0].islower():
                    # Check for any usage pattern
                    usage_patterns = [
                        rf'\.{fld.name}\b',  # field access
                        rf'{fld.name}\s*:',  # struct literal
                    ]

                    found = False
                    for pattern in usage_patterns:
                        if len(re.findall(pattern, all_content)) > 1:  # More than definition
                            found = True
                            break

                    if not found:
                        # Check if it's an injection candidate
                        is_injection = any(
                            inj.lower() in fld.name.lower()
                            for inj in self.INJECTION_FIELDS
                        )

                        self.result.findings.append(Finding(
                            severity=Severity.WARNING if is_injection else Severity.INFO,
                            category=WiringCategory.UNUSED_FIELD if not is_injection else WiringCategory.MISSING_INJECTION,
                            message=f"Field '{struct_name}.{fld.name}' ({fld.type}) appears unused",
                            file=fld.file,
                            line=fld.line,
                            suggestion=f"Wire this field: assign in constructor/Set method and use in methods" if is_injection else f"Wire this field into {struct_name}'s logic",
                            wire_to=f"{struct_name} methods"
                        ))

    def _audit_unimplemented_interfaces(self):
        """Check for interfaces that types claim to implement but don't fully.

        Only flags types that:
        1. Implement MORE THAN HALF of the interface methods (strong signal of intent)
        2. Are missing specific methods

        This reduces false positives from types that happen to have
        similarly-named methods but aren't trying to implement the interface.
        """
        for interface_name, methods in self.interface_methods.items():
            # Skip very small interfaces (too many false positives)
            if len(methods) < 2:
                continue

            # Skip common stdlib interfaces
            if interface_name in ['Stringer', 'Error', 'Reader', 'Writer', 'Closer']:
                continue

            # Find types that might implement this interface
            # Look for types that implement at least one method
            potential_implementers = set()
            for method in methods:
                for type_name, type_methods in self.type_methods.items():
                    if method.name in type_methods:
                        potential_implementers.add(type_name)

            # Check if potential implementers have ALL methods
            for type_name in potential_implementers:
                missing_methods = []
                implemented_count = 0
                for method in methods:
                    if method.name in self.type_methods[type_name]:
                        implemented_count += 1
                    else:
                        missing_methods.append(method)

                # Only report if type implements MORE THAN HALF the interface
                # This is a strong signal they intended to implement it
                if missing_methods and implemented_count > len(methods) / 2:
                    # Limit to first 3 missing methods per type to avoid spam
                    for missing in missing_methods[:3]:
                        self.result.findings.append(Finding(
                            severity=Severity.ERROR,
                            category=WiringCategory.UNIMPLEMENTED_INTERFACE,
                            message=f"Type '{type_name}' is missing method '{missing.name}' from interface '{interface_name}' ({implemented_count}/{len(methods)} implemented)",
                            file=missing.file,
                            line=missing.line,
                            suggestion=f"Implement: func (t *{type_name}) {missing.signature}",
                            wire_to=f"{type_name}"
                        ))

    def _audit_orphan_channels(self):
        """Check for channels that are created but never consumed."""
        for chan_key, (file, line, chan_type) in self.channels.items():
            chan_name = chan_key.split(':')[-1]

            # Check if this channel is ever consumed
            if chan_key not in self.channel_consumers and chan_name not in self.channel_consumers:
                self.result.findings.append(Finding(
                    severity=Severity.WARNING,
                    category=WiringCategory.ORPHAN_CHANNEL,
                    message=f"Channel '{chan_name}' (chan {chan_type}) created but never consumed",
                    file=file,
                    line=line,
                    suggestion=f"Add consumer: go func() {{ for v := range {chan_name} {{ /* handle v */ }} }}()",
                    wire_to="Add goroutine consumer"
                ))

    def _audit_dead_callbacks(self):
        """Check for callback functions that are defined but never registered."""
        # Patterns that indicate callback registration
        callback_patterns = ['Callback', 'Hook', 'Handler', 'Listener']

        for func_name, (file, line) in self.handlers.items():
            # Check if this looks like a callback
            is_callback = any(p.lower() in func_name.lower() for p in callback_patterns)

            if is_callback and func_name not in self.registered_handlers:
                self.result.findings.append(Finding(
                    severity=Severity.WARNING,
                    category=WiringCategory.DEAD_CALLBACK,
                    message=f"Callback '{func_name}' defined but never registered",
                    file=file,
                    line=line,
                    suggestion=f"Register with: manager.On*(\"{func_name}\", {func_name})",
                    wire_to="Event registration"
                ))

    def _audit_missing_injections(self, go_files: List[Path]):
        """Check for dependency injection fields that are never set."""
        for filepath in go_files:
            try:
                content = filepath.read_text(encoding='utf-8')
                rel_path = str(filepath.relative_to(self.workspace))

                # Find Set* methods and what fields they set
                set_pattern = re.compile(r'func\s+\(\s*\w+\s+\*?(\w+)\s*\)\s+(Set\w+)\s*\(')
                set_methods = {}

                for match in set_pattern.finditer(content):
                    type_name = match.group(1)
                    method_name = match.group(2)
                    set_methods[f"{type_name}.{method_name}"] = True

                # Find New* functions and check if they call Set* methods or set fields
                new_pattern = re.compile(
                    r'func\s+New(\w+)\s*\([^)]*\)\s*(?:\*?\w+)?\s*\{([^}]+(?:\{[^}]*\}[^}]*)*)\}',
                    re.DOTALL
                )

                for match in new_pattern.finditer(content):
                    type_name = match.group(1)
                    constructor_body = match.group(2)
                    line = content[:match.start()].count('\n') + 1

                    # Check if injection fields for this type exist
                    if type_name in self.struct_fields:
                        for fld in self.struct_fields[type_name]:
                            is_injection = any(
                                inj.lower() in fld.name.lower()
                                for inj in self.INJECTION_FIELDS
                            )

                            if is_injection:
                                # Check if field is set in constructor
                                field_set = (
                                    f'{fld.name}:' in constructor_body or
                                    f'.{fld.name} =' in constructor_body
                                )

                                if not field_set:
                                    self.result.findings.append(Finding(
                                        severity=Severity.WARNING,
                                        category=WiringCategory.MISSING_INJECTION,
                                        message=f"Injection field '{type_name}.{fld.name}' not set in New{type_name}()",
                                        file=rel_path,
                                        line=line,
                                        suggestion=f"Add parameter and set: {fld.name}: {fld.name}, OR provide Set{fld.name.title()}() method",
                                        wire_to=f"New{type_name}() or Set{fld.name.title()}()"
                                    ))

            except Exception:
                pass

    def _audit_unrouted_handlers(self):
        """Check for handler functions that aren't connected to any router."""
        for handler_name, (file, line) in self.handlers.items():
            # Skip if it's clearly an internal helper
            if handler_name.startswith('_') or 'internal' in handler_name.lower():
                continue

            # Skip common false positives
            if handler_name in ['HandleFunc', 'Handler', 'handle', 'Handle']:
                continue

            if handler_name not in self.registered_handlers:
                self.result.findings.append(Finding(
                    severity=Severity.INFO,
                    category=WiringCategory.UNROUTED_HANDLER,
                    message=f"Handler '{handler_name}' may not be registered with any router",
                    file=file,
                    line=line,
                    suggestion=f"Register: router.Handle(\"/path\", {handler_name})",
                    wire_to="Router/Mux registration"
                ))

    def _audit_unused_returns(self, go_files: List[Path]):
        """Check for function calls where return values are discarded."""
        # High-value return patterns that shouldn't be discarded
        important_returns = [
            (r'(\w+)\s*\.\s*Start\s*\(\s*\)', 'Start() error'),
            (r'(\w+)\s*\.\s*Run\s*\(\s*\)', 'Run() error'),
            (r'(\w+)\s*\.\s*Close\s*\(\s*\)', 'Close() error'),
            (r'(\w+)\s*\.\s*Write\s*\([^)]+\)', 'Write() (n, error)'),
            (r'(\w+)\s*\.\s*Send\s*\([^)]+\)', 'Send() error'),
        ]

        for filepath in go_files:
            try:
                content = filepath.read_text(encoding='utf-8')
                rel_path = str(filepath.relative_to(self.workspace))
                lines = content.split('\n')

                for i, line_content in enumerate(lines, 1):
                    # Skip if line has assignment
                    if ':=' in line_content or '=' in line_content.split('//')[0]:
                        continue
                    # Skip defer statements
                    if line_content.strip().startswith('defer'):
                        continue

                    for pattern, ret_type in important_returns:
                        if re.search(pattern, line_content):
                            self.result.findings.append(Finding(
                                severity=Severity.WARNING,
                                category=WiringCategory.UNUSED_RETURN,
                                message=f"Return value ({ret_type}) discarded",
                                file=rel_path,
                                line=i,
                                code_snippet=line_content.strip()[:80],
                                suggestion=f"Capture and handle: err := {line_content.strip()}; if err != nil {{ ... }}",
                                wire_to="Error handling"
                            ))
                            break

            except Exception:
                pass

    def _audit_unused_factories(self):
        """Check for New* functions that are never called."""
        for factory_name, (file, line) in self.factories.items():
            if factory_name not in self.factory_calls:
                # Skip test helpers and internal factories
                if 'test' in factory_name.lower() or 'mock' in factory_name.lower():
                    continue

                self.result.findings.append(Finding(
                    severity=Severity.INFO,
                    category=WiringCategory.UNUSED_FACTORY,
                    message=f"Factory '{factory_name}' is defined but never called",
                    file=file,
                    line=line,
                    suggestion=f"Wire into initialization: obj := {factory_name}(...)",
                    wire_to="Initialization/bootstrap code"
                ))

    def _audit_incomplete_builders(self, go_files: List[Path]):
        """Check for builder patterns that don't call terminal operations."""
        for filepath in go_files:
            try:
                content = filepath.read_text(encoding='utf-8')
                rel_path = str(filepath.relative_to(self.workspace))

                for builder_name, (creation_pattern, terminals) in self.BUILDER_TERMINALS.items():
                    # Find builder creations using specific creation pattern
                    full_pattern = rf'(\w+)\s*:?=\s*[^;\n]*{creation_pattern}'

                    for match in re.finditer(full_pattern, content):
                        var_name = match.group(1)
                        line = content[:match.start()].count('\n') + 1

                        # Skip common false positives
                        if var_name in ['err', '_', 'ok']:
                            continue

                        # Check if any terminal is called on this builder
                        has_terminal = False
                        for terminal in terminals:
                            if re.search(rf'{var_name}\s*\.\s*{terminal}\s*\(', content):
                                has_terminal = True
                                break

                        if not has_terminal:
                            self.result.findings.append(Finding(
                                severity=Severity.WARNING,
                                category=WiringCategory.INCOMPLETE_BUILDER,
                                message=f"{builder_name} '{var_name}' created but terminal ({'/'.join(terminals)}) never called",
                                file=rel_path,
                                line=line,
                                suggestion=f"Complete the builder: result := {var_name}.{terminals[0]}()",
                                wire_to="Builder completion"
                            ))

            except Exception:
                pass

    def _calculate_stats(self):
        """Calculate audit statistics."""
        self.result.stats = {
            "total_findings": len(self.result.findings),
            "errors": sum(1 for f in self.result.findings if f.severity == Severity.ERROR),
            "warnings": sum(1 for f in self.result.findings if f.severity == Severity.WARNING),
            "info": sum(1 for f in self.result.findings if f.severity == Severity.INFO),
            "by_category": {}
        }

        for finding in self.result.findings:
            cat = finding.category.value
            self.result.stats["by_category"][cat] = \
                self.result.stats["by_category"].get(cat, 0) + 1

    def print_report(self) -> bool:
        """Print formatted audit report."""
        print()
        print("=" * 70)
        print("UNWIRED CODE AUDIT SUMMARY")
        print("=" * 70)
        print()
        print("REMEMBER: Wire, Don't Remove!")
        print("Unused code usually needs connection, not deletion.")
        print()

        # Overall status
        has_errors = self.result.stats.get("errors", 0) > 0
        status = "NEEDS WIRING" if has_errors or self.result.stats.get("warnings", 0) > 0 else "WELL WIRED"
        print(f"Status: {status}")
        print()

        # Stats
        print(f"Findings:")
        print(f"  Errors:   {self.result.stats.get('errors', 0)}")
        print(f"  Warnings: {self.result.stats.get('warnings', 0)}")
        print(f"  Info:     {self.result.stats.get('info', 0)}")
        print()

        # By category
        if self.result.stats.get("by_category"):
            print("By Category:")
            for cat, count in sorted(self.result.stats["by_category"].items()):
                print(f"  {cat}: {count}")
            print()

        # Errors
        errors = [f for f in self.result.findings if f.severity == Severity.ERROR]
        if errors:
            print("-" * 70)
            print("ERRORS (Must Wire)")
            print("-" * 70)
            for f in errors:
                print(f"[{f.category.value}] {f.message}")
                if f.file:
                    print(f"  File: {f.file}:{f.line}")
                if self.fix_suggestions and f.suggestion:
                    print(f"  Wire: {f.suggestion}")
                if self.fix_suggestions and f.wire_to:
                    print(f"  Target: {f.wire_to}")
                print()

        # Warnings
        warnings = [f for f in self.result.findings if f.severity == Severity.WARNING]
        if warnings:
            print("-" * 70)
            print("WARNINGS (Should Wire)")
            print("-" * 70)
            for f in warnings[:20]:
                print(f"[{f.category.value}] {f.message}")
                if f.file:
                    print(f"  File: {f.file}:{f.line}")
                if self.fix_suggestions and f.suggestion:
                    print(f"  Wire: {f.suggestion}")
                print()
            if len(warnings) > 20:
                print(f"  ... and {len(warnings) - 20} more warnings")
                print()

        # Info (verbose only)
        if self.verbose:
            infos = [f for f in self.result.findings if f.severity == Severity.INFO]
            if infos[:15]:
                print("-" * 70)
                print("INFO (Consider Wiring)")
                print("-" * 70)
                for f in infos[:15]:
                    print(f"[{f.category.value}] {f.message}")
                    if f.file:
                        print(f"  File: {f.file}:{f.line}")
                if len(infos) > 15:
                    print(f"  ... and {len(infos) - 15} more")
                print()

        print("=" * 70)
        print()
        print("Next Steps:")
        print("  1. Review each finding")
        print("  2. Ask: 'Why was this code written?'")
        print("  3. Wire it to its intended consumer")
        print("  4. Only remove if genuinely obsolete")
        print()

        return not has_errors


def find_workspace(start_path: str) -> Path:
    """Find codeNERD workspace root."""
    workspace = Path(start_path).resolve()
    while workspace != workspace.parent:
        if (workspace / ".nerd").exists() or (workspace / "go.mod").exists():
            return workspace
        workspace = workspace.parent
    return Path(start_path).resolve()


def main():
    parser = argparse.ArgumentParser(
        description="codeNERD Unwired Code Audit - Wire, Don't Remove!",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Philosophy: Unused code usually needs connection, not deletion.

Examples:
  python audit_unwired.py                      # Full audit
  python audit_unwired.py --verbose            # With all INFO findings
  python audit_unwired.py --fix-suggestions    # Include wiring suggestions
  python audit_unwired.py --component campaign # Focus on campaign
  python audit_unwired.py --json               # JSON for tooling
"""
    )
    parser.add_argument("workspace", nargs="?", default=".", help="Workspace path")
    parser.add_argument("--verbose", "-v", action="store_true", help="Show INFO findings")
    parser.add_argument("--json", action="store_true", help="Output as JSON")
    parser.add_argument("--component", "-c", help="Focus on specific component")
    parser.add_argument("--fix-suggestions", "-f", action="store_true",
                        help="Include detailed wiring suggestions")

    args = parser.parse_args()

    workspace = find_workspace(args.workspace)
    auditor = UnwiredAuditor(
        str(workspace),
        verbose=args.verbose,
        fix_suggestions=args.fix_suggestions,
        component=args.component
    )
    result = auditor.audit()

    if args.json:
        output = {
            "timestamp": result.timestamp,
            "workspace": result.workspace,
            "philosophy": "Wire, Don't Remove",
            "stats": result.stats,
            "findings": [
                {
                    "severity": f.severity.value,
                    "category": f.category.value,
                    "message": f.message,
                    "file": f.file,
                    "line": f.line,
                    "code_snippet": f.code_snippet,
                    "suggestion": f.suggestion,
                    "wire_to": f.wire_to,
                }
                for f in result.findings
            ],
        }
        print(json.dumps(output, indent=2))
    else:
        success = auditor.print_report()
        sys.exit(0 if success else 1)


if __name__ == "__main__":
    main()
