#!/usr/bin/env python3
"""
parse_log.py - Convert codeNERD log files to Mangle facts

Parses log files from .nerd/logs/ and outputs Mangle facts for declarative analysis.

Log format: YYYY/MM/DD HH:MM:SS.microseconds [LEVEL] message
Filename format: {date}_{category}.log

Output facts:
    log_entry(timestamp_ms, /category, /level, "message", "filename", line_number).
"""

import argparse
import json
import os
import re
import sys
from datetime import datetime
from pathlib import Path
from typing import Generator, NamedTuple, Optional


class LogEntry(NamedTuple):
    """Parsed log entry."""
    timestamp_ms: int
    category: str
    level: str
    message: str
    filename: str
    line_number: int


# Regex to parse log lines
# Format: 2025/12/08 10:30:45.123456 [LEVEL] message
LOG_PATTERN = re.compile(
    r'^(\d{4}/\d{2}/\d{2})\s+'  # Date
    r'(\d{2}:\d{2}:\d{2}\.\d+)\s+'  # Time with microseconds
    r'\[(\w+)\]\s+'  # Level
    r'(.*)$'  # Message
)

# Extract category from filename pattern: {date}_{category}.log
FILENAME_PATTERN = re.compile(r'^\d{4}-\d{2}-\d{2}_(.+)\.log$')


def parse_timestamp(date_str: str, time_str: str) -> int:
    """Convert log timestamp to milliseconds since epoch."""
    # Parse: 2025/12/08 10:30:45.123456
    datetime_str = f"{date_str} {time_str}"
    try:
        # Handle microseconds
        if '.' in time_str:
            dt = datetime.strptime(datetime_str, "%Y/%m/%d %H:%M:%S.%f")
        else:
            dt = datetime.strptime(datetime_str, "%Y/%m/%d %H:%M:%S")
        return int(dt.timestamp() * 1000)
    except ValueError:
        return 0


def extract_category_from_filename(filename: str) -> str:
    """Extract category from log filename."""
    basename = os.path.basename(filename)
    match = FILENAME_PATTERN.match(basename)
    if match:
        return match.group(1)
    # Fallback: use filename without extension
    return os.path.splitext(basename)[0]


def escape_mangle_string(s: str) -> str:
    """Escape a string for Mangle string literal."""
    # Escape backslashes first, then quotes
    s = s.replace('\\', '\\\\')
    s = s.replace('"', '\\"')
    s = s.replace('\n', '\\n')
    s = s.replace('\r', '\\r')
    s = s.replace('\t', '\\t')
    return s


def parse_log_file(
    filepath: str,
    after: Optional[datetime] = None,
    before: Optional[datetime] = None,
    min_level: str = "debug",
    category_filter: Optional[str] = None
) -> Generator[LogEntry, None, None]:
    """Parse a log file and yield LogEntry objects."""

    level_priority = {"debug": 0, "info": 1, "warn": 2, "error": 3}
    min_level_num = level_priority.get(min_level.lower(), 0)

    category = extract_category_from_filename(filepath)

    # Apply category filter
    if category_filter and category != category_filter:
        return

    filename = os.path.basename(filepath)

    try:
        with open(filepath, 'r', encoding='utf-8', errors='replace') as f:
            for line_num, line in enumerate(f, 1):
                line = line.rstrip()
                if not line:
                    continue

                match = LOG_PATTERN.match(line)
                if not match:
                    continue

                date_str, time_str, level, message = match.groups()

                # Check level filter
                level_lower = level.lower()
                if level_priority.get(level_lower, 0) < min_level_num:
                    continue

                # Parse timestamp
                timestamp_ms = parse_timestamp(date_str, time_str)

                # Apply time filters
                if after or before:
                    entry_dt = datetime.fromtimestamp(timestamp_ms / 1000)
                    if after and entry_dt < after:
                        continue
                    if before and entry_dt > before:
                        continue

                yield LogEntry(
                    timestamp_ms=timestamp_ms,
                    category=category,
                    level=level_lower,
                    message=message,
                    filename=filename,
                    line_number=line_num
                )
    except IOError as e:
        print(f"# Error reading {filepath}: {e}", file=sys.stderr)


def entry_to_mangle(entry: LogEntry) -> str:
    """Convert a LogEntry to a Mangle fact."""
    escaped_msg = escape_mangle_string(entry.message)
    escaped_file = escape_mangle_string(entry.filename)
    return f'log_entry({entry.timestamp_ms}, /{entry.category}, /{entry.level}, "{escaped_msg}", "{escaped_file}", {entry.line_number}).'


def entry_to_json(entry: LogEntry) -> dict:
    """Convert a LogEntry to JSON-compatible dict."""
    return {
        "timestamp_ms": entry.timestamp_ms,
        "category": entry.category,
        "level": entry.level,
        "message": entry.message,
        "filename": entry.filename,
        "line_number": entry.line_number
    }


def entry_to_csv(entry: LogEntry) -> str:
    """Convert a LogEntry to CSV line."""
    # Escape CSV fields
    msg = entry.message.replace('"', '""')
    return f'{entry.timestamp_ms},"{entry.category}","{entry.level}","{msg}","{entry.filename}",{entry.line_number}'


def generate_schema() -> str:
    """Generate the Mangle schema declarations."""
    return '''# =============================================================================
# codeNERD Log Analysis Schema
# Generated by parse_log.py
# =============================================================================

# Core log entry fact
# log_entry(Timestamp, Category, Level, Message, File, Line)
Decl log_entry(Time.Type<int>, Category.Type<name>, Level.Type<name>, Message.Type<string>, File.Type<string>, Line.Type<int>).

# =============================================================================
# DERIVED PREDICATES
# =============================================================================

# Error entries only
Decl error_entry(Time.Type<int>, Category.Type<name>, Message.Type<string>).
error_entry(T, C, M) :- log_entry(T, C, /error, M, _, _).

# Warning entries only
Decl warning_entry(Time.Type<int>, Category.Type<name>, Message.Type<string>).
warning_entry(T, C, M) :- log_entry(T, C, /warn, M, _, _).

# Info entries only
Decl info_entry(Time.Type<int>, Category.Type<name>, Message.Type<string>).
info_entry(T, C, M) :- log_entry(T, C, /info, M, _, _).

# Debug entries only
Decl debug_entry(Time.Type<int>, Category.Type<name>, Message.Type<string>).
debug_entry(T, C, M) :- log_entry(T, C, /debug, M, _, _).

# Category event stream
Decl category_event(Category.Type<name>, Time.Type<int>, Level.Type<name>).
category_event(C, T, L) :- log_entry(T, C, L, _, _, _).

# First entry per category (session start marker)
Decl first_entry(Category.Type<name>, Time.Type<int>).
first_entry(C, MinT) :-
    log_entry(_, C, _, _, _, _) |>
    do fn:group_by(C),
    let MinT = fn:Min(T).

# Last entry per category (most recent)
Decl last_entry(Category.Type<name>, Time.Type<int>).
last_entry(C, MaxT) :-
    log_entry(_, C, _, _, _, _) |>
    do fn:group_by(C),
    let MaxT = fn:Max(T).

# Entry count by category
Decl entry_count(Category.Type<name>, Count.Type<int>).
entry_count(C, N) :-
    log_entry(_, C, _, _, _, _) |>
    do fn:group_by(C),
    let N = fn:Count().

# Error count by category
Decl error_count(Category.Type<name>, Count.Type<int>).
error_count(C, N) :-
    error_entry(_, C, _) |>
    do fn:group_by(C),
    let N = fn:Count().

# =============================================================================
# CORRELATION PREDICATES
# =============================================================================

# Events correlated within time window (default 100ms)
Decl correlated(Time1.Type<int>, Cat1.Type<name>, Time2.Type<int>, Cat2.Type<name>).
correlated(T1, C1, T2, C2) :-
    log_entry(T1, C1, _, _, _, _),
    log_entry(T2, C2, _, _, _, _),
    C1 != C2,
    T2 > T1,
    fn:minus(T2, T1) < 100.

# Error context (events before an error within 500ms)
Decl error_context(ErrorTime.Type<int>, ErrorCat.Type<name>, PriorTime.Type<int>, PriorCat.Type<name>, PriorMsg.Type<string>).
error_context(ET, EC, PT, PC, PM) :-
    error_entry(ET, EC, _),
    log_entry(PT, PC, _, PM, _, _),
    PT < ET,
    fn:minus(ET, PT) < 500.

# =============================================================================
# EXECUTION FLOW PREDICATES
# =============================================================================

# Sequential flow edges (consecutive events within 50ms)
Decl flow_edge(FromCat.Type<name>, ToCat.Type<name>, Time.Type<int>).
flow_edge(C1, C2, T2) :-
    log_entry(T1, C1, _, _, _, _),
    log_entry(T2, C2, _, _, _, _),
    C1 != C2,
    T2 > T1,
    fn:minus(T2, T1) < 50.

# Transitive reachability
Decl reachable(FromCat.Type<name>, ToCat.Type<name>).
reachable(C1, C2) :- flow_edge(C1, C2, _).
reachable(C1, C3) :- flow_edge(C1, C2, _), reachable(C2, C3).

'''


def main():
    parser = argparse.ArgumentParser(
        description='Convert codeNERD log files to Mangle facts',
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog='''
Examples:
    # Parse all logs
    python parse_log.py .nerd/logs/*.log > session.mg

    # Parse specific categories
    python parse_log.py .nerd/logs/*kernel*.log .nerd/logs/*shards*.log > focus.mg

    # Filter by time
    python parse_log.py --after "2025-12-08 10:00:00" .nerd/logs/*.log > window.mg

    # Output as JSON
    python parse_log.py --format json .nerd/logs/*.log > logs.json
        '''
    )

    parser.add_argument('files', nargs='*', help='Log files to parse')
    parser.add_argument('--output', '-o', help='Output file (default: stdout)')
    parser.add_argument('--after', help='Only entries after this datetime (YYYY-MM-DD HH:MM:SS)')
    parser.add_argument('--before', help='Only entries before this datetime (YYYY-MM-DD HH:MM:SS)')
    parser.add_argument('--category', '-c', help='Filter to specific category')
    parser.add_argument('--level', '-l', default='debug',
                       choices=['debug', 'info', 'warn', 'error'],
                       help='Minimum log level (default: debug)')
    parser.add_argument('--format', '-f', default='mangle',
                       choices=['mangle', 'json', 'csv'],
                       help='Output format (default: mangle)')
    parser.add_argument('--no-schema', action='store_true',
                       help='Omit schema declarations (for appending to existing facts)')
    parser.add_argument('--schema-only', action='store_true',
                       help='Output only the schema declarations')

    args = parser.parse_args()

    # Parse time filters
    after_dt = None
    before_dt = None
    if args.after:
        try:
            after_dt = datetime.strptime(args.after, "%Y-%m-%d %H:%M:%S")
        except ValueError:
            print(f"Error: Invalid --after datetime format: {args.after}", file=sys.stderr)
            sys.exit(2)
    if args.before:
        try:
            before_dt = datetime.strptime(args.before, "%Y-%m-%d %H:%M:%S")
        except ValueError:
            print(f"Error: Invalid --before datetime format: {args.before}", file=sys.stderr)
            sys.exit(2)

    # Open output
    out = sys.stdout
    if args.output:
        out = open(args.output, 'w', encoding='utf-8')

    try:
        # Schema-only mode
        if args.schema_only:
            out.write(generate_schema())
            return

        # Write schema header (for Mangle format)
        if args.format == 'mangle' and not args.no_schema:
            out.write(generate_schema())
            out.write('\n# =============================================================================\n')
            out.write('# LOG FACTS\n')
            out.write('# =============================================================================\n\n')

        # JSON format wrapper
        if args.format == 'json':
            import json
            entries = []

        # CSV header
        if args.format == 'csv':
            out.write('timestamp_ms,category,level,message,filename,line_number\n')

        # Process files
        if not args.files:
            print("No log files specified. Use --help for usage.", file=sys.stderr)
            sys.exit(1)

        total_entries = 0
        for filepath in args.files:
            # Expand globs on Windows
            if '*' in filepath:
                from glob import glob
                expanded = glob(filepath)
            else:
                expanded = [filepath]

            for fp in expanded:
                if not os.path.isfile(fp):
                    print(f"# Skipping non-file: {fp}", file=sys.stderr)
                    continue

                for entry in parse_log_file(
                    fp,
                    after=after_dt,
                    before=before_dt,
                    min_level=args.level,
                    category_filter=args.category
                ):
                    total_entries += 1

                    if args.format == 'mangle':
                        out.write(entry_to_mangle(entry) + '\n')
                    elif args.format == 'json':
                        entries.append(entry_to_json(entry))
                    elif args.format == 'csv':
                        out.write(entry_to_csv(entry) + '\n')

        # Finalize JSON output
        if args.format == 'json':
            import json
            json.dump(entries, out, indent=2)
            out.write('\n')

        # Summary comment
        if args.format == 'mangle':
            out.write(f'\n# Total log entries: {total_entries}\n')

        print(f"Parsed {total_entries} log entries", file=sys.stderr)

    finally:
        if args.output:
            out.close()


if __name__ == '__main__':
    main()
