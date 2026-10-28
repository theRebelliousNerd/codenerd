package prompt

import (
	"context"
	"fmt"
)

// Preflight is the work every compile does before it can even look in the
// cache: clone the kernel, assert the context, collect the injected atoms and
// fingerprint the whole evaluated snapshot. It is correctness work -- the
// fingerprint is what stops an unchanged context from reusing a prompt built
// on stale kernel evidence -- but it was paid in full on every hit and by
// every singleflight joiner.
//
// The fingerprint is a pure function of the cloned state, the asserted
// context facts and the injection bounds. When the kernel can name its state
// (KernelSnapshotIdentifier) and certifies that equal names mean equal
// derived state (Reusable: no rule reads an external predicate), the result
// is memoized under that name. A later compile over the same kernel
// generation and context reads the memo instead of cloning; a hit then
// completes with no clone at all. Any write to the kernel advances its
// generation, so the memo can only be consulted for the exact state it
// describes. Unknown or non-reusable identities take the full path.

type compilePreflight struct {
	// identity is the kernel identity the dynamic snapshot describes.
	identity    KernelSnapshotIdentity
	hasIdentity bool
	dynamic     []*PromptAtom
	fingerprint string
	reused      bool

	// kernel is the private selection scope, nil until acquired.
	kernel  KernelQuerier
	release func()
}

func (c *JITPromptCompiler) runPreflight(ctx context.Context, cc *CompilationContext, config CompilerConfig, configRevision uint64) (*compilePreflight, error) {
	pf := &compilePreflight{release: func() {}}
	if identifier, ok := c.kernel.(KernelSnapshotIdentifier); ok {
		if identity, known := identifier.SnapshotIdentity(); known && identity.Reusable {
			if entry, hit := c.preflight.Get(newPreflightKey(identity, cc, configRevision)); hit {
				pf.identity, pf.hasIdentity = identity, true
				pf.dynamic, pf.fingerprint = entry.dynamic, entry.fingerprint
				pf.reused = true
				c.counters.preflightReuses.Add(1)
				return pf, nil
			}
		}
	}
	if err := c.acquireScope(ctx, cc, pf); err != nil {
		pf.release()
		return nil, err
	}
	if err := c.snapshotScope(cc, config, configRevision, pf); err != nil {
		pf.release()
		return nil, err
	}
	return pf, nil
}

// acquireScope creates the private selection scope and asserts the context
// facts into it. The scope's own identity, when it reports one, replaces
// whatever identity the preflight held.
func (c *JITPromptCompiler) acquireScope(ctx context.Context, cc *CompilationContext, pf *compilePreflight) error {
	kernel, release, err := acquireCompilationKernel(c.kernel)
	if err != nil {
		return fmt.Errorf("create isolated prompt compilation scope: %w", err)
	}
	pf.kernel, pf.release = kernel, release
	if err := ctx.Err(); err != nil {
		return err
	}
	if kernel != nil {
		if err := kernel.AssertBatch(cc.ToContextFacts()); err != nil {
			return fmt.Errorf("assert private prompt context: %w", err)
		}
	}
	return nil
}

// scopeIdentity reports the identity the scope was cloned from.
func scopeIdentity(kernel KernelQuerier) (KernelSnapshotIdentity, bool) {
	scoped, ok := kernel.(KernelScopeIdentity)
	if !ok {
		return KernelSnapshotIdentity{}, false
	}
	return scoped.BaseIdentity()
}

// snapshotScope computes the dynamic snapshot on the acquired scope and
// memoizes it when the scope's identity is reusable.
func (c *JITPromptCompiler) snapshotScope(cc *CompilationContext, config CompilerConfig, configRevision uint64, pf *compilePreflight) error {
	dynamic, fingerprint, err := c.collectDynamicKernelSnapshotWithConfig(pf.kernel, cc, config)
	if err != nil {
		return fmt.Errorf("capture dynamic prompt snapshot: %w", err)
	}
	pf.dynamic, pf.fingerprint = dynamic, fingerprint
	pf.identity, pf.hasIdentity = scopeIdentity(pf.kernel)
	if pf.hasIdentity && pf.identity.Reusable {
		c.preflight.Put(newPreflightKey(pf.identity, cc, configRevision),
			preflightEntry{dynamic: dynamic, fingerprint: fingerprint})
	}
	return nil
}

// ensureSelectionScope gives a reused preflight the private scope a miss
// needs. If the scope was cloned from the same identity the memo described,
// its snapshot is the memo's and nothing is recomputed. Otherwise the kernel
// moved in between, the snapshot is recomputed on the scope, and refreshed
// reports whether the fingerprint (and so the cache key) changed.
func (c *JITPromptCompiler) ensureSelectionScope(ctx context.Context, cc *CompilationContext, config CompilerConfig, configRevision uint64, pf *compilePreflight) (refreshed bool, err error) {
	if !pf.reused {
		return false, nil
	}
	memo := pf.identity
	if err := c.acquireScope(ctx, cc, pf); err != nil {
		return false, err
	}
	if identity, ok := scopeIdentity(pf.kernel); ok && identity == memo {
		return false, nil
	}
	previous := pf.fingerprint
	if err := c.snapshotScope(cc, config, configRevision, pf); err != nil {
		return false, err
	}
	return pf.fingerprint != previous, nil
}
