// Package snapshot is the workspace-level store for fact snapshots. It owns
// the canonical location (`<workspace>/.nerd/snapshots/`), the naming rules,
// and the listing/resolution logic; the on-disk encoding belongs to
// internal/persist/factsnap.
//
// The split exists so that callers (the `nerd snapshot` CLI, kernel debug
// export) never invent their own paths or extensions. A snapshot referenced by
// bare name resolves the same way from every call site, which is what makes
// "export here, import there" a usable operator workflow rather than a
// filename-guessing game.
//
// Rehydration policy: this package reads and writes files only. Asserting a
// snapshot's facts back into a kernel is deliberately left to the caller,
// because a snapshot is untrusted input the moment it leaves the process that
// wrote it — see Docs/architecture/persist/09-SAFETY-AND-INVARIANTS.md.
package snapshot

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"codenerd/internal/persist/factsnap"
	"codenerd/internal/types"
)

// DirName is the workspace-relative directory holding fact snapshots.
const DirName = ".nerd/snapshots"

// Dir returns the canonical snapshot directory for a workspace root.
func Dir(root string) string {
	if root == "" {
		root = "."
	}
	return filepath.Join(root, ".nerd", "snapshots")
}

// Entry describes one snapshot on disk.
type Entry struct {
	// Name is the logical name: the filename with its codec extension removed.
	Name string
	// Path is the absolute-or-workspace-relative file path.
	Path string
	// Codec is "gzip", "zstd" or "json" (legacy).
	Codec string
	Bytes int64
	// ModTime is the file's mtime, which is when the export landed.
	ModTime time.Time
	// Verifiable reports whether an integrity sidecar accompanies the file.
	Verifiable bool
}

// DefaultName builds a timestamped snapshot name, e.g. "kernel-20260815-140501".
// Timestamps sort lexicographically, which is why List can order by name and
// still read chronologically for same-prefix exports.
func DefaultName(prefix string) string {
	if prefix == "" {
		prefix = "snapshot"
	}
	return fmt.Sprintf("%s-%s", prefix, time.Now().Format("20060102-150405"))
}

// SanitizeName rejects anything that would escape the snapshot directory or
// produce a file the resolver could not find again. Snapshot names come from
// operator input and land in a filesystem path, so this is a containment
// boundary, not cosmetics.
func SanitizeName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("snapshot: empty name")
	}
	if name != filepath.Base(name) || strings.ContainsAny(name, `/\`) {
		return "", fmt.Errorf("snapshot: name %q must not contain a path separator", name)
	}
	if name == "." || name == ".." || strings.HasPrefix(name, ".") {
		return "", fmt.Errorf("snapshot: name %q must not start with a dot", name)
	}
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case r == '-', r == '_', r == '.':
		default:
			return "", fmt.Errorf("snapshot: name %q contains unsupported character %q "+
				"(use letters, digits, '-', '_', '.')", name, r)
		}
	}
	// A name that already carries a codec suffix would otherwise produce
	// "foo.sc.gz.sc.gz" on export and never resolve on import.
	name = strings.TrimSuffix(name, factsnap.ExtGzip)
	name = strings.TrimSuffix(name, factsnap.ExtZstd)
	if name == "" {
		return "", fmt.Errorf("snapshot: name is only a codec suffix")
	}
	return name, nil
}

// Export writes facts to the workspace snapshot directory under name and
// returns the file that was written.
func Export(root, name string, facts []types.Fact, codec factsnap.Codec) (string, error) {
	clean, err := SanitizeName(name)
	if err != nil {
		return "", err
	}
	return factsnap.WritePath(filepath.Join(Dir(root), clean), facts, factsnap.Options{Codec: codec})
}

// Resolve turns an operator-supplied reference into a snapshot path. A
// reference may be a full path, a bare filename, or a logical name; the last
// form is tried against every known codec extension so `nerd snapshot import
// kernel-20260815-140501` works without the operator typing ".sc.gz".
func Resolve(root, ref string) (string, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return "", fmt.Errorf("snapshot: empty reference")
	}

	candidates := []string{}
	if strings.ContainsAny(ref, `/\`) {
		// Explicit path: use it as given, but still allow the bare-name form
		// so "-w .. ./snap" style references resolve.
		candidates = append(candidates, ref, ref+factsnap.ExtGzip, ref+factsnap.ExtZstd)
	} else {
		base := filepath.Join(Dir(root), ref)
		candidates = append(candidates,
			base,
			base+factsnap.ExtGzip,
			base+factsnap.ExtZstd,
			base+factsnap.ExtJSON,
			ref,
		)
	}

	for _, c := range candidates {
		if st, err := os.Stat(c); err == nil && !st.IsDir() {
			return c, nil
		}
	}
	return "", fmt.Errorf("snapshot: no snapshot matching %q under %s", ref, Dir(root))
}

// Import resolves ref and decodes it. Integrity verification happens inside
// factsnap.Read when a sidecar is present.
func Import(root, ref string) ([]types.Fact, string, error) {
	path, err := Resolve(root, ref)
	if err != nil {
		return nil, "", err
	}
	facts, err := factsnap.Read(path)
	if err != nil {
		return nil, path, err
	}
	return facts, path, nil
}

// List enumerates snapshots newest-first. A missing directory is not an error:
// a workspace that has never exported anything is a normal state, not a fault.
func List(root string) ([]Entry, error) {
	dir := Dir(root)
	items, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("snapshot: read %s: %w", dir, err)
	}

	var out []Entry
	for _, item := range items {
		if item.IsDir() {
			continue
		}
		name := item.Name()
		// Sidecars and interrupted writes are bookkeeping, not snapshots.
		if strings.HasSuffix(name, factsnap.ExtSHA256) || strings.Contains(name, ".tmp") || strings.HasPrefix(name, ".") {
			continue
		}
		codec := ""
		logical := name
		switch {
		case strings.HasSuffix(name, factsnap.ExtGzip):
			codec, logical = "gzip", strings.TrimSuffix(name, factsnap.ExtGzip)
		case strings.HasSuffix(name, factsnap.ExtZstd):
			codec, logical = "zstd", strings.TrimSuffix(name, factsnap.ExtZstd)
		case strings.HasSuffix(name, factsnap.ExtJSON):
			codec, logical = "json", strings.TrimSuffix(name, factsnap.ExtJSON)
		default:
			continue
		}
		info, err := item.Info()
		if err != nil {
			continue
		}
		path := filepath.Join(dir, name)
		out = append(out, Entry{
			Name:       logical,
			Path:       path,
			Codec:      codec,
			Bytes:      info.Size(),
			ModTime:    info.ModTime(),
			Verifiable: factsnap.HasSidecar(path),
		})
	}

	sort.Slice(out, func(i, j int) bool {
		if !out[i].ModTime.Equal(out[j].ModTime) {
			return out[i].ModTime.After(out[j].ModTime)
		}
		return out[i].Name < out[j].Name
	})
	return out, nil
}

// PredicateCount is one row of a snapshot's shape summary.
type PredicateCount struct {
	Predicate string
	Count     int
}

// Summarize counts facts per predicate, largest first. Operators need to know
// what a snapshot contains before deciding to assert it, and printing 100k
// facts is not an answer.
func Summarize(facts []types.Fact) []PredicateCount {
	counts := map[string]int{}
	for _, f := range facts {
		counts[f.Predicate]++
	}
	out := make([]PredicateCount, 0, len(counts))
	for p, c := range counts {
		out = append(out, PredicateCount{Predicate: p, Count: c})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Predicate < out[j].Predicate
	})
	return out
}

// CodecFor maps an operator-facing codec name onto a factsnap codec.
func CodecFor(name string) (factsnap.Codec, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "", "auto", "gzip", "gz":
		return factsnap.CodecGzip, nil
	case "zstd", "zst":
		return factsnap.CodecZstd, nil
	default:
		return factsnap.CodecAuto, fmt.Errorf("snapshot: unknown codec %q (want gzip or zstd)", name)
	}
}
