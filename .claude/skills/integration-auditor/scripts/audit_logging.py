#!/usr/bin/env python3
"""
Logging Coverage Auditor for codeNERD

Deep audit of logging usage across the codebase:
- Category usage consistency
- Silent code paths (no logging)
- Error handling without logging
- Shard execution logging

Usage:
    python audit_logging.py [workspace_path] [--verbose] [--json]
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
class FileAudit:
    path: str
    has_logging_import: bool = False
    logging_calls: List[Tuple[str, int]] = field(default_factory=list)  # [(category, line), ...]
    error_returns_without_log: List[int] = field(default_factory=list)
    silent_functions: List[str] = field(default_factory=list)

@dataclass
class AuditResult:
    files: Dict[str, FileAudit] = field(default_factory=dict)
    category_usage: Dict[str, int] = field(default_factory=dict)
    findings: List[Finding] = field(default_factory=list)
    stats: Dict[str, int] = field(default_factory=dict)

# The 22 logging categories from logger.go
LOGGING_CATEGORIES = {
    "boot": "CategoryBoot",
    "session": "CategorySession",
    "kernel": "CategoryKernel",
    "api": "CategoryAPI",
    "perception": "CategoryPerception",
    "articulation": "CategoryArticulation",
    "routing": "CategoryRouting",
    "tools": "CategoryTools",
    "virtual_store": "CategoryVirtualStore",
    "shards": "CategoryShards",
    "coder": "CategoryCoder",
    "tester": "CategoryTester",
    "reviewer": "CategoryReviewer",
    "researcher": "CategoryResearcher",
    "system_shards": "CategorySystemShards",
    "dream": "CategoryDream",
    "autopoiesis": "CategoryAutopoiesis",
    "campaign": "CategoryCampaign",
    "context": "CategoryContext",
    "world": "CategoryWorld",
    "embedding": "CategoryEmbedding",
    "store": "CategoryStore",
}

# Component to recommended category mapping
COMPONENT_CATEGORY_MAP = {
    "internal/shards/coder": "coder",
    "internal/shards/tester": "tester",
    "internal/shards/reviewer": "reviewer",
    "internal/shards/researcher": "researcher",
    "internal/shards/system": "system_shards",
    "internal/shards": "shards",
    "internal/core/kernel": "kernel",
    "internal/core/virtual_store": "virtual_store",
    "internal/perception": "perception",
    "internal/articulation": "articulation",
    "internal/autopoiesis": "autopoiesis",
    "internal/campaign": "campaign",
    "internal/context": "context",
    "internal/world": "world",
    "internal/store": "store",
    "internal/embedding": "embedding",
    "cmd/nerd": "session",
}

class LoggingAuditor:
    def __init__(self, workspace: str, verbose: bool = False):
        self.workspace = Path(workspace)
        self.verbose = verbose
        self.result = AuditResult()
        # Initialize category usage
        for cat in LOGGING_CATEGORIES:
            self.result.category_usage[cat] = 0

    def audit(self) -> AuditResult:
        """Run complete logging audit."""
        print(f"[*] Auditing logging coverage in {self.workspace}")
        print()

        # Phase 1: Scan Go files for logging usage
        self._scan_all_files()

        # Phase 2: Check for silent code paths
        self._check_silent_paths()

        # Phase 3: Check error handling logging
        self._check_error_logging()

        # Phase 4: Check category consistency
        self._check_category_consistency()

        # Calculate stats
        self._calculate_stats()

        return self.result

    def _scan_all_files(self):
        """Scan all Go files for logging patterns."""
        for go_file in self.workspace.rglob("*.go"):
            # Skip vendor and test files for the main audit
            rel_path = str(go_file.relative_to(self.workspace))
            if "vendor" in rel_path:
                continue

            try:
                content = go_file.read_text(encoding='utf-8')
            except:
                continue

            lines = content.split('\n')
            file_audit = FileAudit(path=rel_path)

            # Check for logging import
            if 'codenerd/internal/logging' in content or '"internal/logging"' in content:
                file_audit.has_logging_import = True

            # Find all logging calls
            # Patterns: logging.Category(), logging.CategoryDebug(), etc.
            # Also: logging.Get(CategoryX).Info/Debug/Warn/Error
            logging_patterns = [
                # Convenience functions: logging.Coder(), logging.CoderDebug()
                r'logging\.(\w+)\s*\(',
                # Get pattern: logging.Get(CategoryX)
                r'logging\.Get\s*\(\s*(?:logging\.)?Category(\w+)\s*\)',
            ]

            for i, line in enumerate(lines, 1):
                for pattern in logging_patterns:
                    for match in re.finditer(pattern, line):
                        func_name = match.group(1)

                        # Map function name to category
                        category = self._func_to_category(func_name)
                        if category:
                            file_audit.logging_calls.append((category, i))
                            self.result.category_usage[category] += 1

            # Find error returns
            error_pattern = r'return\s+.*(?:err|error|fmt\.Errorf|errors\.)'
            for i, line in enumerate(lines, 1):
                if re.search(error_pattern, line, re.IGNORECASE):
                    # Check if there's logging nearby (within 3 lines before)
                    context_start = max(0, i - 4)
                    context = '\n'.join(lines[context_start:i])

                    has_logging = any(
                        'logging.' in l or '.Error(' in l or '.Warn(' in l
                        for l in lines[context_start:i]
                    )

                    if not has_logging:
                        file_audit.error_returns_without_log.append(i)

            self.result.files[rel_path] = file_audit

    def _func_to_category(self, func_name: str) -> Optional[str]:
        """Map logging function name to category."""
        # Remove Debug/Error/Warn/Info suffix if present
        base_name = func_name
        for suffix in ['Debug', 'Error', 'Warn', 'Info']:
            if func_name.endswith(suffix) and func_name != suffix:
                base_name = func_name[:-len(suffix)]
                break

        # Handle special cases
        name_map = {
            'Boot': 'boot',
            'Session': 'session',
            'Kernel': 'kernel',
            'API': 'api',
            'Perception': 'perception',
            'Articulation': 'articulation',
            'Routing': 'routing',
            'Tools': 'tools',
            'VirtualStore': 'virtual_store',
            'Shards': 'shards',
            'Coder': 'coder',
            'Tester': 'tester',
            'Reviewer': 'reviewer',
            'Researcher': 'researcher',
            'SystemShards': 'system_shards',
            'Dream': 'dream',
            'Autopoiesis': 'autopoiesis',
            'Campaign': 'campaign',
            'Context': 'context',
            'World': 'world',
            'Embedding': 'embedding',
            'Store': 'store',
        }

        return name_map.get(base_name)

    def _check_silent_paths(self):
        """Check for components with no logging."""
        # Key directories that should have logging
        critical_dirs = [
            "internal/shards/coder",
            "internal/shards/tester",
            "internal/shards/reviewer",
            "internal/shards/researcher",
            "internal/shards/system",
            "internal/core",
            "internal/perception",
            "internal/autopoiesis",
        ]

        for critical_dir in critical_dirs:
            full_path = self.workspace / critical_dir
            if not full_path.exists():
                continue

            # Check if any files in this dir have logging
            has_logging = False
            total_files = 0

            for go_file in full_path.rglob("*.go"):
                if "_test.go" in str(go_file):
                    continue
                total_files += 1

                rel_path = str(go_file.relative_to(self.workspace))
                if rel_path in self.result.files:
                    if self.result.files[rel_path].logging_calls:
                        has_logging = True
                        break

            if total_files > 0 and not has_logging:
                self.result.findings.append(Finding(
                    severity=Severity.WARNING,
                    message=f"No logging found in {critical_dir}/ ({total_files} Go files)",
                    suggestion=f"Add logging using appropriate category for this component"
                ))

    def _check_error_logging(self):
        """Check for error returns without logging."""
        # Aggregate error returns without logging by directory
        unlogged_errors: Dict[str, int] = {}

        for file_path, file_audit in self.result.files.items():
            if file_audit.error_returns_without_log:
                dir_path = str(Path(file_path).parent)
                if dir_path not in unlogged_errors:
                    unlogged_errors[dir_path] = 0
                unlogged_errors[dir_path] += len(file_audit.error_returns_without_log)

        # Report directories with many unlogged errors
        threshold = 5  # Report if more than 5 unlogged error returns
        for dir_path, count in sorted(unlogged_errors.items(), key=lambda x: -x[1]):
            if count >= threshold:
                self.result.findings.append(Finding(
                    severity=Severity.INFO,
                    message=f"{dir_path}/ has {count} error returns without nearby logging",
                    suggestion="Consider adding logging before error returns for debugging"
                ))

    def _check_category_consistency(self):
        """Check that files use appropriate logging categories."""
        for file_path, file_audit in self.result.files.items():
            if not file_audit.logging_calls:
                continue

            # Determine expected category based on file path
            expected_category = None
            for path_prefix, category in COMPONENT_CATEGORY_MAP.items():
                if file_path.startswith(path_prefix.replace('/', os.sep)):
                    expected_category = category
                    break

            if not expected_category:
                continue

            # Check if any calls use a different category
            used_categories = set(cat for cat, _ in file_audit.logging_calls)

            for cat in used_categories:
                # Skip generic categories that are always OK
                if cat in ['boot', 'session']:
                    continue

                if cat != expected_category:
                    # Check if it's a valid cross-category use
                    if cat in ['api', 'kernel', 'shards']:
                        # These are commonly used across components
                        continue

                    self.result.findings.append(Finding(
                        severity=Severity.INFO,
                        message=f"{file_path} uses category '{cat}' but expected '{expected_category}'",
                        suggestion=f"Consider using logging.{expected_category.title().replace('_', '')}() for consistency"
                    ))

    def _calculate_stats(self):
        """Calculate audit statistics."""
        total_files = len(self.result.files)
        files_with_logging = sum(1 for f in self.result.files.values() if f.logging_calls)
        total_log_calls = sum(len(f.logging_calls) for f in self.result.files.values())
        categories_used = sum(1 for c, count in self.result.category_usage.items() if count > 0)

        errors = sum(1 for f in self.result.findings if f.severity == Severity.ERROR)
        warnings = sum(1 for f in self.result.findings if f.severity == Severity.WARNING)

        self.result.stats = {
            "total_go_files": total_files,
            "files_with_logging": files_with_logging,
            "total_log_calls": total_log_calls,
            "categories_used": categories_used,
            "total_categories": len(LOGGING_CATEGORIES),
            "logging_coverage_pct": round(100 * files_with_logging / total_files) if total_files > 0 else 0,
            "errors": errors,
            "warnings": warnings,
        }

    def print_report(self) -> bool:
        """Print formatted audit report."""
        print("=" * 70)
        print("LOGGING COVERAGE AUDIT REPORT")
        print("=" * 70)
        print()

        # Summary
        print(f"Total Go Files:       {self.result.stats['total_go_files']}")
        print(f"Files with Logging:   {self.result.stats['files_with_logging']}")
        print(f"Logging Coverage:     {self.result.stats['logging_coverage_pct']}%")
        print(f"Total Log Calls:      {self.result.stats['total_log_calls']}")
        print(f"Categories Used:      {self.result.stats['categories_used']}/{self.result.stats['total_categories']}")
        print()
        print(f"Errors: {self.result.stats['errors']}  Warnings: {self.result.stats['warnings']}")
        print()

        # Category usage breakdown
        print("-" * 70)
        print("CATEGORY USAGE")
        print("-" * 70)

        # Sort by usage count, highest first
        sorted_cats = sorted(
            self.result.category_usage.items(),
            key=lambda x: -x[1]
        )

        for cat, count in sorted_cats:
            bar = "#" * min(count // 5, 30)  # Scale for display
            status = "OK" if count > 0 else "UNUSED"
            print(f"  {cat:20} {count:5} {bar:30} [{status}]")

        # Unused categories warning
        unused = [cat for cat, count in self.result.category_usage.items() if count == 0]
        if unused:
            print()
            print(f"  Unused categories: {', '.join(unused)}")

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
            if infos[:10]:  # Limit to first 10
                print("-" * 70)
                print("INFO (showing first 10)")
                print("-" * 70)
                for f in infos[:10]:
                    print(f"[INFO] {f.message}")
                if len(infos) > 10:
                    print(f"  ... and {len(infos) - 10} more")
                print()

        # Top files without logging
        if self.verbose:
            files_no_logging = [
                f for f, a in self.result.files.items()
                if not a.logging_calls and not "_test.go" in f
            ]
            critical_no_logging = [
                f for f in files_no_logging
                if any(c in f for c in ['shards', 'core', 'perception'])
            ]
            if critical_no_logging[:10]:
                print("-" * 70)
                print("CRITICAL FILES WITHOUT LOGGING (showing first 10)")
                print("-" * 70)
                for f in critical_no_logging[:10]:
                    print(f"  {f}")
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
    parser = argparse.ArgumentParser(description="Audit codeNERD logging coverage")
    parser.add_argument("workspace", nargs="?", default=".", help="Workspace path")
    parser.add_argument("--verbose", "-v", action="store_true", help="Show all findings")
    parser.add_argument("--json", action="store_true", help="Output as JSON")

    args = parser.parse_args()

    workspace = find_workspace(args.workspace)
    auditor = LoggingAuditor(str(workspace), verbose=args.verbose)
    result = auditor.audit()

    if args.json:
        output = {
            "category_usage": result.category_usage,
            "files_with_logging": sum(1 for f in result.files.values() if f.logging_calls),
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
