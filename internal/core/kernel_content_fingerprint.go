package core

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"reflect"
	"slices"
	"strconv"
)

// =============================================================================
// CONTENT FINGERPRINT
// =============================================================================
// The JIT prompt compiler keys its compile cache on the content of the kernel
// a compile reads, not on which write produced it. It used to compute that by
// listing every fact (QueryAll) and JSON-encoding each one: on a 345k-fact
// world kernel ~1 s of every ~1.5 s compile (measured 2026-10-29).
//
// The kernel's derived facts are a function of its program and its EDB, so
// those two name its content. The program is named by the hash of its text
// (the compiled-program cache key). The EDB is named by a multiset hash its
// index keeps current on every insert and remove: the sum of a 128-bit digest
// of each held fact's typed host value. Equal EDBs under equal programs have
// equal fingerprints whatever order or route their facts arrived by; any
// added or removed fact changes the sum.
//
// The digest reads the typed value, not the deduplication key. That key is
// type-erasing (int64(1) and float64(1) both spell "1"), but admission is
// not: under an /any bound the first is a Number and the second a Float64,
// and a rule premise fp(1) matches only the Number. A digest of the key gave
// those two EDBs one fingerprint while they answered differently (audit
// mangle-content-fingerprint-typed-identity). Admission is a function of the
// held host value and the program, so equal typed values under one program
// admit equal atoms; host values that differ but admit alike (int and int64
// under a /number bound) only cost a cache miss, never a false hit.
//
// Held facts are immutable after admission (the kernel's fact list, atom
// cache and atom memo already rely on it), so the digest a removal subtracts
// is the one its insertion added.
//
// One input is not in the EDB: an external predicate's answers. A program
// with a rule over one (a volatile head) gets a fingerprint unique to this
// state instead, which never matches another: no false reuse, no reuse.

// edbDigest is one held fact's contribution to the EDB content sum.
func edbDigest(f Fact) [2]uint64 {
	buf := make([]byte, 0, 64+len(f.Predicate))
	buf = append(buf, f.Predicate...)
	buf = append(buf, 0)
	for _, arg := range f.Args {
		buf = appendTypedValue(buf, arg)
		buf = append(buf, 0)
	}
	h := fnv.New128a()
	_, _ = h.Write(buf)
	var sum [16]byte
	h.Sum(sum[:0])
	return [2]uint64{binary.BigEndian.Uint64(sum[:8]), binary.BigEndian.Uint64(sum[8:])}
}

// appendTypedValue appends a lossless, deterministic encoding of a host
// value: its Go type and its value, recursively through containers, so two
// values share an encoding only when they are the same value of the same
// type. Strings are quoted, so every element is self-delimiting.
func appendTypedValue(buf []byte, v any) []byte {
	switch t := v.(type) {
	case nil:
		return append(buf, "nil"...)
	case string:
		return strconv.AppendQuote(append(buf, 's'), t)
	case MangleAtom:
		return strconv.AppendQuote(append(buf, 'a'), string(t))
	case bool:
		if t {
			return append(buf, "b1"...)
		}
		return append(buf, "b0"...)
	case int64:
		return strconv.AppendInt(append(buf, "i64:"...), t, 10)
	case int:
		return strconv.AppendInt(append(buf, "int:"...), int64(t), 10)
	case float64:
		return strconv.AppendFloat(append(buf, "f64:"...), t, 'g', -1, 64)
	case json.Number:
		// Its canonical key parses the number; its admitted form keeps the
		// spelling ("1" and "1.0" are different strings to Mangle).
		return strconv.AppendQuote(append(buf, "jn"...), string(t))
	case []any:
		buf = append(buf, "[]any["...)
		for _, e := range t {
			buf = append(appendTypedValue(buf, e), ',')
		}
		return append(buf, ']')
	case map[string]any:
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		slices.Sort(keys)
		buf = append(buf, "map[string]any{"...)
		for _, k := range keys {
			buf = append(strconv.AppendQuote(buf, k), ':')
			buf = append(appendTypedValue(buf, t[k]), ',')
		}
		return append(buf, '}')
	default:
		return appendTypedReflect(buf, reflect.ValueOf(v))
	}
}

// appendTypedReflect covers every other host type: its full type name, then
// the elements of a slice, array or map (map entries sorted by their own
// typed encoding, which is unique per key, so the order is deterministic),
// or the canonical value of a scalar, which is lossless within one type.
func appendTypedReflect(buf []byte, rv reflect.Value) []byte {
	if !rv.IsValid() {
		return append(buf, "nil"...)
	}
	typ := rv.Type()
	buf = strconv.AppendQuote(buf, typ.PkgPath()+" "+typ.String())
	switch rv.Kind() {
	case reflect.Slice, reflect.Array:
		buf = append(buf, '[')
		for i := 0; i < rv.Len(); i++ {
			buf = append(appendTypedValue(buf, rv.Index(i).Interface()), ',')
		}
		return append(buf, ']')
	case reflect.Map:
		entries := make([]string, 0, rv.Len())
		iter := rv.MapRange()
		for iter.Next() {
			e := appendTypedValue(nil, iter.Key().Interface())
			e = appendTypedValue(append(e, ':'), iter.Value().Interface())
			entries = append(entries, string(e))
		}
		slices.Sort(entries)
		buf = append(buf, '{')
		for _, e := range entries {
			buf = append(append(buf, e...), ',')
		}
		return append(buf, '}')
	default:
		return strconv.AppendQuote(buf, canonValue(rv.Interface()))
	}
}

func (x *edbIndex) addContent(f Fact) {
	d := edbDigest(f)
	x.content[0] += d[0]
	x.content[1] += d[1]
}

func (x *edbIndex) subtractContent(f Fact) {
	d := edbDigest(f)
	x.content[0] -= d[0]
	x.content[1] -= d[1]
}

// ContentFingerprint names the kernel's complete fact content, base and
// derived, after evaluating any pending write. Two kernels with equal
// fingerprints answer every query alike. It fails exactly when QueryAll
// would: a failed evaluation is not a snapshot.
func (k *RealKernel) ContentFingerprint() (string, error) {
	if k == nil {
		return "", fmt.Errorf("content fingerprint: kernel is nil")
	}
	if err := k.ensureEvaluated(); err != nil {
		return "", err
	}
	k.mu.RLock()
	defer k.mu.RUnlock()
	if err := k.readableLocked(); err != nil {
		return "", err
	}
	h := sha256.New()
	_, _ = h.Write(k.programKey[:])
	var buf [8]byte
	write := func(v uint64) {
		binary.BigEndian.PutUint64(buf[:], v)
		_, _ = h.Write(buf[:])
	}
	if k.factIndex != nil {
		write(k.factIndex.content[0])
		write(k.factIndex.content[1])
		write(uint64(k.factIndex.size))
	}
	if k.cone == nil || len(k.cone.volatile) > 0 {
		id := k.snapshotIdentityLocked()
		write(id.Instance)
		write(id.Generation)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
