"""Bounded parser for the live text and JSON formats in internal/logging."""

from __future__ import annotations

import json
import math
import re
from dataclasses import dataclass
from datetime import datetime
from pathlib import Path
from typing import Iterator


MAX_LOG_LINE_BYTES = 1 << 20
_TEXT_PREFIX = re.compile(
    r"^(?P<date>\d{4}[/-]\d{2}[/-]\d{2})\s+"
    r"(?P<time>\d{2}:\d{2}:\d{2}(?:\.\d{1,9})?)\s+(?P<message>.*)$"
)
_LEVEL_PREFIX = re.compile(r"^\[(?P<level>DEBUG|INFO|WARN|WARNING|ERROR|FATAL|TRACE)\]\s*(?P<message>.*)$", re.IGNORECASE)
_EMBEDDED_CATEGORY = re.compile(r"^\[(?P<category>[a-z][a-z0-9_]*)\]\s*(?P<message>.*)$", re.IGNORECASE)

# Keep this set aligned with the Category constants in internal/logging/logger.go.
_CATEGORIES = (
    "system_shards",
    "virtual_store",
    "autopoiesis",
    "perception",
    "articulation",
    "researcher",
    "performance",
    "regression",
    "embedding",
    "campaign",
    "reviewer",
    "browser",
    "context",
    "northstar",
    "persist",
    "session",
    "kernel",
    "store",
    "shards",
    "coder",
    "tester",
    "dream",
    "world",
    "tactile",
    "tools",
    "routing",
    "build",
    "boot",
    "api",
    "audit",
    "llm_io",
    "problems",
)


@dataclass(frozen=True)
class LogEntry:
    timestamp_ms: int
    category: str
    level: str
    message: str
    source_file: str
    source_line: int
    line_number: int


def _after_timestamp_ms(after: datetime | None) -> int | None:
    if after is None:
        return None
    return int(after.timestamp() * 1000)


def _timestamp_ms(date: str, time: str) -> int | None:
    normalized_date = date.replace("/", "-")
    if "." in time:
        clock, fraction = time.split(".", 1)
        microseconds = int((fraction[:6]).ljust(6, "0"))
    else:
        clock = time
        microseconds = 0
    try:
        value = datetime.strptime(f"{normalized_date} {clock}", "%Y-%m-%d %H:%M:%S")
    except ValueError:
        return None
    return int(value.replace(microsecond=microseconds).timestamp() * 1000)


def _category_from_filename(path: Path) -> str:
    name = path.name
    if name == "problems.log":
        return "problems"
    if name.endswith(".log"):
        stem = name[:-4]
        for category in sorted(_CATEGORIES, key=len, reverse=True):
            if stem == category or re.search(rf"_{re.escape(category)}(?:\..+)?$", stem):
                return category
    return "unknown"


def _json_entry(line: str, line_number: int, fallback_category: str) -> LogEntry | None:
    if not line.lstrip().startswith("{"):
        return None
    try:
        value = json.loads(line)
    except (json.JSONDecodeError, RecursionError):
        return None
    if not isinstance(value, dict):
        return None

    timestamp = value.get("ts")
    level = value.get("lvl")
    message = value.get("msg")
    category = value.get("cat", fallback_category)
    if isinstance(timestamp, bool) or not isinstance(timestamp, (int, float)):
        return None
    if isinstance(timestamp, float) and not math.isfinite(timestamp):
        return None
    if not isinstance(level, str) or not isinstance(message, str) or not isinstance(category, str):
        return None
    source_file = value.get("file", "")
    source_line = value.get("line", 0)
    if not isinstance(source_file, str):
        source_file = ""
    if isinstance(source_line, bool) or not isinstance(source_line, int):
        source_line = 0
    try:
        timestamp_ms = int(timestamp)
    except (OverflowError, ValueError):
        return None
    return LogEntry(timestamp_ms, category, level, message, source_file, source_line, line_number)


def _text_entry(line: str, line_number: int, fallback_category: str) -> LogEntry | None:
    match = _TEXT_PREFIX.match(line)
    if match is None:
        return None
    timestamp_ms = _timestamp_ms(match.group("date"), match.group("time"))
    level_match = _LEVEL_PREFIX.match(match.group("message"))
    if timestamp_ms is None or level_match is None:
        return None

    level = level_match.group("level").lower()
    if level == "warning":
        level = "warn"
    elif level == "fatal":
        level = "error"
    message = level_match.group("message")
    category = fallback_category
    embedded = _EMBEDDED_CATEGORY.match(message)
    if embedded is not None:
        category = embedded.group("category").lower()
        message = embedded.group("message")
    return LogEntry(timestamp_ms, category, level, message, "", 0, line_number)


def parse_log_file(
    path: str | Path,
    after: datetime | None = None,
    *,
    max_line_bytes: int = MAX_LOG_LINE_BYTES,
) -> Iterator[LogEntry]:
    """Yield valid entries, skipping malformed or oversized records safely.

    Text timestamps use the local timezone, matching Go's local-time logger
    prefix. JSON timestamps are Unix milliseconds. Lines larger than the
    configured cap are drained and ignored so one corrupt record cannot cause
    unbounded memory use.
    """
    if max_line_bytes <= 0:
        raise ValueError("max_line_bytes must be positive")
    after_ms = _after_timestamp_ms(after)
    log_path = Path(path)
    fallback_category = _category_from_filename(log_path)

    with log_path.open("rb") as handle:
        line_number = 0
        while True:
            raw = handle.readline(max_line_bytes + 1)
            if not raw:
                break
            line_number += 1
            oversized = len(raw) > max_line_bytes
            if oversized and not raw.endswith(b"\n"):
                while True:
                    remainder = handle.readline(max_line_bytes + 1)
                    if not remainder or remainder.endswith(b"\n"):
                        break
                continue
            if oversized:
                continue

            line = raw.decode("utf-8", errors="replace").rstrip("\r\n")
            entry = _json_entry(line, line_number, fallback_category)
            if entry is None:
                entry = _text_entry(line, line_number, fallback_category)
            if entry is None or (after_ms is not None and entry.timestamp_ms < after_ms):
                continue
            yield entry
