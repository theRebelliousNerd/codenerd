package core

import "sync/atomic"

// Kernel generations let a consumer that derives something from a snapshot of
// this kernel (the JIT prompt compiler's dynamic-snapshot fingerprint, today)
// reuse that derivation while the kernel provably has not changed, instead of
// cloning and re-walking every fact to find out.
//
// The generation advances in markDirtyLocked, the single funnel every EDB
// write, retraction, clear and program change already passes through (a write
// that skipped it would leave the kernel's own lazy evaluation stale, so the
// two invariants are the same invariant). The instance number separates
// kernels: a reset clone or a second kernel can never alias a generation
// value of another.

var kernelInstanceSeq atomic.Uint64

// SnapshotIdentity names the state a clone was taken from.
//
// Reusable is false when equal identities do NOT imply equal derived state:
// the program has rules whose premises are external (VirtualStore) predicates,
// which every evaluate re-derives from state the generation does not cover,
// or the program changed and its dependency index has not been rebuilt yet.
type SnapshotIdentity struct {
	Instance   uint64
	Generation uint64
	Reusable   bool
}

func (k *RealKernel) instanceLocked() uint64 {
	if id := k.instanceID.Load(); id != 0 {
		return id
	}
	k.instanceID.CompareAndSwap(0, kernelInstanceSeq.Add(1))
	return k.instanceID.Load()
}

// snapshotIdentityLocked reads the identity under at least k.mu.RLock, which
// excludes markDirtyLocked (it runs under the write lock).
func (k *RealKernel) snapshotIdentityLocked() SnapshotIdentity {
	return SnapshotIdentity{
		Instance:   k.instanceLocked(),
		Generation: k.generation.Load(),
		Reusable:   !k.policyDirty && k.cone != nil && len(k.cone.volatile) == 0,
	}
}

// SnapshotIdentity returns the identity of the kernel's current state.
func (k *RealKernel) SnapshotIdentity() SnapshotIdentity {
	if k == nil {
		return SnapshotIdentity{}
	}
	k.mu.RLock()
	defer k.mu.RUnlock()
	return k.snapshotIdentityLocked()
}

// CloneWithIdentity clones the kernel and returns the identity of exactly the
// state that was cloned: both are read under one hold of the read lock, so no
// write can fall between them.
func (k *RealKernel) CloneWithIdentity() (*RealKernel, SnapshotIdentity) {
	k.mu.RLock()
	defer k.mu.RUnlock()
	return k.cloneLocked(), k.snapshotIdentityLocked()
}
