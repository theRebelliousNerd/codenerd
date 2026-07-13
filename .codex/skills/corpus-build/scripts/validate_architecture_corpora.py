#!/usr/bin/env python3
"""Validate codeNERD architecture corpora as an evidence-backed portfolio.

Check mode is deliberately read-only.  It validates structural evidence; it
does not claim to understand prose or prove that runtime wiring is correct.
Verify mode runs only fixed package-test profiles for explicitly selected corpora.
"""

from __future__ import annotations

import argparse
import datetime as dt
import hashlib
import json
import re
import subprocess
import time
import tomllib
from collections import Counter
from pathlib import Path, PurePosixPath
from urllib.parse import unquote


ROOT = Path(__file__).resolve().parents[4]
ARCH = ROOT / "Docs" / "architecture"

CANONICAL = (
    "README.md",
    "IMPLEMENTED_SPEC.md",
    "00-ALIGNMENT-VISION-REVIEW.md",
    "01-VISION.md",
    "02-CURRENT-STATE.md",
    "03-GAP-ANALYSIS.md",
    "04-ARCHITECTURAL-PRINCIPLES.md",
    "05-INTERNAL-ARCHITECTURE.md",
    "06-PUBLIC-API-AND-TYPES.md",
    "07-DEPENDENCY-MAP.md",
    "08-WIRING-AND-INTEGRATION.md",
    "09-SAFETY-AND-INVARIANTS.md",
    "10-TESTING-ALIGNMENT.md",
    "11-OBSERVABILITY.md",
    "12-FAILURE-MODES.md",
    "TODO.md",
    "OPEN-QUESTIONS.md",
    "_progress.md",
)
CLI_CANONICAL = (
    "README.md",
    "IMPLEMENTED_SPEC.md",
    "00-ALIGNMENT-VISION-REVIEW.md",
    "01-VISION-CLI.md",
    "02-CURRENT-STATE-CLI.md",
    "03-GAP-ANALYSIS-CLI.md",
    "04-ARCHITECTURAL-PRINCIPLES-CLI.md",
    "05-COMMAND-ARCHITECTURE.md",
    "06-TUI-CHAT-SURFACE.md",
    "07-UI-PAGES-AND-OUTPUT.md",
    "08-DEPENDENCY-MAP.md",
    "09-CONSTITUTIONAL-SAFETY.md",
    "10-TESTING-ALIGNMENT.md",
    "11-CROSS-SYSTEM-WIRING-JOURNAL.md",
    "12-TELEMETRY-OBSERVABILITY.md",
    "TODO.md",
    "OPEN-QUESTIONS.md",
    "_progress.md",
)
README_SECTIONS = (
    "In one minute",
    "Its place in codeNERD",
    "A representative journey",
    "What exists today",
    "North star",
    "Improvement frontier",
    "Choose a reading route",
)
LEGACY_PATTERNS = (
    re.compile(r"^01-DOMAIN-MODEL\.md$", re.IGNORECASE),
    re.compile(r"^02-CURRENT-STATE-.+\.md$", re.IGNORECASE),
    re.compile(r"^03-GAP-ANALYSIS-.+\.md$", re.IGNORECASE),
    re.compile(r"^04-INVARIANTS-AND-GATES\.md$", re.IGNORECASE),
    re.compile(r"^05-CROSS-SYSTEM-WIRING\.md$", re.IGNORECASE),
    re.compile(r"^06-TESTING-STRATEGY\.md$", re.IGNORECASE),
    re.compile(r"^08-FAILURE-MODES\.md$", re.IGNORECASE),
)
SOURCE_RESIDUE = re.compile(
    r"\b(?:Storyworld|PageKit|Orval|GraphCAD|Marine Layer)\b", re.IGNORECASE
)
REFERENCE_TOKEN = re.compile(
    r"`(((?:planned|example):)?(?:internal|cmd|tests|scripts|tools)/[^`\s]+)`",
    re.IGNORECASE,
)
MARKDOWN_LINK = re.compile(r"(?<!!)\[[^\]]+\]\(([^)]+)\)")
HEADING = re.compile(r"^#{1,6}\s+(.+?)\s*#*\s*$", re.MULTILINE)
EXPLICIT_ANCHOR = re.compile(r"<(?:a\s+[^>]*name|[^>]+\s+id)=[\"']([^\"']+)[\"']", re.IGNORECASE)
FEATURE_BLOCK = re.compile(r"<!--\s*NERD_FEATURE\s*(.*?)-->", re.DOTALL | re.IGNORECASE)
FEATURE_FIELD = re.compile(r"^\s*([A-Za-z0-9_-]+)\s*:\s*(.*?)\s*$", re.MULTILINE)
KNOWN_FILE_SUFFIXES = {".go", ".mg", ".md", ".toml", ".json", ".yaml", ".yml", ".db", ".py"}
ID_PATTERN = re.compile(r"^[a-z][a-z0-9]*(?:-[a-z0-9]+)*$")
FEATURE_STATUSES = {"proposed", "accepted", "in_progress", "verified", "deferred", "rejected"}
FEATURE_KINDS = {"truth-gap", "leverage", "north-star", "moonshot"}
FEATURE_FIELDS = {"id", "owner", "status", "kind", "depends_on", "affects"}
EXCLUSION_FIELDS = {"path", "classification", "reason", "review_on"}
EXCLUSION_CLASSIFICATIONS = {
    "ecosystem-governance",
    "generated",
    "vendor",
    "test-fixture",
    "documentation",
    "non-runtime",
}


def _safe_repo_path(value: object, *, field: str) -> tuple[str | None, str | None]:
    if not isinstance(value, str) or not value.strip():
        return None, f"{field} must be a non-empty repo-relative path"
    value = value.strip()
    pure = PurePosixPath(value)
    if (
        "\\" in value
        or pure.is_absolute()
        or any(part in {"", ".", ".."} for part in pure.parts)
        or any(character in value for character in "*?[]")
    ):
        return None, f"{field} must be a normalized repo-relative path: {value!r}"
    return pure.as_posix(), None


def _date_error(value: object, field: str) -> str | None:
    if isinstance(value, dt.datetime):
        return f"{field} must be an ISO date, not a datetime"
    if isinstance(value, dt.date):
        return None
    if isinstance(value, str):
        try:
            dt.date.fromisoformat(value)
            return None
        except ValueError:
            pass
    return f"{field} must be an ISO date (YYYY-MM-DD)"


def _string_list(value: object, *, field: str, require_nonempty: bool) -> tuple[list[str], list[str]]:
    errors: list[str] = []
    if not isinstance(value, list):
        return [], [f"{field} must be an array of repo-relative paths"]
    if require_nonempty and not value:
        errors.append(f"{field} must not be empty")
    normalized: list[str] = []
    for item in value:
        path, error = _safe_repo_path(item, field=field)
        if error:
            errors.append(error)
        elif path is not None:
            normalized.append(path)
    duplicates = sorted(name for name, count in Counter(normalized).items() if count > 1)
    if duplicates:
        errors.append(f"{field} contains duplicates: " + ", ".join(duplicates))
    return normalized, errors


def _manifest(corpus: Path) -> tuple[dict, list[str]]:
    path = corpus / "corpus.toml"
    if not path.is_file():
        return {}, ["missing corpus.toml ownership manifest"]
    try:
        data = tomllib.loads(path.read_text(encoding="utf-8"))
    except (OSError, tomllib.TOMLDecodeError) as exc:
        return {}, [f"invalid corpus.toml: {exc}"]

    errors: list[str] = []
    if data.get("schema_version") != 1:
        errors.append("corpus.toml schema_version must be 1")
    corpus_id = data.get("id")
    if corpus_id != corpus.name:
        errors.append(f"corpus.toml id must be {corpus.name!r}")
    if not isinstance(corpus_id, str) or not ID_PATTERN.fullmatch(corpus_id):
        errors.append("corpus.toml id must be a lowercase kebab-case identifier")

    kind = data.get("kind")
    if kind not in {"realized", "proposed"}:
        errors.append("corpus.toml kind must be realized or proposed")

    common = {"schema_version", "id", "kind", "entrypoint", "implemented_spec"}
    if kind == "realized":
        allowed = common | {"source_roots", "verified_on"}
    elif kind == "proposed":
        allowed = common | {"planned_source_roots"}
    else:
        allowed = common | {"source_roots", "planned_source_roots", "verified_on"}
    unknown = sorted(set(data) - allowed)
    if unknown:
        errors.append("corpus.toml has unsupported fields: " + ", ".join(unknown))

    if data.get("entrypoint") != "README.md":
        errors.append("corpus.toml entrypoint must be README.md")
    if "implemented_spec" in data and data.get("implemented_spec") != "IMPLEMENTED_SPEC.md":
        errors.append("corpus.toml implemented_spec must be IMPLEMENTED_SPEC.md")

    if kind == "realized":
        if data.get("implemented_spec") != "IMPLEMENTED_SPEC.md":
            errors.append("realized corpus.toml requires implemented_spec")
        roots, root_errors = _string_list(
            data.get("source_roots"), field="source_roots", require_nonempty=True
        )
        errors.extend(root_errors)
        data["source_roots"] = roots
        if "planned_source_roots" in data:
            errors.append("realized corpus.toml must not declare planned_source_roots")
        for source in roots:
            candidate = ROOT / source
            if not candidate.exists():
                errors.append(f"source root does not exist: {source!r}")
            elif not candidate.is_dir():
                errors.append(f"source root must be a directory: {source!r}")
        date_error = _date_error(data.get("verified_on"), "verified_on")
        if date_error:
            errors.append(date_error)
    elif kind == "proposed":
        if "source_roots" in data:
            errors.append("proposed corpus.toml must use planned_source_roots, not source_roots")
        roots, root_errors = _string_list(
            data.get("planned_source_roots"),
            field="planned_source_roots",
            require_nonempty=True,
        )
        errors.extend(root_errors)
        data["planned_source_roots"] = roots
        if "verified_on" in data:
            errors.append("proposed corpus.toml must not claim verified_on")

    return data, errors


def _heading_slug(value: str) -> str:
    value = re.sub(r"<[^>]+>", "", value)
    value = re.sub(r"[`*_~]", "", value).strip().lower()
    value = re.sub(r"[^\w\- ]", "", value, flags=re.UNICODE)
    return re.sub(r"\s+", "-", value)


def _anchors(path: Path) -> set[str]:
    text = path.read_text(encoding="utf-8", errors="replace")
    anchors = {match.group(1) for match in EXPLICIT_ANCHOR.finditer(text)}
    counts: Counter[str] = Counter()
    for match in HEADING.finditer(text):
        base = _heading_slug(match.group(1))
        if not base:
            continue
        count = counts[base]
        counts[base] += 1
        anchors.add(base if count == 0 else f"{base}-{count}")
    return anchors


def _link_errors(corpus: Path, markdown: list[Path]) -> list[str]:
    errors: list[str] = []
    anchor_cache: dict[Path, set[str]] = {}
    for path in markdown:
        text = path.read_text(encoding="utf-8", errors="replace")
        for match in MARKDOWN_LINK.finditer(text):
            raw = match.group(1).strip().strip("<>")
            target = raw.split(maxsplit=1)[0].strip("'\"")
            if not target or target.startswith(("http://", "https://", "mailto:")):
                continue
            decoded = unquote(target.split("?", 1)[0])
            path_part, separator, fragment = decoded.partition("#")
            candidate = path if not path_part else path.parent / path_part
            if not candidate.exists():
                errors.append(
                    f"broken link in {path.relative_to(corpus).as_posix()}: {raw}"
                )
                continue
            if separator and fragment and candidate.is_file() and candidate.suffix.lower() == ".md":
                anchors = anchor_cache.setdefault(candidate, _anchors(candidate))
                if fragment not in anchors:
                    errors.append(
                        f"broken anchor in {path.relative_to(corpus).as_posix()}: {raw}"
                    )
    return errors


def _symbol_exists(path: Path, symbol: str) -> bool:
    symbol = unquote(symbol).strip()
    if not symbol:
        return True
    # Predicate arity and qualified Go notation are evidence notation, not path syntax.
    needle = symbol.rsplit("/", 1)[0].rsplit(".", 1)[-1]
    if not re.fullmatch(r"[A-Za-z_][A-Za-z0-9_]*", needle):
        return True
    candidates = [path] if path.is_file() else [
        item
        for item in path.rglob("*")
        if item.is_file() and item.suffix.lower() in {".go", ".mg", ".py"}
    ]
    pattern = re.compile(rf"(?<![A-Za-z0-9_]){re.escape(needle)}(?![A-Za-z0-9_])")
    for candidate in candidates:
        try:
            if pattern.search(candidate.read_text(encoding="utf-8", errors="replace")):
                return True
        except OSError:
            continue
    return False


def _unresolved_source_refs(corpus: Path, markdown: list[Path]) -> list[str]:
    unresolved: set[str] = set()
    for path in markdown:
        text = path.read_text(encoding="utf-8", errors="replace")
        for match in REFERENCE_TOKEN.finditer(text):
            token = match.group(1).rstrip("/")
            lowered = token.lower()
            if lowered.startswith(("planned:", "example:")) or "..." in token:
                continue
            source, separator, symbol = token.partition("#")
            if any(marker in source for marker in "*?["):
                matches = [candidate for candidate in ROOT.glob(source) if candidate.exists()]
                if not matches:
                    unresolved.add(token)
                elif separator and not any(_symbol_exists(candidate, symbol) for candidate in matches):
                    unresolved.add(token)
                continue
            suffix = Path(source).suffix
            if suffix and suffix not in KNOWN_FILE_SUFFIXES:
                # Preserve legacy shorthand such as internal/session.Executor.
                continue
            candidate = ROOT / source
            if not candidate.exists():
                unresolved.add(token)
            elif separator and not _symbol_exists(candidate, symbol):
                unresolved.add(token)
    return sorted(unresolved)


def _parse_feature_array(value: str, *, feature_id: str, field: str) -> tuple[list[str], str | None]:
    if not (value.startswith("[") and value.endswith("]")):
        return [], f"feature {feature_id} field {field} must be an array"
    body = value[1:-1].strip()
    if not body:
        return [], None
    values = [item.strip().strip("'\"") for item in body.split(",")]
    if any(not item or not ID_PATTERN.fullmatch(item) for item in values):
        return [], f"feature {feature_id} field {field} contains an invalid identifier"
    if len(values) != len(set(values)):
        return [], f"feature {feature_id} field {field} contains duplicates"
    return values, None


def _feature_cards(corpus: Path, markdown: list[Path]) -> tuple[list[dict], list[str]]:
    cards: list[dict] = []
    errors: list[str] = []
    for path in markdown:
        text = path.read_text(encoding="utf-8", errors="replace")
        for block in FEATURE_BLOCK.finditer(text):
            relative = path.relative_to(corpus).as_posix()
            if relative != "TODO.md":
                errors.append(f"NERD_FEATURE cards are authoritative only in TODO.md: {relative}")
            pairs = [(match.group(1).lower(), match.group(2).strip()) for match in FEATURE_FIELD.finditer(block.group(1))]
            duplicate_fields = sorted(name for name, count in Counter(name for name, _ in pairs).items() if count > 1)
            if duplicate_fields:
                errors.append("NERD_FEATURE has duplicate fields in " + relative + ": " + ", ".join(duplicate_fields))
            fields = {name: value.strip(" '\"") for name, value in pairs}
            missing = sorted(FEATURE_FIELDS - set(fields))
            unknown = sorted(set(fields) - FEATURE_FIELDS)
            if missing:
                errors.append("NERD_FEATURE missing fields in " + relative + ": " + ", ".join(missing))
            if unknown:
                errors.append("NERD_FEATURE has unsupported fields in " + relative + ": " + ", ".join(unknown))

            feature_id = fields.get("id", "<missing>")
            if feature_id != "<missing>" and not ID_PATTERN.fullmatch(feature_id):
                errors.append(f"feature id must be lowercase kebab-case: {feature_id}")
            if feature_id != "<missing>" and not feature_id.startswith(f"{corpus.name}-"):
                errors.append(f"feature id must be corpus-prefixed: {feature_id}")
            if fields.get("owner") != corpus.name:
                errors.append(f"feature {feature_id} owner must be {corpus.name!r}")
            if fields.get("status") not in FEATURE_STATUSES:
                errors.append(f"feature {feature_id} has invalid or missing status")
            if fields.get("kind") not in FEATURE_KINDS:
                errors.append(f"feature {feature_id} has invalid or missing kind")
            arrays: dict[str, list[str]] = {}
            for field in ("depends_on", "affects"):
                if field not in fields:
                    continue
                values, error = _parse_feature_array(
                    fields[field], feature_id=feature_id, field=field
                )
                arrays[field] = values
                if error:
                    errors.append(error)
            if "affects" in arrays and not arrays["affects"]:
                errors.append(f"feature {feature_id} field affects must not be empty")
            cards.append(
                {
                    "id": feature_id,
                    "owner": fields.get("owner"),
                    "status": fields.get("status"),
                    "kind": fields.get("kind"),
                    "depends_on": arrays.get("depends_on", []),
                    "affects": arrays.get("affects", []),
                    "path": relative,
                }
            )
    return cards, errors


def validate(corpus: Path, strict: bool = False) -> dict:
    errors: list[str] = []
    warnings: list[str] = []
    if not corpus.is_dir():
        return {
            "corpus": str(corpus),
            "id": corpus.name,
            "kind": None,
            "source_roots": [],
            "planned_source_roots": [],
            "valid": False,
            "errors": ["corpus directory does not exist"],
            "warnings": [],
            "measurements": {},
            "features": [],
        }

    manifest, manifest_errors = _manifest(corpus)
    errors.extend(manifest_errors)
    required = CLI_CANONICAL if corpus.name == "cli" else CANONICAL
    for name in required:
        if not (corpus / name).is_file():
            errors.append(f"missing canonical document {name}")

    markdown = sorted(corpus.rglob("*.md"))
    if not markdown:
        errors.append("corpus contains no Markdown")
    for path in markdown:
        text = path.read_text(encoding="utf-8", errors="replace")
        if SOURCE_RESIDUE.search(text):
            errors.append(
                f"source-repository residue in {path.relative_to(corpus).as_posix()}"
            )

    link_errors = _link_errors(corpus, markdown)
    unresolved = _unresolved_source_refs(corpus, markdown)
    cards, card_errors = _feature_cards(corpus, markdown)
    # CLI semantic variants are canonical.  A general legacy regex must never
    # reclassify a filename already required by the active corpus manifest.
    legacy = sorted(
        path.name
        for path in corpus.glob("*.md")
        if path.name not in required
        and any(pattern.match(path.name) for pattern in LEGACY_PATTERNS)
    )

    errors.extend(link_errors)
    errors.extend(card_errors)
    if unresolved:
        (errors if strict else warnings).append(
            "unresolved source references: " + ", ".join(unresolved)
        )
    if legacy:
        (errors if strict else warnings).append(
            "superseded legacy documents remain: " + ", ".join(legacy)
        )

    readme = corpus / "README.md"
    missing_sections: list[str] = []
    if readme.is_file():
        text = readme.read_text(encoding="utf-8", errors="replace")
        missing_sections = [
            section
            for section in README_SECTIONS
            if not re.search(
                rf"^##+\s+{re.escape(section)}\s*$",
                text,
                re.MULTILINE | re.IGNORECASE,
            )
        ]
    if missing_sections:
        (errors if strict else warnings).append(
            "README missing human-entry sections: " + ", ".join(missing_sections)
        )
    if not cards:
        (errors if strict else warnings).append("no NERD_FEATURE uplift cards")

    return {
        "corpus": str(corpus),
        "id": manifest.get("id", corpus.name),
        "kind": manifest.get("kind"),
        "source_roots": manifest.get("source_roots", []),
        "planned_source_roots": manifest.get("planned_source_roots", []),
        "valid": not errors,
        "errors": errors,
        "warnings": warnings,
        "measurements": {
            "markdown_files": len(markdown),
            "feature_cards": len(cards),
            "legacy_documents": len(legacy),
            "broken_links": len(link_errors),
            "unresolved_source_refs": len(unresolved),
            "missing_readme_sections": len(missing_sections),
        },
        "features": cards,
    }


def discover() -> list[Path]:
    if not ARCH.is_dir():
        return []
    return sorted(
        path
        for path in ARCH.iterdir()
        if path.is_dir() and not path.name.startswith(("_", "."))
    )


def _coverage_pattern(value: object) -> tuple[str | None, str | None]:
    if not isinstance(value, str) or not value.strip():
        return None, "coverage pattern must be a non-empty string"
    value = value.strip()
    pure = PurePosixPath(value)
    if "\\" in value or pure.is_absolute() or any(part in {"", ".", ".."} for part in pure.parts):
        return None, f"coverage pattern must be repo-relative and normalized: {value!r}"
    return value, None


def _portfolio_registry() -> tuple[dict, list[str], set[str], dict[str, dict]]:
    path = ARCH / "portfolio.toml"
    if not path.is_file():
        return {}, ["missing Docs/architecture/portfolio.toml"], set(), {}
    try:
        data = tomllib.loads(path.read_text(encoding="utf-8"))
    except (OSError, tomllib.TOMLDecodeError) as exc:
        return {}, [f"invalid portfolio.toml: {exc}"], set(), {}

    errors: list[str] = []
    if data.get("schema_version") != 1:
        errors.append("portfolio.toml schema_version must be 1")
    unknown = sorted(set(data) - {"schema_version", "coverage_patterns", "corpus_ids", "exclusions"})
    if unknown:
        errors.append("portfolio.toml has unsupported fields: " + ", ".join(unknown))

    corpus_ids = data.get("corpus_ids")
    if not isinstance(corpus_ids, list) or not corpus_ids:
        errors.append("portfolio.toml corpus_ids must be a non-empty array")
        corpus_ids = []
    else:
        invalid_ids = sorted(
            repr(item)
            for item in corpus_ids
            if not isinstance(item, str) or not ID_PATTERN.fullmatch(item)
        )
        if invalid_ids:
            errors.append("portfolio.toml has invalid corpus ids: " + ", ".join(invalid_ids))
        duplicate_ids = sorted(name for name, count in Counter(corpus_ids).items() if count > 1)
        if duplicate_ids:
            errors.append("portfolio.toml has duplicate corpus ids: " + ", ".join(duplicate_ids))

    patterns = data.get("coverage_patterns")
    coverage: set[str] = set()
    if not isinstance(patterns, list) or not patterns:
        errors.append("portfolio.toml coverage_patterns must be a non-empty array")
    else:
        for raw_pattern in patterns:
            pattern, error = _coverage_pattern(raw_pattern)
            if error:
                errors.append(error)
                continue
            assert pattern is not None
            # Coverage records subsystem surfaces.  A broad pattern such as
            # ``internal/*`` must not turn package-level README files into
            # phantom source owners.
            matches = sorted(match for match in ROOT.glob(pattern) if match.is_dir())
            if not matches:
                errors.append(f"coverage pattern matches nothing: {pattern!r}")
                continue
            for match in matches:
                try:
                    relative = match.relative_to(ROOT).as_posix()
                except ValueError:
                    errors.append(f"coverage pattern escaped repository: {pattern!r}")
                    continue
                coverage.add(relative)

    exclusions: dict[str, dict] = {}
    raw_exclusions = data.get("exclusions", [])
    if not isinstance(raw_exclusions, list):
        errors.append("portfolio.toml exclusions must be an array of tables")
        raw_exclusions = []
    for index, exclusion in enumerate(raw_exclusions):
        label = f"exclusions[{index}]"
        if not isinstance(exclusion, dict):
            errors.append(f"{label} must be a table")
            continue
        missing = sorted(EXCLUSION_FIELDS - set(exclusion))
        unknown_fields = sorted(set(exclusion) - EXCLUSION_FIELDS)
        if missing:
            errors.append(f"{label} missing fields: " + ", ".join(missing))
        if unknown_fields:
            errors.append(f"{label} has unsupported fields: " + ", ".join(unknown_fields))
        exclusion_path, path_error = _safe_repo_path(exclusion.get("path"), field=f"{label}.path")
        if path_error:
            errors.append(path_error)
            continue
        assert exclusion_path is not None
        if exclusion_path in exclusions:
            errors.append(f"duplicate exclusion path: {exclusion_path}")
        exclusions[exclusion_path] = exclusion
        candidate = ROOT / exclusion_path
        if not candidate.exists():
            errors.append(f"exclusion path does not exist: {exclusion_path}")
        classification = exclusion.get("classification")
        if classification not in EXCLUSION_CLASSIFICATIONS:
            errors.append(f"{label}.classification is unsupported: {classification!r}")
        reason = exclusion.get("reason")
        if not isinstance(reason, str) or len(reason.strip()) < 12:
            errors.append(f"{label}.reason must be a specific non-empty explanation")
        date_error = _date_error(exclusion.get("review_on"), f"{label}.review_on")
        if date_error:
            errors.append(date_error)

    data["corpus_ids"] = corpus_ids
    return data, errors, coverage, exclusions


def _overlap(left: str, right: str) -> bool:
    left_parts = PurePosixPath(left).parts
    right_parts = PurePosixPath(right).parts
    shortest = min(len(left_parts), len(right_parts))
    return left_parts[:shortest] == right_parts[:shortest]


def _ownership_errors(
    results: list[dict], coverage: set[str] | None, exclusions: dict[str, dict] | None
) -> list[str]:
    errors: list[str] = []
    owned: list[tuple[str, str]] = []
    planned: list[tuple[str, str]] = []
    for item in results:
        for source in item.get("source_roots", []):
            owned.append((source, item["id"]))
        for source in item.get("planned_source_roots", []):
            planned.append((source, item["id"]))

    for index, (left_root, left_owner) in enumerate(owned):
        for right_root, right_owner in owned[index + 1 :]:
            if _overlap(left_root, right_root):
                relation = "duplicate" if left_root == right_root else "nested"
                errors.append(
                    f"{relation} source-root ownership: {left_root} ({left_owner}) and "
                    f"{right_root} ({right_owner})"
                )

    for planned_root, planned_owner in planned:
        for live_root, live_owner in owned:
            if _overlap(planned_root, live_root):
                errors.append(
                    f"planned source root overlaps realized ownership: {planned_root} "
                    f"({planned_owner}) and {live_root} ({live_owner})"
                )

    if coverage is None or exclusions is None:
        return errors

    owner_counts = Counter(root for root, _ in owned)
    excluded_paths = set(exclusions)
    for target in sorted(coverage):
        count = owner_counts[target]
        excluded = target in excluded_paths
        if count == 0 and not excluded:
            errors.append(f"covered source surface has no owner or exclusion: {target}")
        elif count > 1:
            errors.append(f"covered source surface has multiple owners: {target}")
        elif count == 1 and excluded:
            errors.append(f"covered source surface is both owned and excluded: {target}")
    for root, owner in owned:
        if root not in coverage:
            errors.append(f"source root is outside portfolio coverage: {root} ({owner})")
    for excluded in excluded_paths:
        for root, owner in owned:
            if _overlap(excluded, root):
                errors.append(f"exclusion overlaps source ownership: {excluded} and {root} ({owner})")
    return errors


def portfolio(corpora: list[Path], strict: bool, enforce_inventory: bool = True) -> dict:
    results = [validate(path, strict=strict) for path in corpora]
    ids = [item["id"] for item in results]
    feature_ids = [feature["id"] for item in results for feature in item["features"]]
    errors: list[str] = []
    duplicate_corpora = sorted(name for name, count in Counter(ids).items() if count > 1)
    duplicate_features = sorted(name for name, count in Counter(feature_ids).items() if count > 1)
    if duplicate_corpora:
        errors.append("duplicate corpus ids: " + ", ".join(duplicate_corpora))
    if duplicate_features:
        errors.append("duplicate feature ids: " + ", ".join(duplicate_features))

    registry: dict = {}
    coverage: set[str] | None = None
    exclusions: dict[str, dict] | None = None
    if enforce_inventory:
        registry, registry_errors, coverage, exclusions = _portfolio_registry()
        errors.extend(registry_errors)
        registered = registry.get("corpus_ids", [])
        directory_ids = [path.name for path in corpora]
        missing_directories = sorted(set(registered) - set(directory_ids))
        unregistered_directories = sorted(set(directory_ids) - set(registered))
        if missing_directories:
            errors.append("registered corpus directories missing: " + ", ".join(missing_directories))
        if unregistered_directories:
            errors.append("unregistered corpus directories: " + ", ".join(unregistered_directories))
        if not missing_directories and not unregistered_directories and directory_ids != registered:
            errors.append("portfolio corpus_ids order does not match discovered corpus order")
        manifest_ids = set(ids)
        if set(registered) != manifest_ids:
            errors.append("portfolio corpus_ids do not match corpus.toml ids")
        for item in results:
            if item["id"] == "cli" and item.get("kind") == "realized" and item.get("source_roots") != ["cmd/nerd"]:
                errors.append("cli corpus must own exactly cmd/nerd")
            internal_root = f"internal/{item['id']}"
            if internal_root in (coverage or set()) and item.get("kind") == "realized" and internal_root not in item.get("source_roots", []):
                errors.append(f"{item['id']} corpus must own {internal_root}")

    errors.extend(_ownership_errors(results, coverage, exclusions))

    totals = Counter()
    for item in results:
        totals.update(item["measurements"])
    return {
        "schema_version": 2,
        "valid": not errors and all(item["valid"] for item in results),
        "strict": strict,
        "errors": errors,
        "measurements": {
            "corpora": len(results),
            "covered_source_surfaces": len(coverage or ()),
            "excluded_source_surfaces": len(exclusions or ()),
            **dict(totals),
        },
        "corpora": results,
    }


def _verification_command(corpus: dict) -> list[str] | None:
    if corpus.get("kind") != "realized":
        return None
    corpus_id = corpus.get("id")
    if corpus_id == "cli" and corpus.get("source_roots") == ["cmd/nerd"]:
        return ["go", "test", "-count=1", "./cmd/nerd/..."]
    expected = f"internal/{corpus_id}"
    if expected in corpus.get("source_roots", []):
        return ["go", "test", "-count=1", f"./{expected}/..."]
    return None


def _git_receipt_metadata() -> dict:
    metadata: dict[str, str | None] = {"commit": None, "dirty_fingerprint": None}
    try:
        commit = subprocess.run(
            ["git", "rev-parse", "HEAD"],
            cwd=ROOT,
            capture_output=True,
            text=True,
            timeout=10,
            check=False,
        )
        status = subprocess.run(
            ["git", "status", "--porcelain=v1"],
            cwd=ROOT,
            capture_output=True,
            text=True,
            timeout=10,
            check=False,
        )
    except (OSError, subprocess.TimeoutExpired):
        return metadata
    if commit.returncode == 0:
        metadata["commit"] = commit.stdout.strip()
    if status.returncode == 0:
        metadata["dirty_fingerprint"] = hashlib.sha256(
            status.stdout.encode("utf-8")
        ).hexdigest()
    return metadata


def _bounded_output(value: str | bytes | None, limit: int = 4000) -> str:
    if value is None:
        return ""
    if isinstance(value, bytes):
        value = value.decode("utf-8", errors="replace")
    if len(value) <= limit:
        return value
    return value[:limit] + f"\n... truncated {len(value) - limit} characters"


def _verify_corpus(corpus: dict, timeout_seconds: int) -> dict:
    command = _verification_command(corpus)
    receipt = {
        "corpus": corpus.get("id"),
        "command": command,
        "timeout_seconds": timeout_seconds,
        "valid": False,
        "exit_code": None,
        "timed_out": False,
        "duration_seconds": 0.0,
        "stdout": "",
        "stderr": "",
        **_git_receipt_metadata(),
    }
    if command is None:
        receipt["stderr"] = "no fixed verification profile for corpus"
        return receipt

    started = time.monotonic()
    try:
        result = subprocess.run(
            command,
            cwd=ROOT,
            capture_output=True,
            text=True,
            timeout=timeout_seconds,
            check=False,
        )
        receipt["exit_code"] = result.returncode
        receipt["stdout"] = _bounded_output(result.stdout)
        receipt["stderr"] = _bounded_output(result.stderr)
        receipt["valid"] = result.returncode == 0
    except subprocess.TimeoutExpired as exc:
        receipt["timed_out"] = True
        receipt["stdout"] = _bounded_output(exc.stdout)
        receipt["stderr"] = _bounded_output(exc.stderr)
        receipt["stderr"] += "\nverification exceeded hard timeout"
    except OSError as exc:
        receipt["stderr"] = str(exc)
    finally:
        receipt["duration_seconds"] = round(time.monotonic() - started, 3)
    return receipt


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--check",
        action="store_true",
        help="Run deterministic read-only validation (the default mode)",
    )
    parser.add_argument("--corpus", action="append", type=Path, help="Corpus directory; repeatable")
    parser.add_argument(
        "--strict",
        action="store_true",
        help="Fail on readability, legacy, uplift, and source-reference gaps",
    )
    parser.add_argument(
        "--verify",
        action="store_true",
        help="Run fixed package-test profiles; requires at least one --corpus",
    )
    parser.add_argument(
        "--verify-timeout-seconds",
        type=int,
        default=120,
        help="Per-corpus hard timeout for --verify (1-300; default 120)",
    )
    parser.add_argument("--json", action="store_true")
    args = parser.parse_args()
    if args.verify and not args.corpus:
        parser.error("--verify requires at least one explicit --corpus")
    if not 1 <= args.verify_timeout_seconds <= 300:
        parser.error("--verify-timeout-seconds must be between 1 and 300")
    corpora = [path.resolve() for path in args.corpus] if args.corpus else discover()
    payload = portfolio(
        sorted(corpora),
        strict=args.strict,
        enforce_inventory=not bool(args.corpus),
    )
    if args.verify:
        receipts = [
            _verify_corpus(corpus, args.verify_timeout_seconds)
            for corpus in payload["corpora"]
        ]
        payload["verification"] = {
            "valid": all(receipt["valid"] for receipt in receipts),
            "receipts": receipts,
        }
        payload["valid"] = payload["valid"] and payload["verification"]["valid"]
    if args.json:
        print(json.dumps(payload, indent=2))
    else:
        print(
            f"{'PASS' if payload['valid'] else 'FAIL'} corpora={payload['measurements']['corpora']} "
            f"features={payload['measurements']['feature_cards']} "
            f"legacy={payload['measurements']['legacy_documents']} "
            f"broken_links={payload['measurements']['broken_links']} "
            f"unresolved_refs={payload['measurements']['unresolved_source_refs']}"
        )
        for error in payload["errors"]:
            print(f"  ERROR: {error}")
        for item in payload["corpora"]:
            if item["errors"] or item["warnings"]:
                print(f"  {item['id']}: {'FAIL' if item['errors'] else 'WARN'}")
                for message in item["errors"]:
                    print(f"    ERROR: {message}")
                for message in item["warnings"]:
                    print(f"    WARNING: {message}")
        for receipt in payload.get("verification", {}).get("receipts", []):
            print(
                f"  VERIFY {receipt['corpus']}: "
                f"{'PASS' if receipt['valid'] else 'FAIL'} "
                f"duration={receipt['duration_seconds']}s"
            )
    return 0 if payload["valid"] else 1


if __name__ == "__main__":
    raise SystemExit(main())
