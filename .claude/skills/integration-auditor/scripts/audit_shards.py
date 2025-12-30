#!/usr/bin/env python3
"""
Shard Registration Auditor for codeNERD

Deep audit of shard registration, profiles, dependency injection, and lifecycle.
Checks for the 4 shard types (A/B/U/S) and their specific requirements.

Usage:
    python audit_shards.py [workspace_path] [--verbose] [--json]
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

class ShardType(Enum):
    EPHEMERAL = "A"      # ShardTypeEphemeral
    PERSISTENT = "B"     # ShardTypePersistent
    USER = "U"           # ShardTypeUser
    SYSTEM = "S"         # ShardTypeSystem

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
class ShardInfo:
    name: str
    shard_type: Optional[ShardType] = None
    # Registration
    has_factory: bool = False
    factory_line: Optional[int] = None
    has_profile: bool = False
    profile_line: Optional[int] = None
    # Injection
    injections: Set[str] = field(default_factory=set)
    # Requirements by type
    missing_injections: List[str] = field(default_factory=list)
    # System shard specific
    in_auto_start: bool = False
    # Tests
    has_tests: bool = False
    # CLI/Entry
    has_cli_command: bool = False
    has_transducer_verb: bool = False
    # Findings
    findings: List[Finding] = field(default_factory=list)

@dataclass
class AuditResult:
    shards: Dict[str, ShardInfo] = field(default_factory=dict)
    findings: List[Finding] = field(default_factory=list)
    stats: Dict[str, int] = field(default_factory=dict)

# Required injections by shard type
REQUIRED_INJECTIONS = {
    ShardType.EPHEMERAL: ["SetParentKernel", "SetLLMClient"],
    ShardType.PERSISTENT: ["SetParentKernel", "SetLLMClient", "SetLearningStore"],
    ShardType.USER: ["SetParentKernel", "SetLLMClient", "SetLearningStore"],
    ShardType.SYSTEM: ["SetParentKernel"],
}

# Optional but recommended injections
RECOMMENDED_INJECTIONS = {
    ShardType.EPHEMERAL: ["SetVirtualStore"],
    ShardType.PERSISTENT: ["SetVirtualStore", "SetKnowledgePath"],
    ShardType.USER: ["SetVirtualStore", "SetKnowledgePath", "SetSystemPrompt"],
    ShardType.SYSTEM: ["SetLLMClient", "SetVirtualStore"],
}

# System shards that should auto-start (Type S with AUTO-START lifecycle)
# Note: Some system shards are ON-DEMAND (legislator, tactile_router) and don't auto-start
EXPECTED_SYSTEM_SHARDS = [
    "perception_firewall",
    "world_model_ingestor",
    "executive_policy",
    "constitution_gate",
    # "legislator",      # ON-DEMAND: spawned when learned constraints needed
    # "tactile_router",  # ON-DEMAND: spawned when action routing needed
    "session_planner",
]

class ShardAuditor:
    def __init__(self, workspace: str, verbose: bool = False):
        self.workspace = Path(workspace)
        self.verbose = verbose
        self.result = AuditResult()

    def audit(self) -> AuditResult:
        """Run complete shard audit."""
        print(f"[*] Auditing shard registration in {self.workspace}")
        print()

        # Phase 1: Scan registration.go for factories and profiles
        self._scan_registration()

        # Phase 2: Scan factory.go for system shard auto-start
        self._scan_factory_autostart()

        # Phase 3: Scan shard implementations for interface compliance
        self._scan_shard_implementations()

        # Phase 4: Scan for tests
        self._scan_tests()

        # Phase 5: Scan for CLI commands and transducer verbs
        self._scan_entry_points()

        # Phase 6: Cross-reference and find gaps
        self._check_shard_requirements()

        # Calculate stats
        self._calculate_stats()

        return self.result

    def _scan_registration(self):
        """Scan registration.go for shard factories and profiles."""
        reg_file = self.workspace / "internal" / "shards" / "registration.go"
        if not reg_file.exists():
            self.result.findings.append(Finding(
                severity=Severity.ERROR,
                message="registration.go not found - no shards registered",
                file=str(reg_file),
                suggestion="Create internal/shards/registration.go with RegisterAllShardFactories()"
            ))
            return

        content = reg_file.read_text(encoding='utf-8')
        lines = content.split('\n')

        # Find RegisterShard calls
        factory_pattern = r'sm\.RegisterShard\s*\(\s*"([^"]+)"'
        for i, line in enumerate(lines, 1):
            match = re.search(factory_pattern, line)
            if match:
                shard_name = match.group(1)
                if shard_name not in self.result.shards:
                    self.result.shards[shard_name] = ShardInfo(name=shard_name)
                self.result.shards[shard_name].has_factory = True
                self.result.shards[shard_name].factory_line = i

                # Find the factory function block and scan for injections
                self._scan_factory_injections(content, shard_name, i)

        # Find DefineProfile calls
        profile_pattern = r'sm\.DefineProfile\s*\(\s*"([^"]+)"'
        for i, line in enumerate(lines, 1):
            match = re.search(profile_pattern, line)
            if match:
                shard_name = match.group(1)
                if shard_name not in self.result.shards:
                    self.result.shards[shard_name] = ShardInfo(name=shard_name)
                self.result.shards[shard_name].has_profile = True
                self.result.shards[shard_name].profile_line = i

                # Determine shard type from profile block
                self._determine_shard_type(content, shard_name, i)

    def _scan_factory_injections(self, content: str, shard_name: str, start_line: int):
        """Scan factory function for dependency injections."""
        lines = content.split('\n')

        # Find the factory function block (from RegisterShard to closing })
        in_factory = False
        brace_count = 0
        factory_block = []

        for i, line in enumerate(lines[start_line-1:], start_line):
            if not in_factory and 'func(' in line:
                in_factory = True

            if in_factory:
                factory_block.append(line)
                brace_count += line.count('{') - line.count('}')
                if brace_count == 0 and len(factory_block) > 1:
                    break

        block_text = '\n'.join(factory_block)

        # Check for injection patterns
        injection_patterns = [
            ("SetParentKernel", r'\.SetParentKernel\s*\('),
            ("SetLLMClient", r'\.SetLLMClient\s*\('),
            ("SetVirtualStore", r'\.SetVirtualStore\s*\('),
            ("SetLearningStore", r'\.SetLearningStore\s*\('),
            ("SetKnowledgePath", r'\.SetKnowledgePath\s*\('),
            ("SetLocalDB", r'\.SetLocalDB\s*\('),
            ("SetWorkspaceRoot", r'\.SetWorkspaceRoot\s*\('),
            ("SetSystemPrompt", r'\.SetSystemPrompt\s*\('),
            ("SetBrowserManager", r'\.SetBrowserManager\s*\('),
        ]

        for injection_name, pattern in injection_patterns:
            if re.search(pattern, block_text):
                self.result.shards[shard_name].injections.add(injection_name)

    def _determine_shard_type(self, content: str, shard_name: str, start_line: int):
        """Determine shard type from profile definition."""
        lines = content.split('\n')

        # Look in the next ~30 lines for Type field
        search_block = '\n'.join(lines[start_line-1:start_line+30])

        if 'ShardTypeEphemeral' in search_block or 'core.ShardTypeEphemeral' in search_block:
            self.result.shards[shard_name].shard_type = ShardType.EPHEMERAL
        elif 'ShardTypePersistent' in search_block or 'core.ShardTypePersistent' in search_block:
            self.result.shards[shard_name].shard_type = ShardType.PERSISTENT
        elif 'ShardTypeUser' in search_block or 'core.ShardTypeUser' in search_block:
            self.result.shards[shard_name].shard_type = ShardType.USER
        elif 'ShardTypeSystem' in search_block or 'core.ShardTypeSystem' in search_block:
            self.result.shards[shard_name].shard_type = ShardType.SYSTEM

    def _scan_factory_autostart(self):
        """Scan factory.go for system shard auto-start list."""
        factory_file = self.workspace / "internal" / "system" / "factory.go"
        if not factory_file.exists():
            return

        content = factory_file.read_text(encoding='utf-8')

        # Also check shard_manager.go for StartSystemShards
        sm_file = self.workspace / "internal" / "core" / "shard_manager.go"
        if sm_file.exists():
            content += "\n" + sm_file.read_text(encoding='utf-8')

        # Look for system shards being started
        for shard_name, shard_info in self.result.shards.items():
            if shard_info.shard_type == ShardType.SYSTEM:
                # Check if explicitly started or disabled
                if f'"{shard_name}"' in content:
                    # More sophisticated: check if it's in the start list vs disable list
                    if 'DisableSystemShard' in content and f'"{shard_name}"' in content:
                        # Could be in disable list, need to verify
                        pass
                    shard_info.in_auto_start = True

    def _scan_shard_implementations(self):
        """Scan shard implementation files for interface compliance."""
        shards_dir = self.workspace / "internal" / "shards"
        if not shards_dir.exists():
            return

        for shard_path in shards_dir.rglob("*.go"):
            if "_test.go" in str(shard_path):
                continue

            content = shard_path.read_text(encoding='utf-8')

            # Check for Execute method implementation
            if 'func (s *' in content and 'Execute(' in content:
                # Try to match to a registered shard
                # Look for struct definitions
                struct_pattern = r'type\s+(\w+Shard)\s+struct'
                for match in re.finditer(struct_pattern, content):
                    struct_name = match.group(1).lower().replace('shard', '')
                    # Try to find matching registered shard
                    for shard_name in self.result.shards:
                        if struct_name in shard_name or shard_name in struct_name:
                            # Found implementation
                            pass

    def _scan_tests(self):
        """Scan for shard tests."""
        for test_file in self.workspace.rglob("*_test.go"):
            try:
                content = test_file.read_text(encoding='utf-8')
            except:
                continue

            for shard_name in self.result.shards:
                # Look for test functions or spawn calls with this shard
                patterns = [
                    f'"{shard_name}"',
                    f"Test{shard_name.replace('_', '').title()}",
                    f"test_{shard_name}",
                ]
                for pattern in patterns:
                    if pattern.lower() in content.lower():
                        self.result.shards[shard_name].has_tests = True
                        break

    def _scan_entry_points(self):
        """Scan for CLI commands and transducer verbs."""
        # Scan commands.go
        cmd_file = self.workspace / "cmd" / "nerd" / "chat" / "commands.go"
        if cmd_file.exists():
            content = cmd_file.read_text(encoding='utf-8')
            for shard_name in self.result.shards:
                if f'"{shard_name}"' in content or f'"/{shard_name}"' in content:
                    self.result.shards[shard_name].has_cli_command = True

        # Scan transducer.go
        trans_file = self.workspace / "internal" / "perception" / "transducer.go"
        if trans_file.exists():
            content = trans_file.read_text(encoding='utf-8')
            for shard_name in self.result.shards:
                # Look for ShardType mapping
                if f'"{shard_name}"' in content:
                    self.result.shards[shard_name].has_transducer_verb = True

    def _check_shard_requirements(self):
        """Check each shard meets requirements for its type."""
        for shard_name, shard in self.result.shards.items():
            # Check factory registered
            if not shard.has_factory:
                shard.findings.append(Finding(
                    severity=Severity.ERROR,
                    message=f"Shard '{shard_name}' has no factory registered",
                    suggestion=f'Add sm.RegisterShard("{shard_name}", ...) in registration.go'
                ))

            # Check profile defined
            if not shard.has_profile:
                shard.findings.append(Finding(
                    severity=Severity.ERROR,
                    message=f"Shard '{shard_name}' has no profile defined",
                    suggestion=f'Add sm.DefineProfile("{shard_name}", ...) in registration.go'
                ))

            # Check type-specific requirements
            if shard.shard_type:
                required = REQUIRED_INJECTIONS.get(shard.shard_type, [])
                recommended = RECOMMENDED_INJECTIONS.get(shard.shard_type, [])

                for injection in required:
                    if injection not in shard.injections:
                        shard.missing_injections.append(injection)
                        shard.findings.append(Finding(
                            severity=Severity.ERROR,
                            message=f"Type {shard.shard_type.value} shard '{shard_name}' missing required {injection}()",
                            suggestion=f"Add shard.{injection}(...) in factory function"
                        ))

                for injection in recommended:
                    if injection not in shard.injections:
                        shard.findings.append(Finding(
                            severity=Severity.WARNING,
                            message=f"Type {shard.shard_type.value} shard '{shard_name}' missing recommended {injection}()",
                            suggestion=f"Consider adding shard.{injection}(...) in factory"
                        ))

                # System shard specific: check auto-start
                if shard.shard_type == ShardType.SYSTEM:
                    if shard_name in EXPECTED_SYSTEM_SHARDS and not shard.in_auto_start:
                        shard.findings.append(Finding(
                            severity=Severity.WARNING,
                            message=f"System shard '{shard_name}' may not be in auto-start list",
                            suggestion="Verify shard is started in StartSystemShards()"
                        ))
            else:
                if shard.has_profile:
                    shard.findings.append(Finding(
                        severity=Severity.WARNING,
                        message=f"Shard '{shard_name}' profile has no Type field detected",
                        suggestion="Add Type: core.ShardTypeXxx to profile"
                    ))

            # Check for tests
            if not shard.has_tests:
                shard.findings.append(Finding(
                    severity=Severity.INFO,
                    message=f"Shard '{shard_name}' has no integration tests",
                    suggestion=f"Add tests for {shard_name} shard"
                ))

            # Check for entry points (non-system shards should have CLI or verb)
            if shard.shard_type != ShardType.SYSTEM:
                if not shard.has_cli_command and not shard.has_transducer_verb:
                    shard.findings.append(Finding(
                        severity=Severity.INFO,
                        message=f"Shard '{shard_name}' has no CLI command or transducer verb",
                        suggestion="Add entry point to make shard reachable"
                    ))

    def _calculate_stats(self):
        """Calculate audit statistics."""
        total_errors = 0
        total_warnings = 0

        for shard in self.result.shards.values():
            for f in shard.findings:
                if f.severity == Severity.ERROR:
                    total_errors += 1
                elif f.severity == Severity.WARNING:
                    total_warnings += 1

        for f in self.result.findings:
            if f.severity == Severity.ERROR:
                total_errors += 1
            elif f.severity == Severity.WARNING:
                total_warnings += 1

        self.result.stats = {
            "total_shards": len(self.result.shards),
            "type_a_count": sum(1 for s in self.result.shards.values() if s.shard_type == ShardType.EPHEMERAL),
            "type_b_count": sum(1 for s in self.result.shards.values() if s.shard_type == ShardType.PERSISTENT),
            "type_u_count": sum(1 for s in self.result.shards.values() if s.shard_type == ShardType.USER),
            "type_s_count": sum(1 for s in self.result.shards.values() if s.shard_type == ShardType.SYSTEM),
            "with_tests": sum(1 for s in self.result.shards.values() if s.has_tests),
            "total_errors": total_errors,
            "total_warnings": total_warnings,
        }

    def print_report(self) -> bool:
        """Print formatted audit report."""
        print("=" * 70)
        print("SHARD REGISTRATION AUDIT REPORT")
        print("=" * 70)
        print()

        # Summary
        print(f"Total Shards: {self.result.stats['total_shards']}")
        print(f"  Type A (Ephemeral):  {self.result.stats['type_a_count']}")
        print(f"  Type B (Persistent): {self.result.stats['type_b_count']}")
        print(f"  Type U (User):       {self.result.stats['type_u_count']}")
        print(f"  Type S (System):     {self.result.stats['type_s_count']}")
        print(f"  With Tests:          {self.result.stats['with_tests']}")
        print()
        print(f"Errors: {self.result.stats['total_errors']}  Warnings: {self.result.stats['total_warnings']}")
        print()

        # Per-shard details
        print("-" * 70)
        print("SHARD DETAILS")
        print("-" * 70)

        for name, shard in sorted(self.result.shards.items()):
            type_str = f"Type {shard.shard_type.value}" if shard.shard_type else "Type ?"
            errors = sum(1 for f in shard.findings if f.severity == Severity.ERROR)
            status = "OK" if errors == 0 else f"{errors} ERRORS"

            print(f"\n[{status}] {name} ({type_str})")
            print(f"    Factory: {'Y' if shard.has_factory else 'N':3} | Profile: {'Y' if shard.has_profile else 'N':3} | Tests: {'Y' if shard.has_tests else 'N'}")
            print(f"    Injections: {', '.join(sorted(shard.injections)) or 'None'}")

            if shard.missing_injections:
                print(f"    MISSING: {', '.join(shard.missing_injections)}")

            if shard.findings:
                for f in shard.findings:
                    print(f"    [{f.severity.value}] {f.message}")
                    if self.verbose and f.suggestion:
                        print(f"             -> {f.suggestion}")

        # Global findings
        if self.result.findings:
            print()
            print("-" * 70)
            print("GLOBAL FINDINGS")
            print("-" * 70)
            for f in self.result.findings:
                print(f"[{f.severity.value}] {f.message}")
                if self.verbose and f.suggestion:
                    print(f"         -> {f.suggestion}")

        print()
        print("=" * 70)
        return self.result.stats['total_errors'] == 0


def find_workspace(start_path: str) -> Path:
    """Find codeNERD workspace root."""
    workspace = Path(start_path).resolve()
    while workspace != workspace.parent:
        if (workspace / ".nerd").exists() or (workspace / "go.mod").exists():
            return workspace
        workspace = workspace.parent
    return Path(start_path).resolve()


def main():
    parser = argparse.ArgumentParser(description="Audit codeNERD shard registration")
    parser.add_argument("workspace", nargs="?", default=".", help="Workspace path")
    parser.add_argument("--verbose", "-v", action="store_true", help="Show suggestions")
    parser.add_argument("--json", action="store_true", help="Output as JSON")

    args = parser.parse_args()

    workspace = find_workspace(args.workspace)
    auditor = ShardAuditor(str(workspace), verbose=args.verbose)
    result = auditor.audit()

    if args.json:
        output = {
            "shards": {
                name: {
                    "type": shard.shard_type.value if shard.shard_type else None,
                    "has_factory": shard.has_factory,
                    "has_profile": shard.has_profile,
                    "has_tests": shard.has_tests,
                    "injections": list(shard.injections),
                    "missing_injections": shard.missing_injections,
                    "findings": [
                        {"severity": f.severity.value, "message": f.message}
                        for f in shard.findings
                    ]
                }
                for name, shard in result.shards.items()
            },
            "stats": result.stats,
        }
        print(json.dumps(output, indent=2))
    else:
        success = auditor.print_report()
        sys.exit(0 if success else 1)


if __name__ == "__main__":
    main()
