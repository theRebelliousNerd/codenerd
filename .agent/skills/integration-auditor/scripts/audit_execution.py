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
    python audit_execution.py [workspace_path] [--verbose] [--json] [--component X]

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
from typing import List, Dict, Set, Optional, Tuple
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
    stats: Dict[str, int] = field(default_factory=dict)


class ExecutionAuditor:
    """Audits Go code for execution wiring gaps."""

    # Patterns for objects that need execution methods called
    EXECUTION_PATTERNS = {
        # pattern: (creation_regex, required_methods, severity)
        "Orchestrator": (
            r'(\w+)\s*:?=\s*\w*\.?NewOrchestrator\s*\(',
            ["Run", "Start"],
            Severity.ERROR
        ),
        "Server": (
            r'(\w+)\s*:?=\s*\w*\.?NewServer\s*\(',
            ["Start", "ListenAndServe", "Serve"],
            Severity.ERROR
        ),
        "Watcher": (
            r'(\w+)\s*:?=\s*\w*\.?NewWatcher\s*\(',
            ["Start", "Watch", "Run"],
            Severity.WARNING
        ),
        "Ticker": (
            r'(\w+)\s*:?=\s*time\.NewTicker\s*\(',
            [".C"],  # Channel read
            Severity.WARNING
        ),
        "Timer": (
            r'(\w+)\s*:?=\s*time\.NewTimer\s*\(',
            [".C"],  # Channel read
            Severity.WARNING
        ),
        "Context": (
            r'(\w+),\s*(\w+)\s*:?=\s*context\.WithCancel\s*\(',
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

    def __init__(self, workspace: str, verbose: bool = False, component: Optional[str] = None):
        self.workspace = Path(workspace)
        self.verbose = verbose
        self.component = component
        self.result = ExecutionAuditResult(
            timestamp=datetime.now().isoformat(),
            workspace=str(self.workspace)
        )

    def audit(self) -> ExecutionAuditResult:
        """Run all execution wiring audits."""
        print("=" * 70)
        print("EXECUTION WIRING AUDIT")
        print("=" * 70)
        print(f"Workspace: {self.workspace}")
        print()

        # Collect all Go files
        go_files = self._collect_go_files()
        print(f"Found {len(go_files)} Go files to audit")
        print()

        # Run audits
        print("[1/6] Checking object execution (New*() without Run())...")
        self._audit_object_execution(go_files)

        print("[2/6] Checking channel listeners...")
        self._audit_channel_listeners(go_files)

        print("[3/6] Checking Bubbletea message handlers...")
        self._audit_message_handlers(go_files)

        print("[4/6] Checking struct field assignments...")
        self._audit_field_assignments(go_files)

        print("[5/6] Checking goroutine spawning...")
        self._audit_goroutine_spawning(go_files)

        print("[6/6] Checking reference storage...")
        self._audit_reference_storage(go_files)

        # Calculate stats
        self._calculate_stats()

        return self.result

    def _collect_go_files(self) -> List[Path]:
        """Collect all Go files in the workspace."""
        go_files = []
        exclude_dirs = {'.git', 'vendor', 'node_modules', '.nerd', 'testdata'}

        for root, dirs, files in os.walk(self.workspace):
            # Filter out excluded directories
            dirs[:] = [d for d in dirs if d not in exclude_dirs]

            for file in files:
                if file.endswith('.go') and not file.endswith('_test.go'):
                    filepath = Path(root) / file
                    # Apply component filter if specified
                    if self.component:
                        if self.component.lower() in str(filepath).lower():
                            go_files.append(filepath)
                    else:
                        go_files.append(filepath)

        return go_files

    def _audit_object_execution(self, go_files: List[Path]):
        """Check that objects are actually executed after creation."""
        for filepath in go_files:
            try:
                content = filepath.read_text(encoding='utf-8')
                lines = content.split('\n')

                for pattern_name, (creation_regex, required_methods, severity) in self.EXECUTION_PATTERNS.items():
                    # Find all creations
                    for match in re.finditer(creation_regex, content):
                        var_name = match.group(1)
                        line_num = content[:match.start()].count('\n') + 1

                        # Check if any required method is called on this variable
                        method_found = False
                        for method in required_methods:
                            if method.startswith('.'):
                                # Channel read pattern
                                if f'{var_name}{method}' in content or f'<-{var_name}{method}' in content:
                                    method_found = True
                                    break
                            else:
                                # Method call pattern
                                method_pattern = rf'{var_name}\.{method}\s*\('
                                if re.search(method_pattern, content):
                                    method_found = True
                                    break

                        if not method_found:
                            # Check if variable is returned or stored in struct field
                            return_pattern = rf'return\s+[^;]*{var_name}'
                            field_assign_pattern = rf'\w+\.(\w+)\s*=\s*{var_name}'

                            if re.search(return_pattern, content) or re.search(field_assign_pattern, content):
                                # Variable is returned or stored - might be used elsewhere
                                continue

                            self.result.findings.append(Finding(
                                severity=severity,
                                message=f"{pattern_name} '{var_name}' created but {'/'.join(required_methods)} never called",
                                file=str(filepath.relative_to(self.workspace)),
                                line=line_num,
                                suggestion=f"Add {var_name}.{required_methods[0]}() or store in struct field",
                                pattern="object_execution"
                            ))

            except Exception as e:
                if self.verbose:
                    print(f"  Warning: Could not read {filepath}: {e}")

    def _audit_channel_listeners(self, go_files: List[Path]):
        """Check that created channels are read from."""
        for filepath in go_files:
            try:
                content = filepath.read_text(encoding='utf-8')

                for match in self.CHANNEL_PATTERN.finditer(content):
                    chan_name = match.group(1)
                    chan_type = match.group(2)
                    line_num = content[:match.start()].count('\n') + 1

                    # Skip if channel is a function parameter or return value
                    if chan_name in ['ch', 'c', 'done', 'quit', 'stop', 'ctx']:
                        continue

                    # Check if channel is read from
                    read_pattern = rf'<-\s*{chan_name}|{chan_name}\s*<-|range\s+{chan_name}'
                    select_pattern = rf'case\s+[^:]*<-\s*{chan_name}'

                    if not re.search(read_pattern, content) and not re.search(select_pattern, content):
                        # Check if channel is passed to another function
                        param_pattern = rf'\([^)]*{chan_name}[^)]*\)'
                        if re.search(param_pattern, content):
                            continue

                        self.result.findings.append(Finding(
                            severity=Severity.WARNING,
                            message=f"Channel '{chan_name}' (chan {chan_type}) created but never read from",
                            file=str(filepath.relative_to(self.workspace)),
                            line=line_num,
                            suggestion=f"Add goroutine to read from {chan_name} or pass to consumer",
                            pattern="channel_listener"
                        ))

            except Exception as e:
                if self.verbose:
                    print(f"  Warning: Could not read {filepath}: {e}")

    def _audit_message_handlers(self, go_files: List[Path]):
        """Check that Bubbletea message types have handlers."""
        # First, collect all message types
        msg_types: Dict[str, Tuple[str, int]] = {}  # name -> (file, line)

        for filepath in go_files:
            try:
                content = filepath.read_text(encoding='utf-8')

                for match in self.MSG_TYPE_PATTERN.finditer(content):
                    msg_name = match.group(1)
                    line_num = content[:match.start()].count('\n') + 1
                    msg_types[msg_name] = (str(filepath.relative_to(self.workspace)), line_num)

            except Exception:
                pass

        # Now check for handlers in Update() functions
        handled_msgs: Set[str] = set()

        for filepath in go_files:
            try:
                content = filepath.read_text(encoding='utf-8')

                # Find all case statements in Update functions
                # Look for patterns like "case fooMsg:" or "case *fooMsg:"
                case_pattern = re.compile(r'case\s+\*?(\w+Msg)\s*:')
                for match in case_pattern.finditer(content):
                    handled_msgs.add(match.group(1))

            except Exception:
                pass

        # Report unhandled messages
        for msg_name, (filepath, line) in msg_types.items():
            if msg_name not in handled_msgs:
                # Skip common/internal messages
                if msg_name in ['tea.Msg', 'Msg']:
                    continue

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
        # Collect field checks and assignments per file
        field_checks: Dict[str, Set[str]] = {}  # "receiver.field" -> set of files
        field_assigns: Dict[str, Set[str]] = {}

        for filepath in go_files:
            try:
                content = filepath.read_text(encoding='utf-8')
                rel_path = str(filepath.relative_to(self.workspace))

                # Find field checks (m.field != nil, m.field == nil)
                for match in self.FIELD_CHECK_PATTERN.finditer(content):
                    receiver = match.group(1)
                    field_name = match.group(2)
                    key = f"{receiver}.{field_name}"
                    if key not in field_checks:
                        field_checks[key] = set()
                    field_checks[key].add(rel_path)

                # Find field assignments (m.field = ...)
                for match in self.FIELD_ASSIGN_PATTERN.finditer(content):
                    receiver = match.group(1)
                    field_name = match.group(2)
                    key = f"{receiver}.{field_name}"
                    if key not in field_assigns:
                        field_assigns[key] = set()
                    field_assigns[key].add(rel_path)

            except Exception:
                pass

        # Find fields that are checked but never assigned
        for key, check_files in field_checks.items():
            # Skip common patterns that are false positives
            if any(x in key for x in ['ctx.', 'err.', 'req.', 'resp.']):
                continue

            if key not in field_assigns:
                # Only report if it looks like a significant field
                _, field_name = key.split('.', 1)
                if len(field_name) > 2 and not field_name.startswith('_'):
                    self.result.findings.append(Finding(
                        severity=Severity.ERROR,
                        message=f"Field '{key}' is nil-checked but never assigned",
                        file=list(check_files)[0],
                        suggestion=f"Add assignment: {key} = NewSomething() somewhere",
                        pattern="field_assignment"
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
            try:
                content = filepath.read_text(encoding='utf-8')
                lines = content.split('\n')

                for pattern, name in blocking_patterns:
                    for match in re.finditer(pattern, content):
                        line_num = content[:match.start()].count('\n') + 1

                        # Check if this is inside a goroutine
                        # Look backwards for 'go func' or 'go methodName'
                        context_start = max(0, match.start() - 500)
                        context = content[context_start:match.start()]

                        # Count 'go func' vs function starts to see if we're in a goroutine
                        go_count = len(re.findall(r'\bgo\s+func\s*\(', context))
                        func_count = len(re.findall(r'\bfunc\s*\(', context))

                        # Also check for methods that return tea.Cmd (Bubbletea pattern)
                        is_tea_cmd = 'tea.Cmd' in content[:match.start()].split('\n')[-20:]

                        if go_count == 0 and not is_tea_cmd:
                            # Not clearly in a goroutine
                            # Check if the function itself is meant to be called with go
                            func_match = re.search(r'func\s+\([^)]+\)\s+(\w+)', content[:match.start()])
                            if func_match:
                                func_name = func_match.group(1)
                                if func_name in ['Run', 'Start', 'Listen', 'Serve']:
                                    continue  # These are expected to be called with go

                            self.result.findings.append(Finding(
                                severity=Severity.INFO,
                                message=f"Potentially blocking call '{name}' not in goroutine",
                                file=str(filepath.relative_to(self.workspace)),
                                line=line_num,
                                suggestion="Consider wrapping in 'go func() { ... }()' if this blocks",
                                pattern="goroutine_spawn"
                            ))

            except Exception:
                pass

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
            try:
                content = filepath.read_text(encoding='utf-8')

                for candidate in store_candidates:
                    pattern = rf'(\w+)\s*:=\s*\w*\.?New{candidate}\s*\('

                    for match in re.finditer(pattern, content):
                        var_name = match.group(1)
                        line_num = content[:match.start()].count('\n') + 1

                        # Check if variable is assigned to a struct field
                        field_assign = rf'\w+\.\w+\s*=\s*{var_name}'
                        return_pattern = rf'return\s+[^;]*{var_name}'

                        if not re.search(field_assign, content) and not re.search(return_pattern, content):
                            # Check the scope - is this in a function that returns quickly?
                            # Find the enclosing function
                            func_start = content.rfind('func ', 0, match.start())
                            if func_start != -1:
                                func_end = content.find('\n}', match.start())
                                if func_end != -1:
                                    func_content = content[func_start:func_end]

                                    # If function is short and returns something else, flag it
                                    if len(func_content.split('\n')) < 30:
                                        # Check if var is used later in the function
                                        usage_after = content[match.end():func_end]
                                        if var_name not in usage_after.replace(f'{var_name} :=', ''):
                                            self.result.findings.append(Finding(
                                                severity=Severity.WARNING,
                                                message=f"{candidate} '{var_name}' created as local var but may be lost when function returns",
                                                file=str(filepath.relative_to(self.workspace)),
                                                line=line_num,
                                                suggestion=f"Consider storing in struct field: m.{var_name[0].lower()}{var_name[1:]} = {var_name}",
                                                pattern="reference_storage"
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
        component=args.component
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
    else:
        success = auditor.print_report()
        sys.exit(0 if success else 1)


if __name__ == "__main__":
    main()
