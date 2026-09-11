#!/usr/bin/env bash
# Dead-code budget: fail when unreachable production code grows.
#
# The recurring defect in this repo is capability that exists, is tested in
# isolation, and is never called. The 2026-09 hardening pass found several the
# hard way — a Thunderdome gate that always passed, two registered tools that
# errored on every call, a feature flag with no reader, 730 lines of
# impact-analysis with no production caller, a complete Mangle learning loop
# with no Go producer.
#
# Its Mangle twin is gated by TestStarvedPredicateBudget in
# internal/core/defaults. This is the Go half.
#
# It is a script rather than a Go test on purpose: the analysis needs
# golang.org/x/tools/cmd/deadcode, and a test that silently skips when the
# module cache is cold gives false assurance while one that requires the
# network makes `go test ./...` fragile. Run it in CI, where the network is a
# given.
#
#   scripts/deadcode-budget.sh          check against the baseline
#   scripts/deadcode-budget.sh --update rewrite the baseline
#
# The analysis is rapid type-aware reachability (RTA) from every main package,
# so unlike a grep it understands interfaces and method sets. It reports
# functions unreachable from any binary — which includes things that are fine
# (platform-specific paths, accessors kept for symmetry) alongside things that
# are not. The number is not meant to reach zero. It is meant not to grow
# without someone saying why.

set -euo pipefail

cd "$(dirname "$0")/.."

BASELINE="scripts/testdata/deadcode-baseline.txt"
# PINNED, not @latest.
#
# A drift gate whose own analysis tool floats can go red with no code change,
# and that is the worst failure available to a gate: a red nobody can reproduce
# teaches everybody to ignore the job, and then the drift it exists to catch
# goes unseen too. This repo has written that sentence down twice already,
# about the action linter and about the JSON budget, and then left @latest in
# the one gate whose baseline is a hash of an analysis result.
#
# It bit exactly that way: CI reported NewAnthropicClient as newly unreachable
# on a commit where the local run -- same code, same go.mod toolchain, same
# CGO_CFLAGS, same GOOS/GOARCH -- reported no drift, and the call is a plain
# one from newRawClientFromConfig in client_factory.go. The baseline is only a
# measurement if the thing measuring it holds still.
#
# Bump deliberately, with the baseline regenerated in the same commit, so a
# change in the analysis is a change somebody chose.
TOOL="golang.org/x/tools/cmd/deadcode@v0.50.0"

# The baseline is GOOS-specific, and this gate runs on Linux for that reason.
#
# Reachability is computed per build configuration, so a file excluded by build
# tag is not analysed and its functions are simply absent from the report — not
# "reachable", just invisible. This job first ran on windows-latest against a
# Linux-recorded baseline and reported 38 entries as "no longer unreachable":
# every function in platform_linux.go, platform_unix.go and open_other.go. None
# of them had changed. The reverse would report Windows-only code as newly dead.
#
# Pinning GOOS here is a guard, not a portability fix. It cannot make the gate
# run anywhere, because cross-compiling turns cgo off and this module depends on
# go-tree-sitter, which needs it — so a cross-GOOS analysis fails to typecheck
# rather than producing a different answer. The pin makes the mismatch loud and
# immediate instead of silently comparing two different build configurations.
# GOARCH is pinned with it because build tags select on both.
ANALYSIS_GOOS="${DEADCODE_GOOS:-linux}"
ANALYSIS_GOARCH="${DEADCODE_GOARCH:-amd64}"

TOOLBIN=""
ensure_tool() {
    [[ -n "$TOOLBIN" ]] && return
    local dir
    dir="$(mktemp -d)"
    # Built for the host: GOOS/GOARCH here decide what binary we get, so they
    # must stay native even though the analysis below targets another platform.
    GOBIN="$dir" go install "$TOOL" >&2
    TOOLBIN="$dir/deadcode"
    [[ -x "$TOOLBIN" ]] || TOOLBIN="$dir/deadcode.exe"
}

report() {
    # Strip line:col so the baseline survives edits that only move code, and
    # normalise separators and line endings: CI runs on Windows, where the tool
    # emits backslash paths and the shell may add CR. Without both, every entry
    # in a Linux-generated baseline reads as new.
    ensure_tool
    local raw err
    err="$(mktemp)"
    # Report analysis failure instead of dying silently. set -e plus pipefail
    # turns a failed analysis into a bare exit 1 with no output, which is the
    # worst thing a gate can do: it looks identical to a real finding and
    # teaches everyone to ignore the job.
    if ! raw="$(GOOS="$ANALYSIS_GOOS" GOARCH="$ANALYSIS_GOARCH" "$TOOLBIN" ./cmd/... 2>"$err")"; then
        echo "deadcode analysis failed for GOOS=$ANALYSIS_GOOS GOARCH=$ANALYSIS_GOARCH (host $(go env GOOS)):" >&2
        sed 's/^/  /' "$err" >&2
        if [[ "$ANALYSIS_GOOS" != "$(go env GOOS)" ]]; then
            echo >&2
            echo "The analysis target does not match the host, which disables cgo; this module" >&2
            echo "needs it (go-tree-sitter). Run this gate on $ANALYSIS_GOOS, or regenerate the" >&2
            echo "baseline for your platform with DEADCODE_GOOS=$(go env GOOS) $0 --update -- but" >&2
            echo "note the committed baseline is Linux's and the two are not interchangeable." >&2
        fi
        rm -f "$err"
        return 1
    fi
    rm -f "$err"
    printf '%s\n' "$raw" \
        | tr -d '\r' \
        | tr '\\' '/' \
        | sed -E 's/^([^:]+):[0-9]+:[0-9]+: unreachable func: /\1\t/' \
        | sort -u
}

mkdir -p "$(dirname "$BASELINE")"

if [[ "${1:-}" == "--update" ]]; then
    report > "$BASELINE"
    echo "Wrote $BASELINE ($(wc -l < "$BASELINE") entries)."
    exit 0
fi

if [[ ! -f "$BASELINE" ]]; then
    echo "No baseline at $BASELINE. Create it with: $0 --update" >&2
    exit 2
fi

CURRENT="$(mktemp)"
KNOWN="$(mktemp)"
trap 'rm -f "$CURRENT" "$KNOWN"' EXIT
report > "$CURRENT"
# Normalise the checked-in baseline the same way, so a CRLF checkout compares.
tr -d '\r' < "$BASELINE" | sort -u > "$KNOWN"

ADDED="$(comm -13 "$KNOWN" "$CURRENT" || true)"
REMOVED="$(comm -23 "$KNOWN" "$CURRENT" || true)"

status=0

if [[ -n "$ADDED" ]]; then
    echo "New unreachable functions ($(echo "$ADDED" | wc -l)):"
    echo "$ADDED" | sed 's/^/  /'
    echo
    echo "Each is production code nothing calls. Wire it, delete it, or — if it is"
    echo "genuinely reachable only from a path this analysis cannot see — record it"
    echo "with: $0 --update, and say why in the commit."
    status=1
fi

if [[ -n "$REMOVED" ]]; then
    echo "Functions that are no longer unreachable ($(echo "$REMOVED" | wc -l)):"
    echo "$REMOVED" | sed 's/^/  /'
    echo
    echo "Good. Refresh the baseline so the count stays a real measurement:"
    echo "  $0 --update"
    status=1
fi

if [[ $status -eq 0 ]]; then
    echo "Dead-code budget holds: $(wc -l < "$KNOWN") known unreachable functions, no drift."
fi

exit $status
