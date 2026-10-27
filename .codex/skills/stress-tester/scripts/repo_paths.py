"""Repository-root discovery shared by the stress-tester command line tools."""

from __future__ import annotations

from pathlib import Path


def find_repo_root(anchor: str | Path) -> Path:
    """Find codeNERD's root by source markers, independent of the caller CWD."""
    start = Path(anchor).expanduser().resolve()
    if start.is_file():
        start = start.parent

    for candidate in (start, *start.parents):
        if (candidate / "go.mod").is_file() and (candidate / "internal" / "logging" / "logger.go").is_file():
            return candidate

    raise FileNotFoundError(f"could not find codeNERD repository markers above {start}")
