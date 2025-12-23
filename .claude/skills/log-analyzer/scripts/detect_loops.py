#!/usr/bin/env python3
"""
detect_loops.py - Fast loop and anomaly detection for codeNERD logs

A regex-based loop detection script that outputs JSON for programmatic use.
This is a quick alternative to the full Mangle-based logquery tool.

Usage:
    python detect_loops.py .nerd/logs/*.log
    python detect_loops.py .nerd/logs/*.log --threshold 5
    python detect_loops.py .nerd/logs/*.log --pretty

Output: JSON with detected anomalies, root causes, and recommendations.
"""

import argparse
import json
import os
import re
import sys
from collections import defaultdict
from datetime import datetime
from pathlib import Path
from typing import Dict, List, Optional, Tuple


# =============================================================================
# REGEX PATTERNS
# =============================================================================

# Log line pattern: YYYY/MM/DD HH:MM:SS.microseconds [LEVEL] message
LOG_PATTERN = re.compile(
    r'^(\d{4}/\d{2}/\d{2})\s+'
    r'(\d{2}:\d{2}:\d{2}\.\d+)\s+'
    r'\[(\w+)\]\s+'
    r'(.*)$'
)

# tools.log: Executing tool: code_search (action=/analyze_code, target=jit system, call_id=action-xxx)
TOOL_EXEC_START = re.compile(
    r'Executing tool:\s*(\w+)\s*'
    r'\(action=(/[\w_]+),\s*'
    r'target=([^,]+),\s*'
    r'call_id=([^)]+)\)'
)

# tools.log: Tool execution completed: code_search (call_id=xxx, duration=1.17s, result_len=44)
TOOL_EXEC_COMPLETE = re.compile(
    r'Tool execution completed:\s*(\w+)\s*'
    r'\(call_id=([^,]+),\s*'
    r'duration=([^,]+),\s*'
    r'result_len=(\d+)\)'
)

# virtual_store.log: Routing action: predicate=next_action, args=4
ACTION_ROUTING = re.compile(
    r'Routing action:\s*predicate=(\w+),\s*args=(\d+)'
)

# virtual_store.log: Action analyze_code completed: success=true, output_len=44
ACTION_COMPLETED = re.compile(
    r'Action\s+(\w+)\s+completed:\s*success=(\w+),\s*output_len=(\d+)'
)

# shards.log: APIScheduler: shard X waiting for slot (active=5/5, waiting=1)
SLOT_WAITING = re.compile(
    r'APIScheduler:\s*shard\s+(\S+)\s+waiting for slot\s*'
    r'\(active=(\d+)/(\d+),\s*waiting=(\d+)\)'
)

# shards.log: APIScheduler: shard X acquired slot after 40.1027628s
SLOT_ACQUIRED = re.compile(
    r'APIScheduler:\s*shard\s+(\S+)\s+acquired slot after\s*([0-9.]+)s'
)


# =============================================================================
# DATA STRUCTURES
# =============================================================================

class LoopDetector:
    """Detects loops and anomalies in codeNERD logs."""

    def __init__(self, threshold: int = 5):
        self.threshold = threshold

        # Tool execution tracking
        self.call_id_counts: Dict[str, int] = defaultdict(int)
        self.call_id_first_time: Dict[str, str] = {}
        self.call_id_last_time: Dict[str, str] = {}
        self.call_id_action: Dict[str, str] = {}

        # Action completion tracking
        self.action_counts: Dict[str, int] = defaultdict(int)
        self.action_first_time: Dict[str, str] = {}
        self.action_last_time: Dict[str, str] = {}
        self.action_result_lens: Dict[str, List[int]] = defaultdict(list)

        # Routing tracking
        self.routing_counts: Dict[str, int] = defaultdict(int)
        self.routing_first_time: Dict[str, str] = {}
        self.routing_last_time: Dict[str, str] = {}

        # Slot starvation tracking
        self.slot_waiting_events: List[dict] = []
        self.slot_acquired_events: List[dict] = []

        # Log files processed
        self.log_files: List[str] = []
        self.total_entries = 0

    def parse_file(self, filepath: str) -> None:
        """Parse a single log file."""
        self.log_files.append(os.path.basename(filepath))

        try:
            with open(filepath, 'r', encoding='utf-8', errors='replace') as f:
                for line in f:
                    self._parse_line(line.rstrip())
        except IOError as e:
            print(f"Warning: Could not read {filepath}: {e}", file=sys.stderr)

    def _parse_line(self, line: str) -> None:
        """Parse a single log line."""
        if not line:
            return

        match = LOG_PATTERN.match(line)
        if not match:
            return

        self.total_entries += 1
        date_str, time_str, level, message = match.groups()
        timestamp = f"{date_str} {time_str}"

        # Tool execution start
        m = TOOL_EXEC_START.search(message)
        if m:
            tool_name, action, target, call_id = m.groups()
            self.call_id_counts[call_id] += 1
            if call_id not in self.call_id_first_time:
                self.call_id_first_time[call_id] = timestamp
            self.call_id_last_time[call_id] = timestamp
            self.call_id_action[call_id] = action

        # Tool execution complete
        m = TOOL_EXEC_COMPLETE.search(message)
        if m:
            tool_name, call_id, duration, result_len = m.groups()
            # Already counted in start event

        # Action routing
        m = ACTION_ROUTING.search(message)
        if m:
            predicate, arg_count = m.groups()
            self.routing_counts[predicate] += 1
            if predicate not in self.routing_first_time:
                self.routing_first_time[predicate] = timestamp
            self.routing_last_time[predicate] = timestamp

        # Action completed
        m = ACTION_COMPLETED.search(message)
        if m:
            action, success, output_len = m.groups()
            self.action_counts[action] += 1
            if action not in self.action_first_time:
                self.action_first_time[action] = timestamp
            self.action_last_time[action] = timestamp
            self.action_result_lens[action].append(int(output_len))

        # Slot waiting
        m = SLOT_WAITING.search(message)
        if m:
            shard_id, active, max_slots, waiting = m.groups()
            self.slot_waiting_events.append({
                'timestamp': timestamp,
                'shard_id': shard_id,
                'active': int(active),
                'max_slots': int(max_slots),
                'waiting': int(waiting)
            })

        # Slot acquired
        m = SLOT_ACQUIRED.search(message)
        if m:
            shard_id, wait_duration = m.groups()
            self.slot_acquired_events.append({
                'timestamp': timestamp,
                'shard_id': shard_id,
                'wait_duration_ms': int(float(wait_duration) * 1000)
            })

    def analyze(self) -> dict:
        """Analyze collected data and return JSON-compatible results."""
        anomalies = []

        # Detect action loops
        for action, count in self.action_counts.items():
            if count > self.threshold:
                result_lens = self.action_result_lens[action]
                identical_results = len(set(result_lens)) == 1 and len(result_lens) > 1

                anomaly = {
                    'type': 'action_loop',
                    'severity': 'critical' if count > 20 else 'high',
                    'action': action,
                    'count': count,
                    'first_time': self.action_first_time.get(action, ''),
                    'last_time': self.action_last_time.get(action, ''),
                    'evidence': {
                        'identical_results': identical_results,
                        'result_lens': list(set(result_lens)) if result_lens else []
                    }
                }

                # Determine root cause
                root_cause = self._determine_root_cause(action, identical_results)
                anomaly['root_cause'] = root_cause

                anomalies.append(anomaly)

        # Detect repeated call_ids (same ID used multiple times)
        for call_id, count in self.call_id_counts.items():
            if count > 2:  # More than 2 uses of same call_id is suspicious
                anomalies.append({
                    'type': 'repeated_call_id',
                    'severity': 'critical' if count > 10 else 'high',
                    'call_id': call_id,
                    'action': self.call_id_action.get(call_id, 'unknown'),
                    'count': count,
                    'first_time': self.call_id_first_time.get(call_id, ''),
                    'last_time': self.call_id_last_time.get(call_id, ''),
                    'root_cause': {
                        'diagnosis': 'call_id_not_regenerated',
                        'explanation': 'Same call_id used across multiple executions, indicating state not advancing',
                        'suggested_fix': 'Check if call_id is being regenerated for each new action execution'
                    }
                })

        # Detect routing stagnation
        for predicate, count in self.routing_counts.items():
            if count > 10:
                anomalies.append({
                    'type': 'routing_stagnation',
                    'severity': 'high',
                    'predicate': predicate,
                    'count': count,
                    'first_time': self.routing_first_time.get(predicate, ''),
                    'last_time': self.routing_last_time.get(predicate, ''),
                    'root_cause': {
                        'diagnosis': 'kernel_rule_stuck',
                        'explanation': f'Predicate {predicate} queried {count} times without state change',
                        'suggested_fix': 'Check Mangle policy rules for missing state transition conditions'
                    }
                })

        # Detect slot starvation
        shard_max_waiting: Dict[str, int] = defaultdict(int)
        for event in self.slot_waiting_events:
            if event['waiting'] > shard_max_waiting[event['shard_id']]:
                shard_max_waiting[event['shard_id']] = event['waiting']

        for shard_id, max_waiting in shard_max_waiting.items():
            if max_waiting > 3:
                anomalies.append({
                    'type': 'slot_starvation',
                    'severity': 'high',
                    'shard_id': shard_id,
                    'max_waiting': max_waiting,
                    'wait_events': len([e for e in self.slot_waiting_events if e['shard_id'] == shard_id])
                })

        # Long slot waits
        for event in self.slot_acquired_events:
            if event['wait_duration_ms'] > 10000:  # > 10 seconds
                anomalies.append({
                    'type': 'long_slot_wait',
                    'severity': 'high',
                    'shard_id': event['shard_id'],
                    'wait_duration_ms': event['wait_duration_ms'],
                    'timestamp': event['timestamp']
                })

        # Build summary
        summary = {
            'total_anomalies': len(anomalies),
            'critical': len([a for a in anomalies if a.get('severity') == 'critical']),
            'high': len([a for a in anomalies if a.get('severity') == 'high']),
            'loops_detected': len([a for a in anomalies if a['type'] == 'action_loop']),
            'affected_actions': list(set(a.get('action', '') for a in anomalies if a.get('action')))
        }

        return {
            'analysis_timestamp': datetime.now().isoformat(),
            'log_files': self.log_files,
            'total_entries_processed': self.total_entries,
            'anomalies': anomalies,
            'summary': summary
        }

    def _determine_root_cause(self, action: str, identical_results: bool) -> dict:
        """Determine the root cause of an action loop."""

        # Check for identical results (tool caching)
        if identical_results:
            result_lens = self.action_result_lens[action]
            return {
                'diagnosis': 'tool_caching',
                'explanation': f'Tool returns identical result (len={result_lens[0]}) every time',
                'suggested_fix': 'Check if tool is returning cached/dummy response instead of executing'
            }

        # Check for routing stagnation
        if self.routing_counts.get('next_action', 0) > 10:
            return {
                'diagnosis': 'kernel_rule_stuck',
                'explanation': 'next_action predicate returns same action repeatedly',
                'suggested_fix': 'Check Mangle policy rules for missing state transition after action completion'
            }

        # Check for slot starvation correlation
        if self.slot_waiting_events:
            return {
                'diagnosis': 'slot_starvation_correlated',
                'explanation': 'Loop correlates with slot starvation, likely caused by loop consuming all slots',
                'suggested_fix': 'Fix the loop; slot starvation is a symptom not the cause'
            }

        # Default: missing fact update
        return {
            'diagnosis': 'missing_fact_update',
            'explanation': 'Action completes with success=true but no kernel fact is asserted, causing next_action to derive the same action repeatedly',
            'suggested_fix': 'Check VirtualStore.RouteAction() for missing fact assertion after tool execution'
        }


# =============================================================================
# MAIN
# =============================================================================

def main():
    parser = argparse.ArgumentParser(
        description='Detect loops and anomalies in codeNERD logs (JSON output)',
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog='''
Examples:
    python detect_loops.py .nerd/logs/*.log
    python detect_loops.py .nerd/logs/*.log --threshold 3
    python detect_loops.py .nerd/logs/*.log --pretty > anomalies.json
        '''
    )

    parser.add_argument('files', nargs='*', help='Log files to analyze')
    parser.add_argument('--threshold', '-t', type=int, default=5,
                       help='Minimum count to consider as loop (default: 5)')
    parser.add_argument('--pretty', '-p', action='store_true',
                       help='Pretty-print JSON output')
    parser.add_argument('--output', '-o', help='Output file (default: stdout)')

    args = parser.parse_args()

    if not args.files:
        print("No log files specified. Use --help for usage.", file=sys.stderr)
        sys.exit(1)

    detector = LoopDetector(threshold=args.threshold)

    # Process files
    for filepath in args.files:
        # Expand globs on Windows
        if '*' in filepath:
            from glob import glob
            expanded = glob(filepath)
        else:
            expanded = [filepath]

        for fp in expanded:
            if os.path.isfile(fp):
                detector.parse_file(fp)
            else:
                print(f"Warning: Skipping non-file: {fp}", file=sys.stderr)

    # Analyze and output
    results = detector.analyze()

    # Output
    if args.pretty:
        output = json.dumps(results, indent=2)
    else:
        output = json.dumps(results)

    if args.output:
        with open(args.output, 'w', encoding='utf-8') as f:
            f.write(output)
            f.write('\n')
    else:
        print(output)

    # Summary to stderr
    summary = results['summary']
    print(f"\nAnalyzed {detector.total_entries} log entries from {len(detector.log_files)} files", file=sys.stderr)
    print(f"Found {summary['total_anomalies']} anomalies ({summary['critical']} critical, {summary['high']} high)", file=sys.stderr)

    if summary['loops_detected'] > 0:
        print(f"Loops detected in actions: {', '.join(summary['affected_actions'])}", file=sys.stderr)


if __name__ == '__main__':
    main()
