# Overlay clone isolation must close the raw-store escape

## Decision and evidence boundary

Preserve the new read-through clone optimization, but close or explicitly narrow the public `RealKernel.GetStore` capability before treating an overlay's base as immutable. The getter returns the real mutable store. A retained handle can change that store after cloning without passing through the new copy-on-write boundary, and the clone can observe the new membership.

**Scope is deliberately narrow:** this is a source-supported **low-level Go API ownership refinement**, not an observed production regression, model-accessible exploit or demonstrated Dreamer authorization bypass. No production caller of `RealKernel.GetStore` was found. A caller must deliberately mutate the escaped store. Ordinary kernel mutation paths already have the relevant ownership checks.

Source freeze, rechecked 2026-10-29 at 09:42 UTC:

- Main: **`f84ce0df57b21de3725b8fa02d13e88f74192714`**.
- Active Mangle lane `lane/worktree-agent-a70bb621a19639eb0`: **`4b497227e9187fd6d53ff85fc6c5332ace5e118c`**.
- The overlay implementation is **branch-only** at this freeze. Its introducing commit is `18536c8f984c0ccedef8aac81a5af226d4b35ed4`.

Inspection included current root/Mangle/prompt guidance, the branch delta and prior audits through `research-main-20261029-0920`. An independent source challenge sustained the bounded witness and confirmed the absence of an identified production caller. No tests, implementation, inference, training, profiling, local Git, computer tasks or runtime reproduction were performed.

## 1. The implemented improvement and its open boundary

The branch's `overlayStore` reads an immutable base, masks removed atoms and keeps added atoms privately. A clean kernel with a plain store hands a clone an overlay over that exact store, then sets `storeShared`. The parent's next normal in-place write wraps its own store before modifying it. If the parent's store is already an overlay, cloning copies its private writes while sharing only the underlying base. This is a sound ownership design **if all writes to the base are mediated**. [Declared invariant](https://github.com/theRebelliousNerd/codenerd/blob/4b497227e9187fd6d53ff85fc6c5332ace5e118c/internal/core/kernel_overlay_store.go#L11-L26), [sharing and ownership transition](https://github.com/theRebelliousNerd/codenerd/blob/4b497227e9187fd6d53ff85fc6c5332ace5e118c/internal/core/kernel_overlay_store.go#L182-L209).

The remaining escape is unchanged `GetStore`: it acquires the kernel read lock, returns `k.store`, and releases the lock. The returned object is not a snapshot or a mediating wrapper. Its `FactStore` interface includes `Add` and `Merge`; removable implementations additionally expose `Remove`. Calling those methods later does not run `ownStoreLocked`, a kernel admission method, index maintenance or generation accounting. [Branch getter](https://github.com/theRebelliousNerd/codenerd/blob/4b497227e9187fd6d53ff85fc6c5332ace5e118c/internal/core/kernel_eval.go#L509-L514), [inspected upstream interface](https://github.com/google/mangle/blob/7cd2543ac1817f8b92d42efe3c6ef82dededbeeb/factstore/factstore.go#L47-L84).

The lack of parent accounting is **not new**: main already exposes this mutable getter. The new consequence is cross-clone membership contamination, because the branch replaces separate store membership with shared read-through membership. [Main getter](https://github.com/theRebelliousNerd/codenerd/blob/f84ce0df57b21de3725b8fa02d13e88f74192714/internal/core/kernel_eval.go#L465-L470), [main copied clone](https://github.com/theRebelliousNerd/codenerd/blob/f84ce0df57b21de3725b8fa02d13e88f74192714/internal/core/kernel_eval.go#L564-L574), [main copy implementation](https://github.com/theRebelliousNerd/codenerd/blob/f84ce0df57b21de3725b8fa02d13e88f74192714/internal/core/kernel_eval.go#L649-L662).

## 2. Sequential source-derived witness — not executed

Use an isolated trusted program declaring an ordinary, non-external predicate:

```mangle
Decl snapshot_probe(X) bound [/string].
```

1. Populate and successfully evaluate a parent kernel with `snapshot_probe("before")`. Ensure it is clean and its evaluated store is plain rather than already an overlay.
2. Retain `raw := parent.GetStore()`.
3. Create `child := parent.Clone()`.
4. Invoke the low-level handle directly: `raw.Add(ast.NewAtom("snapshot_probe", ast.String("later")))`. Do not invoke a kernel assertion method or otherwise dirty the child.
5. Read `child.Query("snapshot_probe")`.

On the branch, the clone's base is the same object as `raw`. `overlayStore.GetFacts` reads that base on each scan, and no child tombstone hides the newly added atom. Clean clone setup marks `factsDirty=false`; `ensureEvaluated` returns immediately, and ordinary `Query` reads the materialized store. Therefore the source predicts that the child returns the post-clone `"later"` row, despite that row not entering the child's retained EDB/index through its mutation API. [Overlay reads](https://github.com/theRebelliousNerd/codenerd/blob/4b497227e9187fd6d53ff85fc6c5332ace5e118c/internal/core/kernel_overlay_store.go#L104-L118), [clean clone admission](https://github.com/theRebelliousNerd/codenerd/blob/4b497227e9187fd6d53ff85fc6c5332ace5e118c/internal/core/kernel_eval.go#L609-L619), [lazy-evaluation fast path](https://github.com/theRebelliousNerd/codenerd/blob/4b497227e9187fd6d53ff85fc6c5332ace5e118c/internal/core/kernel_eval.go#L474-L480), [ordinary query scan](https://github.com/theRebelliousNerd/codenerd/blob/4b497227e9187fd6d53ff85fc6c5332ace5e118c/internal/core/kernel_query.go#L40-L107).

The simpler observation is `child.GetStore().Contains(theNewAtom)`; using ordinary `Query` establishes that the effect need not remain confined to a second low-level accessor.

On main's copied-store path, adding a *new atom* to the parent's store after cloning cannot add that atom to the child's independently copied membership. This comparison does not claim old clones deep-copied every AST object: their atoms were shared, so argument mutation is a different question. The witness changes only membership, not atom arguments.

No concurrency, lattice, external predicate, evaluation error, arbitrary rule ordering or special authorization predicate is needed. The analogous removal witness requires a removable escaped base. It is useful future coverage, not an additional reproduced result.

## 3. Counterevidence and precise reachability

- **No current production caller identified.** Repository code search found the `RealKernel.GetStore` definition plus unrelated MCP getters, calls and tests; inspection of all 12 changed branch Go files found no new call. Thus no stock path has been shown to mutate the raw base. An exported internal-package method being callable is not proof that ordinary model requests reach it.
- **Normal writes are protected.** Cone evaluation calls `ownStoreLocked` before editing in place; heartbeat replacement does too. Full evaluation constructs a fresh store and resets the sharing flag. Do not report ordinary `Assert`, retraction, heartbeat or full evaluation as the bypass. [Cone ownership](https://github.com/theRebelliousNerd/codenerd/blob/4b497227e9187fd6d53ff85fc6c5332ace5e118c/internal/core/kernel_eval_cone.go#L339-L350), [heartbeat ownership](https://github.com/theRebelliousNerd/codenerd/blob/4b497227e9187fd6d53ff85fc6c5332ace5e118c/internal/core/kernel_facts.go#L593-L605), [fresh full-store publication](https://github.com/theRebelliousNerd/codenerd/blob/4b497227e9187fd6d53ff85fc6c5332ace5e118c/internal/core/kernel_eval.go#L393-L409).
- **Not every clone shape leaks this way.** Cloning an existing overlay forks its private additions/tombstones. A later mutation of that parent overlay's private additions does not alter the fork's copied additions. The proposed witness specifically retains a plain store that subsequently becomes a shared base. Dirty-parent clones also do not take the clean read-through shortcut.
- **The added tests cover normal ownership well.** They compare randomized overlay operations against a plain store, check a fork stays unchanged, verify the base does not change through overlay methods, and check wrapping/flattening. They do not exercise an escaped writable base handle. These tests were inspected, not run. [Overlay tests](https://github.com/theRebelliousNerd/codenerd/blob/4b497227e9187fd6d53ff85fc6c5332ace5e118c/internal/core/kernel_overlay_store_test.go#L31-L115).
- **The witness is host-side.** The essential read-through and raw escape are CodeNERD code. The upstream interface citation uses the accessible v0.7.0 GitHub mirror; exact downloaded Codeberg module bytes were not independently compared. No detailed upstream evaluator behavior is needed for the membership argument.

## 4. Prior-work relationship

The existing performance audit explicitly required an immutable parent base for overlays. This finding is an implementation acceptance check against that requirement, not a new proposal to invent overlays. Its earlier stored-fact ownership appendix concerned caller-owned `Fact.Args` and `GetBaseFacts` aliases. The present sequence uses the raw mutable `FactStore` and adds an entirely new atom; it does not depend on those argument aliases remaining unfixed. [Overlay prerequisite](https://github.com/theRebelliousNerd/codenerd/blob/f84ce0df57b21de3725b8fa02d13e88f74192714/Docs/audits/mangle-kernel-performance.md#L59-L62), [previous ownership finding](https://github.com/theRebelliousNerd/codenerd/blob/f84ce0df57b21de3725b8fa02d13e88f74192714/Docs/audits/mangle-kernel-performance.md#L202-L285).

The recent generic-engine merge-invalidation audit concerns stale downstream derivations in a different engine adapter. This issue requires no merge declaration and concerns the action kernel's clone membership boundary. [Prior research](https://github.com/theRebelliousNerd/codenerd/blob/83aa2027ee122c01f532bb0b0e33106d391af185/Docs/audits/mangle-generic-merge-invalidation.md).

## 5. Proposed contract and bounded remedies

**Contract:** once a clean evaluated store is shared as a clone base, no previously or subsequently issued public handle can mutate its membership. A clone's evaluated facts stay at the cloned snapshot until mutation through that clone's own supported API.

Prefer the smallest compatible remedy:

1. Inventory external/internal consumers and remove or deprecate the raw mutable getter if none needs it. Existing query/snapshot APIs should be the normal read boundary.
2. If advanced consumers require a fact store, return an independently owned snapshot **on that request**, rather than copying the whole store for every clone.
3. If a live access facility is required, mediate it under the kernel's ownership boundary. Writes must go through canonical mutation/admission/index/epoch semantics, not merely call `ownStoreLocked` then mutate materialized IDB.
4. A read-only interface annotation alone is not ownership: returning the same concrete mutable object permits a type assertion, and exposing borrowed mutable atom argument arrays can create another escape. Define and enforce the wrapper/snapshot lifetime and value-ownership contract.

Retaining a raw pointer **before** cloning is the important control: making only future getter calls wrap the current store would not revoke already issued handles.

## 6. Costs and fair evaluation — proposed, not run

**Costs.** Removing an unused escape is small code but an API-compatibility decision. An on-demand owned snapshot costs O(store rows) iteration/allocation and may need value copying for mutable AST structures; charge that cost to advanced snapshot requests, not every Dreamer/JIT clone. A live wrapper adds locking and callback-lifetime complexity, with potential deadlocks if callbacks re-enter the kernel. More comprehensive immutable value ownership can increase admission/export allocations. Track retained base lifetimes, peak memory and GC, as well as clone/cone time. No inference, embeddings, training, provider calls or new persistent cache is needed; no speedup or dollar saving is claimed.

**Future evaluation:**

- Retain a raw-capability equivalent before cloning and attempt post-clone Add/Remove; ordinary child Query must remain unchanged or the mutation must be refused by contract.
- Cover plain simple and indexed bases, handles obtained before/after clone, repeated clones, an already-overlay parent, flattening and full-evaluation replacement.
- Preserve the existing normal parent/child mutation isolation tests, and compare complete materialized sets against independent snapshot references.
- Test argument ownership separately from membership so fixes for one cannot mask the other. Include the supported reader/writer/callback contract and a concurrency check only after that contract is defined.
- Measure advanced-read snapshot allocations/latency, regular clone and cone latency, lock hold, retained bases and GC. Compare equal isolation semantics; a cheaper clone that accepts post-snapshot rows is not a valid faster baseline.

**Exit condition:** every publicly obtainable store capability has an explicit ownership/lifetime contract, an issued capability cannot mutate a shared base, and ordinary clone performance retains the overlay optimization. Until a real caller or authorized reproduction is established, keep the finding labeled a branch-only API boundary refinement.

