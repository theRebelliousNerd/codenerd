// Package factsnap persists slices of facts to disk using the Mangle fork's
// SimpleColumn columnar format, wrapped in either gzip or zstd compression.
//
// SimpleColumn writes a header (predicate symbols + arities + fact counts)
// followed by one column per argument position, which compresses extremely
// well compared with JSON: repeated predicate names and atom prefixes are
// stored once, columnar layout keeps similar values adjacent, and Mangle's
// percent-escaped constant encoding stays text-friendly while remaining
// dense.
//
// Files written by this package use one of two extensions:
//   - ".sc.gz"  — SimpleColumn + gzip       (default; no extra deps)
//   - ".sc.zst" — SimpleColumn + zstd       (smaller; requires klauspost/zstd)
//
// Both formats are auto-detected on Read based on the path suffix. Legacy
// JSON snapshots (".json") can be loaded via the LegacyJSON helper for
// migration paths.
package factsnap

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"codeberg.org/TauCeti/mangle-go/ast"
	"codeberg.org/TauCeti/mangle-go/factstore"
	"github.com/klauspost/compress/zstd"

	"codenerd/internal/types"
)

// ExtGzip is the canonical file extension for gzip-wrapped SimpleColumn snapshots.
const ExtGzip = ".sc.gz"

// ExtZstd is the canonical file extension for zstd-wrapped SimpleColumn snapshots.
const ExtZstd = ".sc.zst"

// ExtJSON is the legacy JSON snapshot extension recognised on read.
const ExtJSON = ".json"

// Codec selects the wrapping compression used by Write.
type Codec int

const (
	// CodecAuto picks gzip by default (matches the .sc.gz extension).
	CodecAuto Codec = iota
	// CodecGzip writes SimpleColumn data wrapped in gzip.
	CodecGzip
	// CodecZstd writes SimpleColumn data wrapped in zstd.
	CodecZstd
)

// Write serialises facts to path using SimpleColumn + gzip. The path should
// end in ExtGzip (".sc.gz"); if it does not, the extension is appended.
func Write(path string, facts []types.Fact) error {
	return WriteCodec(path, facts, CodecGzip)
}

// WriteCodec serialises facts to path using the requested codec. If the path
// suffix does not match the codec the canonical suffix is appended.
func WriteCodec(path string, facts []types.Fact, codec Codec) error {
	if codec == CodecAuto {
		codec = CodecGzip
	}
	path = ensureExt(path, codec)

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("factsnap: mkdir: %w", err)
	}

	store := factstore.NewSimpleInMemoryStore()
	for i, f := range facts {
		atom, err := f.ToAtom()
		if err != nil {
			return fmt.Errorf("factsnap: fact %d (%s) to atom: %w", i, f.Predicate, err)
		}
		store.Add(atom)
	}

	var raw bytes.Buffer
	sc := factstore.SimpleColumn{Deterministic: true}
	if err := sc.WriteTo(store, &raw); err != nil {
		return fmt.Errorf("factsnap: simplecolumn write: %w", err)
	}

	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return fmt.Errorf("factsnap: create %s: %w", tmp, err)
	}
	cleanupTmp := true
	defer func() {
		if cleanupTmp {
			_ = os.Remove(tmp)
		}
	}()

	switch codec {
	case CodecGzip:
		gw := gzip.NewWriter(f)
		if _, err := io.Copy(gw, &raw); err != nil {
			_ = gw.Close()
			_ = f.Close()
			return fmt.Errorf("factsnap: gzip write: %w", err)
		}
		if err := gw.Close(); err != nil {
			_ = f.Close()
			return fmt.Errorf("factsnap: gzip close: %w", err)
		}
	case CodecZstd:
		zw, err := zstd.NewWriter(f)
		if err != nil {
			_ = f.Close()
			return fmt.Errorf("factsnap: zstd writer: %w", err)
		}
		if _, err := io.Copy(zw, &raw); err != nil {
			_ = zw.Close()
			_ = f.Close()
			return fmt.Errorf("factsnap: zstd write: %w", err)
		}
		if err := zw.Close(); err != nil {
			_ = f.Close()
			return fmt.Errorf("factsnap: zstd close: %w", err)
		}
	default:
		_ = f.Close()
		return fmt.Errorf("factsnap: unknown codec %d", codec)
	}

	if err := f.Sync(); err != nil {
		_ = f.Close()
		return fmt.Errorf("factsnap: sync: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("factsnap: close: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("factsnap: rename %s -> %s: %w", tmp, path, err)
	}
	cleanupTmp = false
	return nil
}

// Read loads facts from path, auto-detecting the codec from the file suffix.
// Legacy ".json" snapshots are tolerated via LegacyJSON-compatible decoding
// when the slice was encoded with types.Fact's JSON shape.
func Read(path string) ([]types.Fact, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("factsnap: read %s: %w", path, err)
	}

	switch detectCodec(path) {
	case CodecGzip:
		store, err := factstore.NewSimpleColumnStoreFromGzipBytes(data)
		if err != nil {
			return nil, fmt.Errorf("factsnap: gzip decode %s: %w", path, err)
		}
		return collectFacts(store)
	case CodecZstd:
		store, err := factstore.NewSimpleColumnStoreFromZstdBytes(data)
		if err != nil {
			return nil, fmt.Errorf("factsnap: zstd decode %s: %w", path, err)
		}
		return collectFacts(store)
	default:
		// Legacy JSON path. We accept []types.Fact as JSON.
		var facts []types.Fact
		if err := json.Unmarshal(data, &facts); err != nil {
			return nil, fmt.Errorf("factsnap: legacy json decode %s: %w", path, err)
		}
		return facts, nil
	}
}

// LegacyJSON loads facts from a JSON snapshot regardless of extension. This is
// a migration helper for code paths that still emit JSON.
func LegacyJSON(path string) ([]types.Fact, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("factsnap: legacy read %s: %w", path, err)
	}
	var facts []types.Fact
	if err := json.Unmarshal(data, &facts); err != nil {
		return nil, fmt.Errorf("factsnap: legacy json decode %s: %w", path, err)
	}
	return facts, nil
}

// CanonicalPath returns path rewritten with the canonical extension for codec.
// Useful when a caller has a logical name and wants to know the on-disk file.
func CanonicalPath(path string, codec Codec) string {
	return ensureExt(path, codec)
}

func ensureExt(path string, codec Codec) string {
	switch codec {
	case CodecGzip:
		if strings.HasSuffix(path, ExtGzip) {
			return path
		}
		// Strip a bare ".json" or trailing dot, then append.
		path = strings.TrimSuffix(path, ExtJSON)
		return path + ExtGzip
	case CodecZstd:
		if strings.HasSuffix(path, ExtZstd) {
			return path
		}
		path = strings.TrimSuffix(path, ExtJSON)
		return path + ExtZstd
	default:
		return path
	}
}

func detectCodec(path string) Codec {
	switch {
	case strings.HasSuffix(path, ExtGzip):
		return CodecGzip
	case strings.HasSuffix(path, ExtZstd):
		return CodecZstd
	default:
		// Treat anything else as legacy JSON.
		return -1
	}
}

func collectFacts(store *factstore.SimpleColumnStore) ([]types.Fact, error) {
	if store == nil {
		return nil, errors.New("factsnap: nil store")
	}
	preds := store.ListPredicates()
	var out []types.Fact
	for _, p := range preds {
		if err := store.GetFacts(ast.NewQuery(p), func(a ast.Atom) error {
			out = append(out, atomToFact(a))
			return nil
		}); err != nil {
			return nil, fmt.Errorf("factsnap: collect facts for %s/%d: %w", p.Symbol, p.Arity, err)
		}
	}
	return out, nil
}

// atomToFact mirrors the conversion in internal/core/kernel_query.go but is
// duplicated here to avoid an import cycle. Argument typing intentionally
// matches the most common forms: name constants become MangleAtom (preserving
// the leading slash so round-trips stay symmetric), string/number/float/bool
// constants are returned as their Go equivalents.
func atomToFact(a ast.Atom) types.Fact {
	args := make([]any, len(a.Args))
	for i, term := range a.Args {
		args[i] = baseTermToValue(term)
	}
	return types.Fact{
		Predicate: a.Predicate.Symbol,
		Args:      args,
	}
}

func baseTermToValue(term ast.BaseTerm) any {
	c, ok := term.(ast.Constant)
	if !ok {
		return fmt.Sprintf("%v", term)
	}
	switch c.Type {
	case ast.NameType:
		// Preserve the slash so that Write -> Read returns MangleAtom-shaped
		// values that ToAtom() will encode the same way next round.
		return types.MangleAtom(c.Symbol)
	case ast.StringType, ast.BytesType:
		return c.Symbol
	case ast.NumberType:
		return c.NumValue
	case ast.Float64Type:
		v, _ := c.Float64Value()
		return v
	default:
		return c.Symbol
	}
}
