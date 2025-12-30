import os
import re
import argparse
import sys
import json
from pathlib import Path
from enum import Enum
from typing import List, Dict, Tuple, Optional
from dataclasses import dataclass, field

# ANSI colors for output
GREEN = '\033[92m'
YELLOW = '\033[93m'
RED = '\033[91m'
BLUE = '\033[94m'
CYAN = '\033[96m'
RESET = '\033[0m'

class Severity(Enum):
    ERROR = "ERROR"    # Must fix - blocks deployment
    WARN = "WARN"      # Should fix - quality issue
    INFO = "INFO"      # Suggestion - improvement opportunity
    OK = "OK"          # Passed check

@dataclass
class Issue:
    """Represents a single audit issue"""
    severity: Severity
    check_name: str
    message: str
    file_path: Path
    prompt_name: str
    line_number: Optional[int] = None

@dataclass
class PromptStats:
    """Statistics for a single prompt"""
    name: str
    file_path: Path
    length: int
    is_functional: bool
    is_shard: bool
    issues: List[Issue] = field(default_factory=list)

    def error_count(self) -> int:
        return sum(1 for i in self.issues if i.severity == Severity.ERROR)

    def warn_count(self) -> int:
        return sum(1 for i in self.issues if i.severity == Severity.WARN)

    def info_count(self) -> int:
        return sum(1 for i in self.issues if i.severity == Severity.INFO)

@dataclass
class AuditReport:
    """Complete audit report with statistics"""
    prompts: List[PromptStats] = field(default_factory=list)
    files_scanned: int = 0

    def total_issues(self) -> int:
        return sum(len(p.issues) for p in self.prompts)

    def error_count(self) -> int:
        return sum(p.error_count() for p in self.prompts)

    def warn_count(self) -> int:
        return sum(p.warn_count() for p in self.prompts)

    def info_count(self) -> int:
        return sum(p.info_count() for p in self.prompts)

    def ok_count(self) -> int:
        return sum(1 for p in self.prompts if len(p.issues) == 0)

    def average_length(self) -> float:
        if not self.prompts:
            return 0.0
        return sum(p.length for p in self.prompts) / len(self.prompts)

class PromptAuditor:
    # Anti-patterns from anti-patterns.md
    ANTI_PATTERNS = {
        'surface_before_control': {
            'severity': Severity.ERROR,
            'keywords': ['surface_response', 'control_packet'],
            'check': 'thought_first_ordering'
        },
        'raw_text_output': {
            'severity': Severity.ERROR,
            'keywords': ['JSON', 'NEVER output raw text'],
            'check': 'json_enforcement'
        },
        'missing_reasoning': {
            'severity': Severity.ERROR,
            'keywords': ['reasoning_trace', 'REASONING', 'MANDATORY'],
            'check': 'reasoning_directive'
        },
        'context_starvation': {
            'severity': Severity.WARN,
            'keywords': ['%s', '{{.', 'SessionContext'],
            'check': 'context_injection'
        },
        'tool_hallucination': {
            'severity': Severity.ERROR,
            'keywords': ['AVAILABLE TOOLS', 'ONLY tools', 'MUST NOT invent'],
            'check': 'tool_constraints'
        },
        'artifact_amnesia': {
            'severity': Severity.ERROR,
            'keywords': ['artifact_type', 'MANDATORY'],
            'check': 'artifact_classification'
        },
        'intent_misclassification': {
            'severity': Severity.WARN,
            'keywords': ['DISAMBIGUATION', 'category', 'verb'],
            'check': 'disambiguation_rules'
        },
        'constitutional_bypass': {
            'severity': Severity.ERROR,
            'keywords': ['CONSTITUTIONAL', 'FORBIDS', 'BLOCKS'],
            'check': 'constitutional_awareness'
        },
        'permission_escalation': {
            'severity': Severity.ERROR,
            'keywords': ['PERMISSIONS', 'YOU MAY', 'YOU MAY NOT'],
            'check': 'permission_boundaries'
        },
        'feature_creep': {
            'severity': Severity.WARN,
            'keywords': ['SCOPE', 'Do ONLY', 'FORBIDDEN WITHOUT'],
            'check': 'scope_discipline'
        },
        'over_engineering': {
            'severity': Severity.INFO,
            'keywords': ['SIMPLICITY', 'MINIMUM needed'],
            'check': 'simplicity_principle'
        },
        'copy_paste_syndrome': {
            'severity': Severity.INFO,
            'keywords': ['STYLE MATCHING', 'MATCH THE CODEBASE'],
            'check': 'style_matching'
        },
        'context_flooding': {
            'severity': Severity.INFO,
            'keywords': ['PRIORITY', 'Token Budget', 'Limit'],
            'check': 'priority_ordering'
        },
        'context_ignorance': {
            'severity': Severity.WARN,
            'keywords': ['WHY', 'PURPOSE', 'IMPLICATION'],
            'check': 'context_explanation'
        },
        'injection_vulnerability': {
            'severity': Severity.ERROR,
            'keywords': ['ANTI-INJECTION', 'sanitize', 'delimiter'],
            'check': 'injection_protection'
        }
    }

    def __init__(self, root_dir: str, min_length: int = 8000, shard_length: int = 15000,
                 verbose: bool = False, fail_on: Optional[str] = None):
        self.root_dir = Path(root_dir)
        self.min_length_functional = min_length
        self.min_length_shard = shard_length
        self.verbose = verbose
        self.fail_on = Severity[fail_on] if fail_on else None
        self.report = AuditReport()

        # Regex patterns for finding prompts in Go code
        self.prompt_var_pattern = re.compile(
            r'const\s+(\w*(?:System|User)?Prompt)\s*=\s*`([^`]*)`',
            re.MULTILINE | re.DOTALL
        )
        self.func_pattern = re.compile(
            r'func\s+\(.*?\)[\s\S]*?fmt\.Sprintf\(`([^`]*)`',
            re.MULTILINE | re.DOTALL
        )

    def scan_codebase(self):
        """Scan the codebase for prompts"""
        for root, dirs, files in os.walk(self.root_dir):
            for file in files:
                if file.endswith(".go"):
                    self.audit_file(Path(root) / file)

        self.report.files_scanned = sum(1 for _ in self.root_dir.rglob("*.go"))

    def audit_file(self, file_path: Path):
        """Audit a single file for prompts"""
        try:
            with open(file_path, 'r', encoding='utf-8') as f:
                content = f.read()
        except Exception as e:
            return

        # Find prompts defined as constants
        for match in self.prompt_var_pattern.finditer(content):
            name, prompt_text = match.groups()
            self.check_prompt(file_path, name, prompt_text)

        # Find prompts in string literals inside functions
        for match in self.func_pattern.finditer(content):
            prompt_text = match.group(1)
            if "You are" in prompt_text or "OUTPUT FORMAT" in prompt_text:
                self.check_prompt(file_path, "InlineFuncPrompt", prompt_text)

    def check_prompt(self, file_path: Path, prompt_name: str, text: str):
        """Check a prompt for all compliance issues"""
        # Determine prompt type
        is_shard = self._is_shard_prompt(text, prompt_name)
        is_functional = not is_shard

        stats = PromptStats(
            name=prompt_name,
            file_path=file_path,
            length=len(text),
            is_functional=is_functional,
            is_shard=is_shard
        )

        # Run all checks
        self._check_length(stats, text)
        self._check_structural(stats, text)
        self._check_semantic(stats, text)
        self._check_specialist(stats, text)
        self._check_anti_patterns(stats, text)

        self.report.prompts.append(stats)

    def _is_shard_prompt(self, text: str, name: str) -> bool:
        """Determine if this is a shard prompt"""
        shard_indicators = [
            "reasoning_trace" in text,
            "control_packet" in text,
            "Shard" in name,
            "System" in name and len(text) > 10000
        ]
        return any(shard_indicators)

    def _check_length(self, stats: PromptStats, text: str):
        """Check if prompt meets minimum length requirements"""
        min_required = self.min_length_shard if stats.is_shard else self.min_length_functional
        prompt_type = "Shard" if stats.is_shard else "Functional"

        if stats.length < min_required:
            stats.issues.append(Issue(
                severity=Severity.WARN,
                check_name="length_check",
                message=f"{prompt_type} prompt insufficient context depth: {stats.length} chars (minimum: {min_required})",
                file_path=stats.file_path,
                prompt_name=stats.name
            ))

    def _check_structural(self, stats: PromptStats, text: str):
        """Check structural requirements"""
        if not stats.is_shard:
            return

        # Check for control_packet schema
        if "control_packet" not in text:
            stats.issues.append(Issue(
                severity=Severity.ERROR,
                check_name="schema_completeness",
                message="Missing 'control_packet' schema definition",
                file_path=stats.file_path,
                prompt_name=stats.name
            ))

        # Check for surface_response schema
        if "surface_response" not in text:
            stats.issues.append(Issue(
                severity=Severity.ERROR,
                check_name="schema_completeness",
                message="Missing 'surface_response' schema definition",
                file_path=stats.file_path,
                prompt_name=stats.name
            ))

        # Check for thought-first ordering
        ordering_keywords = ["BEFORE", "precede", "prior to", "THOUGHT-FIRST"]
        if not any(k in text for k in ordering_keywords):
            stats.issues.append(Issue(
                severity=Severity.ERROR,
                check_name="thought_first_ordering",
                message="Missing THOUGHT-FIRST ordering directive (control_packet BEFORE surface_response)",
                file_path=stats.file_path,
                prompt_name=stats.name
            ))

        # Check Piggyback field ordering
        control_idx = text.find("control_packet")
        surface_idx = text.find("surface_response")
        if control_idx > 0 and surface_idx > 0 and surface_idx < control_idx:
            stats.issues.append(Issue(
                severity=Severity.ERROR,
                check_name="piggyback_ordering",
                message="Piggyback field ordering error: surface_response appears before control_packet in schema",
                file_path=stats.file_path,
                prompt_name=stats.name
            ))

        # Check for reasoning directive
        if "ReasoningTraceDirective" not in text and "reasoning_trace" not in text:
            stats.issues.append(Issue(
                severity=Severity.ERROR,
                check_name="reasoning_directive",
                message="Missing reasoning_trace directive",
                file_path=stats.file_path,
                prompt_name=stats.name
            ))

        # Check for artifact classification in mutation prompts
        if ("project_code" in text or "mutation" in text.lower()) and "artifact_type" not in text:
            stats.issues.append(Issue(
                severity=Severity.ERROR,
                check_name="artifact_classification",
                message="Missing 'artifact_type' field (required for mutation prompts)",
                file_path=stats.file_path,
                prompt_name=stats.name
            ))

        # Check for SessionContext injection markers
        if "%s" not in text and "{{." not in text and "SessionContext" not in text:
            stats.issues.append(Issue(
                severity=Severity.WARN,
                check_name="context_injection",
                message="Potential context starvation: No dynamic injection markers (%s or {{.}})",
                file_path=stats.file_path,
                prompt_name=stats.name
            ))

    def _check_semantic(self, stats: PromptStats, text: str):
        """Check semantic requirements"""
        # Check for context category usage
        context_categories = [
            "diagnostics", "test state", "git state", "file_topology",
            "symbol_graph", "PRIORITY", "CurrentDiagnostics"
        ]
        has_context_category = any(cat in text for cat in context_categories)
        if stats.is_shard and not has_context_category:
            stats.issues.append(Issue(
                severity=Severity.INFO,
                check_name="context_categories",
                message="Consider adding context category markers (diagnostics, test state, git state, etc.)",
                file_path=stats.file_path,
                prompt_name=stats.name
            ))

        # Check for tool steering quality
        if "tool" in text.lower() and "AVAILABLE TOOLS" not in text:
            stats.issues.append(Issue(
                severity=Severity.WARN,
                check_name="tool_steering",
                message="Tool references found but missing 'AVAILABLE TOOLS' section",
                file_path=stats.file_path,
                prompt_name=stats.name
            ))

        # Check control packet references mangle atoms
        if "control_packet" in text:
            mangle_refs = ["mangle", "intent", "atoms", "user_intent", "next_action"]
            if not any(ref in text.lower() for ref in mangle_refs):
                stats.issues.append(Issue(
                    severity=Severity.WARN,
                    check_name="mangle_atoms",
                    message="Control packet does not reference mangle atoms (intent, user_intent, etc.)",
                    file_path=stats.file_path,
                    prompt_name=stats.name
                ))

        # Calculate specificity score
        specific_terms = len(re.findall(r'\b[A-Z][a-z]+[A-Z][a-z]+\b', text))  # CamelCase terms
        generic_terms = len(re.findall(r'\b(thing|stuff|item|data|value|object)\b', text, re.IGNORECASE))
        if generic_terms > 10:
            specificity = specific_terms / (generic_terms + 1)
            if specificity < 2.0:
                stats.issues.append(Issue(
                    severity=Severity.INFO,
                    check_name="specificity_score",
                    message=f"Low specificity score: {specificity:.2f} (many generic terms like 'thing', 'stuff')",
                    file_path=stats.file_path,
                    prompt_name=stats.name
                ))

        # Check for hallucination catalog in shard prompts
        if stats.is_shard:
            hallucination_markers = ["hallucination", "DO NOT invent", "MUST NOT", "ONLY", "constraints"]
            has_catalog = sum(1 for m in hallucination_markers if m in text) >= 2
            if not has_catalog:
                stats.issues.append(Issue(
                    severity=Severity.WARN,
                    check_name="hallucination_catalog",
                    message="Missing hallucination prevention section (DO NOT invent, constraints, etc.)",
                    file_path=stats.file_path,
                    prompt_name=stats.name
                ))

    def _check_specialist(self, stats: PromptStats, text: str):
        """Check specialist-specific requirements"""
        # Check for KnowledgeAtom placeholder for Type B/U
        is_specialist = "specialist" in text.lower() or "Type B" in text or "Type U" in text
        if is_specialist:
            if "KnowledgeAtom" not in text and "knowledge" not in text.lower():
                stats.issues.append(Issue(
                    severity=Severity.WARN,
                    check_name="specialist_knowledge",
                    message="Specialist prompt missing KnowledgeAtom or knowledge injection markers",
                    file_path=stats.file_path,
                    prompt_name=stats.name
                ))

            # Check for SpecialistHints
            if "SpecialistHints" not in text and "specialist" in text.lower():
                stats.issues.append(Issue(
                    severity=Severity.INFO,
                    check_name="specialist_hints",
                    message="Consider adding SpecialistHints injection for domain-specific guidance",
                    file_path=stats.file_path,
                    prompt_name=stats.name
                ))

            # Check for domain constraint sections
            if "DOMAIN" not in text and "SCOPE" not in text:
                stats.issues.append(Issue(
                    severity=Severity.INFO,
                    check_name="domain_constraints",
                    message="Consider adding explicit DOMAIN or SCOPE constraint section",
                    file_path=stats.file_path,
                    prompt_name=stats.name
                ))

    def _check_anti_patterns(self, stats: PromptStats, text: str):
        """Check for anti-patterns from anti-patterns.md"""
        for pattern_name, pattern_def in self.ANTI_PATTERNS.items():
            keywords = pattern_def['keywords']
            check_name = pattern_def['check']

            # Check if any of the protective keywords are present
            has_protection = any(kw in text for kw in keywords)

            # Pattern-specific logic
            if pattern_name == 'surface_before_control':
                # Already checked in structural
                continue

            elif pattern_name == 'raw_text_output':
                if stats.is_shard and "NEVER output raw text" not in text and "JUST JSON" not in text:
                    stats.issues.append(Issue(
                        severity=Severity.ERROR,
                        check_name=check_name,
                        message="Missing JSON enforcement directive (NEVER output raw text)",
                        file_path=stats.file_path,
                        prompt_name=stats.name
                    ))

            elif pattern_name == 'missing_reasoning':
                if stats.is_shard and not has_protection:
                    stats.issues.append(Issue(
                        severity=Severity.ERROR,
                        check_name=check_name,
                        message="Missing mandatory reasoning trace directive",
                        file_path=stats.file_path,
                        prompt_name=stats.name
                    ))

            elif pattern_name == 'tool_hallucination':
                if "tool" in text.lower() and not has_protection:
                    stats.issues.append(Issue(
                        severity=Severity.ERROR,
                        check_name=check_name,
                        message="Tool references without explicit constraint list (risk of tool hallucination)",
                        file_path=stats.file_path,
                        prompt_name=stats.name
                    ))

            elif pattern_name == 'artifact_amnesia':
                # Already checked in structural
                continue

            elif pattern_name == 'constitutional_bypass':
                if stats.is_shard and not has_protection:
                    stats.issues.append(Issue(
                        severity=Severity.ERROR,
                        check_name=check_name,
                        message="Missing Constitutional awareness section",
                        file_path=stats.file_path,
                        prompt_name=stats.name
                    ))

            elif pattern_name == 'permission_escalation':
                if stats.is_shard and "Shard" in stats.name and not has_protection:
                    stats.issues.append(Issue(
                        severity=Severity.ERROR,
                        check_name=check_name,
                        message="Missing explicit permission boundaries (YOU MAY/MAY NOT)",
                        file_path=stats.file_path,
                        prompt_name=stats.name
                    ))

            elif pattern_name == 'feature_creep':
                if not has_protection and stats.length > 5000:
                    stats.issues.append(Issue(
                        severity=Severity.INFO,
                        check_name=check_name,
                        message="Consider adding scope discipline section (Do ONLY what asked)",
                        file_path=stats.file_path,
                        prompt_name=stats.name
                    ))

    def output_console(self):
        """Output audit report to console"""
        print("\n" + "="*60)
        print(f"{CYAN}=== Prompt Audit Report ==={RESET}\n")
        print(f"Scanning: {self.root_dir}\n")

        # Group issues by file and severity
        files_with_issues = {}
        for prompt in self.report.prompts:
            if prompt.issues or self.verbose:
                key = str(prompt.file_path)
                if key not in files_with_issues:
                    files_with_issues[key] = []
                files_with_issues[key].append(prompt)

        # Output by file
        for file_path in sorted(files_with_issues.keys()):
            prompts = files_with_issues[file_path]
            for prompt in prompts:
                if prompt.issues:
                    # Group issues by severity
                    errors = [i for i in prompt.issues if i.severity == Severity.ERROR]
                    warns = [i for i in prompt.issues if i.severity == Severity.WARN]
                    infos = [i for i in prompt.issues if i.severity == Severity.INFO]

                    for issue in errors:
                        print(f"{RED}[ERROR]{RESET} {file_path} :: {prompt.name}")
                        print(f"  - {issue.message}")

                    for issue in warns:
                        print(f"{YELLOW}[WARN]{RESET} {file_path} :: {prompt.name}")
                        print(f"  - {issue.message}")

                    for issue in infos:
                        print(f"{BLUE}[INFO]{RESET} {file_path} :: {prompt.name}")
                        print(f"  - {issue.message}")

                    print()

                elif self.verbose:
                    print(f"{GREEN}[OK]{RESET} {file_path} :: {prompt.name}")
                    print(f"  - Prompt length: {prompt.length} chars")
                    print()

        # Output summary
        print("\n" + "="*60)
        print(f"{CYAN}=== Summary ==={RESET}\n")
        print(f"Files Scanned: {self.report.files_scanned}")
        print(f"Prompts Found: {len(self.report.prompts)}")
        print(f"Average Length: {self.report.average_length():.0f} chars")
        print()
        print(f"{RED}Errors: {self.report.error_count()}{RESET}")
        print(f"{YELLOW}Warnings: {self.report.warn_count()}{RESET}")
        print(f"{BLUE}Info: {self.report.info_count()}{RESET}")
        print(f"{GREEN}Passed: {self.report.ok_count()}{RESET}")
        print()

    def output_json(self) -> str:
        """Output audit report as JSON"""
        data = {
            "summary": {
                "files_scanned": self.report.files_scanned,
                "prompts_found": len(self.report.prompts),
                "average_length": self.report.average_length(),
                "errors": self.report.error_count(),
                "warnings": self.report.warn_count(),
                "info": self.report.info_count(),
                "passed": self.report.ok_count()
            },
            "prompts": []
        }

        for prompt in self.report.prompts:
            prompt_data = {
                "name": prompt.name,
                "file": str(prompt.file_path),
                "length": prompt.length,
                "type": "shard" if prompt.is_shard else "functional",
                "issues": []
            }

            for issue in prompt.issues:
                prompt_data["issues"].append({
                    "severity": issue.severity.value,
                    "check": issue.check_name,
                    "message": issue.message
                })

            data["prompts"].append(prompt_data)

        return json.dumps(data, indent=2)

    def output_markdown(self) -> str:
        """Output audit report as Markdown"""
        lines = [
            "# Prompt Audit Report",
            "",
            f"**Root Directory:** `{self.root_dir}`",
            f"**Date:** {__import__('datetime').datetime.now().strftime('%Y-%m-%d %H:%M:%S')}",
            "",
            "## Summary",
            "",
            f"- **Files Scanned:** {self.report.files_scanned}",
            f"- **Prompts Found:** {len(self.report.prompts)}",
            f"- **Average Length:** {self.report.average_length():.0f} chars",
            "",
            f"- **Errors:** {self.report.error_count()}",
            f"- **Warnings:** {self.report.warn_count()}",
            f"- **Info:** {self.report.info_count()}",
            f"- **Passed:** {self.report.ok_count()}",
            "",
            "## Issues by File",
            ""
        ]

        # Group by file
        files_with_issues = {}
        for prompt in self.report.prompts:
            if prompt.issues:
                key = str(prompt.file_path)
                if key not in files_with_issues:
                    files_with_issues[key] = []
                files_with_issues[key].append(prompt)

        if not files_with_issues:
            lines.append("No issues found!")
        else:
            for file_path in sorted(files_with_issues.keys()):
                lines.append(f"### `{file_path}`")
                lines.append("")

                for prompt in files_with_issues[file_path]:
                    lines.append(f"#### {prompt.name} ({prompt.length} chars)")
                    lines.append("")

                    errors = [i for i in prompt.issues if i.severity == Severity.ERROR]
                    warns = [i for i in prompt.issues if i.severity == Severity.WARN]
                    infos = [i for i in prompt.issues if i.severity == Severity.INFO]

                    if errors:
                        lines.append("**Errors:**")
                        for issue in errors:
                            lines.append(f"- {issue.message}")
                        lines.append("")

                    if warns:
                        lines.append("**Warnings:**")
                        for issue in warns:
                            lines.append(f"- {issue.message}")
                        lines.append("")

                    if infos:
                        lines.append("**Info:**")
                        for issue in infos:
                            lines.append(f"- {issue.message}")
                        lines.append("")

        # Statistics breakdown
        lines.extend([
            "## Statistics",
            "",
            "### By Severity",
            "",
            "| Severity | Count |",
            "|----------|-------|",
            f"| ERROR | {self.report.error_count()} |",
            f"| WARN | {self.report.warn_count()} |",
            f"| INFO | {self.report.info_count()} |",
            f"| OK | {self.report.ok_count()} |",
            "",
            "### By Check Type",
            ""
        ])

        # Count by check type
        check_counts = {}
        for prompt in self.report.prompts:
            for issue in prompt.issues:
                check_counts[issue.check_name] = check_counts.get(issue.check_name, 0) + 1

        if check_counts:
            lines.append("| Check | Count |")
            lines.append("|-------|-------|")
            for check, count in sorted(check_counts.items(), key=lambda x: -x[1]):
                lines.append(f"| {check} | {count} |")
        else:
            lines.append("All checks passed!")

        lines.append("")
        return "\n".join(lines)

    def should_fail(self) -> bool:
        """Determine if audit should fail based on --fail-on flag"""
        if not self.fail_on:
            return False

        if self.fail_on == Severity.ERROR:
            return self.report.error_count() > 0
        elif self.fail_on == Severity.WARN:
            return self.report.error_count() > 0 or self.report.warn_count() > 0

        return False

def main():
    parser = argparse.ArgumentParser(
        description="Audit codeNERD prompts for protocol compliance.",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Examples:
  # Basic scan with console output
  python audit_prompts.py --root ./internal

  # Verbose output showing all checks
  python audit_prompts.py --root ./internal --verbose

  # JSON output for CI/CD integration
  python audit_prompts.py --root ./internal --json > audit.json

  # Markdown report
  python audit_prompts.py --root ./internal --markdown report.md

  # Fail on warnings or higher
  python audit_prompts.py --root ./internal --fail-on WARN
        """
    )

    parser.add_argument("--root", default=".",
                       help="Root directory to scan (default: current dir)")
    parser.add_argument("--verbose", action="store_true",
                       help="Show all checks including passed")
    parser.add_argument("--json", action="store_true",
                       help="Output as JSON to stdout")
    parser.add_argument("--markdown", type=str,
                       help="Output markdown report to file")
    parser.add_argument("--min-length", type=int, default=8000,
                       help="Minimum prompt length for functional prompts (default: 8000)")
    parser.add_argument("--shard-length", type=int, default=15000,
                       help="Minimum prompt length for shard prompts (default: 15000)")
    parser.add_argument("--fail-on", choices=["ERROR", "WARN"],
                       help="Exit 1 if issues at this level or higher")

    args = parser.parse_args()

    auditor = PromptAuditor(
        root_dir=args.root,
        min_length=args.min_length,
        shard_length=args.shard_length,
        verbose=args.verbose,
        fail_on=args.fail_on
    )

    auditor.scan_codebase()

    if args.json:
        print(auditor.output_json())
    elif args.markdown:
        markdown_content = auditor.output_markdown()
        with open(args.markdown, 'w', encoding='utf-8') as f:
            f.write(markdown_content)
        print(f"Markdown report written to {args.markdown}")
    else:
        auditor.output_console()

    if auditor.should_fail():
        sys.exit(1)
    else:
        sys.exit(0)

if __name__ == "__main__":
    main()
