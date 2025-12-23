#!/usr/bin/env python3
"""
Action Layer Auditor for codeNERD

Deep audit of action wiring:
- CLI commands in commands.go
- Transducer verb corpus
- VirtualStore action handlers
- Permission checks

Usage:
    python audit_actions.py [workspace_path] [--verbose] [--json]
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
class CLICommand:
    name: str
    handler_line: Optional[int] = None
    routes_to_shard: Optional[str] = None
    has_help_text: bool = False

@dataclass
class TransducerVerb:
    verb: str
    shard_type: Optional[str] = None
    synonyms: List[str] = field(default_factory=list)
    line: Optional[int] = None

@dataclass
class ActionType:
    name: str
    const_line: Optional[int] = None
    handler_line: Optional[int] = None
    has_permission_check: bool = False

@dataclass
class AuditResult:
    cli_commands: Dict[str, CLICommand] = field(default_factory=dict)
    transducer_verbs: Dict[str, TransducerVerb] = field(default_factory=dict)
    action_types: Dict[str, ActionType] = field(default_factory=dict)
    action_handlers: Set[str] = field(default_factory=set)
    findings: List[Finding] = field(default_factory=list)
    stats: Dict[str, int] = field(default_factory=dict)

class ActionAuditor:
    def __init__(self, workspace: str, verbose: bool = False):
        self.workspace = Path(workspace)
        self.verbose = verbose
        self.result = AuditResult()

    def audit(self) -> AuditResult:
        """Run complete action layer audit."""
        print(f"[*] Auditing action layer in {self.workspace}")
        print()

        # Phase 1: Scan CLI commands
        self._scan_cli_commands()

        # Phase 2: Scan transducer verb corpus
        self._scan_transducer()

        # Phase 3: Scan action type constants
        self._scan_action_constants()

        # Phase 4: Scan VirtualStore action handlers
        self._scan_action_handlers()

        # Phase 5: Cross-reference and find gaps
        self._check_completeness()

        # Calculate stats
        self._calculate_stats()

        return self.result

    def _scan_cli_commands(self):
        """Scan CLI command handlers."""
        cmd_paths = [
            self.workspace / "cmd" / "nerd" / "chat" / "commands.go",
            self.workspace / "cmd" / "nerd" / "commands.go",
        ]

        for cmd_file in cmd_paths:
            if not cmd_file.exists():
                continue

            content = cmd_file.read_text(encoding='utf-8')
            lines = content.split('\n')

            # Find handleCommand or similar switch statements
            in_switch = False
            current_command = None

            for i, line in enumerate(lines, 1):
                # Look for case statements with command strings
                case_match = re.search(r'case\s+"(/?\w+)"', line)
                if case_match:
                    cmd_name = case_match.group(1)
                    if not cmd_name.startswith('/'):
                        cmd_name = '/' + cmd_name

                    self.result.cli_commands[cmd_name] = CLICommand(
                        name=cmd_name,
                        handler_line=i
                    )
                    current_command = cmd_name

                # Look for shard spawn calls to determine routing
                if current_command:
                    shard_match = re.search(r'(?:Spawn|SpawnShard)\s*\(\s*[^,]*,?\s*"(\w+)"', line)
                    if shard_match:
                        self.result.cli_commands[current_command].routes_to_shard = shard_match.group(1)

            # Scan for help text
            help_pattern = r'(?:help|usage|description).*"(/?\w+)"'
            for match in re.finditer(help_pattern, content, re.IGNORECASE):
                cmd_name = match.group(1)
                if cmd_name in self.result.cli_commands:
                    self.result.cli_commands[cmd_name].has_help_text = True

    def _scan_transducer(self):
        """Scan transducer verb corpus."""
        trans_paths = [
            self.workspace / "internal" / "perception" / "transducer.go",
            self.workspace / "internal" / "perception" / "verbs.go",
        ]

        for trans_file in trans_paths:
            if not trans_file.exists():
                continue

            content = trans_file.read_text(encoding='utf-8')
            lines = content.split('\n')

            # Look for VerbCorpus initialization or VerbEntry definitions
            # Pattern: {Verb: "...", ShardType: "...", Synonyms: []string{...}}
            verb_pattern = r'Verb:\s*"(\w+)"'
            shard_pattern = r'ShardType:\s*"(\w+)"'
            synonyms_pattern = r'Synonyms:\s*\[\]string\{([^}]*)\}'

            in_verb_entry = False
            current_verb = None
            current_line = 0

            for i, line in enumerate(lines, 1):
                verb_match = re.search(verb_pattern, line)
                if verb_match:
                    verb = verb_match.group(1)
                    self.result.transducer_verbs[verb] = TransducerVerb(verb=verb, line=i)
                    current_verb = verb
                    current_line = i

                if current_verb:
                    shard_match = re.search(shard_pattern, line)
                    if shard_match:
                        self.result.transducer_verbs[current_verb].shard_type = shard_match.group(1)

                    synonyms_match = re.search(synonyms_pattern, line)
                    if synonyms_match:
                        synonyms_str = synonyms_match.group(1)
                        synonyms = [s.strip().strip('"') for s in synonyms_str.split(',') if s.strip()]
                        self.result.transducer_verbs[current_verb].synonyms = synonyms

            # Also check for verb constants
            const_pattern = r'Verb(\w+)\s*=\s*"(\w+)"'
            for match in re.finditer(const_pattern, content):
                verb_name = match.group(2)
                if verb_name not in self.result.transducer_verbs:
                    self.result.transducer_verbs[verb_name] = TransducerVerb(verb=verb_name)

    def _scan_action_constants(self):
        """Scan for action type constants."""
        # Check virtual_store.go and action-related files
        action_paths = [
            self.workspace / "internal" / "core" / "virtual_store.go",
            self.workspace / "internal" / "core" / "actions.go",
            self.workspace / "internal" / "tactile" / "actions.go",
        ]

        for action_file in action_paths:
            if not action_file.exists():
                continue

            content = action_file.read_text(encoding='utf-8')
            lines = content.split('\n')

            # Look for action constant definitions
            # Pattern: ActionXxx = "xxx" or ActionXxx ActionType = "xxx"
            const_pattern = r'(Action\w+)\s*(?:ActionType)?\s*=\s*"([^"]+)"'

            for i, line in enumerate(lines, 1):
                match = re.search(const_pattern, line)
                if match:
                    const_name = match.group(1)
                    action_name = match.group(2)

                    if action_name not in self.result.action_types:
                        self.result.action_types[action_name] = ActionType(name=action_name)
                    self.result.action_types[action_name].const_line = i

                    # Store mapping from const name to action name for handler matching
                    if not hasattr(self.result, 'const_to_action'):
                        self.result.const_to_action = {}
                    self.result.const_to_action[const_name] = action_name

    def _scan_action_handlers(self):
        """Scan VirtualStore Execute for action handlers."""
        vs_file = self.workspace / "internal" / "core" / "virtual_store.go"
        if not vs_file.exists():
            self.result.findings.append(Finding(
                severity=Severity.WARNING,
                message="virtual_store.go not found - cannot audit action handlers"
            ))
            return

        content = vs_file.read_text(encoding='utf-8')
        lines = content.split('\n')

        # Find Execute method and extract handled actions
        in_execute = False
        brace_count = 0

        for i, line in enumerate(lines, 1):
            # Look for executeAction method (the actual action handler)
            if 'func (v *VirtualStore) executeAction(' in line or 'func (vs *VirtualStore) executeAction(' in line:
                in_execute = True
                continue

            if in_execute:
                brace_count += line.count('{') - line.count('}')

                # Look for case statements in switch
                case_match = re.search(r'case\s+(Action\w+)', line)
                if case_match:
                    const_name = case_match.group(1)

                    # Use the const-to-action mapping if available
                    const_to_action = getattr(self.result, 'const_to_action', {})
                    if const_name in const_to_action:
                        action_name = const_to_action[const_name]
                    else:
                        # Fallback: convert CamelCase to snake_case
                        action_name = const_name[6:] if const_name.startswith('Action') else const_name
                        action_name = re.sub(r'([a-z0-9])([A-Z])', r'\1_\2', action_name).lower()

                    self.result.action_handlers.add(action_name)

                    # Also look for the actual string value
                    str_case = re.search(r'case\s+"(\w+)"', line)
                    if str_case:
                        self.result.action_handlers.add(str_case.group(1))

                # Check for permission checks
                if 'checkPermission' in line or 'HasPermission' in line:
                    # Try to associate with current action
                    pass

                # Exit when method ends
                if brace_count <= 0 and in_execute and i > 10:
                    in_execute = False

    def _check_completeness(self):
        """Cross-reference and find gaps."""
        # Check CLI commands route to valid shards
        for cmd_name, cmd in self.result.cli_commands.items():
            if not cmd.routes_to_shard:
                # Some commands might not need a shard (like /help)
                if cmd_name not in ['/help', '/quit', '/exit', '/clear', '/status']:
                    self.result.findings.append(Finding(
                        severity=Severity.INFO,
                        message=f"CLI command '{cmd_name}' doesn't explicitly spawn a shard",
                        line=cmd.handler_line,
                        suggestion="Verify command implementation routes to appropriate handler"
                    ))

        # Check transducer verbs map to shards
        for verb, verb_info in self.result.transducer_verbs.items():
            if not verb_info.shard_type:
                self.result.findings.append(Finding(
                    severity=Severity.WARNING,
                    message=f"Transducer verb '{verb}' has no ShardType mapping",
                    line=verb_info.line,
                    suggestion=f"Add ShardType: \"xxx\" to verb entry"
                ))

        # Check action types have handlers
        for action_name, action in self.result.action_types.items():
            action_lower = action_name.lower()
            has_handler = (
                action_lower in self.result.action_handlers or
                action_name in self.result.action_handlers
            )

            if not has_handler:
                self.result.findings.append(Finding(
                    severity=Severity.ERROR,
                    message=f"Action type '{action_name}' defined but no handler in Execute()",
                    line=action.const_line,
                    suggestion=f'Add case Action{action_name}: handler in VirtualStore.Execute()'
                ))

        # Check for handlers without constants (potential dead code or magic strings)
        known_actions = {a.lower() for a in self.result.action_types}
        for handler in self.result.action_handlers:
            if handler.lower() not in known_actions:
                # Could be using inline string
                self.result.findings.append(Finding(
                    severity=Severity.INFO,
                    message=f"Handler for '{handler}' uses inline string instead of constant",
                    suggestion=f"Define Action{handler.title()} constant for better maintainability"
                ))

    def _calculate_stats(self):
        """Calculate audit statistics."""
        errors = sum(1 for f in self.result.findings if f.severity == Severity.ERROR)
        warnings = sum(1 for f in self.result.findings if f.severity == Severity.WARNING)

        self.result.stats = {
            "cli_commands": len(self.result.cli_commands),
            "transducer_verbs": len(self.result.transducer_verbs),
            "action_types": len(self.result.action_types),
            "action_handlers": len(self.result.action_handlers),
            "errors": errors,
            "warnings": warnings,
        }

    def print_report(self) -> bool:
        """Print formatted audit report."""
        print("=" * 70)
        print("ACTION LAYER AUDIT REPORT")
        print("=" * 70)
        print()

        # Summary
        print(f"CLI Commands:      {self.result.stats['cli_commands']}")
        print(f"Transducer Verbs:  {self.result.stats['transducer_verbs']}")
        print(f"Action Types:      {self.result.stats['action_types']}")
        print(f"Action Handlers:   {self.result.stats['action_handlers']}")
        print()
        print(f"Errors: {self.result.stats['errors']}  Warnings: {self.result.stats['warnings']}")
        print()

        # CLI Commands
        if self.result.cli_commands:
            print("-" * 70)
            print("CLI COMMANDS")
            print("-" * 70)
            for name, cmd in sorted(self.result.cli_commands.items()):
                shard = cmd.routes_to_shard or "N/A"
                print(f"  {name:20} -> {shard}")
            print()

        # Transducer Verbs
        if self.result.transducer_verbs and self.verbose:
            print("-" * 70)
            print("TRANSDUCER VERBS")
            print("-" * 70)
            for verb, info in sorted(self.result.transducer_verbs.items()):
                shard = info.shard_type or "?"
                synonyms = ", ".join(info.synonyms) if info.synonyms else "none"
                print(f"  {verb:15} -> {shard:15} (synonyms: {synonyms})")
            print()

        # Errors
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

        # Info (verbose only)
        if self.verbose:
            infos = [f for f in self.result.findings if f.severity == Severity.INFO]
            if infos:
                print("-" * 70)
                print("INFO")
                print("-" * 70)
                for f in infos:
                    print(f"[INFO] {f.message}")
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
    parser = argparse.ArgumentParser(description="Audit codeNERD action layer")
    parser.add_argument("workspace", nargs="?", default=".", help="Workspace path")
    parser.add_argument("--verbose", "-v", action="store_true", help="Show all findings")
    parser.add_argument("--json", action="store_true", help="Output as JSON")

    args = parser.parse_args()

    workspace = find_workspace(args.workspace)
    auditor = ActionAuditor(str(workspace), verbose=args.verbose)
    result = auditor.audit()

    if args.json:
        output = {
            "cli_commands": {
                name: {
                    "routes_to_shard": cmd.routes_to_shard,
                    "has_help": cmd.has_help_text,
                }
                for name, cmd in result.cli_commands.items()
            },
            "transducer_verbs": {
                verb: {
                    "shard_type": info.shard_type,
                    "synonyms": info.synonyms,
                }
                for verb, info in result.transducer_verbs.items()
            },
            "action_types": list(result.action_types.keys()),
            "action_handlers": list(result.action_handlers),
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
