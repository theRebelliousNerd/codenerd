#!/usr/bin/env python3
"""
Stress Test Log Analyzer

Integrates with log-analyzer skill to analyze logs after stress tests.
Parses logs, runs predefined Mangle queries, and generates a summary report.

Usage:
    python analyze_stress_logs.py [--logs-dir DIR] [--output FILE] [--verbose]

Examples:
    python analyze_stress_logs.py
    python analyze_stress_logs.py --logs-dir .nerd/logs --output report.md
    python analyze_stress_logs.py --verbose
"""

import argparse
import os
import subprocess
import sys
import tempfile
from datetime import datetime
from pathlib import Path

# Paths relative to codeNERD root
CODENERD_ROOT = Path(__file__).parent.parent.parent.parent.parent
LOG_ANALYZER_DIR = CODENERD_ROOT / ".claude" / "skills" / "log-analyzer" / "scripts"
STRESS_QUERIES = CODENERD_ROOT / ".claude" / "skills" / "stress-tester" / "assets" / "stress_queries.mg"

# Built-in queries to run
BUILTIN_QUERIES = [
    "errors",
    "warnings",
    "kernel-errors",
    "shard-errors",
    "api-errors",
]

# Custom predicates to query from stress_queries.mg
CUSTOM_PREDICATES = [
    "panic_detected",
    "nil_pointer_error",
    "oom_event",
    "timeout_event",
    "deadline_exceeded",
    "queue_full",
    "limit_exceeded",
    "gas_limit_hit",
    "critical_issue",
]


def find_log_files(logs_dir: Path) -> list:
    """Find all log files in the logs directory."""
    if not logs_dir.exists():
        return []
    return list(logs_dir.glob("*.log"))


def parse_logs(log_files: list, output_file: Path, verbose: bool = False) -> bool:
    """Parse logs using log-analyzer's parse_log.py."""
    parse_script = LOG_ANALYZER_DIR / "parse_log.py"

    if not parse_script.exists():
        print(f"Error: parse_log.py not found at {parse_script}")
        return False

    cmd = [
        sys.executable,
        str(parse_script),
        "--no-schema",
    ] + [str(f) for f in log_files]

    if verbose:
        print(f"Running: {' '.join(cmd)}")

    try:
        result = subprocess.run(
            cmd,
            capture_output=True,
            text=True,
            cwd=str(LOG_ANALYZER_DIR),
        )

        # Filter to just log_entry facts
        facts = "\n".join(
            line for line in result.stdout.split("\n")
            if line.startswith("log_entry")
        )

        with open(output_file, "w") as f:
            f.write(facts)

        if verbose:
            print(f"Parsed {len(facts.split(chr(10)))} facts to {output_file}")

        return True

    except Exception as e:
        print(f"Error parsing logs: {e}")
        return False


def run_logquery(facts_file: Path, query: str, is_builtin: bool = True, verbose: bool = False) -> str:
    """Run a query using logquery tool."""
    logquery = LOG_ANALYZER_DIR / "logquery" / "logquery.exe"

    if not logquery.exists():
        # Try to build it
        print("logquery.exe not found, attempting to build...")
        build_result = subprocess.run(
            ["go", "build", "-o", "logquery.exe", "."],
            cwd=str(LOG_ANALYZER_DIR / "logquery"),
            capture_output=True,
        )
        if build_result.returncode != 0:
            return f"Error: Could not build logquery: {build_result.stderr.decode()}"

    if is_builtin:
        cmd = [str(logquery), str(facts_file), "--builtin", query, "--limit", "100"]
    else:
        cmd = [str(logquery), str(facts_file), "--query", query, "--limit", "100"]

    if verbose:
        print(f"Running: {' '.join(cmd)}")

    try:
        result = subprocess.run(cmd, capture_output=True, text=True, timeout=60)
        return result.stdout.strip()
    except subprocess.TimeoutExpired:
        return "Query timed out"
    except Exception as e:
        return f"Error: {e}"


def count_results(output: str) -> int:
    """Count the number of results from logquery output."""
    if not output or output.startswith("Error") or output.startswith("Query"):
        return 0
    # Count non-empty lines
    return len([l for l in output.split("\n") if l.strip()])


def generate_report(
    logs_dir: Path,
    facts_file: Path,
    results: dict,
    verbose: bool = False
) -> str:
    """Generate markdown report from analysis results."""
    report = []

    report.append("# Stress Test Log Analysis Report")
    report.append(f"\n**Generated:** {datetime.now().isoformat()}")
    report.append(f"**Logs Directory:** `{logs_dir}`")
    report.append(f"**Facts File:** `{facts_file}`")

    # Summary
    report.append("\n## Summary")
    report.append("")

    critical_count = 0
    warning_count = 0

    for name, data in results.items():
        count = data["count"]
        if name in ["panic_detected", "nil_pointer_error", "oom_event", "critical_issue"]:
            critical_count += count
        elif name in ["errors", "warnings", "timeout_event"]:
            warning_count += count

    if critical_count > 0:
        report.append(f"**Status: FAILED** - {critical_count} critical issues found")
    elif warning_count > 10:
        report.append(f"**Status: WARNING** - {warning_count} warnings found")
    else:
        report.append("**Status: PASSED** - No critical issues")

    # Critical Issues
    report.append("\n## Critical Issues")
    report.append("")

    critical_queries = ["panic_detected", "nil_pointer_error", "oom_event", "critical_issue"]
    has_critical = False

    for query in critical_queries:
        if query in results and results[query]["count"] > 0:
            has_critical = True
            report.append(f"### {query.replace('_', ' ').title()}")
            report.append(f"**Count:** {results[query]['count']}")
            report.append("")
            report.append("```")
            report.append(results[query]["output"][:2000])  # Truncate if needed
            report.append("```")
            report.append("")

    if not has_critical:
        report.append("No critical issues detected.")

    # Error Summary
    report.append("\n## Error Summary")
    report.append("")
    report.append("| Category | Count |")
    report.append("|----------|-------|")

    for name in BUILTIN_QUERIES:
        if name in results:
            report.append(f"| {name} | {results[name]['count']} |")

    # Resource Issues
    report.append("\n## Resource Issues")
    report.append("")

    resource_queries = ["queue_full", "limit_exceeded", "gas_limit_hit", "timeout_event", "deadline_exceeded"]

    for query in resource_queries:
        if query in results and results[query]["count"] > 0:
            report.append(f"### {query.replace('_', ' ').title()}")
            report.append(f"**Count:** {results[query]['count']}")
            report.append("")
            if verbose:
                report.append("```")
                report.append(results[query]["output"][:1000])
                report.append("```")
            report.append("")

    # Recommendations
    report.append("\n## Recommendations")
    report.append("")

    if results.get("panic_detected", {}).get("count", 0) > 0:
        report.append("- **CRITICAL:** Investigate panic sources immediately")

    if results.get("oom_event", {}).get("count", 0) > 0:
        report.append("- **CRITICAL:** Memory limits exceeded - reduce load or increase limits")

    if results.get("queue_full", {}).get("count", 0) > 0:
        report.append("- **WARNING:** Spawn queue saturated - reduce concurrent requests")

    if results.get("gas_limit_hit", {}).get("count", 0) > 0:
        report.append("- **WARNING:** Mangle gas limit hit - simplify rules or increase limit")

    if results.get("timeout_event", {}).get("count", 0) > 5:
        report.append("- **WARNING:** Multiple timeouts - check for blocking operations")

    if critical_count == 0 and warning_count < 10:
        report.append("- System handled stress test well")
        report.append("- Consider increasing stress intensity")

    return "\n".join(report)


def main():
    parser = argparse.ArgumentParser(
        description='Analyze stress test logs using log-analyzer skill'
    )
    parser.add_argument(
        '--logs-dir', '-l',
        type=Path,
        default=CODENERD_ROOT / ".nerd" / "logs",
        help='Directory containing log files (default: .nerd/logs)'
    )
    parser.add_argument(
        '--output', '-o',
        type=Path,
        help='Output file for report (default: stdout)'
    )
    parser.add_argument(
        '--verbose', '-v',
        action='store_true',
        help='Verbose output'
    )

    args = parser.parse_args()

    print(f"Analyzing stress test logs...")
    print(f"Logs directory: {args.logs_dir}")

    # Find log files
    log_files = find_log_files(args.logs_dir)
    if not log_files:
        print(f"No log files found in {args.logs_dir}")
        sys.exit(1)

    print(f"Found {len(log_files)} log files")

    # Parse logs to facts
    with tempfile.NamedTemporaryFile(mode='w', suffix='.mg', delete=False) as f:
        facts_file = Path(f.name)

    if not parse_logs(log_files, facts_file, args.verbose):
        sys.exit(1)

    # Run queries
    results = {}

    print("Running builtin queries...")
    for query in BUILTIN_QUERIES:
        output = run_logquery(facts_file, query, is_builtin=True, verbose=args.verbose)
        results[query] = {
            "output": output,
            "count": count_results(output),
        }
        if args.verbose:
            print(f"  {query}: {results[query]['count']} results")

    print("Running custom queries...")
    for predicate in CUSTOM_PREDICATES:
        output = run_logquery(facts_file, predicate, is_builtin=False, verbose=args.verbose)
        results[predicate] = {
            "output": output,
            "count": count_results(output),
        }
        if args.verbose:
            print(f"  {predicate}: {results[predicate]['count']} results")

    # Generate report
    report = generate_report(args.logs_dir, facts_file, results, args.verbose)

    if args.output:
        with open(args.output, 'w') as f:
            f.write(report)
        print(f"\nReport written to {args.output}")
    else:
        print("\n" + "=" * 60 + "\n")
        print(report)

    # Cleanup
    try:
        os.unlink(facts_file)
    except:
        pass

    # Exit with status based on critical issues
    critical = sum(
        results.get(q, {}).get("count", 0)
        for q in ["panic_detected", "nil_pointer_error", "oom_event"]
    )
    sys.exit(1 if critical > 0 else 0)


if __name__ == '__main__':
    main()
