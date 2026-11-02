# Research cache persistence must remain bound to its workspace owner

**Status:** source-supported conditional defect and proposed ownership contract. Static inspection only; no tests, application execution, reproductions, benchmarks, model calls or implementation changes.

**Source freeze:** theRebelliousNerd/codenerd at **bb19350a2c8acd10522d2f60140b41a3aa3457e3**, inspected 2026-11-02. All repository source links below are pinned to that revision.

## Finding and scope

A second workspace's boot attempt can redirect the durable research-cache writes of an already live Cortex, even when the second boot fails before configuration validation completes.

The resolver for workspace B calls EnableDiskCache(B), which changes the disk directory of a process-global ResearchCache. It then loads B's configuration. A parse or validation failure returns an error but does not restore the previous cache destination. Meanwhile, Cortex A's executor continues passing its own workspace A through the tool context. The research_cache_set handler ignores that context and writes through the same singleton, now targeting B. [Resolver][resolve] [Boot order][core-init] [Cache binding][enable] [Caller context][tool-dispatch] [Handler][handler]

The necessary deployment condition is **multiple workspace boot attempts within one process, with A retained and subsequently used**. The production API explicitly accommodates multiple live Cortex identities, but this review did not identify an ordinary TUI command that switches workspaces while retaining the old session. Independent command-line processes do not share this singleton. This is not a claim that two unrelated terminal processes redirect each other's cache destinations. [Multiple live owners][multi-owner] [Keyed boot API][wrapper]

The main integrity failure is an admitted A operation persisting under B's directory. Confidentiality exposure is conditional on the cached value and access to that misplaced artifact; this is not demonstrated remote exfiltration or an operating-system access-control bypass.

## Prior art: extend Q9 rather than rediscover it

Tools architecture question Q9 already identifies process-global in-memory research-cache sharing as unsuitable for multi-tenant use and asks whether ownership should be process, session or workspace. The general isolation concern is therefore prior art. The failure-mode corpus also records global cache pollution in tests. [Q9][q9] [Existing failure mode][test-pollution]

The distinct extension is concrete and durable:

- Persistence is implemented and documented under a workspace's .nerd/cache/research.
- Its destination is mutable process state, not the workspace carried by the admitted call.
- Merely resolving another workspace publishes that state before configuration acceptance and before acquisition of the keyed-Cortex boot semaphore.
- Failed boot cleanup does not roll it back.
- A later write from the still-live original Cortex can create an on-disk copy in the rejected workspace.

This is more specific than key collisions or a shared-memory cache returning another session's value. It requires neither a shared key nor concurrent execution. The disk-cache TODO credits implementation and boot wiring; this report preserves those improvements while challenging their workspace-ownership promise. [Persistence documentation][persist-doc] [Disk scope comment][disk-contract] [Pre-lock resolution][keyed-boot]

The research-observation audit concerns source identity, acquisition outcomes, continuations and working-context adoption, not which workspace owns this singleton's persistence destination. Targeted searches of current audits, journeys, tools architecture and QA for research cache, cache_disk, EnableDiskCache and rebinding found no exact failed-boot durable-destination report. This is a bounded screen, not exhaustive historical novelty. [Adjacent research audit][prior-research]

## Implemented production chain

### Boot resolution is already an effect

resolveWorkspaceRoot canonicalizes the requested root, updates the process workspace environment and global tool registry root, then calls research.EnableDiskCache. The cache call is not delayed until a Cortex is accepted. EnableDiskCache obtains the default cache, constructs the workspace's cache directory, installs that directory and hydrates unexpired entries. SetDiskDir holds its own mutex, creates the directory if needed and assigns its dir field. [Resolution][resolve] [Enable][enable] [Directory assignment][disk-dir]

The direct BootCortexWithConfig path runs initCoreComponents first. That step invokes the resolver and only afterward calls LoadUserConfig, unless a configuration override was supplied. The loader can reject malformed JSON, removed keys, invalid explicit limits or contradictory settings. A missing config file is a different case: it returns defaults rather than this failure. [Boot steps][steps] [Initialization][core-init] [Configuration refusal][config-refusal]

GetOrBootCortex has the same boundary. It resolves the root and loads configuration before acquiring cortexCacheMu. Unreadable or invalid identity is not reused; the wrapper routes a nil loaded config to ordinary BootCortex so the detailed error is returned. Thus the “failed boots are not cached” guarantee protects the Cortex map, not the side effects already performed by resolution. [Wrapper][wrapper] [Keyed boot][keyed-boot]

On error, rollbackBootContext closes a partial Cortex. The inspected Close implementation drains and releases its resources and identity-bound global registrations, but neither it nor the rollback restores or detaches the default research cache's disk directory. The cache is not a Cortex-owned field. [Rollback][rollback] [Close implementation][close]

### A correctly scoped tool context does not select the cache

This is not a general absence of workspace propagation. The factory sets ExecutorConfig.WorkspaceRoot for each boot. The executor resolves that configured root and injects it into the tool context immediately before modular dispatch. The registry preserves a nonempty explicit root and otherwise injects its own configured root. File-tool WorkspaceRoot prefers the context over the environment. These existing controls should remain. [Factory root][executor-root] [Root resolution][root-reader] [Dispatch][tool-dispatch] [Registry][registry] [File-tool precedence][guard-root]

The research handler takes a context argument but does not use it to select a cache. It validates nonempty key/value, calls getDefaultCache, and invokes Set. getDefaultCache initializes one cache through sync.Once; Set stores the entry in its shared map and then calls c.disk.write. [Singleton][singleton] [Handler][handler] [Set][set]

These are registered built-in research tools. VirtualStore hydrates the catalog families, the catalog includes research, and research.RegisterAll installs CacheSetTool. They are not dormant helper functions. Admission is still conditional: the effect classifier marks cache_set as a write, execution checks the allowlist and write guard, and the session has its executive gate before dispatch. The witness assumes an otherwise admitted cache operation. A safe_action classification is not a universal permission grant. [Hydration][hydrate] [Catalog][catalog] [Research registration][registered] [Effect][effect] [Registry admission][registry] [Constitution entry][constitution]

### Exact durable destination

diskStore.write first computes a path using the disk store's currently installed directory. For key K after B's successful directory rebinding, the destination is:

**B/.nerd/cache/research/<first 16 hexadecimal characters of SHA-256(K)>.json**

The file contains schema, original key, full value, source and creation/expiry timestamps. It is written through a sibling .tmp file and renamed; the requested creation mode is 0600. The hashed filename prevents the key itself becoming a path segment. None of these controls associates the directory with A's context. [Hash][hash] [Path selection][disk-dir] [JSON and file write][disk-write]

The main witness is entirely sequential and begins after rebinding finishes. No Go data race is needed: the mutex safely protects the wrong shared selection. A write already holding a computed path before a later rebind can remain directed to its earlier path; this report does not claim that every in-flight write is redirected mid-write.

## Minimal static witness

This is an unrun evaluation design, not a reproduced incident.

1. In one process, successfully boot Cortex A at an ordinary, writable workspace A. Retain its executor or admitted tool-dispatch path.
2. Prepare a different ordinary writable workspace B with malformed .nerd/config.json. Choose no symlink tricks, shared cache key or existing cache entry.
3. Attempt BootCortexWithConfig for B without a config override, or call GetOrBootCortex for B. B's canonical root and cache directory installation succeed. Configuration loading then rejects B.
4. Allow rollback to return its error. Do not resolve A again between that error and the next operation.
5. Through A's existing, admitted research tool path, store a fresh unique key K and value V.
6. Inspect both workspace cache directories. The source-derived expectation is a new entry containing V under B's hash-named file, not A's, despite the tool context still naming A.

This demonstrates failed admission changing another owner's future side effect, without needing a fully booted B, malicious data, simultaneous goroutines or a hash collision. Repeatedly reacquiring A through GetOrBootCortex can rebind the singleton back to A before returning a cached A; that masks the witness but does not protect a retained A object. [Pre-cache-hit resolution][keyed-boot]

The chosen failure happens before B's execution-layer hydration. It therefore does not require B to replace A's executive/write-guard binding; A's already permitted operation can remain on its existing dispatch path while only the cache destination has changed. [Boot order][steps] [Executive binding during hydration][hydrate]

Successful B boot has the same durable ownership issue. Moving EnableDiskCache after config validation would remove the early-failure trigger but would not make coexisting A and B safe.

## Impact, reset controls and limits

**Established conditional effects**

- A newly cached value can be persisted under the wrong workspace and replace an existing same-key record there. The unique-key witness avoids depending on replacement semantics.
- The result can still report “Cached” because persistence is best-effort and errors are swallowed. The decisive witness assumes the wrong-directory write succeeds; it does not rely on treating failed persistence as a successful durable save. [Handler result][handler] [Best-effort design][disk-contract]
- The directory switch hydrates B into the same memory map without clearing A. That is the existing Q9 family of sharing problems, not the novelty claim here. It does not automatically flush all existing A values into B; the main persisted-copy witness uses a new Set after rebinding. [Hydration][hydrate-disk]
- Get fallback, expiry cleanup and Clear use the current disk directory too. A Clear operation, if admitted, clears the shared memory map and current disk directory. It is not necessary for the cache_set witness and has narrower availability than a normal set. [Get][get] [Clear][clear] [Disk read/remove/clear][disk-read]
- The existing statistics tool exposes the current persistent-store directory, which is useful diagnosis, but it also reads the singleton rather than an A-owned cache. [Current statistics][stats]

**Controls and counterexamples**

- Separate processes have independent defaultCache instances. One workspace for the process lifetime avoids destination rebinding.
- If SetDiskDir cannot create/access B's directory, it returns before assigning dir; that specific attempt does not redirect writes to B. If B's config is absent rather than malformed, it need not fail at configuration loading. [Directory setter][disk-dir] [Config absence][config-refusal]
- Empty-root EnableDiskCache disables persistence, but does not partition or clear the shared memory cache. This helper is not invoked by the inspected failed-boot rollback. [Enable][enable]
- Cache keys cannot request arbitrary paths through this writer. Filename hashing and stored-key verification remain useful, but they do not establish workspace ownership. [Path construction][disk-dir] [Disk validation][disk-read]
- A readable misplaced record requires the operating-system principal or another authorized reader to access B's artifact. Requested file mode 0600 is not encryption, and its effective semantics depend on platform/filesystem. No assertion is made that an unrelated OS user can read it.
- Only content supplied to the cache is copied by Set. This finding does not imply that every file in A, every research result or credentials are automatically cached.
- Expiry is a validity check with lazy removal on reads/hydration, not proof that bytes disappear at the deadline. Subsequent synchronization or collection of B's directory could broaden exposure, but those external actions were not traced or assumed. [Disk entry and expiry][disk-read]
- No actual data disclosure, user harm, frequency or complete multi-tenant threat model was measured.

## Required ownership contract

For an operation admitted in workspace W, select one cache owner before the operation begins. Its memory namespace, durable directory, reads, writes, expiry removal, clearing and statistics must all refer to that owner for the operation's lifetime. A failed or unrelated boot must not change those selections.

The contract should cover:

1. **Selection:** a canonical workspace identity or an explicitly injected cache handle, not the most recently resolved process workspace.
2. **Failure isolation:** preparing B may allocate B-owned resources, but failure cannot repoint A or require A to re-resolve itself.
3. **Lifetime:** close or rollback releases only resources owned by that attempt; outstanding operations retain their intended binding until completion or cancellation.
4. **Namespace:** memory entries and durable records share the same workspace partition. A workspace prefix in the key alone is insufficient if A still writes the prefixed entry under B's directory.
5. **Fallback:** an unavailable owner yields a documented memory-only scoped cache or explicit unavailability. It must not silently select another workspace's last durable store.
6. **Honest observability:** expose which owner handled the operation and distinguish memory acceptance from persistence status without logging sensitive values.

These are proposed contracts, not claims that the current API offers them.

## Smallest credible design direction and costs

Prefer a cache instance with an immutable durable directory, owned by the workspace runtime or acquired from a canonical-workspace map. Bind it to the registered handler or pass an explicit handle through the established request path. Same-workspace consumers may share one instance if ownership and cleanup are explicit; different workspaces must not share an unpartitioned entry map.

Resolution should become identity computation rather than a cache-publication side effect. Construct or acquire the scoped cache at a deliberate boot boundary and release it by owner identity on rollback/close. If context-based lookup is chosen, resolve one owner for the entire operation. Calling global EnableDiskCache(ctxRoot) before each Set only moves the race: another operation can rebind between selection and use. Extending the boot semaphore or adding a mutex around directory assignment likewise does not fix later retained A calls.

The repository records that an unused EnableDiskCacheFromContext helper was removed. Reviving a name without wiring actual get/set/clear/stat consumers is not a fix. Preserve the existing hash paths, TTL checks, corrupt-record misses, stored-key comparison and bounded in-memory capacity. [Removed unused helper][removed-helper] [Existing cache semantics][get] [Disk checks][disk-read]

Cost expectations, not measurements:

- With W retained workspace caches and E entries per workspace, naive partitioning can grow memory toward O(W·E), instead of the current single 1,000-entry default. Define an overall retention/eviction policy without evicting an in-use owner.
- Current hydration reads candidate JSON files and sorts admitted unexpired entries by creation time before filling memory. For D candidates, file I/O and retained values can dominate the O(D log D) sorting term. Scoped reuse can avoid repeated hydration, but a retained map also costs memory. [Hydration][hydrate-disk]
- Stable handles remove rebind contention but require lifecycle bookkeeping. Per-operation lookup adds a canonical-identity/map lookup; do not repeatedly scan disk for each call.
- Isolation may reduce cross-workspace hits and increase legitimate refetches. Those costs should not be “optimized” by restoring silent data sharing. No token, latency or API-spend savings are claimed.

## Evaluation plan — all unrun

Use small temporary workspaces and harmless sentinel values. Compare the pinned behavior with one ownership change at a time, preserving admission rules, TTL, entry limits and disk-error policy.

| Case | Required invariant |
|---|---|
| A live; B malformed config; A sets a new key | V appears only in A's owned store; B failure cannot redirect A. |
| A live; B boot succeeds; alternate A/B sets | Every durable and memory value remains with its admitted owner. |
| B directory installation fails | No accidental reuse of A as B's store; documented fallback is visible. |
| Cache hit returns existing Cortex A | Reuse neither needs nor performs process-global destination repair. |
| Same key, different values in A/B | No cross-workspace memory hit, overwrite or expiry deletion. |
| Different keys and no prior entries | Main defect cannot be dismissed as collision or stale-memory contamination. |
| In-flight Set versus B acquisition/close | Captured owner remains stable; cancellation/cleanup does not repoint it. |
| Clear and expired Get in A | Only A's namespace and disk artifacts are affected. |
| Memory-only and disabled persistence | Existing useful fallback preserved without cross-owner sharing. |
| Corrupt record, TTL expiry, traversal-shaped key | Existing safety and miss semantics preserved. |
| Restart A and B separately | Each restores only its own persisted cache. |
| Missing/rejected workspace identity | No accidental environment/global fallback to an unrelated owner. |

Existing disk tests deliberately bypass the singleton for isolated persistence cases; the singleton test checks one configured workspace and disables persistence afterward. Those are useful controls but do not exercise retained A after a rejected B boot. Extend the production-boundary fixture rather than proving only two manually separate cache objects work. [Isolated tests][tests] [Singleton test][singleton-test]

Observe the context root, selected cache owner, attempted disk path, memory namespace, files and tool/persistence result separately. Also retain allowlist, executive-gate and write-guard negative controls: a refused operation must not be made callable to make the demonstration easier. Race-detector checks, if later authorized, supplement but cannot replace the sequential ownership oracle.

Measure cold/warm acquisition time, hydration reads, resident memory per owner, eviction/refetch rates, disk write failures, operation latency and lifecycle cleanup under many workspaces. Hold workloads and admission fixed. If agent-level savings are later studied, report correct scoped hits and accepted task outcomes alongside requests, bytes and model cost.

## Primary grounding

Saltzer and Schroeder's principles of least common mechanism and complete mediation apply directly to shared state and initialization/recovery paths. The engineering inference here is that an application workspace boundary must remain attached to the selected storage operation; a mutex around a shared selector is not that boundary. [The Protection of Information in Computer Systems, 1975](https://web.mit.edu/Saltzer/www/publications/protection/Basic.html)

Hardy's confused-deputy account distinguishes knowing a destination from possessing the appropriate authority to write it. An immutable workspace-owned cache handle follows that design lesson by carrying target identity with the operation. This analogy does not establish a malicious exploit or a strong security capability in Go. [The Confused Deputy, 1988](https://www.cis.upenn.edu/~sga001/classes/cis331f19/resources/hardy88confused.pdf)

**Checkable exit:** after any admitted, rejected or failed B boot attempt, a retained A operation uses A's cache owner across both tiers without relying on a global rebind, and all existing admission, key-containment and best-effort availability controls remain intact.

[q9]: https://github.com/theRebelliousNerd/codenerd/blob/bb19350a2c8acd10522d2f60140b41a3aa3457e3/Docs/architecture/tools/OPEN-QUESTIONS.md#L43-L46
[persist-doc]: https://github.com/theRebelliousNerd/codenerd/blob/bb19350a2c8acd10522d2f60140b41a3aa3457e3/Docs/architecture/tools/TODO.md#L68-L75
[removed-helper]: https://github.com/theRebelliousNerd/codenerd/blob/bb19350a2c8acd10522d2f60140b41a3aa3457e3/Docs/architecture/tools/TODO.md#L98-L105
[test-pollution]: https://github.com/theRebelliousNerd/codenerd/blob/bb19350a2c8acd10522d2f60140b41a3aa3457e3/Docs/architecture/tools/12-FAILURE-MODES.md#L105-L113
[prior-research]: https://github.com/theRebelliousNerd/codenerd/blob/bb19350a2c8acd10522d2f60140b41a3aa3457e3/Docs/audits/research-observation-identity-and-admission.md#L16-L38
[resolve]: https://github.com/theRebelliousNerd/codenerd/blob/bb19350a2c8acd10522d2f60140b41a3aa3457e3/internal/system/factory.go#L218-L253
[wrapper]: https://github.com/theRebelliousNerd/codenerd/blob/bb19350a2c8acd10522d2f60140b41a3aa3457e3/internal/system/factory.go#L256-L281
[keyed-boot]: https://github.com/theRebelliousNerd/codenerd/blob/bb19350a2c8acd10522d2f60140b41a3aa3457e3/internal/system/factory.go#L287-L348
[core-init]: https://github.com/theRebelliousNerd/codenerd/blob/bb19350a2c8acd10522d2f60140b41a3aa3457e3/internal/system/factory.go#L995-L1029
[config-refusal]: https://github.com/theRebelliousNerd/codenerd/blob/bb19350a2c8acd10522d2f60140b41a3aa3457e3/internal/config/user_config.go#L617-L670
[steps]: https://github.com/theRebelliousNerd/codenerd/blob/bb19350a2c8acd10522d2f60140b41a3aa3457e3/internal/system/factory.go#L2891-L2964
[rollback]: https://github.com/theRebelliousNerd/codenerd/blob/bb19350a2c8acd10522d2f60140b41a3aa3457e3/internal/system/factory.go#L2967-L2984
[close]: https://github.com/theRebelliousNerd/codenerd/blob/bb19350a2c8acd10522d2f60140b41a3aa3457e3/internal/system/cortex_close.go#L26-L341
[multi-owner]: https://github.com/theRebelliousNerd/codenerd/blob/bb19350a2c8acd10522d2f60140b41a3aa3457e3/internal/system/factory.go#L475-L490
[executor-root]: https://github.com/theRebelliousNerd/codenerd/blob/bb19350a2c8acd10522d2f60140b41a3aa3457e3/internal/system/factory.go#L2572-L2592
[root-reader]: https://github.com/theRebelliousNerd/codenerd/blob/bb19350a2c8acd10522d2f60140b41a3aa3457e3/internal/session/build_verify.go#L147-L156
[tool-dispatch]: https://github.com/theRebelliousNerd/codenerd/blob/bb19350a2c8acd10522d2f60140b41a3aa3457e3/internal/session/executor_tools.go#L2744-L2779
[registry]: https://github.com/theRebelliousNerd/codenerd/blob/bb19350a2c8acd10522d2f60140b41a3aa3457e3/internal/tools/registry.go#L346-L423
[guard-root]: https://github.com/theRebelliousNerd/codenerd/blob/bb19350a2c8acd10522d2f60140b41a3aa3457e3/internal/tools/workspace_guard.go#L17-L55
[effect]: https://github.com/theRebelliousNerd/codenerd/blob/bb19350a2c8acd10522d2f60140b41a3aa3457e3/internal/tools/effects.go#L18-L35
[constitution]: https://github.com/theRebelliousNerd/codenerd/blob/bb19350a2c8acd10522d2f60140b41a3aa3457e3/internal/core/defaults/policy/constitution.mg#L263-L272
[hydrate]: https://github.com/theRebelliousNerd/codenerd/blob/bb19350a2c8acd10522d2f60140b41a3aa3457e3/internal/core/virtual_store_tools.go#L41-L83
[catalog]: https://github.com/theRebelliousNerd/codenerd/blob/bb19350a2c8acd10522d2f60140b41a3aa3457e3/internal/tools/catalog/catalog.go#L33-L48
[registered]: https://github.com/theRebelliousNerd/codenerd/blob/bb19350a2c8acd10522d2f60140b41a3aa3457e3/internal/tools/research/register.go#L36-L49
[singleton]: https://github.com/theRebelliousNerd/codenerd/blob/bb19350a2c8acd10522d2f60140b41a3aa3457e3/internal/tools/research/cache.go#L25-L58
[set]: https://github.com/theRebelliousNerd/codenerd/blob/bb19350a2c8acd10522d2f60140b41a3aa3457e3/internal/tools/research/cache.go#L95-L115
[handler]: https://github.com/theRebelliousNerd/codenerd/blob/bb19350a2c8acd10522d2f60140b41a3aa3457e3/internal/tools/research/cache.go#L196-L245
[get]: https://github.com/theRebelliousNerd/codenerd/blob/bb19350a2c8acd10522d2f60140b41a3aa3457e3/internal/tools/research/cache.go#L61-L92
[clear]: https://github.com/theRebelliousNerd/codenerd/blob/bb19350a2c8acd10522d2f60140b41a3aa3457e3/internal/tools/research/cache.go#L117-L124
[hash]: https://github.com/theRebelliousNerd/codenerd/blob/bb19350a2c8acd10522d2f60140b41a3aa3457e3/internal/tools/research/cache.go#L150-L157
[disk-contract]: https://github.com/theRebelliousNerd/codenerd/blob/bb19350a2c8acd10522d2f60140b41a3aa3457e3/internal/tools/research/cache_disk.go#L16-L53
[disk-dir]: https://github.com/theRebelliousNerd/codenerd/blob/bb19350a2c8acd10522d2f60140b41a3aa3457e3/internal/tools/research/cache_disk.go#L55-L101
[disk-write]: https://github.com/theRebelliousNerd/codenerd/blob/bb19350a2c8acd10522d2f60140b41a3aa3457e3/internal/tools/research/cache_disk.go#L103-L130
[disk-read]: https://github.com/theRebelliousNerd/codenerd/blob/bb19350a2c8acd10522d2f60140b41a3aa3457e3/internal/tools/research/cache_disk.go#L133-L188
[hydrate-disk]: https://github.com/theRebelliousNerd/codenerd/blob/bb19350a2c8acd10522d2f60140b41a3aa3457e3/internal/tools/research/cache_disk.go#L191-L249
[enable]: https://github.com/theRebelliousNerd/codenerd/blob/bb19350a2c8acd10522d2f60140b41a3aa3457e3/internal/tools/research/cache_disk.go#L252-L269
[tests]: https://github.com/theRebelliousNerd/codenerd/blob/bb19350a2c8acd10522d2f60140b41a3aa3457e3/internal/tools/research/cache_disk_test.go#L11-L54
[singleton-test]: https://github.com/theRebelliousNerd/codenerd/blob/bb19350a2c8acd10522d2f60140b41a3aa3457e3/internal/tools/research/cache_disk_test.go#L182-L197
[stats]: https://github.com/theRebelliousNerd/codenerd/blob/bb19350a2c8acd10522d2f60140b41a3aa3457e3/internal/tools/research/cache.go#L287-L318

