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

Detected Patterns (v2.3.0):
- Action loops (same action repeated >5 times)
- Repeated call_ids
- Routing stagnation
- Slot starvation
- Message duplication (exact message repeated)
- Timestamp duplicates (multiple messages at same microsecond)
- JIT compilation spam
- Repeated initialization
- Database lock cascades
- Rate limit cascades
- Empty LLM responses
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

# JIT compilation: JIT compiled prompt: 51145 bytes, 53 atoms, 13.9% budget
JIT_COMPILED = re.compile(
    r'JIT compiled prompt:\s*(\d+)\s*bytes,\s*(\d+)\s*atoms,\s*([0-9.]+)%\s*budget'
)

# Assembling system prompt: Assembling system prompt for shard=X (type=Y)
ASSEMBLING_PROMPT = re.compile(
    r'Assembling system prompt for shard=(\S+)\s*\(type=(\S+)\)'
)

# Initialization patterns
INIT_PATTERNS = [
    re.compile(r'Initializing (\w+) with'),
    re.compile(r'(\w+) initialized successfully'),
    re.compile(r'Creating new FileScope'),
    re.compile(r'CompositeExecutor initialized'),
    re.compile(r'Kernel attached to VirtualStore'),
    re.compile(r'Permission cache built'),
    re.compile(r'Starting incremental workspace scan'),
]

# Database lock: database is locked
DB_LOCK = re.compile(r'database is locked')

# Rate limit: rate limit exceeded (429)
RATE_LIMIT = re.compile(r'rate limit exceeded.*\(429\)|429.*rate limit', re.IGNORECASE)

# LLM timeout: LLM call timed out after
LLM_TIMEOUT = re.compile(r'LLM call timed out after\s*([0-9.]+[smh]?)')

# Empty LLM response: Processing LLM response (attempt #1, length=0 bytes)
EMPTY_LLM_RESPONSE = re.compile(r'Processing LLM response.*length=0 bytes')

# FeedbackLoop failures
FEEDBACK_LOOP_FAILED = re.compile(r'FeedbackLoop failed after (\d+) attempts')

# Context deadline exceeded
CONTEXT_DEADLINE = re.compile(r'context deadline exceeded')

# Provider detection duplicates
PROVIDER_DETECT = re.compile(r'DetectProvider: using provider=(\S+)')

# Migration failures
MIGRATION_FAILED = re.compile(r'Migration.*failed')


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

        # === NEW PATTERN TRACKING (v2.3.0) ===

        # Message duplication tracking (exact message -> list of timestamps)
        self.message_timestamps: Dict[str, List[str]] = defaultdict(list)

        # Timestamp duplicate tracking (timestamp -> list of messages)
        self.timestamp_messages: Dict[str, List[str]] = defaultdict(list)

        # JIT compilation tracking
        self.jit_compilations: List[dict] = []
        self.jit_by_shard: Dict[str, List[dict]] = defaultdict(list)

        # Initialization tracking
        self.init_events: List[dict] = []
        self.init_windows: List[dict] = []  # Groups of init events within 5s

        # Database lock tracking
        self.db_lock_events: List[dict] = []

        # Rate limit tracking
        self.rate_limit_events: List[dict] = []

        # LLM timeout tracking
        self.llm_timeout_events: List[dict] = []

        # Empty LLM response tracking
        self.empty_response_events: List[dict] = []

        # FeedbackLoop failure tracking
        self.feedback_loop_failures: List[dict] = []

        # Context deadline tracking
        self.context_deadline_events: List[dict] = []

        # Log files processed
        self.log_files: List[str] = []
        self.total_entries = 0

        # Category tracking for cross-category analysis
        self.category_counts: Dict[str, int] = defaultdict(int)
        self.category_errors: Dict[str, int] = defaultdict(int)
        self.category_warnings: Dict[str, int] = defaultdict(int)

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

        # Track message duplicates and timestamp duplicates
        self.message_timestamps[message].append(timestamp)
        self.timestamp_messages[timestamp].append(message)

        # Category tracking (extract from log filename context, or infer from message)
        # This is done at file level in parse_file

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

        # === NEW PATTERN DETECTION (v2.3.0) ===

        # JIT compilation
        m = JIT_COMPILED.search(message)
        if m:
            bytes_count, atoms, budget_pct = m.groups()
            jit_event = {
                'timestamp': timestamp,
                'bytes': int(bytes_count),
                'atoms': int(atoms),
                'budget_pct': float(budget_pct)
            }
            self.jit_compilations.append(jit_event)

        # Assembling prompt (to correlate with JIT)
        m = ASSEMBLING_PROMPT.search(message)
        if m:
            shard, shard_type = m.groups()
            jit_event = {
                'timestamp': timestamp,
                'shard': shard,
                'type': shard_type
            }
            self.jit_by_shard[shard].append(jit_event)

        # Initialization events
        for pat in INIT_PATTERNS:
            if pat.search(message):
                self.init_events.append({
                    'timestamp': timestamp,
                    'message': message[:100]  # Truncate for storage
                })
                break

        # Database lock
        if DB_LOCK.search(message):
            self.db_lock_events.append({
                'timestamp': timestamp,
                'message': message[:100]
            })

        # Rate limit
        if RATE_LIMIT.search(message):
            self.rate_limit_events.append({
                'timestamp': timestamp,
                'message': message[:100]
            })

        # LLM timeout
        m = LLM_TIMEOUT.search(message)
        if m:
            duration = m.group(1)
            self.llm_timeout_events.append({
                'timestamp': timestamp,
                'duration': duration
            })

        # Empty LLM response
        if EMPTY_LLM_RESPONSE.search(message):
            self.empty_response_events.append({
                'timestamp': timestamp,
                'message': message[:100]
            })

        # FeedbackLoop failures
        m = FEEDBACK_LOOP_FAILED.search(message)
        if m:
            attempts = m.group(1)
            self.feedback_loop_failures.append({
                'timestamp': timestamp,
                'attempts': int(attempts),
                'message': message[:100]
            })

        # Context deadline exceeded
        if CONTEXT_DEADLINE.search(message):
            self.context_deadline_events.append({
                'timestamp': timestamp,
                'message': message[:100]
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

        # === NEW ANOMALY DETECTION (v2.3.0) ===

        # Detect message duplication (same exact message appearing multiple times)
        for message, timestamps in self.message_timestamps.items():
            if len(timestamps) >= self.threshold:
                # Skip generic messages that are expected to repeat
                if self._is_expected_repeat(message):
                    continue
                anomalies.append({
                    'type': 'message_duplication',
                    'severity': 'high' if len(timestamps) > 10 else 'medium',
                    'message': message[:80] + '...' if len(message) > 80 else message,
                    'count': len(timestamps),
                    'first_time': timestamps[0],
                    'last_time': timestamps[-1],
                    'root_cause': {
                        'diagnosis': 'repeated_log_message',
                        'explanation': f'Exact same message logged {len(timestamps)} times',
                        'suggested_fix': 'Check for polling loops or redundant logging calls'
                    }
                })

        # Detect timestamp duplicates (multiple messages at EXACT same microsecond)
        for timestamp, messages in self.timestamp_messages.items():
            if len(messages) > 2:  # More than 2 messages at same microsecond is suspicious
                unique_messages = list(set(messages))
                if len(unique_messages) > 1:  # Different messages at same time
                    anomalies.append({
                        'type': 'timestamp_collision',
                        'severity': 'medium',
                        'timestamp': timestamp,
                        'message_count': len(messages),
                        'unique_messages': len(unique_messages),
                        'sample_messages': [m[:50] for m in unique_messages[:3]],
                        'root_cause': {
                            'diagnosis': 'concurrent_logging',
                            'explanation': f'{len(messages)} messages logged at identical timestamp',
                            'suggested_fix': 'Check for race conditions or multiple goroutines logging simultaneously'
                        }
                    })
                else:  # Same message at same time (true duplicate)
                    anomalies.append({
                        'type': 'exact_duplicate',
                        'severity': 'high',
                        'timestamp': timestamp,
                        'count': len(messages),
                        'message': messages[0][:80],
                        'root_cause': {
                            'diagnosis': 'duplicate_log_call',
                            'explanation': f'Same message logged {len(messages)} times at exact same microsecond',
                            'suggested_fix': 'Check for duplicate logging calls in the same code path'
                        }
                    })

        # Detect JIT compilation spam
        if len(self.jit_compilations) > self.threshold:
            # Group by bytes count to detect identical compilations
            bytes_counts = [j['bytes'] for j in self.jit_compilations]
            most_common_bytes = max(set(bytes_counts), key=bytes_counts.count) if bytes_counts else 0
            same_bytes_count = bytes_counts.count(most_common_bytes)

            if same_bytes_count >= self.threshold:
                anomalies.append({
                    'type': 'jit_spam',
                    'severity': 'critical' if same_bytes_count > 15 else 'high',
                    'total_compilations': len(self.jit_compilations),
                    'identical_compilations': same_bytes_count,
                    'bytes': most_common_bytes,
                    'first_time': self.jit_compilations[0]['timestamp'],
                    'last_time': self.jit_compilations[-1]['timestamp'],
                    'root_cause': {
                        'diagnosis': 'jit_cache_miss',
                        'explanation': f'JIT compiler produced identical {most_common_bytes}-byte prompt {same_bytes_count} times',
                        'suggested_fix': 'Check JIT prompt caching in PromptAssembler, possible missing cache key'
                    }
                })

        # Detect per-shard JIT spam
        for shard, events in self.jit_by_shard.items():
            if len(events) > self.threshold:
                anomalies.append({
                    'type': 'shard_jit_spam',
                    'severity': 'high',
                    'shard': shard,
                    'count': len(events),
                    'first_time': events[0]['timestamp'],
                    'last_time': events[-1]['timestamp'],
                    'root_cause': {
                        'diagnosis': 'shard_prompt_loop',
                        'explanation': f'Shard {shard} triggered {len(events)} prompt assemblies',
                        'suggested_fix': 'Check shard execution loop for redundant prompt assembly calls'
                    }
                })

        # Detect initialization spam (repeated re-init)
        if len(self.init_events) > 10:
            # Group init events by time windows (events within 5 seconds of each other)
            windows = self._group_events_by_window(self.init_events, window_seconds=5)
            if len(windows) > 3:
                anomalies.append({
                    'type': 'init_spam',
                    'severity': 'critical' if len(windows) > 10 else 'high',
                    'init_windows': len(windows),
                    'total_init_events': len(self.init_events),
                    'root_cause': {
                        'diagnosis': 'repeated_initialization',
                        'explanation': f'System re-initialized {len(windows)} times during session',
                        'suggested_fix': 'Check for crash loops, health check restarts, or polling that triggers re-init'
                    }
                })

        # Detect database lock cascade
        if len(self.db_lock_events) >= 3:
            anomalies.append({
                'type': 'db_lock_cascade',
                'severity': 'critical' if len(self.db_lock_events) > 10 else 'high',
                'count': len(self.db_lock_events),
                'first_time': self.db_lock_events[0]['timestamp'],
                'last_time': self.db_lock_events[-1]['timestamp'],
                'root_cause': {
                    'diagnosis': 'sqlite_contention',
                    'explanation': f'Database locked {len(self.db_lock_events)} times, indicating write contention',
                    'suggested_fix': 'Check for concurrent DB writers, consider WAL mode or connection pooling'
                }
            })

        # Detect rate limit cascade
        if len(self.rate_limit_events) >= 3:
            anomalies.append({
                'type': 'rate_limit_cascade',
                'severity': 'critical',
                'count': len(self.rate_limit_events),
                'first_time': self.rate_limit_events[0]['timestamp'],
                'last_time': self.rate_limit_events[-1]['timestamp'],
                'root_cause': {
                    'diagnosis': 'api_rate_exhausted',
                    'explanation': f'Hit rate limit {len(self.rate_limit_events)} times in quick succession',
                    'suggested_fix': 'Implement exponential backoff, reduce parallel requests, or increase rate limit'
                }
            })

        # Detect LLM timeout cascade
        if len(self.llm_timeout_events) >= 3:
            anomalies.append({
                'type': 'llm_timeout_cascade',
                'severity': 'high',
                'count': len(self.llm_timeout_events),
                'first_time': self.llm_timeout_events[0]['timestamp'],
                'last_time': self.llm_timeout_events[-1]['timestamp'],
                'durations': [e['duration'] for e in self.llm_timeout_events[:5]],
                'root_cause': {
                    'diagnosis': 'llm_overload',
                    'explanation': f'{len(self.llm_timeout_events)} LLM calls timed out',
                    'suggested_fix': 'Check API provider status, increase timeout, or reduce prompt size'
                }
            })

        # Detect empty LLM responses
        if len(self.empty_response_events) >= 2:
            anomalies.append({
                'type': 'empty_llm_responses',
                'severity': 'high',
                'count': len(self.empty_response_events),
                'first_time': self.empty_response_events[0]['timestamp'],
                'last_time': self.empty_response_events[-1]['timestamp'],
                'root_cause': {
                    'diagnosis': 'llm_empty_response',
                    'explanation': f'LLM returned {len(self.empty_response_events)} empty (0-byte) responses',
                    'suggested_fix': 'Check prompt validity, API quota, or safety filter blocks'
                }
            })

        # Detect FeedbackLoop failure cascade
        if len(self.feedback_loop_failures) >= 3:
            anomalies.append({
                'type': 'feedback_loop_failures',
                'severity': 'critical',
                'count': len(self.feedback_loop_failures),
                'first_time': self.feedback_loop_failures[0]['timestamp'],
                'last_time': self.feedback_loop_failures[-1]['timestamp'],
                'root_cause': {
                    'diagnosis': 'autopoiesis_blocked',
                    'explanation': f'FeedbackLoop failed {len(self.feedback_loop_failures)} times',
                    'suggested_fix': 'Check LLM connectivity, validation rules, or budget exhaustion'
                }
            })

        # Detect context deadline cascade
        if len(self.context_deadline_events) >= 5:
            anomalies.append({
                'type': 'context_deadline_cascade',
                'severity': 'critical',
                'count': len(self.context_deadline_events),
                'first_time': self.context_deadline_events[0]['timestamp'],
                'last_time': self.context_deadline_events[-1]['timestamp'],
                'root_cause': {
                    'diagnosis': 'timeout_cascade',
                    'explanation': f'{len(self.context_deadline_events)} operations hit context deadline',
                    'suggested_fix': 'Check timeout configuration, network latency, or operation complexity'
                }
            })

        # Build summary
        summary = {
            'total_anomalies': len(anomalies),
            'critical': len([a for a in anomalies if a.get('severity') == 'critical']),
            'high': len([a for a in anomalies if a.get('severity') == 'high']),
            'medium': len([a for a in anomalies if a.get('severity') == 'medium']),
            'loops_detected': len([a for a in anomalies if a['type'] == 'action_loop']),
            'jit_issues': len([a for a in anomalies if 'jit' in a['type']]),
            'timeout_issues': len([a for a in anomalies if 'timeout' in a['type'] or 'deadline' in a['type']]),
            'rate_limit_issues': len([a for a in anomalies if 'rate_limit' in a['type']]),
            'duplication_issues': len([a for a in anomalies if 'duplicate' in a['type'] or 'spam' in a['type']]),
            'affected_actions': list(set(a.get('action', '') for a in anomalies if a.get('action')))
        }

        return {
            'analysis_timestamp': datetime.now().isoformat(),
            'log_files': self.log_files,
            'total_entries_processed': self.total_entries,
            'anomalies': anomalies,
            'summary': summary
        }

    def _is_expected_repeat(self, message: str) -> bool:
        """Check if a message is expected to repeat (not anomalous)."""
        expected_patterns = [
            'heartbeat',
            'keepalive',
            'health check',
            'polling',
            'tick',
        ]
        msg_lower = message.lower()
        return any(pat in msg_lower for pat in expected_patterns)

    def _group_events_by_window(self, events: List[dict], window_seconds: int = 5) -> List[List[dict]]:
        """Group events that occur within window_seconds of each other."""
        if not events:
            return []

        windows = []
        current_window = [events[0]]

        for i in range(1, len(events)):
            # Parse timestamps and compare (simplified - assumes same date)
            prev_time = events[i-1]['timestamp']
            curr_time = events[i]['timestamp']

            # Extract time portion and compare
            prev_secs = self._timestamp_to_seconds(prev_time)
            curr_secs = self._timestamp_to_seconds(curr_time)

            if curr_secs is not None and prev_secs is not None:
                if curr_secs - prev_secs <= window_seconds:
                    current_window.append(events[i])
                else:
                    if current_window:
                        windows.append(current_window)
                    current_window = [events[i]]
            else:
                current_window.append(events[i])

        if current_window:
            windows.append(current_window)

        return windows

    def _timestamp_to_seconds(self, timestamp: str) -> Optional[float]:
        """Convert timestamp string to seconds since midnight."""
        try:
            # Format: "YYYY/MM/DD HH:MM:SS.microseconds"
            parts = timestamp.split(' ')
            if len(parts) >= 2:
                time_part = parts[1]
                h, m, rest = time_part.split(':')
                s = float(rest)
                return int(h) * 3600 + int(m) * 60 + s
        except (ValueError, IndexError):
            pass
        return None

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
