#!/usr/bin/env python3
"""
codeNERD Execution Wiring Auditor

Detects "code exists but doesn't execute" issues:
- Objects created but never run (New*() without Run()/Start())
- Local variables that should be stored in struct fields
- Channels created but never read
- Bubbletea message types without handlers
- Background goroutines not spawned
- Struct fields checked but never assigned

Usage:
    python cmd/tools/integration_audit/audit_execution.py [workspace_path] [--verbose] [--json] [--component X]

Examples:
    python audit_execution.py                    # Full audit
    python audit_execution.py --verbose          # With suggestions
    python audit_execution.py --component campaign  # Focus on campaign
"""

import os
import re
import sys
import argparse
import json
from pathlib import Path
from dataclasses import dataclass, field
from typing import Any, List, Dict, Set, Optional, TextIO, Tuple
from enum import Enum
from datetime import datetime


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
    pattern: Optional[str] = None  # Which pattern detected this


@dataclass
class ExecutionAuditResult:
    timestamp: str = ""
    workspace: str = ""
    findings: List[Finding] = field(default_factory=list)
    stats: Dict[str, Any] = field(default_factory=dict)


@dataclass(frozen=True)
class GoFunction:
    start: int
    body_start: int
    end: int
    receiver: Optional[str]
    receiver_type: Optional[str]
    name: str


GO_FUNC_PATTERN = re.compile(
    r'^func\s+(?:\(\s*(\w+)\s+\*?([\w.]+(?:\[[^\]]+\])?)\s*\)\s+)?(\w+)\s*\(',
    re.MULTILINE,
)


def sanitize_go_source(source: str) -> str:
    """Blank comments and literals while preserving offsets and newlines."""
    chars = list(source)
    i = 0
    state = "code"
    quote = ""
    while i < len(chars):
        ch = chars[i]
        nxt = chars[i + 1] if i + 1 < len(chars) else ""
        if state == "code":
            if ch == '/' and nxt == '/':
                chars[i] = chars[i + 1] = ' '
                i += 2
                state = "line_comment"
                continue
            if ch == '/' and nxt == '*':
                chars[i] = chars[i + 1] = ' '
                i += 2
                state = "block_comment"
                continue
            if ch in ('"', "'", '`'):
                quote = ch
                chars[i] = ' '
                i += 1
                state = "raw_string" if ch == '`' else "quoted"
                continue
        elif state == "line_comment":
            if ch == '\n':
                state = "code"
            else:
                chars[i] = ' '
            i += 1
            continue
        elif state == "block_comment":
            if ch == '*' and nxt == '/':
                chars[i] = chars[i + 1] = ' '
                i += 2
                state = "code"
                continue
            if ch != '\n':
                chars[i] = ' '
            i += 1
            continue
        elif state == "raw_string":
            if ch == quote:
                state = "code"
            if ch != '\n':
                chars[i] = ' '
            i += 1
            continue
        elif state == "quoted":
            if ch == '\\':
                chars[i] = ' '
                if i + 1 < len(chars) and chars[i + 1] != '\n':
                    chars[i + 1] = ' '
                i += 2
                continue
            if ch == quote:
                state = "code"
            if ch != '\n':
                chars[i] = ' '
            i += 1
            continue
        i += 1
    return ''.join(chars)


def matching_brace(source: str, opening: int) -> int:
    depth = 0
    for i in range(opening, len(source)):
        if source[i] == '{':
            depth += 1
        elif source[i] == '}':
            depth -= 1
            if depth == 0:
                return i + 1
    return len(source)


def go_functions(source: str) -> List[GoFunction]:
    functions: List[GoFunction] = []
    for match in GO_FUNC_PATTERN.finditer(source):
        body_start = source.find('{', match.end())
        if body_start == -1:
            continue
        functions.append(GoFunction(
            start=match.start(),
            body_start=body_start,
            end=matching_brace(source, body_start),
            receiver=match.group(1),
            receiver_type=match.group(2),
            name=match.group(3),
        ))
    return functions


def enclosing_function(functions: List[GoFunction], position: int) -> Optional[GoFunction]:
    for function in functions:
        if function.start <= position < function.end:
            return function
    return None


def inside_go_func(source: str, position: int) -> bool:
    for match in re.finditer(r'\bgo\s+func\s*\(', source[:position]):
        opening = source.find('{', match.end())
        if opening != -1 and opening < position < matching_brace(source, opening):
            return True
    return False


class ExecutionAuditor:
    """Audits Go code for execution wiring gaps."""

    # Patterns for objects that need execution methods called
    EXECUTION_PATTERNS = {
        # pattern: (creation_regex, required_methods, severity)
        "Orchestrator": (
            r'^\s*(\w+)(?:\s*,\s*\w+)*\s*:?=\s*\w*\.?NewOrchestrator\s*\(',
            ["Run", "Start"],
            Severity.ERROR
        ),
        "Server": (
            r'^\s*(\w+)(?:\s*,\s*\w+)*\s*:?=\s*\w*\.?NewServer\s*\(',
            ["Start", "ListenAndServe", "Serve"],
            Severity.ERROR
        ),
        "Watcher": (
            r'^\s*(\w+)(?:\s*,\s*\w+)*\s*:?=\s*\w*\.?NewWatcher\s*\(',
            ["Start", "Watch", "Run"],
            Severity.WARNING
        ),
        "Ticker": (
            r'^\s*(\w+)\s*:?=\s*time\.NewTicker\s*\(',
            [".C"],  # Channel read
            Severity.WARNING
        ),
        "Timer": (
            r'^\s*(\w+)\s*:?=\s*time\.NewTimer\s*\(',
            [".C"],  # Channel read
            Severity.WARNING
        ),
        "Context": (
            r'^\s*(\w+),\s*(\w+)\s*:?=\s*context\.WithCancel\s*\(',
            ["cancel"],  # cancel function should be called
            Severity.INFO
        ),
    }

    # Bubbletea message pattern
    MSG_TYPE_PATTERN = re.compile(r'type\s+(\w+Msg)\s+(?:struct|=)')

    # Channel creation pattern
    CHANNEL_PATTERN = re.compile(r'(\w+)\s*:?=\s*make\s*\(\s*chan\s+([^,\)]+)')

    # Struct field check pattern (for detecting fields that are checked but not assigned)
    FIELD_CHECK_PATTERN = re.compile(r'\b(\w+)\.(\w+)\s*[!=]=\s*nil')
    FIELD_ASSIGN_PATTERN = re.compile(r'\b(\w+)\.(\w+)\s*=\s*[^=]')

    def __init__(
        self,
        workspace: str,
        verbose: bool = False,
        component: Optional[str] = None,
        progress_stream: Optional[TextIO] = None,
    ):
        self.workspace = Path(workspace)
        self.verbose = verbose
        self.component = component
        self.progress_stream = progress_stream or sys.stdout
        self._read_failures: Set[str] = set()
        self.result = ExecutionAuditResult(
            timestamp=datetime.now().isoformat(),
            workspace=str(self.workspace)
        )

    def audit(self) -> ExecutionAuditResult:
        """Run all execution wiring audits."""
        self._progress("=" * 70)
        self._progress("EXECUTION WIRING AUDIT")
        self._progress("=" * 70)
        self._progress(f"Workspace: {self.workspace}")
        self._progress()

        # Collect all Go files
        go_files = self._collect_go_files()
        self._progress(f"Found {len(go_files)} Go files to audit")
        self._progress()

        # Run audits
        self._progress("[1/6] Checking object execution (New*() without Run())...")
        self._audit_object_execution(go_files)

        self._progress("[2/6] Checking channel listeners...")
        self._audit_channel_listeners(go_files)

        self._progress("[3/6] Checking Bubbletea message handlers...")
        self._audit_message_handlers(go_files)

        self._progress("[4/6] Checking struct field assignments...")
        self._audit_field_assignments(go_files)

        self._progress("[5/6] Checking goroutine spawning...")
        self._audit_goroutine_spawning(go_files)

        self._progress("[6/6] Checking reference storage...")
        self._audit_reference_storage(go_files)

        # Calculate stats
        self._calculate_stats()

        return self.result

    def _progress(self, message: str = "") -> None:
        print(message, file=self.progress_stream)

    def _read_source(self, filepath: Path) -> Optional[str]:
        try:
            return filepath.read_text(encoding='utf-8')
        except Exception as exc:
            rel_path = str(filepath.relative_to(self.workspace))
            if rel_path not in self._read_failures:
                self._read_failures.add(rel_path)
                self.result.findings.append(Finding(
                    severity=Severity.WARNING,
                    message=f"Skipped unreadable Go file: {exc}",
                    file=rel_path,
                    pattern="audit_surface",
                ))
            return None

    def _collect_go_files(self) -> List[Path]:
        """Collect all Go files in the workspace."""
        go_files = []
        exclude_dirs = {
            '.git', '.nerd', '.claude', '.codex', '.agents', '.grok',
            'vendor', 'node_modules', 'testdata', '__pycache__',
        }

        for root, dirs, files in os.walk(self.workspace):
            # Filter out excluded directories
            dirs[:] = [d for d in dirs if d not in exclude_dirs]

            for file in files:
                if file.endswith('.go') and not file.endswith('_test.go'):
                    filepath = Path(root) / file
                    # Apply component filter if specified
                    if self.component:
                        rel_parts = filepath.relative_to(self.workspace).parts
                        component = self.component.casefold()
                        if any(part.casefold() == component for part in rel_parts):
                            go_files.append(filepath)
                    else:
                        go_files.append(filepath)

        return go_files

    def _audit_object_execution(self, go_files: List[Path]):
        """Check that objects are actually executed after creation."""
        for filepath in go_files:
            content = self._read_source(filepath)
            if content is None:
                continue
            source = sanitize_go_source(content)
            functions = go_functions(source)

            for pattern_name, (creation_regex, required_methods, severity) in self.EXECUTION_PATTERNS.items():
                for match in re.finditer(creation_regex, source, re.MULTILINE):
                    function = enclosing_function(functions, match.start())
                    if function is None:
                        continue
                    scope = source[function.start:function.end]
                    var_name = match.group(2) if pattern_name == "Context" else match.group(1)
                    escaped = re.escape(var_name)
                    line_num = source[:match.start()].count('\n') + 1

                    method_found = False
                    for method in required_methods:
                        if pattern_name == "Context":
                            call_pattern = rf'\b{escaped}\s*\('
                        elif method.startswith('.'):
                            call_pattern = rf'\b{escaped}{re.escape(method)}\b'
                        else:
                            call_pattern = rf'\b{escaped}\.{re.escape(method)}\w*\s*\('
                        if re.search(call_pattern, scope):
                            method_found = True
                            break
                    if method_found:
                        continue

                    after_creation = source[match.end():function.end]
                    returned = re.search(rf'(?m)\breturn[^\n]*\b{escaped}\b', after_creation)
                    stored = re.search(
                        rf'\b\w+\.\w+\s*=\s*\b{escaped}\b|\b\w+\s*:\s*\b{escaped}\b',
                        after_creation,
                    )
                    if returned or stored:
                        continue

                    first_method = required_methods[0]
                    suggestion = (
                        f"Call {var_name}() or defer {var_name}()"
                        if pattern_name == "Context"
                        else f"Add {var_name}{first_method if first_method.startswith('.') else '.' + first_method + '()'} or store in struct field"
                    )
                    self.result.findings.append(Finding(
                        severity=severity,
                        message=f"{pattern_name} '{var_name}' created but {'/'.join(required_methods)} never called",
                        file=str(filepath.relative_to(self.workspace)),
                        line=line_num,
                        suggestion=suggestion,
                        pattern="object_execution",
                    ))

    def _audit_channel_listeners(self, go_files: List[Path]):
        """Check that created channels are read from."""
        for filepath in go_files:
            content = self._read_source(filepath)
            if content is None:
                continue
            source = sanitize_go_source(content)
            functions = go_functions(source)

            for match in self.CHANNEL_PATTERN.finditer(source):
                function = enclosing_function(functions, match.start())
                if function is None:
                    continue
                scope = source[function.start:function.end]
                chan_name = match.group(1)
                chan_type = match.group(2)
                escaped = re.escape(chan_name)
                line_num = source[:match.start()].count('\n') + 1

                read_pattern = rf'<-\s*\b{escaped}\b|\b{escaped}\b\s*<-|\brange\s+\b{escaped}\b'
                if re.search(read_pattern, scope):
                    continue

                # Passing the channel as an actual argument transfers ownership
                # to another consumer. Restrict the suppression to call syntax
                # inside the enclosing function; comments and strings are blanked.
                passed_pattern = rf'\b\w+(?:\.\w+)*\s*\([^)]*\b{escaped}\b[^)]*\)'
                if re.search(passed_pattern, scope):
                    continue

                self.result.findings.append(Finding(
                    severity=Severity.WARNING,
                    message=f"Channel '{chan_name}' (chan {chan_type}) created but never read from",
                    file=str(filepath.relative_to(self.workspace)),
                    line=line_num,
                    suggestion=f"Add goroutine to read from {chan_name} or pass to consumer",
                    pattern="channel_listener",
                ))

    def _audit_message_handlers(self, go_files: List[Path]):
        """Check that Bubbletea message types have handlers."""
        # First, collect all message types
        msg_types: Dict[str, Tuple[str, int]] = {}  # name -> (file, line)

        for filepath in go_files:
            content = self._read_source(filepath)
            if content is None:
                continue
            source = sanitize_go_source(content)
            for match in self.MSG_TYPE_PATTERN.finditer(source):
                msg_name = match.group(1)
                line_num = source[:match.start()].count('\n') + 1
                msg_types[msg_name] = (str(filepath.relative_to(self.workspace)), line_num)

        # Now check for handlers in Update() functions
        handled_msgs: Set[str] = set()

        for filepath in go_files:
            content = self._read_source(filepath)
            if content is None:
                continue
            source = sanitize_go_source(content)

            # Handle single, pointer, qualified, and comma-separated type cases.
            for case in re.finditer(r'\bcase\s+([^:]+):', source):
                for msg_name in re.findall(r'(?:\b\w+\.)?\*?(\w+Msg)\b', case.group(1)):
                    handled_msgs.add(msg_name)
            # Also recognize explicit type assertions outside a switch.
            for assertion in re.finditer(r'\.\(\s*(?:\w+\.)?\*?(\w+Msg)\s*\)', source):
                handled_msgs.add(assertion.group(1))

        # Report unhandled messages
        for msg_name, (filepath, line) in msg_types.items():
            if msg_name not in handled_msgs:
                self.result.findings.append(Finding(
                    severity=Severity.WARNING,
                    message=f"Message type '{msg_name}' defined but no case handler in Update()",
                    file=filepath,
                    line=line,
                    suggestion=f"Add 'case {msg_name}:' handler in Update() method",
                    pattern="message_handler"
                ))

    def _audit_field_assignments(self, go_files: List[Path]):
        """Check that struct fields that are checked are also assigned."""
        # Key by package path + receiver type + field. Local receiver spellings
        # (k, c, m) are not identities and routinely collide across Go types.
        field_checks: Dict[str, Set[Tuple[str, int]]] = {}
        field_assigns: Set[str] = set()
        display_names: Dict[str, str] = {}
        embedded_types: Dict[Tuple[str, str], Set[str]] = {}

        for filepath in go_files:
            content = self._read_source(filepath)
            if content is None:
                continue
            source = sanitize_go_source(content)
            functions = go_functions(source)
            rel_path = str(filepath.relative_to(self.workspace))
            package_key = str(filepath.parent.relative_to(self.workspace)).casefold()

            for struct_match in re.finditer(r'\btype\s+(\w+)\s+struct\s*\{', source):
                opening = source.find('{', struct_match.start())
                body = source[opening + 1:matching_brace(source, opening) - 1]
                owner = struct_match.group(1)
                for line in body.splitlines():
                    embedded = re.fullmatch(
                        r'\s*\*?(?:\w+\.)?(\w+(?:\[[^\]]+\])?)\s*', line
                    )
                    if embedded:
                        embedded_types.setdefault((package_key, owner), set()).add(
                            embedded.group(1).split('[', 1)[0]
                        )

            def typed_key(match: re.Match) -> Optional[str]:
                function = enclosing_function(functions, match.start())
                if (
                    function is None
                    or function.receiver is None
                    or function.receiver_type is None
                    or match.group(1) != function.receiver
                ):
                    return None
                receiver_type = function.receiver_type.split('[', 1)[0]
                field_name = match.group(2)
                key = f"{package_key}:{receiver_type}.{field_name}"
                display_names[key] = f"{receiver_type}.{field_name}"
                return key

            for match in self.FIELD_CHECK_PATTERN.finditer(source):
                key = typed_key(match)
                if key is None:
                    continue
                field_checks.setdefault(key, set()).add(
                    (rel_path, source[:match.start()].count('\n') + 1)
                )

            for match in self.FIELD_ASSIGN_PATTERN.finditer(source):
                key = typed_key(match)
                if key is not None:
                    field_assigns.add(key)

            # Constructors usually initialize fields with &Type{field: value}.
            # Walk balanced literals so a similarly named field in a later
            # literal cannot suppress the finding.
            for literal in re.finditer(r'\b([A-Za-z_]\w*(?:\[[^\]]+\])?)\s*\{', source):
                type_name = literal.group(1).split('[', 1)[0]
                opening = source.find('{', literal.start())
                body = source[opening + 1:matching_brace(source, opening) - 1]
                for field_match in re.finditer(r'(?:^|[,\n])\s*([A-Za-z_]\w*)\s*:', body):
                    field_assigns.add(f"{package_key}:{type_name}.{field_match.group(1)}")

        # Find fields that are checked but never assigned
        for key, checks in field_checks.items():
            package_key, typed_field = key.split(':', 1)
            receiver_type, field_name = typed_field.split('.', 1)

            def assigned_on_type(type_name: str, seen: Set[str]) -> bool:
                if type_name in seen:
                    return False
                seen.add(type_name)
                if f"{package_key}:{type_name}.{field_name}" in field_assigns:
                    return True
                return any(
                    assigned_on_type(embedded, seen)
                    for embedded in embedded_types.get((package_key, type_name), set())
                )

            if not assigned_on_type(receiver_type, set()):
                display_name = display_names[key]
                if len(field_name) > 2 and not field_name.startswith('_'):
                    rel_path, line = sorted(checks)[0]
                    self.result.findings.append(Finding(
                        # Regex evidence cannot prove reflection/generated
                        # wiring is absent; keep this actionable but non-fatal.
                        severity=Severity.WARNING,
                        message=f"Field '{display_name}' is nil-checked but no assignment was found",
                        file=rel_path,
                        line=line,
                        suggestion=f"Confirm {display_name} is initialized in a constructor or setter",
                        pattern="field_assignment",
                    ))

    def _audit_goroutine_spawning(self, go_files: List[Path]):
        """Check for blocking operations that should be in goroutines."""
        # Patterns that suggest blocking operations
        blocking_patterns = [
            (r'\.Run\s*\(\s*ctx', 'Run()'),
            (r'\.Listen\s*\(', 'Listen()'),
            (r'time\.Sleep\s*\([^)]*time\.(Minute|Hour)', 'Long Sleep'),
            (r'for\s*\{\s*select\s*\{', 'Select loop'),
        ]

        for filepath in go_files:
            content = self._read_source(filepath)
            if content is None:
                continue
            source = sanitize_go_source(content)
            functions = go_functions(source)

            for pattern, name in blocking_patterns:
                for match in re.finditer(pattern, source):
                    function = enclosing_function(functions, match.start())
                    if function is None:
                        continue
                    header = source[function.start:function.body_start]
                    if (
                        inside_go_func(source, match.start())
                        or 'tea.Cmd' in header
                        or function.name in ['Run', 'Start', 'Listen', 'Serve']
                    ):
                        continue

                    self.result.findings.append(Finding(
                        severity=Severity.INFO,
                        message=f"Potentially blocking call '{name}' not in goroutine",
                        file=str(filepath.relative_to(self.workspace)),
                        line=source[:match.start()].count('\n') + 1,
                        suggestion="Consider wrapping in 'go func() { ... }()' if this blocks",
                        pattern="goroutine_spawn",
                    ))

    def _audit_reference_storage(self, go_files: List[Path]):
        """Check that objects that should be stored aren't just local variables."""
        # Objects that typically need to be stored for later access
        store_candidates = [
            'Orchestrator',
            'Manager',
            'Controller',
            'Service',
            'Client',
            'Connection',
            'Session',
        ]

        for filepath in go_files:
            content = self._read_source(filepath)
            if content is None:
                continue
            source = sanitize_go_source(content)
            functions = go_functions(source)

            for candidate in store_candidates:
                pattern = rf'(\w+)\s*:=\s*\w*\.?New{candidate}\s*\('
                for match in re.finditer(pattern, source):
                    function = enclosing_function(functions, match.start())
                    if function is None:
                        continue
                    scope = source[function.start:function.end]
                    if len(scope.split('\n')) >= 30:
                        continue

                    var_name = match.group(1)
                    escaped = re.escape(var_name)
                    after_creation = source[match.end():function.end]
                    stored = re.search(
                        rf'\b\w+\.\w+\s*=\s*\b{escaped}\b|\b\w+\s*:\s*\b{escaped}\b',
                        after_creation,
                    )
                    returned = re.search(rf'(?m)\breturn[^\n]*\b{escaped}\b', after_creation)
                    used = re.search(rf'\b{escaped}\b', after_creation)
                    if stored or returned or used:
                        continue

                    self.result.findings.append(Finding(
                        severity=Severity.WARNING,
                        message=f"{candidate} '{var_name}' created as local var but may be lost when function returns",
                        file=str(filepath.relative_to(self.workspace)),
                        line=source[:match.start()].count('\n') + 1,
                        suggestion=f"Consider storing in struct field: m.{var_name[0].lower()}{var_name[1:]} = {var_name}",
                        pattern="reference_storage",
                    ))

    def _calculate_stats(self):
        """Calculate audit statistics."""
        self.result.stats = {
            "total_findings": len(self.result.findings),
            "errors": sum(1 for f in self.result.findings if f.severity == Severity.ERROR),
            "warnings": sum(1 for f in self.result.findings if f.severity == Severity.WARNING),
            "info": sum(1 for f in self.result.findings if f.severity == Severity.INFO),
            "patterns": {}
        }

        # Count by pattern
        for finding in self.result.findings:
            if finding.pattern:
                self.result.stats["patterns"][finding.pattern] = \
                    self.result.stats["patterns"].get(finding.pattern, 0) + 1

    def print_report(self) -> bool:
        """Print formatted audit report."""
        print()
        print("=" * 70)
        print("EXECUTION WIRING AUDIT SUMMARY")
        print("=" * 70)
        print()

        # Overall status
        has_errors = self.result.stats.get("errors", 0) > 0
        status = "FAIL" if has_errors else "PASS"
        print(f"Status: {status}")
        print()

        # Stats
        print(f"Findings:")
        print(f"  Errors:   {self.result.stats.get('errors', 0)}")
        print(f"  Warnings: {self.result.stats.get('warnings', 0)}")
        print(f"  Info:     {self.result.stats.get('info', 0)}")
        print()

        # By pattern
        if self.result.stats.get("patterns"):
            print("By Pattern:")
            for pattern, count in sorted(self.result.stats["patterns"].items()):
                print(f"  {pattern}: {count}")
            print()

        # Errors
        errors = [f for f in self.result.findings if f.severity == Severity.ERROR]
        if errors:
            print("-" * 70)
            print("ERRORS (Must Fix)")
            print("-" * 70)
            for f in errors:
                print(f"[{f.pattern}] {f.message}")
                if f.file:
                    print(f"  File: {f.file}:{f.line}")
                if self.verbose and f.suggestion:
                    print(f"  Fix: {f.suggestion}")
                print()

        # Warnings
        warnings = [f for f in self.result.findings if f.severity == Severity.WARNING]
        if warnings:
            print("-" * 70)
            print("WARNINGS")
            print("-" * 70)
            for f in warnings[:15]:  # Limit
                print(f"[{f.pattern}] {f.message}")
                if f.file:
                    print(f"  File: {f.file}:{f.line}")
                if self.verbose and f.suggestion:
                    print(f"  Fix: {f.suggestion}")
                print()
            if len(warnings) > 15:
                print(f"  ... and {len(warnings) - 15} more warnings")
                print()

        # Info (verbose only)
        if self.verbose:
            infos = [f for f in self.result.findings if f.severity == Severity.INFO]
            if infos[:10]:
                print("-" * 70)
                print("INFO (showing first 10)")
                print("-" * 70)
                for f in infos[:10]:
                    print(f"[{f.pattern}] {f.message}")
                    if f.file:
                        print(f"  File: {f.file}:{f.line}")
                if len(infos) > 10:
                    print(f"  ... and {len(infos) - 10} more")
                print()

        print("=" * 70)

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
        description="codeNERD Execution Wiring Audit",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Examples:
  python audit_execution.py                    # Full audit
  python audit_execution.py --verbose          # With suggestions
  python audit_execution.py --component campaign  # Focus on campaign
  python audit_execution.py --json             # JSON for tooling
"""
    )
    parser.add_argument("workspace", nargs="?", default=".", help="Workspace path")
    parser.add_argument("--verbose", "-v", action="store_true", help="Show detailed suggestions")
    parser.add_argument("--json", action="store_true", help="Output as JSON")
    parser.add_argument("--component", "-c", help="Focus on specific component")

    args = parser.parse_args()

    workspace = find_workspace(args.workspace)
    auditor = ExecutionAuditor(
        str(workspace),
        verbose=args.verbose,
        component=args.component,
        progress_stream=sys.stderr if args.json else sys.stdout,
    )
    result = auditor.audit()

    if args.json:
        output = {
            "timestamp": result.timestamp,
            "workspace": result.workspace,
            "stats": result.stats,
            "findings": [
                {
                    "severity": f.severity.value,
                    "message": f.message,
                    "file": f.file,
                    "line": f.line,
                    "suggestion": f.suggestion,
                    "pattern": f.pattern,
                }
                for f in result.findings
            ],
        }
        print(json.dumps(output, indent=2))
        sys.exit(0 if result.stats.get("errors", 0) == 0 else 1)
    else:
        success = auditor.print_report()
        sys.exit(0 if success else 1)


if __name__ == "__main__":
    main()
