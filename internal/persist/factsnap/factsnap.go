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
// Both formats are auto-detected on Read from the path suffix, falling back to
// a container magic-byte sniff when the suffix was lost (renamed, copied
// through a tool that strips extensions). Legacy JSON snapshots (".json") can
// be loaded via the LegacyJSON helper for migration paths.
//
// Every write also emits a `<path>.sha256` sidecar in sha256sum(1) format.
// Read verifies it when present, so a snapshot truncated by a full disk or
// corrupted in transit fails loudly instead of silently yielding a short fact
// set that a caller would then assert into the kernel as truth.
package factsnap

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"codeberg.org/TauCeti/mangle-go/ast"
	"codeberg.org/TauCeti/mangle-go/factstore"
	"github.com/klauspost/compress/zstd"

	"codenerd/internal/atomicfile"
	"codenerd/internal/logging"
	"codenerd/internal/types"
)

// ExtGzip is the canonical file extension for gzip-wrapped SimpleColumn snapshots.
const ExtGzip = ".sc.gz"

// ExtZstd is the canonical file extension for zstd-wrapped SimpleColumn snapshots.
const ExtZstd = ".sc.zst"

// ExtJSON is the legacy JSON snapshot extension recognised on read.
const ExtJSON = ".json"

// ExtSHA256 is the suffix of the integrity sidecar written next to a snapshot.
// The file is sha256sum(1)-compatible ("<hex>  <basename>\n") so operators can
// verify a snapshot with standard tooling, not just with this package.
const ExtSHA256 = ".sha256"

// Container magic bytes, used when the caller handed us a file whose suffix
// was stripped somewhere between Write and Read.
var (
	magicGzip = []byte{0x1f, 0x8b}
	magicZstd = []byte{0x28, 0xb5, 0x2f, 0xfd}
)

// ErrIntegrity is returned when a snapshot's bytes disagree with its .sha256
// sidecar. Callers should treat it as "this file is not the snapshot that was
// written" and refuse to assert its contents.
var ErrIntegrity = errors.New("factsnap: integrity check failed")

// Options controls a write. The zero value is the documented default: gzip
// with an integrity sidecar.
type Options struct {
	// Codec selects the compression container. CodecAuto means gzip.
	Codec Codec
	// NoSidecar suppresses the .sha256 sidecar. Only set this for scratch
	// dumps; without a sidecar Read cannot tell a truncated file from a
	// legitimately small one.
	NoSidecar bool
}

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
	return WriteOptions(path, facts, Options{Codec: codec})
}

// WriteOptions serialises facts to path under opts and returns nothing but the
// error; use CanonicalPath if you need the resulting filename.
func WriteOptions(path string, facts []types.Fact, opts Options) error {
	_, err := writeSnapshot(path, facts, opts)
	return err
}

// WritePath is WriteOptions but returns the file it actually wrote, which is
// path plus the codec's canonical suffix. Callers that report a location to an
// operator should use this rather than re-deriving the name.
func WritePath(path string, facts []types.Fact, opts Options) (string, error) {
	return writeSnapshot(path, facts, opts)
}

func writeSnapshot(path string, facts []types.Fact, opts Options) (string, error) {
	start := time.Now()
	codec := opts.Codec
	if codec == CodecAuto {
		codec = CodecGzip
	}
	if codec != CodecGzip && codec != CodecZstd {
		return "", fmt.Errorf("factsnap: unknown codec %d", codec)
	}
	path = ensureExt(path, codec)

	// A snapshot is two files (data + sidecar) that must agree. The rename of
	// each is atomic on its own, but two writers racing on the same path can
	// interleave into "writer A's data, writer B's digest", which reads back
	// as permanent corruption. Serialise same-path writers in-process.
	defer lockPath(path)()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", fmt.Errorf("factsnap: mkdir: %w", err)
	}

	store := factstore.NewSimpleInMemoryStore()
	for i, f := range facts {
		atom, err := f.ToAtom()
		if err != nil {
			return "", fmt.Errorf("factsnap: fact %d (%s) to atom: %w", i, f.Predicate, err)
		}
		store.Add(atom)
	}

	var raw bytes.Buffer
	sc := factstore.SimpleColumn{Deterministic: true}
	if err := sc.WriteTo(store, &raw); err != nil {
		return "", fmt.Errorf("factsnap: simplecolumn write: %w", err)
	}

	digest := sha256.New()
	if err := writeFileAtomic(path, func(w io.Writer) error {
		// Tee through the digest so the sidecar describes exactly the bytes
		// that landed, not a second independent serialisation of the facts.
		sink := io.MultiWriter(w, digest)
		switch codec {
		case CodecGzip:
			gw := gzip.NewWriter(sink)
			if _, err := io.Copy(gw, bytes.NewReader(raw.Bytes())); err != nil {
				_ = gw.Close()
				return fmt.Errorf("gzip write: %w", err)
			}
			if err := gw.Close(); err != nil {
				return fmt.Errorf("gzip close: %w", err)
			}
		case CodecZstd:
			zw, err := zstd.NewWriter(sink)
			if err != nil {
				return fmt.Errorf("zstd writer: %w", err)
			}
			if _, err := io.Copy(zw, bytes.NewReader(raw.Bytes())); err != nil {
				_ = zw.Close()
				return fmt.Errorf("zstd write: %w", err)
			}
			if err := zw.Close(); err != nil {
				return fmt.Errorf("zstd close: %w", err)
			}
		}
		return nil
	}); err != nil {
		return "", err
	}

	sum := hex.EncodeToString(digest.Sum(nil))
	if !opts.NoSidecar {
		sidecar := path + ExtSHA256
		line := fmt.Sprintf("%s  %s\n", sum, filepath.Base(path))
		if err := writeFileAtomic(sidecar, func(w io.Writer) error {
			_, err := io.WriteString(w, line)
			return err
		}); err != nil {
			// The snapshot itself is already durable; a missing sidecar only
			// costs verification, so do not fail the export over it.
			logging.PersistWarn("factsnap: sidecar write failed for %s: %v", sidecar, err)
		}
	} else {
		// A stale sidecar from a previous write would fail every future Read
		// of the new file, so drop it when the caller opts out.
		_ = os.Remove(path + ExtSHA256)
	}

	onDisk, _ := os.Stat(path)
	var bytesWritten int64
	if onDisk != nil {
		bytesWritten = onDisk.Size()
	}
	logging.PersistDebug("factsnap: wrote %d facts to %s codec=%s bytes=%d sha256=%s in %s",
		len(facts), path, CodecName(codec), bytesWritten, sum[:12], time.Since(start).Round(time.Millisecond))
	return path, nil
}

// pathLocks serialises writers targeting the same snapshot path.
var pathLocks sync.Map // cleaned path -> *sync.Mutex

func lockPath(path string) func() {
	key := filepath.Clean(path)
	if abs, err := filepath.Abs(key); err == nil {
		key = abs
	}
	v, _ := pathLocks.LoadOrStore(key, &sync.Mutex{})
	mu := v.(*sync.Mutex)
	mu.Lock()
	return mu.Unlock
}

// writeFileAtomic writes via a uniquely named temp file in the destination
// directory, fsyncs it, and only then renames over path. The temp name is
// unique per call because a shared "<path>.tmp" lets two concurrent writers
// interleave into one file and rename the mixture over a previously good
// snapshot. The directory is fsynced after the rename so the new name
// survives a crash on filesystems that would otherwise keep the old entry.
func writeFileAtomic(path string, write func(io.Writer) error) (err error) {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp*")
	if err != nil {
		return fmt.Errorf("factsnap: create temp in %s: %w", dir, err)
	}
	tmpName := tmp.Name()
	committed := false
	defer func() {
		if !committed {
			_ = tmp.Close()
			_ = os.Remove(tmpName)
		}
	}()

	if err := write(tmp); err != nil {
		return fmt.Errorf("factsnap: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		return fmt.Errorf("factsnap: sync %s: %w", tmpName, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("factsnap: close %s: %w", tmpName, err)
	}
	if err := os.Chmod(tmpName, 0o644); err != nil {
		return fmt.Errorf("factsnap: chmod %s: %w", tmpName, err)
	}
	if err := atomicfile.Replace(tmpName, path); err != nil {
		return fmt.Errorf("factsnap: rename %s -> %s: %w", tmpName, path, err)
	}
	committed = true

	if d, derr := os.Open(dir); derr == nil {
		_ = d.Sync()
		_ = d.Close()
	}
	return nil
}

// CodecName renders a codec for logs and operator-facing listings.
func CodecName(c Codec) string {
	switch c {
	case CodecGzip:
		return "gzip"
	case CodecZstd:
		return "zstd"
	case CodecAuto:
		return "auto"
	default:
		return "json"
	}
}

// Read loads facts from path, detecting the codec from the file suffix and,
// when the suffix is absent or lies, from the container's magic bytes. If a
// `<path>.sha256` sidecar exists the bytes are verified against it first.
// Legacy ".json" snapshots are tolerated via LegacyJSON-compatible decoding
// when the slice was encoded with types.Fact's JSON shape.
func Read(path string) ([]types.Fact, error) {
	start := time.Now()
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("factsnap: read %s: %w", path, err)
	}
	if err := verifyChecksum(path, data); err != nil {
		return nil, err
	}

	codec := resolveReadCodec(path, data)
	var facts []types.Fact
	switch codec {
	case CodecGzip:
		store, derr := factstore.NewSimpleColumnStoreFromGzipBytes(data)
		if derr != nil {
			return nil, fmt.Errorf("factsnap: gzip decode %s: %w", path, derr)
		}
		facts, err = collectFacts(store)
	case CodecZstd:
		store, derr := factstore.NewSimpleColumnStoreFromZstdBytes(data)
		if derr != nil {
			return nil, fmt.Errorf("factsnap: zstd decode %s: %w", path, derr)
		}
		facts, err = collectFacts(store)
	default:
		// Legacy JSON path. We accept []types.Fact as JSON.
		if jerr := json.Unmarshal(data, &facts); jerr != nil {
			return nil, fmt.Errorf("factsnap: legacy json decode %s: %w", path, jerr)
		}
	}
	if err != nil {
		return nil, err
	}
	logging.PersistDebug("factsnap: read %d facts from %s codec=%s bytes=%d in %s",
		len(facts), path, CodecName(codec), len(data), time.Since(start).Round(time.Millisecond))
	return facts, nil
}

// Verify recomputes the digest of path and compares it with its sidecar.
// It returns nil when no sidecar exists — verification is opt-in by the
// writer, so absence is not a failure — and ErrIntegrity on mismatch.
func Verify(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("factsnap: read %s: %w", path, err)
	}
	return verifyChecksum(path, data)
}

// HasSidecar reports whether path carries an integrity sidecar.
func HasSidecar(path string) bool {
	st, err := os.Stat(path + ExtSHA256)
	return err == nil && !st.IsDir()
}

func verifyChecksum(path string, data []byte) error {
	raw, err := os.ReadFile(path + ExtSHA256)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("factsnap: read sidecar for %s: %w", path, err)
	}
	want := strings.TrimSpace(string(raw))
	if i := strings.IndexAny(want, " \t"); i >= 0 {
		want = want[:i]
	}
	if want == "" {
		return fmt.Errorf("%w: empty sidecar for %s", ErrIntegrity, path)
	}
	sum := sha256.Sum256(data)
	got := hex.EncodeToString(sum[:])
	if !strings.EqualFold(got, want) {
		return fmt.Errorf("%w: %s (sidecar %s, actual %s)", ErrIntegrity, path, want, got)
	}
	return nil
}

// resolveReadCodec prefers what the bytes say over what the name says. A
// container's magic bytes cannot be wrong about the container, whereas a
// suffix survives a rename that changed nothing else — so a ".sc.gz" file
// holding zstd data still decodes instead of erroring out.
func resolveReadCodec(path string, data []byte) Codec {
	sniffed, ok := sniffCodec(data)
	byName := detectCodec(path)
	if !ok {
		return byName
	}
	if byName != sniffed && (byName == CodecGzip || byName == CodecZstd) {
		logging.PersistWarn("factsnap: %s has %s suffix but %s content; trusting content",
			path, CodecName(byName), CodecName(sniffed))
	}
	return sniffed
}

// sniffCodec identifies the compression container from its magic bytes:
// gzip is 1f 8b, zstd frames are 28 b5 2f fd (little-endian 0xFD2FB528).
func sniffCodec(data []byte) (Codec, bool) {
	switch {
	case bytes.HasPrefix(data, magicGzip):
		return CodecGzip, true
	case bytes.HasPrefix(data, magicZstd):
		return CodecZstd, true
	default:
		return CodecAuto, false
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
//
// See baseTermToValue below for the one deliberate divergence from core.
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

// baseTermToValue is the snapshot-side twin of core.baseTermToValue
// (internal/core/kernel_query.go). The two DIVERGE on ast.NameType and that
// divergence is deliberate, not drift:
//
//   - core returns a plain string ("/coder"). Its consumers are query pattern
//     matching and articulation, where callers compare against string literals;
//     core.TestQueryFactMatching pins that behaviour.
//   - factsnap returns types.MangleAtom("/coder"). Its consumer is a file that
//     will be read back and re-encoded with types.Fact.ToAtom(), so the name/
//     string distinction has to survive the trip. A plain string only survives
//     by accident: ToAtom re-promotes "/coder" to a name via
//     isValidMangleNameConstant, but that heuristic rejects anything with a
//     file extension or more than two slashes, so "/a/b/c.go" would silently
//     degrade from a name constant to a string constant on the second hop.
//
// Unifying them therefore means changing one set of consumers, not moving a
// function: a shared helper under internal/types would have to pick one
// behaviour and migrate the other package's callers and tests. That is the
// open item recorded in Docs/architecture/persist/OPEN-QUESTIONS.md (Q5); until
// it is taken, TestNameConstant_WhenRoundTripped_ShouldStayMangleAtom in this
// package and the core query tests pin the two behaviours independently so the
// divergence cannot widen unnoticed.
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
