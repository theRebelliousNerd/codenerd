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
TOOL="golang.org/x/tools/cmd/deadcode@latest"

report() {
    # Strip line:col so the baseline survives edits that only move code.
    go run "$TOOL" ./cmd/... 2>/dev/null \
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
trap 'rm -f "$CURRENT"' EXIT
report > "$CURRENT"

ADDED="$(comm -13 "$BASELINE" "$CURRENT" || true)"
REMOVED="$(comm -23 "$BASELINE" "$CURRENT" || true)"

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
    echo "Dead-code budget holds: $(wc -l < "$BASELINE") known unreachable functions, no drift."
fi

exit $status
