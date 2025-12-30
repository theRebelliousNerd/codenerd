#!/usr/bin/env python3
"""
parse_context_harness.py - Convert context harness test logs to Mangle facts

Parses context harness session logs from .nerd/context-tests/ and outputs Mangle facts
for declarative analysis. Supports cross-referencing with system logs.

Session structure:
    .nerd/context-tests/session-YYYYMMDD-HHMMSS/
        prompts.log
        jit-compilation.log
        spreading-activation.log
        compression.log
        piggyback-protocol.log
        context-feedback.log
        summary.log
        MANIFEST.txt

Output facts:
    jit_compilation(Time, AtomCount, TotalTokens, BudgetUsed).
    activation_score(Time, FactId, Score, Source).
    compression_event(Time, InputTokens, OutputTokens, Ratio).
    checkpoint_result(Turn, Description, Passed, Precision, Recall).
    context_feedback(Time, PredicateId, Rating, Impact).
    piggyback_event(Time, EventType, IntentVerb, ToolCount).
"""

import argparse
import os
import re
import sys
from datetime import datetime
from pathlib import Path
from typing import Generator, NamedTuple, Optional, List


# =============================================================================
# CONTEXT HARNESS EVENT TYPES
# =============================================================================

class JITCompilation(NamedTuple):
    """JIT prompt compilation event."""
    timestamp_ms: int
    atom_count: int
    total_tokens: int
    budget_used: float


class ActivationScore(NamedTuple):
    """Spreading activation score event."""
    timestamp_ms: int
    fact_id: str
    score: float
    source: str


class CompressionEvent(NamedTuple):
    """Semantic compression event."""
    timestamp_ms: int
    input_tokens: int
    output_tokens: int
    ratio: float


class CheckpointResult(NamedTuple):
    """Checkpoint validation result."""
    turn: int
    description: str
    passed: bool
    precision: float
    recall: float


class ContextFeedback(NamedTuple):
    """Context usefulness feedback event."""
    timestamp_ms: int
    predicate_id: str
    rating: str
    impact: float


class PiggybackEvent(NamedTuple):
    """Piggyback protocol event."""
    timestamp_ms: int
    event_type: str
    intent_verb: str
    tool_count: int


class SessionSummary(NamedTuple):
    """Session summary metrics."""
    scenario_name: str
    status: str
    encoding_ratio: float
    avg_precision: float
    avg_recall: float
    avg_f1: float
    token_violations: int
    compression_latency_ms: float
    retrieval_latency_ms: float


# =============================================================================
# PARSING PATTERNS
# =============================================================================

# Session header: Session Start: 2025-12-29 19:04:25
SESSION_START_PATTERN = re.compile(r'Session Start:\s*(\d{4}-\d{2}-\d{2}\s+\d{2}:\d{2}:\d{2})')

# JIT compilation: JIT compiled prompt: 51145 bytes, 42 atoms selected
JIT_COMPILE_PATTERN = re.compile(
    r'JIT compiled prompt:\s*(\d+)\s*bytes?,\s*(\d+)\s*atoms?\s*selected'
)

# JIT with budget: Budget: 8192/16384 tokens (50.0%)
JIT_BUDGET_PATTERN = re.compile(
    r'Budget:\s*(\d+)/(\d+)\s*tokens?\s*\(([0-9.]+)%\)'
)

# Activation score: Fact activation: fact_id=turn_5_topic score=0.85 source=recency
ACTIVATION_PATTERN = re.compile(
    r'Fact activation:\s*fact_id=(\S+)\s+score=([0-9.]+)\s+source=(\w+)'
)

# Compression: Compressed 5000 -> 1200 tokens (ratio=0.24)
COMPRESSION_PATTERN = re.compile(
    r'Compressed\s+(\d+)\s*->\s*(\d+)\s*tokens?\s*\(ratio=([0-9.]+)\)'
)

# Checkpoint: Checkpoint at turn 45 failed: Recall 0.00 < required 0.50
CHECKPOINT_PATTERN = re.compile(
    r'Checkpoint.*?turn\s+(\d+)\s+(passed|failed).*?Precision\s*([0-9.]+).*?Recall\s*([0-9.]+)',
    re.IGNORECASE
)

# Checkpoint result line: Checkpoint 1 (Turn 45): Should recall original error
CHECKPOINT_DESC_PATTERN = re.compile(
    r'Checkpoint\s+\d+\s*\(Turn\s+(\d+)\):\s*(.+)'
)

# Checkpoint metrics: Precision: 0.00% | Recall: 0.00% | F1: 0.00%
CHECKPOINT_METRICS_PATTERN = re.compile(
    r'Precision:\s*([0-9.]+)%.*?Recall:\s*([0-9.]+)%.*?F1:\s*([0-9.]+)%'
)

# Context feedback: Predicate turn_0_topic rated=helpful impact=+0.15
FEEDBACK_PATTERN = re.compile(
    r'Predicate\s+(\S+)\s+rated=(\w+)\s+impact=([+-]?[0-9.]+)'
)

# Piggyback: ControlPacket received: intent=/fix tools=5
PIGGYBACK_PATTERN = re.compile(
    r'(ControlPacket|SurfaceResponse)\s+(\w+).*?intent=(/\w+).*?tools?=(\d+)'
)

# Summary status: STATUS: FAILED or STATUS: PASSED
STATUS_PATTERN = re.compile(r'STATUS:\s*(\w+)')

# Summary metrics: Encoding Ratio: 0.27x
ENCODING_RATIO_PATTERN = re.compile(r'Encoding Ratio:\s*([0-9.]+)x')
PRECISION_PATTERN = re.compile(r'Avg Retrieval Precision:\s*([0-9.]+)%')
RECALL_PATTERN = re.compile(r'Avg Retrieval Recall:\s*([0-9.]+)%')
F1_PATTERN = re.compile(r'Avg F1 Score:\s*([0-9.]+)%')
VIOLATIONS_PATTERN = re.compile(r'Token Budget Violations:\s*(\d+)')

# Scenario name: CONTEXT TEST HARNESS REPORT: Debugging Marathon
SCENARIO_PATTERN = re.compile(r'CONTEXT TEST HARNESS REPORT:\s*(.+)')


def parse_timestamp(datetime_str: str) -> int:
    """Convert datetime string to milliseconds since epoch."""
    try:
        dt = datetime.strptime(datetime_str.strip(), "%Y-%m-%d %H:%M:%S")
        return int(dt.timestamp() * 1000)
    except ValueError:
        return 0


def escape_mangle_string(s: str) -> str:
    """Escape a string for Mangle string literal."""
    s = s.replace('\\', '\\\\')
    s = s.replace('"', '\\"')
    s = s.replace('\n', '\\n')
    s = s.replace('\r', '\\r')
    s = s.replace('\t', '\\t')
    return s


# =============================================================================
# LOG FILE PARSERS
# =============================================================================

def parse_jit_log(filepath: str, session_start: int) -> Generator[JITCompilation, None, None]:
    """Parse jit-compilation.log for JIT events."""
    if not os.path.exists(filepath):
        return

    current_atoms = 0
    current_tokens = 0

    with open(filepath, 'r', encoding='utf-8', errors='replace') as f:
        line_time = session_start
        for line in f:
            # Look for JIT compile event
            match = JIT_COMPILE_PATTERN.search(line)
            if match:
                current_tokens = int(match.group(1))
                current_atoms = int(match.group(2))

            # Look for budget info
            match = JIT_BUDGET_PATTERN.search(line)
            if match:
                used = int(match.group(1))
                total = int(match.group(2))
                pct = float(match.group(3))
                yield JITCompilation(
                    timestamp_ms=line_time,
                    atom_count=current_atoms,
                    total_tokens=current_tokens,
                    budget_used=pct / 100.0
                )
                line_time += 1000  # Increment for ordering


def parse_activation_log(filepath: str, session_start: int) -> Generator[ActivationScore, None, None]:
    """Parse spreading-activation.log for activation scores."""
    if not os.path.exists(filepath):
        return

    with open(filepath, 'r', encoding='utf-8', errors='replace') as f:
        line_time = session_start
        for line in f:
            match = ACTIVATION_PATTERN.search(line)
            if match:
                yield ActivationScore(
                    timestamp_ms=line_time,
                    fact_id=match.group(1),
                    score=float(match.group(2)),
                    source=match.group(3)
                )
                line_time += 10  # Small increment for ordering


def parse_compression_log(filepath: str, session_start: int) -> Generator[CompressionEvent, None, None]:
    """Parse compression.log for compression events."""
    if not os.path.exists(filepath):
        return

    with open(filepath, 'r', encoding='utf-8', errors='replace') as f:
        line_time = session_start
        for line in f:
            match = COMPRESSION_PATTERN.search(line)
            if match:
                yield CompressionEvent(
                    timestamp_ms=line_time,
                    input_tokens=int(match.group(1)),
                    output_tokens=int(match.group(2)),
                    ratio=float(match.group(3))
                )
                line_time += 1000


def parse_feedback_log(filepath: str, session_start: int) -> Generator[ContextFeedback, None, None]:
    """Parse context-feedback.log for feedback events."""
    if not os.path.exists(filepath):
        return

    with open(filepath, 'r', encoding='utf-8', errors='replace') as f:
        line_time = session_start
        for line in f:
            match = FEEDBACK_PATTERN.search(line)
            if match:
                yield ContextFeedback(
                    timestamp_ms=line_time,
                    predicate_id=match.group(1),
                    rating=match.group(2),
                    impact=float(match.group(3))
                )
                line_time += 100


def parse_piggyback_log(filepath: str, session_start: int) -> Generator[PiggybackEvent, None, None]:
    """Parse piggyback-protocol.log for protocol events."""
    if not os.path.exists(filepath):
        return

    with open(filepath, 'r', encoding='utf-8', errors='replace') as f:
        line_time = session_start
        for line in f:
            match = PIGGYBACK_PATTERN.search(line)
            if match:
                yield PiggybackEvent(
                    timestamp_ms=line_time,
                    event_type=match.group(1),
                    intent_verb=match.group(3),
                    tool_count=int(match.group(4))
                )
                line_time += 500


def parse_summary_log(filepath: str) -> tuple[Optional[SessionSummary], List[CheckpointResult]]:
    """Parse summary.log for session summary and checkpoint results."""
    if not os.path.exists(filepath):
        return None, []

    summary = None
    checkpoints = []

    scenario_name = "Unknown"
    status = "unknown"
    encoding_ratio = 0.0
    precision = 0.0
    recall = 0.0
    f1 = 0.0
    violations = 0

    current_checkpoint_turn = 0
    current_checkpoint_desc = ""

    with open(filepath, 'r', encoding='utf-8', errors='replace') as f:
        for line in f:
            # Scenario name
            match = SCENARIO_PATTERN.search(line)
            if match:
                scenario_name = match.group(1).strip()

            # Status
            match = STATUS_PATTERN.search(line)
            if match:
                status = match.group(1).lower()

            # Encoding ratio
            match = ENCODING_RATIO_PATTERN.search(line)
            if match:
                encoding_ratio = float(match.group(1))

            # Precision
            match = PRECISION_PATTERN.search(line)
            if match:
                precision = float(match.group(1))

            # Recall
            match = RECALL_PATTERN.search(line)
            if match:
                recall = float(match.group(1))

            # F1
            match = F1_PATTERN.search(line)
            if match:
                f1 = float(match.group(1))

            # Violations
            match = VIOLATIONS_PATTERN.search(line)
            if match:
                violations = int(match.group(1))

            # Checkpoint description
            match = CHECKPOINT_DESC_PATTERN.search(line)
            if match:
                current_checkpoint_turn = int(match.group(1))
                current_checkpoint_desc = match.group(2).strip()

            # Checkpoint metrics
            match = CHECKPOINT_METRICS_PATTERN.search(line)
            if match and current_checkpoint_turn > 0:
                cp_precision = float(match.group(1)) / 100.0
                cp_recall = float(match.group(2)) / 100.0
                # Determine pass/fail from previous line context
                passed = cp_precision >= 0.1 and cp_recall >= 0.5
                checkpoints.append(CheckpointResult(
                    turn=current_checkpoint_turn,
                    description=current_checkpoint_desc,
                    passed=passed,
                    precision=cp_precision,
                    recall=cp_recall
                ))
                current_checkpoint_turn = 0

    summary = SessionSummary(
        scenario_name=scenario_name,
        status=status,
        encoding_ratio=encoding_ratio,
        avg_precision=precision,
        avg_recall=recall,
        avg_f1=f1,
        token_violations=violations,
        compression_latency_ms=0,
        retrieval_latency_ms=0
    )

    return summary, checkpoints


# =============================================================================
# MANGLE OUTPUT
# =============================================================================

def generate_schema() -> str:
    """Generate Mangle schema for context harness facts."""
    return '''# =============================================================================
# Context Harness Log Analysis Schema
# Generated by parse_context_harness.py
# =============================================================================

# JIT compilation events
Decl jit_compilation(Time.Type<int>, AtomCount.Type<int>, TotalTokens.Type<int>, BudgetUsed.Type<float>).

# Spreading activation score events
Decl activation_score(Time.Type<int>, FactId.Type<string>, Score.Type<float>, Source.Type<string>).

# Semantic compression events
Decl compression_event(Time.Type<int>, InputTokens.Type<int>, OutputTokens.Type<int>, Ratio.Type<float>).

# Checkpoint validation results
Decl checkpoint_result(Turn.Type<int>, Description.Type<string>, Passed.Type<name>, Precision.Type<float>, Recall.Type<float>).

# Context feedback events
Decl context_feedback(Time.Type<int>, PredicateId.Type<string>, Rating.Type<string>, Impact.Type<float>).

# Piggyback protocol events
Decl piggyback_event(Time.Type<int>, EventType.Type<string>, IntentVerb.Type<string>, ToolCount.Type<int>).

# Session summary
Decl session_summary(Scenario.Type<string>, Status.Type<name>, EncodingRatio.Type<float>, Precision.Type<float>, Recall.Type<float>, F1.Type<float>).

# =============================================================================
# DERIVED PREDICATES
# =============================================================================

# Failed checkpoints
Decl failed_checkpoint(Turn.Type<int>, Description.Type<string>, Precision.Type<float>, Recall.Type<float>).
failed_checkpoint(T, D, P, R) :- checkpoint_result(T, D, /false, P, R).

# Low activation scores (potential context loss)
Decl low_activation(Time.Type<int>, FactId.Type<string>, Score.Type<float>).
low_activation(T, F, S) :- activation_score(T, F, S, _), S < 0.3.

# High compression (potential information loss)
Decl aggressive_compression(Time.Type<int>, Ratio.Type<float>).
aggressive_compression(T, R) :- compression_event(T, _, _, R), R < 0.2.

# Negative feedback (noise predicates)
Decl noise_predicate(PredicateId.Type<string>, Impact.Type<float>).
noise_predicate(P, I) :- context_feedback(_, P, "noise", I).

# Helpful predicates
Decl helpful_predicate(PredicateId.Type<string>, Impact.Type<float>).
helpful_predicate(P, I) :- context_feedback(_, P, "helpful", I).

# JIT recompilation (same token count appearing multiple times)
Decl jit_recompilation(TokenCount.Type<int>, Count.Type<int>).
jit_recompilation(Tokens, N) :-
    jit_compilation(_, _, Tokens, _) |>
    do fn:group_by(Tokens),
    let N = fn:Count(),
    N > 1.

'''


def event_to_mangle(event) -> str:
    """Convert an event to a Mangle fact."""
    if isinstance(event, JITCompilation):
        return f'jit_compilation({event.timestamp_ms}, {event.atom_count}, {event.total_tokens}, {event.budget_used:.4f}).'
    elif isinstance(event, ActivationScore):
        return f'activation_score({event.timestamp_ms}, "{escape_mangle_string(event.fact_id)}", {event.score:.4f}, "{event.source}").'
    elif isinstance(event, CompressionEvent):
        return f'compression_event({event.timestamp_ms}, {event.input_tokens}, {event.output_tokens}, {event.ratio:.4f}).'
    elif isinstance(event, CheckpointResult):
        passed = "/true" if event.passed else "/false"
        return f'checkpoint_result({event.turn}, "{escape_mangle_string(event.description)}", {passed}, {event.precision:.4f}, {event.recall:.4f}).'
    elif isinstance(event, ContextFeedback):
        return f'context_feedback({event.timestamp_ms}, "{escape_mangle_string(event.predicate_id)}", "{event.rating}", {event.impact:.4f}).'
    elif isinstance(event, PiggybackEvent):
        return f'piggyback_event({event.timestamp_ms}, "{event.event_type}", "{event.intent_verb}", {event.tool_count}).'
    elif isinstance(event, SessionSummary):
        return f'session_summary("{escape_mangle_string(event.scenario_name)}", /{event.status}, {event.encoding_ratio:.4f}, {event.avg_precision:.4f}, {event.avg_recall:.4f}, {event.avg_f1:.4f}).'
    return ""


# =============================================================================
# MAIN
# =============================================================================

def main():
    parser = argparse.ArgumentParser(
        description='Convert context harness test logs to Mangle facts',
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog='''
Examples:
    # Parse a context harness session
    python parse_context_harness.py .nerd/context-tests/session-20251229-190425/

    # Cross-reference with system logs
    python parse_context_harness.py .nerd/context-tests/session-20251229-190425/ \\
        --cross-ref .nerd/logs/2025-12-29*.log

    # Output to file without schema
    python parse_context_harness.py session/ --no-schema -o facts.mg
        '''
    )

    parser.add_argument('session_dir', help='Context harness session directory')
    parser.add_argument('--output', '-o', help='Output file (default: stdout)')
    parser.add_argument('--cross-ref', nargs='*', help='System log files to cross-reference')
    parser.add_argument('--no-schema', action='store_true', help='Omit schema declarations')
    parser.add_argument('--schema-only', action='store_true', help='Output only schema')
    parser.add_argument('--format', '-f', default='mangle', choices=['mangle', 'json'],
                       help='Output format (default: mangle)')

    args = parser.parse_args()

    # Validate session directory
    session_dir = Path(args.session_dir)
    if not session_dir.is_dir():
        print(f"Error: {args.session_dir} is not a directory", file=sys.stderr)
        sys.exit(1)

    # Open output
    out = sys.stdout
    if args.output:
        out = open(args.output, 'w', encoding='utf-8')

    try:
        # Schema only mode
        if args.schema_only:
            out.write(generate_schema())
            return

        # Write schema
        if args.format == 'mangle' and not args.no_schema:
            out.write(generate_schema())
            out.write('\n# =============================================================================\n')
            out.write('# CONTEXT HARNESS FACTS\n')
            out.write('# =============================================================================\n\n')

        # Get session start time from summary
        summary_path = session_dir / 'summary.log'
        session_start = 0
        if summary_path.exists():
            with open(summary_path, 'r') as f:
                for line in f:
                    match = SESSION_START_PATTERN.search(line)
                    if match:
                        session_start = parse_timestamp(match.group(1))
                        break

        if session_start == 0:
            session_start = int(datetime.now().timestamp() * 1000)

        total_events = 0

        # Parse each log file
        jit_path = session_dir / 'jit-compilation.log'
        for event in parse_jit_log(str(jit_path), session_start):
            out.write(event_to_mangle(event) + '\n')
            total_events += 1

        activation_path = session_dir / 'spreading-activation.log'
        for event in parse_activation_log(str(activation_path), session_start):
            out.write(event_to_mangle(event) + '\n')
            total_events += 1

        compression_path = session_dir / 'compression.log'
        for event in parse_compression_log(str(compression_path), session_start):
            out.write(event_to_mangle(event) + '\n')
            total_events += 1

        feedback_path = session_dir / 'context-feedback.log'
        for event in parse_feedback_log(str(feedback_path), session_start):
            out.write(event_to_mangle(event) + '\n')
            total_events += 1

        piggyback_path = session_dir / 'piggyback-protocol.log'
        for event in parse_piggyback_log(str(piggyback_path), session_start):
            out.write(event_to_mangle(event) + '\n')
            total_events += 1

        # Parse summary
        summary, checkpoints = parse_summary_log(str(summary_path))
        if summary:
            out.write(event_to_mangle(summary) + '\n')
            total_events += 1
        for cp in checkpoints:
            out.write(event_to_mangle(cp) + '\n')
            total_events += 1

        # Summary comment
        if args.format == 'mangle':
            out.write(f'\n# Total context harness events: {total_events}\n')
            out.write(f'# Session directory: {session_dir}\n')

        print(f"Parsed {total_events} context harness events from {session_dir}", file=sys.stderr)

    finally:
        if args.output:
            out.close()


if __name__ == '__main__':
    main()
