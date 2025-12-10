#!/usr/bin/env python3
"""
codeNERD Integration Wiring Auditor - Master Orchestrator

Comprehensive audit of all 39+ integration systems in codeNERD.
This script orchestrates specialized auditors and provides a unified report.

Components:
- audit_shards.py   - Shard registration, profiles, injection, lifecycle
- audit_mangle.py   - Schema declarations, policy rules, virtual predicates
- audit_actions.py  - CLI commands, transducer verbs, action handlers
- audit_logging.py  - Logging coverage and category usage

Usage:
    python audit_wiring.py [workspace_path] [--verbose] [--json] [--component X]

Examples:
    python audit_wiring.py                    # Full audit of current directory
    python audit_wiring.py --verbose          # With suggestions
    python audit_wiring.py --component coder  # Focus on coder shard
    python audit_wiring.py --json             # JSON output for tooling
"""

import os
import sys
import argparse
import json
import subprocess
import re
from pathlib import Path
from dataclasses import dataclass, field
from typing import List, Dict, Optional, Any
from enum import Enum
from datetime import datetime

# Import sub-auditors if they exist, otherwise run as subprocess
try:
    from audit_shards import ShardAuditor
    from audit_mangle import MangleAuditor
    from audit_actions import ActionAuditor
    from audit_logging import LoggingAuditor
    from audit_execution import ExecutionAuditor
    IMPORTS_AVAILABLE = True
except ImportError:
    IMPORTS_AVAILABLE = False

class Severity(Enum):
    ERROR = "ERROR"
    WARNING = "WARNING"
    INFO = "INFO"
    OK = "OK"

@dataclass
class Finding:
    severity: Severity
    category: str  # shards, mangle, actions, logging, cross-system
    message: str
    file: Optional[str] = None
    line: Optional[int] = None
    suggestion: Optional[str] = None

@dataclass
class AuditSummary:
    total_errors: int = 0
    total_warnings: int = 0
    total_info: int = 0
    shards_ok: bool = True
    mangle_ok: bool = True
    actions_ok: bool = True
    logging_ok: bool = True
    cross_system_ok: bool = True
    execution_ok: bool = True

@dataclass
class MasterAuditResult:
    timestamp: str = ""
    workspace: str = ""
    findings: List[Finding] = field(default_factory=list)
    summary: AuditSummary = field(default_factory=AuditSummary)
    sub_results: Dict[str, Any] = field(default_factory=dict)

class MasterAuditor:
    def __init__(self, workspace: str, verbose: bool = False, component: Optional[str] = None):
        self.workspace = Path(workspace)
        self.verbose = verbose
        self.component = component
        self.result = MasterAuditResult(
            timestamp=datetime.now().isoformat(),
            workspace=str(self.workspace)
        )
        self.scripts_dir = Path(__file__).parent

    def audit(self) -> MasterAuditResult:
        """Run all audits and aggregate results."""
        print("=" * 70)
        print("codeNERD INTEGRATION WIRING AUDIT")
        print("=" * 70)
        print(f"Workspace: {self.workspace}")
        print(f"Timestamp: {self.result.timestamp}")
        print()

        # Run individual audits
        print("[1/6] Running Shard Registration Audit...")
        self._run_shard_audit()

        print("[2/6] Running Mangle Schema/Policy Audit...")
        self._run_mangle_audit()

        print("[3/6] Running Action Layer Audit...")
        self._run_action_audit()

        print("[4/6] Running Logging Coverage Audit...")
        self._run_logging_audit()

        print("[5/6] Running Cross-System Integration Check...")
        self._run_cross_system_audit()

        print("[6/6] Running Execution Wiring Audit...")
        self._run_execution_audit()

        # Calculate summary
        self._calculate_summary()

        return self.result

    def _run_shard_audit(self):
        """Run shard registration audit."""
        if IMPORTS_AVAILABLE:
            try:
                auditor = ShardAuditor(str(self.workspace), verbose=self.verbose)
                result = auditor.audit()

                # Convert findings
                for shard_name, shard_info in result.shards.items():
                    if self.component and self.component not in shard_name:
                        continue
                    for f in shard_info.findings:
                        self.result.findings.append(Finding(
                            severity=Severity(f.severity.value),
                            category="shards",
                            message=f"{shard_name}: {f.message}",
                            suggestion=f.suggestion
                        ))

                self.result.sub_results['shards'] = result.stats
                self.result.summary.shards_ok = result.stats.get('total_errors', 0) == 0
            except Exception as e:
                self.result.findings.append(Finding(
                    severity=Severity.ERROR,
                    category="shards",
                    message=f"Shard audit failed: {e}"
                ))
                self.result.summary.shards_ok = False
        else:
            self._run_subprocess_audit("audit_shards.py", "shards")

    def _run_mangle_audit(self):
        """Run Mangle schema/policy audit."""
        if IMPORTS_AVAILABLE:
            try:
                auditor = MangleAuditor(str(self.workspace), verbose=self.verbose)
                result = auditor.audit()

                for f in result.findings:
                    if f.severity.value == "INFO" and not self.verbose:
                        continue
                    self.result.findings.append(Finding(
                        severity=Severity(f.severity.value),
                        category="mangle",
                        message=f.message,
                        file=f.file,
                        line=f.line,
                        suggestion=f.suggestion
                    ))

                self.result.sub_results['mangle'] = result.stats
                self.result.summary.mangle_ok = result.stats.get('errors', 0) == 0
            except Exception as e:
                self.result.findings.append(Finding(
                    severity=Severity.ERROR,
                    category="mangle",
                    message=f"Mangle audit failed: {e}"
                ))
                self.result.summary.mangle_ok = False
        else:
            self._run_subprocess_audit("audit_mangle.py", "mangle")

    def _run_action_audit(self):
        """Run action layer audit."""
        if IMPORTS_AVAILABLE:
            try:
                auditor = ActionAuditor(str(self.workspace), verbose=self.verbose)
                result = auditor.audit()

                for f in result.findings:
                    if f.severity.value == "INFO" and not self.verbose:
                        continue
                    self.result.findings.append(Finding(
                        severity=Severity(f.severity.value),
                        category="actions",
                        message=f.message,
                        file=f.file,
                        line=f.line,
                        suggestion=f.suggestion
                    ))

                self.result.sub_results['actions'] = result.stats
                self.result.summary.actions_ok = result.stats.get('errors', 0) == 0
            except Exception as e:
                self.result.findings.append(Finding(
                    severity=Severity.ERROR,
                    category="actions",
                    message=f"Action audit failed: {e}"
                ))
                self.result.summary.actions_ok = False
        else:
            self._run_subprocess_audit("audit_actions.py", "actions")

    def _run_logging_audit(self):
        """Run logging coverage audit."""
        if IMPORTS_AVAILABLE:
            try:
                auditor = LoggingAuditor(str(self.workspace), verbose=self.verbose)
                result = auditor.audit()

                for f in result.findings:
                    if f.severity.value == "INFO" and not self.verbose:
                        continue
                    self.result.findings.append(Finding(
                        severity=Severity(f.severity.value),
                        category="logging",
                        message=f.message,
                        suggestion=f.suggestion
                    ))

                self.result.sub_results['logging'] = result.stats
                self.result.summary.logging_ok = result.stats.get('errors', 0) == 0
            except Exception as e:
                self.result.findings.append(Finding(
                    severity=Severity.ERROR,
                    category="logging",
                    message=f"Logging audit failed: {e}"
                ))
                self.result.summary.logging_ok = False
        else:
            self._run_subprocess_audit("audit_logging.py", "logging")

    def _run_execution_audit(self):
        """Run execution wiring audit."""
        if IMPORTS_AVAILABLE:
            try:
                auditor = ExecutionAuditor(str(self.workspace), verbose=self.verbose, component=self.component)
                result = auditor.audit()

                for f in result.findings:
                    if f.severity.value == "INFO" and not self.verbose:
                        continue
                    self.result.findings.append(Finding(
                        severity=Severity(f.severity.value),
                        category="execution",
                        message=f.message,
                        file=f.file,
                        line=f.line,
                        suggestion=f.suggestion
                    ))

                self.result.sub_results['execution'] = result.stats
                self.result.summary.execution_ok = result.stats.get('errors', 0) == 0
            except Exception as e:
                self.result.findings.append(Finding(
                    severity=Severity.ERROR,
                    category="execution",
                    message=f"Execution audit failed: {e}"
                ))
                self.result.summary.execution_ok = False
        else:
            self._run_subprocess_audit("audit_execution.py", "execution")

    def _run_subprocess_audit(self, script_name: str, category: str):
        """Run an audit script as subprocess and parse JSON output."""
        script_path = self.scripts_dir / script_name
        if not script_path.exists():
            self.result.findings.append(Finding(
                severity=Severity.WARNING,
                category=category,
                message=f"Audit script {script_name} not found"
            ))
            return

        try:
            result = subprocess.run(
                [sys.executable, str(script_path), str(self.workspace), "--json"],
                capture_output=True,
                text=True,
                timeout=120
            )

            if result.returncode == 0 and result.stdout:
                data = json.loads(result.stdout)
                self.result.sub_results[category] = data.get('stats', {})

                for f in data.get('findings', []):
                    sev = f.get('severity', 'INFO')
                    if sev == "INFO" and not self.verbose:
                        continue
                    self.result.findings.append(Finding(
                        severity=Severity(sev),
                        category=category,
                        message=f.get('message', '')
                    ))

                # Determine OK status based on error count
                stats = data.get('stats', {})
                error_count = stats.get('errors', stats.get('total_errors', 0))
                setattr(self.result.summary, f'{category}_ok', error_count == 0)
            else:
                self.result.findings.append(Finding(
                    severity=Severity.WARNING,
                    category=category,
                    message=f"{script_name} returned non-zero or no output"
                ))
        except Exception as e:
            self.result.findings.append(Finding(
                severity=Severity.ERROR,
                category=category,
                message=f"Failed to run {script_name}: {e}"
            ))

    def _run_cross_system_audit(self):
        """Check cross-system integration points."""
        findings = []

        # Check 1: Verify boot sequence dependencies
        factory_file = self.workspace / "internal" / "system" / "factory.go"
        if factory_file.exists():
            try:
                content = factory_file.read_text(encoding='utf-8')

                # Check kernel boot before queries
                if 'NewRealKernel()' in content:
                    kernel_pos = content.find('NewRealKernel()')
                    evaluate_pos = content.find('kernel.Evaluate()')

                    if evaluate_pos != -1 and evaluate_pos < kernel_pos:
                        findings.append(Finding(
                            severity=Severity.ERROR,
                            category="cross-system",
                            message="Kernel.Evaluate() called before NewRealKernel()",
                            file=str(factory_file),
                            suggestion="Ensure kernel is created and Evaluate() called before any queries"
                        ))

                # Check VirtualStore has kernel reference
                if 'NewVirtualStore' in content and 'SetKernel' not in content:
                    findings.append(Finding(
                        severity=Severity.WARNING,
                        category="cross-system",
                        message="VirtualStore may not have kernel reference set",
                        file=str(factory_file),
                        suggestion="Add virtualStore.SetKernel(kernel) after creation"
                    ))
            except Exception:
                pass

        # Check 2: Verify transducer -> shard routing
        trans_file = self.workspace / "internal" / "perception" / "transducer.go"
        reg_file = self.workspace / "internal" / "shards" / "registration.go"

        if trans_file.exists() and reg_file.exists():
            try:
                trans_content = trans_file.read_text(encoding='utf-8')
                reg_content = reg_file.read_text(encoding='utf-8')

                # Extract shard types from transducer
                shard_types_in_trans = set()
                for match in re.finditer(r'ShardType:\s*"(\w+)"', trans_content):
                    shard_types_in_trans.add(match.group(1))

                # Extract registered shards
                registered_shards = set()
                for match in re.finditer(r'RegisterShard\s*\(\s*"(\w+)"', reg_content):
                    registered_shards.add(match.group(1))

                # Check for mismatches
                unregistered = shard_types_in_trans - registered_shards
                for shard in unregistered:
                    findings.append(Finding(
                        severity=Severity.ERROR,
                        category="cross-system",
                        message=f"Transducer routes to shard '{shard}' but it's not registered",
                        suggestion=f"Add sm.RegisterShard(\"{shard}\", ...) to registration.go"
                    ))
            except Exception:
                pass

        # Check 3: Verify config defaults exist
        config_paths = [
            self.workspace / "internal" / "config" / "config.go",
            self.workspace / "internal" / "core" / "config.go",
        ]
        for config_file in config_paths:
            if config_file.exists():
                try:
                    content = config_file.read_text(encoding='utf-8')
                    if 'DefaultConfig' in content:
                        # Check for essential fields
                        essential_fields = ['APIKey', 'Model', 'Workspace']
                        for f in essential_fields:
                            if f not in content:
                                findings.append(Finding(
                                    severity=Severity.INFO,
                                    category="cross-system",
                                    message=f"Config may be missing '{f}' field",
                                    file=str(config_file)
                                ))
                except Exception:
                    pass
                break

        # Check 4: Verify autopoiesis integration
        autopoiesis_dir = self.workspace / "internal" / "autopoiesis"
        if autopoiesis_dir.exists():
            # Check Ouroboros -> ToolGenerator wiring
            ouroboros_file = autopoiesis_dir / "ouroboros.go"
            if ouroboros_file.exists():
                try:
                    content = ouroboros_file.read_text(encoding='utf-8')
                    if 'ToolGenerator' not in content and 'tool_generator' not in content:
                        findings.append(Finding(
                            severity=Severity.INFO,
                            category="cross-system",
                            message="Ouroboros may not be wired to ToolGenerator shard",
                            file=str(ouroboros_file),
                            suggestion="Verify Ouroboros routes to tool_generator for self-tool creation"
                        ))
                except Exception:
                    pass

        # Add findings
        self.result.findings.extend(findings)
        self.result.summary.cross_system_ok = not any(
            f.severity == Severity.ERROR for f in findings
        )

    def _calculate_summary(self):
        """Calculate aggregate summary."""
        self.result.summary.total_errors = sum(
            1 for f in self.result.findings if f.severity == Severity.ERROR
        )
        self.result.summary.total_warnings = sum(
            1 for f in self.result.findings if f.severity == Severity.WARNING
        )
        self.result.summary.total_info = sum(
            1 for f in self.result.findings if f.severity == Severity.INFO
        )

    def print_report(self) -> bool:
        """Print formatted master audit report."""
        print()
        print("=" * 70)
        print("MASTER AUDIT SUMMARY")
        print("=" * 70)
        print()

        # Overall status
        all_ok = (
            self.result.summary.shards_ok and
            self.result.summary.mangle_ok and
            self.result.summary.actions_ok and
            self.result.summary.logging_ok and
            self.result.summary.cross_system_ok and
            self.result.summary.execution_ok
        )

        status = "PASS" if all_ok else "FAIL"
        print(f"Overall Status: {status}")
        print()

        # Component status
        print("Component Status:")
        print(f"  Shards:       {'OK' if self.result.summary.shards_ok else 'ERRORS'}")
        print(f"  Mangle:       {'OK' if self.result.summary.mangle_ok else 'ERRORS'}")
        print(f"  Actions:      {'OK' if self.result.summary.actions_ok else 'ERRORS'}")
        print(f"  Logging:      {'OK' if self.result.summary.logging_ok else 'ERRORS'}")
        print(f"  Cross-System: {'OK' if self.result.summary.cross_system_ok else 'ERRORS'}")
        print(f"  Execution:    {'OK' if self.result.summary.execution_ok else 'ERRORS'}")
        print()

        # Counts
        print(f"Total Findings:")
        print(f"  Errors:   {self.result.summary.total_errors}")
        print(f"  Warnings: {self.result.summary.total_warnings}")
        print(f"  Info:     {self.result.summary.total_info}")
        print()

        # Errors (always show)
        errors = [f for f in self.result.findings if f.severity == Severity.ERROR]
        if errors:
            print("-" * 70)
            print("ERRORS (Must Fix)")
            print("-" * 70)
            for f in errors:
                print(f"[{f.category.upper()}] {f.message}")
                if self.verbose and f.suggestion:
                    print(f"           -> {f.suggestion}")
            print()

        # Warnings
        warnings = [f for f in self.result.findings if f.severity == Severity.WARNING]
        if warnings:
            print("-" * 70)
            print("WARNINGS")
            print("-" * 70)
            for f in warnings[:20]:  # Limit to first 20
                print(f"[{f.category.upper()}] {f.message}")
                if self.verbose and f.suggestion:
                    print(f"            -> {f.suggestion}")
            if len(warnings) > 20:
                print(f"  ... and {len(warnings) - 20} more warnings")
            print()

        # Info (verbose only)
        if self.verbose:
            infos = [f for f in self.result.findings if f.severity == Severity.INFO]
            if infos[:10]:
                print("-" * 70)
                print("INFO (showing first 10)")
                print("-" * 70)
                for f in infos[:10]:
                    print(f"[{f.category.upper()}] {f.message}")
                if len(infos) > 10:
                    print(f"  ... and {len(infos) - 10} more info items")
                print()

        # Sub-audit stats
        if self.result.sub_results:
            print("-" * 70)
            print("DETAILED STATS")
            print("-" * 70)
            for category, stats in self.result.sub_results.items():
                if isinstance(stats, dict):
                    print(f"\n{category.upper()}:")
                    for key, value in stats.items():
                        print(f"  {key}: {value}")
            print()

        print("=" * 70)

        # Recommendations
        if not all_ok:
            print()
            print("RECOMMENDED ACTIONS:")
            if not self.result.summary.shards_ok:
                print("  1. Fix shard registration issues in internal/shards/registration.go")
            if not self.result.summary.mangle_ok:
                print("  2. Add missing Decl statements to internal/core/defaults/schemas.mg")
            if not self.result.summary.actions_ok:
                print("  3. Add missing action handlers in internal/core/virtual_store.go")
            if not self.result.summary.cross_system_ok:
                print("  4. Check boot sequence and cross-component wiring")
            if not self.result.summary.execution_ok:
                print("  5. Fix execution wiring: ensure Run() called, channels listened, goroutines spawned")
            print()
            print("Run with --verbose for detailed suggestions.")
            print()

        return all_ok


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
        description="codeNERD Integration Wiring Audit",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Examples:
  python audit_wiring.py                    # Full audit
  python audit_wiring.py --verbose          # With suggestions
  python audit_wiring.py --component coder  # Focus on coder
  python audit_wiring.py --json             # JSON for tooling
"""
    )
    parser.add_argument("workspace", nargs="?", default=".", help="Workspace path")
    parser.add_argument("--verbose", "-v", action="store_true", help="Show detailed suggestions")
    parser.add_argument("--json", action="store_true", help="Output as JSON")
    parser.add_argument("--component", "-c", help="Focus on specific component")

    args = parser.parse_args()

    workspace = find_workspace(args.workspace)
    auditor = MasterAuditor(
        str(workspace),
        verbose=args.verbose,
        component=args.component
    )
    result = auditor.audit()

    if args.json:
        output = {
            "timestamp": result.timestamp,
            "workspace": result.workspace,
            "summary": {
                "total_errors": result.summary.total_errors,
                "total_warnings": result.summary.total_warnings,
                "total_info": result.summary.total_info,
                "shards_ok": result.summary.shards_ok,
                "mangle_ok": result.summary.mangle_ok,
                "actions_ok": result.summary.actions_ok,
                "logging_ok": result.summary.logging_ok,
                "cross_system_ok": result.summary.cross_system_ok,
                "execution_ok": result.summary.execution_ok,
            },
            "findings": [
                {
                    "severity": f.severity.value,
                    "category": f.category,
                    "message": f.message,
                    "file": f.file,
                    "line": f.line,
                    "suggestion": f.suggestion,
                }
                for f in result.findings
            ],
            "sub_results": result.sub_results,
        }
        print(json.dumps(output, indent=2))
    else:
        success = auditor.print_report()
        sys.exit(0 if success else 1)


if __name__ == "__main__":
    main()
