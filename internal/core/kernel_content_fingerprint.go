package core

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"hash/fnv"
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
// of each held fact's canonical key. Equal EDBs under equal programs have
// equal fingerprints whatever order or route their facts arrived by; any
// added or removed fact changes the sum.
//
// One input is not in the EDB: an external predicate's answers. A program
// with a rule over one (a volatile head) gets a fingerprint unique to this
// state instead, which never matches another: no false reuse, no reuse.

// edbDigest is one held fact's contribution to the EDB content sum.
func edbDigest(pred, key string) [2]uint64 {
	h := fnv.New128a()
	_, _ = h.Write([]byte(pred))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(key))
	sum := h.Sum(nil)
	return [2]uint64{binary.BigEndian.Uint64(sum[:8]), binary.BigEndian.Uint64(sum[8:])}
}

func (x *edbIndex) addContent(pred, key string) {
	d := edbDigest(pred, key)
	x.content[0] += d[0]
	x.content[1] += d[1]
}

func (x *edbIndex) subtractContent(pred, key string) {
	d := edbDigest(pred, key)
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
