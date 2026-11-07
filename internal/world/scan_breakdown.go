package world

import (
	"sort"
	"strconv"
	"strings"

	"codenerd/internal/logging"
)

// scanBreakdownSample is how many paths of each kind the breakdown names. The
// counts are always complete; the sample is there so an operator can see
// which files a large count is made of without a debugger.
const scanBreakdownSample = 5

// logScanBreakdown records why an incremental scan re-parsed what it did:
// files whose bytes changed, files new to the manifest, files gone from it,
// Go siblings remapped by a declaration change, and importers re-resolved
// because the file set moved. A boot that reports hundreds of changed files
// on a tree with a handful of edits is a change-detection defect, and this
// line is what names the files.
func logScanBreakdown(root string, changed, newFiles, deleted, siblings, importers []string) {
	logging.World("incremental scan: changed=%d new=%d deleted=%d siblings=%d importers=%d%s",
		len(changed), len(newFiles), len(deleted), len(siblings), len(importers),
		scanBreakdownSamples(root, map[string][]string{
			"changed":   changed,
			"new":       newFiles,
			"deleted":   deleted,
			"siblings":  siblings,
			"importers": importers,
		}))
}

// scanBreakdownSamples renders up to scanBreakdownSample sorted, canonical
// paths per non-empty kind, in a fixed kind order.
func scanBreakdownSamples(root string, kinds map[string][]string) string {
	var b strings.Builder
	for _, kind := range []string{"changed", "new", "deleted", "siblings", "importers"} {
		paths := kinds[kind]
		if len(paths) == 0 {
			continue
		}
		canon := make([]string, len(paths))
		for i, p := range paths {
			canon[i] = canonicalScanPath(root, p)
		}
		sort.Strings(canon)
		if len(canon) > scanBreakdownSample {
			canon = canon[:scanBreakdownSample]
		}
		b.WriteString("; ")
		b.WriteString(kind)
		b.WriteString(": ")
		b.WriteString(strings.Join(canon, " "))
		if n := len(paths) - len(canon); n > 0 {
			b.WriteString(" (+")
			b.WriteString(strconv.Itoa(n))
			b.WriteString(" more)")
		}
	}
	return b.String()
}
