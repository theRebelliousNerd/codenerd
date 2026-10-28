#!/usr/bin/env bash
# new-lab.sh <slug> — scaffold a Mangle+Go experiment in _mangle_lab/<slug>/.
# _mangle_lab/ is gitignored, and Go skips "_" directories in ./..., so labs
# never reach the real build. Run one with: go test -v ./_mangle_lab/<slug>/
set -euo pipefail
slug="${1:?usage: new-lab.sh <slug>  (lowercase letters, digits, underscores)}"
[[ "$slug" =~ ^[a-z][a-z0-9_]*$ ]] || { echo "slug must match ^[a-z][a-z0-9_]*$" >&2; exit 2; }
here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
root="$(git -C "$here" rev-parse --show-toplevel)"
lab="$root/_mangle_lab"
mkdir -p "$lab/harness" "$lab/$slug"
cp "$here/lab/harness.go" "$lab/harness/harness.go"
dest="$lab/$slug/${slug}_test.go"
[[ -e "$dest" ]] || sed "s/SLUG/$slug/g" "$here/lab/template_test.go.txt" > "$dest"
echo "$dest"
