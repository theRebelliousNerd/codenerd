package core

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"codenerd/internal/atomicfile"
	"codenerd/internal/logging"
	"codenerd/internal/store"
)

// =============================================================================
// PARSED ORIENTATION PROJECTION, ACROSS PROCESSES
// =============================================================================
// The orientation projection is ~45k ground facts of Mangle text (4.8 MB on
// this repository). Parsing it with the Mangle parser cost ~0.45 s of every
// process start (measured 2026-10-29), though it changes only when
// orientation refreshes. The parse is remembered beside the workspace's
// other caches, keyed by the SHA-256 of the projection's exact bytes: the
// same text always parses to the same declarations and rows, so a cache
// whose key matches is that parse. Any other key is a miss, and the miss
// re-parses and replaces the cache.
//
// Layout: magic, key, uvarint-prefixed declarations, uvarint row count, and
// per row a uvarint-prefixed predicate and uvarint-prefixed arguments in the
// store's binary fact codec.

var projectionCacheMagic = []byte("CNORPRJ1")

// projectionCachePath is where the parsed projection at projectionPath
// (<workspace>/.nerd/orientation/orientation.mg) is remembered:
// <workspace>/.nerd/cache/orientation_projection.bin.
func projectionCachePath(projectionPath string) string {
	nerd := filepath.Dir(filepath.Dir(projectionPath))
	return filepath.Join(nerd, "cache", "orientation_projection.bin")
}

func encodeParsedProjection(key [sha256.Size]byte, decls string, facts []Fact) []byte {
	out := append([]byte(nil), projectionCacheMagic...)
	out = append(out, key[:]...)
	str := func(s []byte) {
		out = binary.AppendUvarint(out, uint64(len(s)))
		out = append(out, s...)
	}
	str([]byte(decls))
	out = binary.AppendUvarint(out, uint64(len(facts)))
	for _, f := range facts {
		str([]byte(f.Predicate))
		str(store.EncodeFactArgsBinary(f.Args))
	}
	return out
}

var errProjectionCacheMiss = errors.New("orientation projection cache miss")

func decodeParsedProjection(data []byte, key [sha256.Size]byte) (string, []Fact, error) {
	head := len(projectionCacheMagic) + sha256.Size
	if len(data) < head || !bytes.Equal(data[:len(projectionCacheMagic)], projectionCacheMagic) ||
		!bytes.Equal(data[len(projectionCacheMagic):head], key[:]) {
		return "", nil, errProjectionCacheMiss
	}
	i := head
	next := func() ([]byte, error) {
		n, w := binary.Uvarint(data[i:])
		if w <= 0 || uint64(len(data)-i-w) < n {
			return nil, fmt.Errorf("orientation projection cache: truncated")
		}
		i += w
		b := data[i : i+int(n)]
		i += int(n)
		return b, nil
	}
	decls, err := next()
	if err != nil {
		return "", nil, err
	}
	count, w := binary.Uvarint(data[i:])
	if w <= 0 {
		return "", nil, fmt.Errorf("orientation projection cache: truncated")
	}
	i += w
	facts := make([]Fact, 0, count)
	for range count {
		pred, err := next()
		if err != nil {
			return "", nil, err
		}
		encoded, err := next()
		if err != nil {
			return "", nil, err
		}
		args, err := store.DecodeFactArgsBinary(encoded)
		if err != nil {
			return "", nil, fmt.Errorf("orientation projection cache: %w", err)
		}
		facts = append(facts, Fact{Predicate: string(pred), Args: args})
	}
	if i != len(data) {
		return "", nil, fmt.Errorf("orientation projection cache: trailing bytes")
	}
	if len(facts) == 0 {
		facts = nil
	}
	return string(decls), facts, nil
}

// cachedProjectionParse returns the parse of body remembered at cachePath, or
// parses it with parse and remembers the result. A cache that cannot be read
// or written costs only the parse.
func cachedProjectionParse(cachePath string, body []byte, parse func() (string, []Fact, error)) (string, []Fact, error) {
	key := sha256.Sum256(body)
	if data, err := os.ReadFile(cachePath); err == nil {
		decls, facts, err := decodeParsedProjection(data, key)
		if err == nil {
			return decls, facts, nil
		}
		if !errors.Is(err, errProjectionCacheMiss) {
			logging.Get(logging.CategoryKernel).Warn("orientation projection cache %s unreadable, re-parsing: %v", cachePath, err)
		}
	}
	decls, facts, err := parse()
	if err != nil {
		return "", nil, err
	}
	if err := os.MkdirAll(filepath.Dir(cachePath), 0o755); err == nil {
		if err := atomicfile.WriteFile(cachePath, encodeParsedProjection(key, decls, facts), 0o644); err != nil {
			logging.Get(logging.CategoryKernel).Warn("orientation projection cache %s not written: %v", cachePath, err)
		}
	}
	return decls, facts, nil
}
