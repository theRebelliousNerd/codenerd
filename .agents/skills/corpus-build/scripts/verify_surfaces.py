#!/usr/bin/env python3
"""Discover and verify integration surfaces for a subsystem.

Two modes:

1. LEGACY DISCOVERY MODE (default, unchanged engine): scans the actual
   directory structure to find candidate surfaces, then reports rough
   evidence counts. The corpus-wiring-auditor agent used this for manual
   DISCOVERY before classifying REQUIRED/OPTIONAL/N-A itself by reading
   the spec.

       python verify_surfaces.py <subsystem>
       python verify_surfaces.py causal --json          (prints JSON to stdout)
       python verify_surfaces.py causal --json out.json (writes JSON to a file)
       python verify_surfaces.py causal --category B

2. REGISTRY MODE (--registry): loads the machine-readable surface registry
   (references/surfaces.yaml), evaluates every entry's applicability
   predicate against a feature manifest (--manifest, the
   .corpus-build/manifests/<subsystem>_manifest.json schema) when given —
   degrading gracefully to path-based applicability when no manifest is
   supplied — mechanically runs each entry's detection greps/globs, and
   emits a PASS / FAIL / N-A / AMBIGUOUS verdict per surface with evidence
   (file:line for grep hits, path for glob hits) to stdout and, optionally,
   to a JSON file.

       python verify_surfaces.py --registry references/surfaces.yaml --subsystem causal
       python verify_surfaces.py --registry references/surfaces.yaml --subsystem causal \\
           --manifest ../../../.corpus-build/manifests/causal_manifest.json --json out.json

   Registry mode is Python-3-stdlib-only by design (no PyYAML dependency):
   references/surfaces.yaml is authored against a deliberately small YAML
   subset (see the module docstring of ``MiniYAML`` below) so this script
   can parse it without a third-party package. It NEVER executes
   go build/go test/make/etc. on its own — the registry's "verification"
   commands are printed as guidance for AMBIGUOUS rows, never run, unless
   a future caller explicitly opts in (there is no such flag today; running
   them is left to the human/agent that adjudicates AMBIGUOUS verdicts).
"""

import argparse
import json
import os
import re
import sys
from pathlib import Path


def find_project_root() -> Path:
    cwd = Path.cwd()
    for parent in [cwd, *cwd.parents]:
        if (parent / "go.mod").exists():
            return parent
    return cwd


def grep_file(path: Path, pattern: str) -> list:
    """Search a file for a pattern, return matching lines with numbers."""
    matches = []
    if not path.exists() or not path.is_file():
        return matches
    try:
        lines = path.read_text(
            encoding="utf-8", errors="replace",
        ).splitlines()
        compiled = re.compile(pattern, re.IGNORECASE)
        for i, line in enumerate(lines, 1):
            if compiled.search(line):
                matches.append({"line": i, "text": line.strip()})
    except (OSError, re.error):
        pass
    return matches


def check_dir_exists(root: Path, rel: str) -> bool:
    return (root / rel).exists()


# ===========================================================================
# LEGACY DISCOVERY MODE (unchanged engine — kept for manual/ad-hoc discovery)
# ===========================================================================


def discover_core_surfaces(root: Path, subsystem: str) -> list:
    """Category A: scan internal/* for registration points."""
    surfaces = []
    internal = root / "internal"
    if not internal.exists():
        return surfaces

    for d in sorted(internal.iterdir()):
        if not d.is_dir() or d.name == subsystem:
            continue
        # Check if subsystem is imported by this directory
        has_import = False
        import_pat = f"internal/{subsystem}"
        for go_file in d.rglob("*.go"):
            if go_file.name.endswith("_test.go"):
                continue
            try:
                content = go_file.read_text(
                    encoding="utf-8", errors="replace",
                )
                if import_pat in content:
                    has_import = True
                    break
            except OSError:
                continue

        surfaces.append({
            "category": "A",
            "id": f"A-{d.name}",
            "surface": f"internal/{d.name}/",
            "discovered": True,
            "has_import": has_import,
        })

    return surfaces


def discover_protocol_surfaces(root: Path, subsystem: str) -> list:
    """Category B: check protocol directories."""
    surfaces = []
    protocols = [
        ("B1", "internal/api/rest/", "REST API"),
        ("B2", "internal/api/middleware/", "REST middleware"),
        ("B3", "internal/api/graphql/", "GraphQL"),
        ("B4", "internal/api/grpc/", "gRPC"),
        ("B5", "internal/api/realtime/", "WebSocket"),
        ("B6", "internal/protocols/mcp/", "MCP protocol"),
        ("B7", "internal/protocols/a2a/", "A2A protocol"),
        ("B8", "internal/adktools/", "ADK tools"),
    ]
    for sid, path, name in protocols:
        full = root / path
        evidence = []
        if full.exists():
            for f in full.rglob("*.go"):
                matches = grep_file(f, subsystem)
                for m in matches[:3]:
                    evidence.append({
                        "file": str(f.relative_to(root)),
                        "line": m["line"],
                    })
        surfaces.append({
            "category": "B", "id": sid, "surface": name,
            "dir_exists": full.exists(),
            "references_found": len(evidence),
            "evidence": evidence[:3],
        })
    return surfaces


def discover_codegen_surfaces(root: Path) -> list:
    """Category C: check codegen artifacts."""
    surfaces = []
    checks = [
        ("C1", "docs/api/openapi.v1.json", "OpenAPI spec"),
        ("C2", "web/dashboard/src/services/generated/",
         "Orval API client"),
        ("C3", "web/dashboard/src/services/generated/websockets.ts",
         "WebSocket TS types"),
        ("C4", "proto/", "Protobuf definitions"),
    ]
    for sid, path, name in checks:
        full = root / path
        surfaces.append({
            "category": "C", "id": sid, "surface": name,
            "exists": full.exists(),
        })
    return surfaces


def discover_client_surfaces(root: Path, subsystem: str) -> list:
    """Category D: check pkg/ client libraries."""
    surfaces = []
    checks = [
        ("D1", "pkg/client/go/", "Go client"),
        ("D2", "pkg/cli/", "CLI definitions"),
        ("D3", "pkg/sdk/", "SDK types"),
    ]
    for sid, path, name in checks:
        full = root / path
        refs = 0
        if full.exists():
            for f in full.rglob("*.go"):
                matches = grep_file(f, subsystem)
                refs += len(matches)
        surfaces.append({
            "category": "D", "id": sid, "surface": name,
            "dir_exists": full.exists(),
            "references_found": refs,
        })
    return surfaces


def discover_binary_surfaces(root: Path, subsystem: str) -> list:
    """Category E: check cmd/ binaries."""
    surfaces = []
    checks = [
        ("E1", "cmd/vectryx/", "Main server"),
        ("E3", "cmd/vectryx-cli/", "CLI tool"),
        ("E4", "cmd/vectryx-seed/", "Seed tool"),
    ]
    for sid, path, name in checks:
        full = root / path
        refs = 0
        if full.exists():
            for f in full.rglob("*.go"):
                matches = grep_file(f, subsystem)
                refs += len(matches)
        surfaces.append({
            "category": "E", "id": sid, "surface": name,
            "dir_exists": full.exists(),
            "references_found": refs,
        })
    return surfaces


def discover_frontend_surfaces(root: Path, subsystem: str) -> list:
    """Category F: check web/dashboard/ and page agents."""
    surfaces = []
    dashboard = root / "web" / "dashboard" / "src"
    agent_dir = root / "internal" / "agents" / "permanent"

    # Dashboard components
    refs = 0
    if dashboard.exists():
        for f in dashboard.rglob("*.tsx"):
            matches = grep_file(f, subsystem)
            refs += len(matches)
        for f in dashboard.rglob("*.ts"):
            if f.suffix == ".tsx":
                continue
            matches = grep_file(f, subsystem)
            refs += len(matches)

    surfaces.append({
        "category": "F", "id": "F1-F5",
        "surface": "Dashboard components",
        "references_found": refs,
    })

    # Page agent
    has_agent = False
    if agent_dir.exists():
        for d in agent_dir.iterdir():
            if d.is_dir() and subsystem in d.name:
                has_agent = True
                break

    surfaces.append({
        "category": "F", "id": "F8-F10",
        "surface": "Pagekit agent + tools + corpus",
        "page_agent_exists": has_agent,
    })

    return surfaces


def discover_config_surfaces(root: Path, subsystem: str) -> list:
    """Category G: check config files."""
    surfaces = []
    configs = ["default", "development", "testing", "production"]
    for cfg in configs:
        path = root / "configs" / f"{cfg}.yaml"
        found = False
        if path.exists():
            matches = grep_file(path, subsystem)
            found = len(matches) > 0
        surfaces.append({
            "category": "G",
            "id": f"G-{cfg}",
            "surface": f"configs/{cfg}.yaml",
            "has_section": found,
        })
    return surfaces


def discover_test_surfaces(root: Path, subsystem: str) -> list:
    """Category I: check test existence."""
    source = root / "internal" / subsystem
    surfaces = []

    test_count = 0
    bench_count = 0
    if source.exists():
        for f in source.rglob("*_test.go"):
            test_count += 1
            try:
                c = f.read_text(encoding="utf-8", errors="replace")
                bench_count += len(
                    re.findall(r"^func Benchmark", c, re.MULTILINE),
                )
            except OSError:
                pass

    surfaces.append({
        "category": "I", "id": "I1-I3",
        "surface": "Tests + benchmarks",
        "test_files": test_count,
        "benchmark_funcs": bench_count,
    })
    return surfaces


def run_legacy_discovery(root: Path, args) -> None:
    all_surfaces = []

    all_surfaces.extend(discover_core_surfaces(root, args.subsystem))
    all_surfaces.extend(
        discover_protocol_surfaces(root, args.subsystem),
    )
    all_surfaces.extend(discover_codegen_surfaces(root))
    all_surfaces.extend(
        discover_client_surfaces(root, args.subsystem),
    )
    all_surfaces.extend(
        discover_binary_surfaces(root, args.subsystem),
    )
    all_surfaces.extend(
        discover_frontend_surfaces(root, args.subsystem),
    )
    all_surfaces.extend(
        discover_config_surfaces(root, args.subsystem),
    )
    all_surfaces.extend(
        discover_test_surfaces(root, args.subsystem),
    )

    if args.category:
        all_surfaces = [
            s for s in all_surfaces
            if s["category"] == args.category.upper()
        ]

    result = {
        "subsystem": args.subsystem,
        "surfaces_discovered": len(all_surfaces),
        "surfaces": all_surfaces,
    }

    if args.json == "-":
        print(json.dumps(result, indent=2, default=str))
    elif args.json:
        with open(args.json, "w", encoding="utf-8") as fh:
            json.dump(result, fh, indent=2, default=str)
        print(f"Wrote {args.json}")
    else:
        for s in all_surfaces:
            cat = s["category"]
            sid = s["id"]
            name = s["surface"]
            detail = ""
            if "has_import" in s:
                detail = " (imported)" if s["has_import"] else ""
            elif "references_found" in s:
                detail = f" ({s['references_found']} refs)"
            elif "has_section" in s:
                detail = " (found)" if s["has_section"] else " (missing)"
            elif "exists" in s:
                detail = " (exists)" if s["exists"] else " (missing)"
            print(f"[{cat}] {sid:8s} {name:40s}{detail}")
        print(f"\n{len(all_surfaces)} surfaces discovered")


# ===========================================================================
# MINI-YAML: stdlib-only loader for the deliberately small subset of YAML
# used by references/surfaces.yaml. Supported grammar:
#   - top-level document is a single block sequence ("- id: ...")
#   - block mappings, 2-space indentation
#   - block sequences ("- " markers), including sequences of mappings
#     ("- grep: X" then sibling keys "  dir: Y" at +2 indent)
#   - flow sequences on one line: [a, b, c] / [] (simple scalars only)
#   - scalars: single/double-quoted strings, bare words, true/false/null,
#     ints/floats
#   - full-line comments only (a line whose first non-space char is '#').
#     NO trailing inline comments — keep prose on its own comment line.
#   - blank lines ignored
# Anything outside this subset (anchors, multi-doc streams, block scalars
# |/>, flow mappings {a: b}, tabs) is intentionally unsupported; authors of
# surfaces.yaml must stay inside the subset above.
# ===========================================================================


class MiniYAMLError(ValueError):
    pass


def _mini_yaml_tokenize(text: str) -> list:
    """Return a list of (indent, content) for every non-blank, non-comment
    line, preserving original line numbers for error messages."""
    tokens = []
    for lineno, raw in enumerate(text.splitlines(), 1):
        stripped = raw.rstrip("\r\n")
        lstripped = stripped.lstrip(" ")
        if lstripped == "" or lstripped.startswith("#"):
            continue
        if "\t" in stripped[: len(stripped) - len(lstripped)]:
            raise MiniYAMLError(f"line {lineno}: tabs not supported in indentation")
        indent = len(stripped) - len(lstripped)
        tokens.append((indent, lstripped, lineno))
    return tokens


def _mini_yaml_split_flow_list(inner: str) -> list:
    items = []
    buf = []
    quote = None
    for ch in inner:
        if quote:
            buf.append(ch)
            if ch == quote:
                quote = None
            continue
        if ch in ("'", '"'):
            quote = ch
            buf.append(ch)
            continue
        if ch == ",":
            items.append("".join(buf).strip())
            buf = []
            continue
        buf.append(ch)
    tail = "".join(buf).strip()
    if tail:
        items.append(tail)
    return items


def _mini_yaml_scalar(raw: str):
    s = raw.strip()
    if s == "":
        return None
    if len(s) >= 2 and s[0] == s[-1] == "'":
        return s[1:-1]
    if len(s) >= 2 and s[0] == s[-1] == '"':
        return s[1:-1]
    if s in ("true", "True"):
        return True
    if s in ("false", "False"):
        return False
    if s in ("null", "~", "None"):
        return None
    if s.startswith("[") and s.endswith("]"):
        inner = s[1:-1].strip()
        if inner == "":
            return []
        return [_mini_yaml_scalar(it) for it in _mini_yaml_split_flow_list(inner)]
    if re.match(r"^-?\d+$", s):
        return int(s)
    if re.match(r"^-?\d+\.\d+$", s):
        return float(s)
    return s


def _mini_yaml_split_key_value(content: str, lineno: int):
    if ":" not in content:
        raise MiniYAMLError(f"line {lineno}: expected 'key: value' or 'key:', got: {content!r}")
    key, _, rest = content.partition(":")
    return key.strip(), rest.strip()


def _mini_yaml_parse_block(tokens: list, i: int, indent: int):
    """Parse the block starting at tokens[i], which must be at exactly
    `indent`. Returns (value, next_i)."""
    if i >= len(tokens) or tokens[i][0] != indent:
        return None, i

    if tokens[i][1].startswith("- "):
        result = []
        while i < len(tokens) and tokens[i][0] == indent and tokens[i][1].startswith("- "):
            item_content = tokens[i][1][2:]
            lineno = tokens[i][2]
            if item_content == "":
                i += 1
                if i < len(tokens) and tokens[i][0] > indent:
                    subvalue, i = _mini_yaml_parse_block(tokens, i, tokens[i][0])
                    result.append(subvalue)
                else:
                    result.append(None)
                continue
            if ":" in item_content:
                mapping = {}
                key, val = _mini_yaml_split_key_value(item_content, lineno)
                i += 1
                if val == "":
                    if i < len(tokens) and tokens[i][0] > indent:
                        subvalue, i = _mini_yaml_parse_block(tokens, i, tokens[i][0])
                        mapping[key] = subvalue
                    else:
                        mapping[key] = None
                else:
                    mapping[key] = _mini_yaml_scalar(val)
                # sibling keys of the same mapping item, at indent+2
                sib_indent = indent + 2
                while (
                    i < len(tokens)
                    and tokens[i][0] == sib_indent
                    and not tokens[i][1].startswith("- ")
                ):
                    k2, v2 = _mini_yaml_split_key_value(tokens[i][1], tokens[i][2])
                    i += 1
                    if v2 == "":
                        if i < len(tokens) and tokens[i][0] > sib_indent:
                            subvalue, i = _mini_yaml_parse_block(tokens, i, tokens[i][0])
                            mapping[k2] = subvalue
                        else:
                            mapping[k2] = None
                    else:
                        mapping[k2] = _mini_yaml_scalar(v2)
                result.append(mapping)
            else:
                result.append(_mini_yaml_scalar(item_content))
                i += 1
        return result, i

    result = {}
    while i < len(tokens) and tokens[i][0] == indent and not tokens[i][1].startswith("- "):
        key, val = _mini_yaml_split_key_value(tokens[i][1], tokens[i][2])
        i += 1
        if val == "":
            if i < len(tokens) and tokens[i][0] > indent:
                subvalue, i = _mini_yaml_parse_block(tokens, i, tokens[i][0])
                result[key] = subvalue
            else:
                result[key] = None
        else:
            result[key] = _mini_yaml_scalar(val)
    return result, i


def mini_yaml_load(text: str):
    """Parse the surfaces.yaml subset described above. Returns whatever the
    top-level document is (a list, for surfaces.yaml)."""
    tokens = _mini_yaml_tokenize(text)
    if not tokens:
        return []
    value, next_i = _mini_yaml_parse_block(tokens, 0, tokens[0][0])
    if next_i != len(tokens):
        raise MiniYAMLError(
            f"trailing unparsed content starting at line {tokens[next_i][2]}",
        )
    return value


def load_registry(path: Path) -> list:
    text = path.read_text(encoding="utf-8")
    data = mini_yaml_load(text)
    if not isinstance(data, list):
        raise MiniYAMLError(f"{path}: expected a top-level YAML sequence of surface entries")
    return data


# ===========================================================================
# REGISTRY MODE: predicate evaluation, detection, verdicts
# ===========================================================================

DEFAULT_EXTENSIONS = (
    ".go", ".ts", ".tsx", ".py", ".md", ".yaml", ".yml", ".json", ".mg",
)

SKIP_DIR_NAMES = {"node_modules", ".git", "dist", "build", "vendor", ".next", "coverage"}

# Hard cap on files scanned by a single detection call — this repo's
# web/dashboard/src/ alone holds 2,500+ .ts/.tsx files; several rows target
# it or its subdirectories. The cap bounds worst-case runtime regardless of
# repo size; a truncation note is folded into the evidence when it fires.
MAX_FILES_PER_SCAN = 4000

# Process-wide caches, shared across every registry entry evaluated in one
# run. Multiple rows legitimately re-scan overlapping directories (e.g.
# F1-F7 all touch subtrees of web/dashboard/src/); listing + content are
# read from disk at most once per path per run.
_DIR_LISTING_CACHE = {}
_FILE_LINES_CACHE = {}
_USER_FACING_CACHE = {}
_NEW_PUBLIC_OP_CACHE = {}


def _iter_files(dpath: Path, exts=DEFAULT_EXTENSIONS):
    """Yield matching-extension files under dpath, pruning SKIP_DIR_NAMES
    subtrees during the walk (not after) and caching the listing per path
    so repeated scans of the same directory in one process are free."""
    if not dpath.exists():
        return
    if dpath.is_file():
        yield dpath
        return
    key = (str(dpath), exts)
    cached = _DIR_LISTING_CACHE.get(key)
    if cached is not None:
        yield from cached
        return
    found = []
    for dirpath, dirnames, filenames in os.walk(dpath):
        dirnames[:] = [d for d in dirnames if d not in SKIP_DIR_NAMES]
        for name in filenames:
            suffix = Path(name).suffix.lower()
            if suffix in exts:
                found.append(Path(dirpath) / name)
                if len(found) >= MAX_FILES_PER_SCAN:
                    break
        if len(found) >= MAX_FILES_PER_SCAN:
            break
    _DIR_LISTING_CACHE[key] = found
    yield from found


def _cached_grep(path: Path, pattern: str) -> list:
    """grep_file with a per-process file-content cache — many detection
    entries re-scan the same file (e.g. internal/config/ under both a
    subsystem-specific grep and a WatchConfig grep)."""
    key = str(path)
    lines = _FILE_LINES_CACHE.get(key)
    if lines is None:
        if not path.exists() or not path.is_file():
            _FILE_LINES_CACHE[key] = []
            return []
        try:
            lines = path.read_text(encoding="utf-8", errors="replace").splitlines()
        except OSError:
            lines = []
        _FILE_LINES_CACHE[key] = lines
    matches = []
    try:
        compiled = re.compile(pattern, re.IGNORECASE)
    except re.error:
        return matches
    for i, line in enumerate(lines, 1):
        if compiled.search(line):
            matches.append({"line": i, "text": line.strip()})
    return matches


def _manifest_feature_field_values(feature: dict, field: str):
    """Return a list of string values to substring-match against for a
    given manifest feature + field name (handles list-typed and
    scalar-typed fields uniformly)."""
    if field not in feature:
        return []
    val = feature[field]
    if val is None:
        return []
    if isinstance(val, list):
        return [str(v) for v in val]
    return [str(val)]


def resolve_dirs(dir_field: str, subsystem: str, manifest: dict, root: Path) -> list:
    """Resolve a registry `dir:` template to concrete on-disk directories
    (or a single file path). Handles the `<subsystem>` placeholder and, for
    virtual subsystems whose code does not literally live at
    internal/<subsystem>/, falls back to the manifest's source_paths."""
    raw = dir_field.replace("<subsystem>", subsystem)
    p = root / raw
    dirs = []
    if p.exists():
        dirs.append(p)
    # Virtual subsystems (e.g. causal) often have NO internal/<subsystem>/
    # of their own, but a same-named directory can also exist for an
    # unrelated reason (a coincidental match) — so when the template used
    # the <subsystem> placeholder as ITS OWN source directory, union the
    # literal path (if any) WITH the manifest's real source_paths rather
    # than letting either one silently shadow the other.
    if "<subsystem>" in dir_field and manifest:
        for sp in manifest.get("source_paths") or []:
            sp_path = root / sp
            if sp_path.is_file():
                sp_path = sp_path.parent
            if sp_path.exists() and sp_path not in dirs:
                dirs.append(sp_path)
    if dirs:
        return dirs
    return [p]


def eval_feature_tag(tag, manifest) -> bool:
    if not manifest:
        return False
    for feat in manifest.get("features", []) or []:
        tags = feat.get("tags") or []
        if any(str(tag).lower() == str(t).lower() for t in tags):
            return True
    return False


def eval_manifest_field_contains(spec: dict, manifest, subsystem: str, root: Path) -> bool:
    field = spec.get("field", "")
    value = str(spec.get("value", "")).lower()
    if manifest:
        for feat in manifest.get("features", []) or []:
            for v in _manifest_feature_field_values(feat, field):
                if value in v.lower():
                    return True
        return False
    # Degrade gracefully: no manifest -> grep the subsystem's own default
    # source directory for the keyword as a substitute signal.
    default_dir = root / "internal" / subsystem
    if default_dir.exists():
        for f in _iter_files(default_dir):
            if _cached_grep(f, re.escape(str(spec.get("value", "")))):
                return True
    return False


def eval_new_public_operation(manifest, subsystem: str, root: Path) -> bool:
    cache_key = (id(manifest), subsystem)
    cached = _NEW_PUBLIC_OP_CACHE.get(cache_key)
    if cached is not None:
        return cached
    exported_func_re = r"^func\s+(\([^)]*\)\s*)?[A-Z]"
    result = False
    if manifest:
        for feat in manifest.get("features", []) or []:
            for fn in feat.get("functions", []) or []:
                if re.search(exported_func_re, str(fn)):
                    result = True
                    break
            if result:
                break
    else:
        default_dir = root / "internal" / subsystem
        if default_dir.exists():
            for f in _iter_files(default_dir, (".go",)):
                if not f.name.endswith("_test.go") and _cached_grep(f, exported_func_re):
                    result = True
                    break
    _NEW_PUBLIC_OP_CACHE[cache_key] = result
    return result


def eval_subsystem_is_user_facing(manifest, subsystem: str, root: Path) -> bool:
    cache_key = (id(manifest), subsystem)
    cached = _USER_FACING_CACHE.get(cache_key)
    if cached is not None:
        return cached

    keywords = ("agents/permanent", "dashboard", "web/dashboard")
    result = False
    if manifest:
        for feat in manifest.get("features", []) or []:
            for ip in feat.get("integration_points", []) or []:
                low = str(ip).lower()
                if any(k in low for k in keywords):
                    result = True
                    break
            if result:
                break

    if not result:
        dash = root / "web" / "dashboard" / "src"
        if dash.exists():
            low_sub = subsystem.lower()
            for dirpath, dirnames, filenames in os.walk(dash):
                dirnames[:] = [d for d in dirnames if d not in SKIP_DIR_NAMES]
                if any(low_sub in name.lower() for name in filenames):
                    result = True
                    break

    if not result:
        agent_dir = root / "internal" / "agents" / "permanent"
        if agent_dir.exists():
            for d in agent_dir.iterdir():
                if d.is_dir() and subsystem.lower() in d.name.lower():
                    result = True
                    break

    _USER_FACING_CACHE[cache_key] = result
    return result


def eval_touches_paths(target_paths: list, subsystem: str, manifest, root: Path) -> bool:
    resolved_targets = [
        str(p).replace("<subsystem>", subsystem).replace("\\", "/").rstrip("/")
        for p in target_paths
    ]
    if manifest and manifest.get("source_paths"):
        candidates = [str(p) for p in manifest["source_paths"]]
    else:
        candidates = [f"internal/{subsystem}/"]
    for cand in candidates:
        cand_norm = cand.replace("\\", "/")
        for tgt in resolved_targets:
            if not tgt:
                continue
            if cand_norm.startswith(tgt) or tgt in cand_norm:
                return True
    return False


def eval_predicate(pred: dict, subsystem: str, manifest, root: Path) -> bool:
    if not isinstance(pred, dict) or len(pred) != 1:
        raise MiniYAMLError(f"malformed predicate entry: {pred!r}")
    (name, val), = pred.items()
    if name == "always":
        return bool(val)
    if name == "feature_tag":
        return eval_feature_tag(val, manifest)
    if name == "manifest_field_contains":
        return eval_manifest_field_contains(val, manifest, subsystem, root)
    if name == "new_public_operation":
        result = eval_new_public_operation(manifest, subsystem, root)
        return result if val else not result
    if name == "subsystem_is_user_facing":
        result = eval_subsystem_is_user_facing(manifest, subsystem, root)
        return result if val else not result
    if name == "touches_paths":
        return eval_touches_paths(val, subsystem, manifest, root)
    raise MiniYAMLError(f"unknown applicability predicate: {name!r}")


def eval_applicability(app: dict, subsystem: str, manifest, root: Path) -> bool:
    if not app:
        return False
    if "any" in app:
        return any(
            eval_predicate(p, subsystem, manifest, root) for p in (app["any"] or [])
        )
    if "all" in app:
        return all(
            eval_predicate(p, subsystem, manifest, root) for p in (app["all"] or [])
        )
    return False


def run_detection(entry: dict, subsystem: str, manifest, root: Path):
    """Returns (per_detection_hit: list[bool], evidence: list[str])."""
    dets = entry.get("detection") or []
    per_hit = []
    evidence = []
    for d in dets:
        dir_field = d.get("dir", "")
        dirs = resolve_dirs(dir_field, subsystem, manifest, root)
        if "grep" in d:
            pattern = d["grep"].replace("<subsystem>", subsystem)
            found = []
            for dpath in dirs:
                for f in _iter_files(dpath):
                    for m in _cached_grep(f, pattern):
                        try:
                            rel = f.relative_to(root).as_posix()
                        except ValueError:
                            rel = str(f)
                        found.append(f"{rel}:{m['line']}")
                        if len(found) >= 5:
                            break
                    if len(found) >= 5:
                        break
                if len(found) >= 5:
                    break
            per_hit.append(bool(found))
            evidence.extend(found[:5])
        elif "glob" in d:
            pattern = d["glob"].replace("<subsystem>", subsystem)
            found = []
            for dpath in dirs:
                if not dpath.exists() or dpath.is_file():
                    continue
                for m in dpath.glob(pattern):
                    try:
                        rel = m.relative_to(root).as_posix()
                    except ValueError:
                        rel = str(m)
                    found.append(rel)
                    if len(found) >= 5:
                        break
                if len(found) >= 5:
                    break
            per_hit.append(bool(found))
            evidence.extend(found[:5])
        else:
            raise MiniYAMLError(f"detection entry has neither 'grep' nor 'glob': {d!r}")
    return per_hit, evidence


def evaluate_entry(entry: dict, subsystem: str, manifest, root: Path) -> dict:
    applicable = eval_applicability(entry.get("applicability") or {}, subsystem, manifest, root)
    judgment = bool(entry.get("judgment", False))
    verdict = None
    evidence = []
    if not applicable:
        verdict = "N-A"
    elif judgment:
        verdict = "AMBIGUOUS"
    else:
        per_hit, evidence = run_detection(entry, subsystem, manifest, root)
        if not per_hit:
            # No detections registered but judgment wasn't set — defensive
            # fallback; treat as needing adjudication rather than a silent FAIL.
            verdict = "AMBIGUOUS"
        elif all(per_hit):
            verdict = "PASS"
        elif not any(per_hit):
            verdict = "FAIL"
        else:
            verdict = "AMBIGUOUS"
    return {
        "id": entry.get("id"),
        "group": entry.get("group"),
        "category": entry.get("category"),
        "name": entry.get("name"),
        "fix_owner": entry.get("fix_owner"),
        "applicable": applicable,
        "verdict": verdict,
        "evidence": evidence,
        "verification": [v.get("cmd") for v in (entry.get("verification") or [])],
    }


def run_registry_mode(root: Path, args) -> None:
    registry_path = Path(args.registry)
    if not registry_path.is_absolute():
        registry_path = (Path.cwd() / registry_path).resolve()
    entries = load_registry(registry_path)

    manifest = None
    if args.manifest:
        manifest_path = Path(args.manifest)
        if not manifest_path.is_absolute():
            manifest_path = (Path.cwd() / manifest_path).resolve()
        if manifest_path.exists():
            manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
        else:
            print(f"warning: --manifest path does not exist: {manifest_path}", file=sys.stderr)

    if args.category:
        entries = [e for e in entries if str(e.get("group", "")).upper() == args.category.upper()]

    results = [evaluate_entry(e, args.subsystem, manifest, root) for e in entries]

    counts = {"PASS": 0, "FAIL": 0, "N-A": 0, "AMBIGUOUS": 0}
    for r in results:
        counts[r["verdict"]] = counts.get(r["verdict"], 0) + 1

    output = {
        "subsystem": args.subsystem,
        "registry": str(registry_path),
        "manifest": str(args.manifest) if args.manifest else None,
        "total_surfaces": len(results),
        "counts": counts,
        "results": results,
    }

    if args.json == "-":
        # Bare `--json`: JSON-only to stdout (legacy convenience), no table.
        print(json.dumps(output, indent=2, default=str))
        return

    if args.json:
        out_path = Path(args.json)
        with open(out_path, "w", encoding="utf-8") as fh:
            json.dump(output, fh, indent=2, default=str)
        print(f"Wrote {out_path}")

    # Human table always prints (in addition to a --json <path> file write).
    for r in results:
        mark = {
            "PASS": "PASS", "FAIL": "FAIL",
            "N-A": "N-A ", "AMBIGUOUS": "AMBI",
        }.get(r["verdict"], r["verdict"])
        ev = f" -- {r['evidence'][0]}" if r["evidence"] else ""
        print(f"[{mark}] {r['id']:32s} ({r['group']}) {r['name']}{ev}")

    print(
        f"\n{output['total_surfaces']} surfaces evaluated for '{args.subsystem}': "
        f"PASS={counts['PASS']} FAIL={counts['FAIL']} "
        f"N-A={counts['N-A']} AMBIGUOUS={counts['AMBIGUOUS']}",
    )


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("subsystem", nargs="?", help="Subsystem name (legacy positional form)")
    parser.add_argument("--subsystem", dest="subsystem_flag", help="Subsystem name (registry mode)")
    parser.add_argument(
        "--json", nargs="?", const="-", default=None,
        help="Bare flag prints JSON to stdout (legacy behavior); "
             "`--json <path>` additionally/instead writes JSON to that path.",
    )
    parser.add_argument("--category", help="Filter by category/group letter (A-I)")
    parser.add_argument("--registry", help="Path to surfaces.yaml — enables REGISTRY MODE")
    parser.add_argument("--manifest", help="Path to a .corpus-build/manifests/<subsystem>_manifest.json")
    args = parser.parse_args()

    # Reconcile subsystem from either the legacy positional arg or --subsystem.
    if args.subsystem_flag:
        args.subsystem = args.subsystem_flag
    if not args.subsystem:
        parser.error("subsystem is required (positional, or --subsystem in registry mode)")

    root = find_project_root()

    if args.registry:
        run_registry_mode(root, args)
        return

    run_legacy_discovery(root, args)


if __name__ == "__main__":
    main()
