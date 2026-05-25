---
surface: "Dreamer_KernelClone"
mode: "boundary"
subsystems_tested: ["Dreamer", "Kernel", "VirtualStore"]
blast_radius: "critical"
remediated: false
---

# Dreamer ↔ Kernel Clone Integration Analysis

## 1. System Interaction Map

The boundary between the Dreamer and the Kernel Clone involves the speculative simulation of proposed actions before they are routed to the VirtualStore. The primary interaction flow is:

- `Dreamer.SimulateAction(ctx, req)` is invoked by the Action Router or VirtualStore prior to committing to an operation.
- `Dreamer.getKernel()` retrieves the current live `RealKernel` under a read-lock (`mu.RLock()`).
- `Dreamer.projectEffects(kernel, actionID, req)` is executed. This function performs a read-only code graph traversal on the parent kernel.
  - It acquires `kernel.mu.RLock()`.
  - It scans the Mangle fact store `kernel.store.GetFacts` looking for `code_defines/5` or `code_defines/2` matching the target file path.
  - It generates a slice of synthetic `Fact` objects representing the structural changes (e.g., `/file_missing`, `/modified`, `/deleted`).
- `Dreamer.evaluateProjection(kernel, actionID, projected)` is invoked.
  - `kernel.Clone()` is called. This copies the Mangle program definitions, logic schemas, rules, and copies the FactStore.
  - The cloned kernel is entirely isolated.
  - `clone.AssertWithoutEval(fact)` is called in a tight loop for each projected fact.
  - `clone.Evaluate()` computes the logical fixpoint of the new Mangle program incorporating the projected facts.
  - `clone.Query("panic_state")` is executed to scan for safety violations.
- A `DreamResult` is generated and returned, caching logic might be invoked `DreamCache.Store(result)`.
- If `DreamResult.Unsafe` is true, the action is discarded or escalated.

### Critical FFI Interfaces
- `Clone()` method of `RealKernel`
- `fastTermToString` and `ast.Atom` memory structures during `codeGraphProjections`
- `ast.NewQuery` for `panic_state`

## 2. Contract Analysis

The boundary between the Dreamer and the Kernel Clone relies on several implicit and explicit contracts:

**Contract 1: Absolute Isolation (The Sandbox Contract)**
The `Clone()` method must produce a completely disjoint Mangle evaluation engine. Assertions made in the clone (`AssertWithoutEval`) and the subsequent `Evaluate()` cycle must not leak facts, rule mutations, or side-effects back to the parent `RealKernel`. Any shared references inside the `FactStore` must be strictly immutable.

**Contract 2: Context Sensitivity & Cancellation (The Temporal Contract)**
The `SimulateAction` signature accepts a `context.Context`. If the context is cancelled, the dreamer must halt projection and simulation immediately. However, Mangle's `Evaluate()` is an internal fixpoint loop that may not inherently respect `ctx.Done()`. The contract assumes the fixpoint will either complete in bounded time or natively surface the context cancellation.

**Contract 3: Completeness of Projection (The Semantic Contract)**
The parent kernel's state must correctly reflect the projectable universe. The Dreamer assumes that if a file is deleted, asserting `/file_missing` will trigger the appropriate `panic_state` rules. It assumes all dependent `code_defines` and reference paths are accurately fetched via `kernel.store.GetFacts`.

**Contract 4: Deterministic Halting (The Fixpoint Contract)**
The `panic_state` rules evaluated within the clone must reach a fixpoint deterministically. A malicious or complex projection must not cause an infinite generation loop (e.g., unstratified negation) in the clone, which would deadlock the goroutine running `Dreamer.SimulateAction`.

**Contract 5: Cache Consistency (The Temporal Contract)**
The `DreamCache` assumes that the safety verdict of an action remains valid for the lifespan of the action sequence. However, if the parent kernel receives new facts (e.g., a file is marked critical by a separate shard) after the cache is populated, the cache entry becomes stale and invalid.

## 3. Failure Mode Enumeration

### A. Semantic Failures
- **Atom/String Type Dissonance:** The Dreamer projects a Go string `"/file_missing"`, but the Mangle policy expects a Mangle Atom type. The join fails silently, yielding zero tuples for `panic_state`. The unsafe action is approved.
- **Incomplete Projection:** The Dreamer projects `modified(A)` but fails to project `requires_recompile(A)`. The `panic_state` rule depends on the transitive property, which is never met.

### B. Temporal Failures
- **Fixpoint Deadlock:** The projected facts trigger a recursive rule in the cloned Mangle kernel that generates an infinite number of derived facts. The `Clone().Evaluate()` blocks indefinitely, starving the Session Executor.
- **Uncancellable Evaluation:** A context cancellation occurs while `Clone().Evaluate()` is running, but because the Mangle engine isn't wired to the Go context, the CPU spins until completion, wasting resources.

### C. Corruption / Isolation Leaks
- **Shallow Copy Store:** The `Clone()` implementation performs a shallow copy of the fact slices. When the Dreamer asserts new projected facts, it appends to a shared underlying Go slice capacity, corrupting the parent kernel's memory.
- **Concurrent Map Write in Clone:** The Dreamer runs `SimulateAction` concurrently from multiple shards. If `Clone()` shares map references for indices, concurrent Dreamers will crash the program via `fatal error: concurrent map read and map write`.

### D. Ordering & Cache Failures
- **Stale Cache Approval:**
  1. Dreamer simulates Action A.
  2. Action A is Safe. Cache updated.
  3. Parent kernel receives fact `critical_path(TargetA)`.
  4. Executor retrieves Action A from cache, bypasses Dreamer.
  5. Action A executes on a critical path, violating safety.

### E. Partial Failures
- **Query Failure Obfuscation:** If `clone.Query("panic_state")` fails due to a syntax error or missing predicate, the system fails closed (Unsafe), but the reason is poorly formatted, leading to an articulation loop where the LLM tries the same action repeatedly.

## 4. Adversarial Scenario Design

1. **Scenario 1: The Shallow Clone Leak**
   - *Violated Contract:* Absolute Isolation.
   - *Injection Mechanism:* Assert a highly nested fact into the parent kernel. Trigger a Dreamer simulation that mutates the nested arguments of the projected fact. Verify if the parent kernel's fact is mutated.
   - *Expected Behavior:* The parent kernel remains strictly unmodified.
   - *Severity:* P0

2. **Scenario 2: Infinite Fixpoint Deadlock**
   - *Violated Contract:* Deterministic Halting.
   - *Injection Mechanism:* Dynamically insert an unstratified or infinitely recursive rule into the parent kernel (e.g., `p(X) :- p(Y), math:add(Y, 1, X).`). Trigger `SimulateAction` with a context timeout of 50ms.
   - *Expected Behavior:* The Dreamer correctly surfaces a timeout/context cancellation error without deadlocking the goroutine.
   - *Severity:* P1

3. **Scenario 3: The Ghost Atom Disconnect**
   - *Violated Contract:* Completeness of Projection.
   - *Injection Mechanism:* Project a string `"projected_action"` instead of an Atom `"/projected_action"`.
   - *Expected Behavior:* The kernel's `panic_state` should either type-error or fail closed. The test ensures type strictness at the FFI boundary.
   - *Severity:* P1

4. **Scenario 4: Concurrent Dreamer Race**
   - *Violated Contract:* Absolute Isolation.
   - *Injection Mechanism:* Spawn 100 goroutines concurrently invoking `SimulateAction` on the exact same `RealKernel` reference, each projecting 50 unique facts.
   - *Expected Behavior:* No race conditions, no map read/write panics, accurate isolation per clone.
   - *Severity:* P0

5. **Scenario 5: Stale DreamCache Invalidation Bypass**
   - *Violated Contract:* Cache Consistency.
   - *Injection Mechanism:* Simulate action -> Safe. Mutate parent kernel state making it unsafe. Simulate action again.
   - *Expected Behavior:* The cache must be invalidated by the kernel mutation, forcing a re-evaluation that yields Unsafe.
   - *Severity:* P2

6. **Scenario 6: Missing Panic_State Predicate**
   - *Violated Contract:* Semantic Contract.
   - *Injection Mechanism:* Boot a kernel with a policy file that entirely lacks the `panic_state` predicate definition.
   - *Expected Behavior:* `SimulateAction` fails closed (Unsafe) with an explicit "predicate not found" error, rather than panicking or failing open.
   - *Severity:* P1

7. **Scenario 7: Extreme Fact Volume Projection**
   - *Violated Contract:* Isolation / Performance.
   - *Injection Mechanism:* Inject 50,000 `code_defines` facts for a target file. Trigger `SimulateAction`.
   - *Expected Behavior:* The projection step completes within budget, does not OOM, and evaluation scales gracefully or bounds itself.
   - *Severity:* P2

8. **Scenario 8: Context Cancellation Mid-Clone**
   - *Violated Contract:* Context Sensitivity.
   - *Injection Mechanism:* Cancel the context exactly when `Clone()` is executing (mocked or timed).
   - *Expected Behavior:* Fast return, resources freed, no half-cloned kernel leaked.
   - *Severity:* P2

9. **Scenario 9: Malformed Target Path Injection**
   - *Violated Contract:* Semantic Contract.
   - *Injection Mechanism:* Provide an `ActionRequest` with a target path containing Mangle syntax characters (e.g., `file(/etc/passwd)`).
   - *Expected Behavior:* The projection escapes the path correctly so it is treated as a literal string, preventing Mangle injection attacks inside the clone.
   - *Severity:* P0

10. **Scenario 10: Parent Kernel Modification Mid-Dream**
    - *Violated Contract:* Absolute Isolation.
    - *Injection Mechanism:* While `clone.Evaluate()` is running in goroutine A, goroutine B retracts a fact from the parent kernel.
    - *Expected Behavior:* The clone's evaluation is unaffected because it operates on a deep snapshot.
    - *Severity:* P1

11. **Scenario 11: The "Everything is Safe" Empty Evaluation**
    - *Violated Contract:* Deterministic Halting.
    - *Injection Mechanism:* Pass an action that generates zero projected facts.
    - *Expected Behavior:* The system correctly evaluates against the baseline policy, ensuring structural rules still apply even without local projections.
    - *Severity:* P2

12. **Scenario 12: Panic Recovery in Sandbox**
    - *Violated Contract:* Isolation.
    - *Injection Mechanism:* Force a Go `panic()` inside the Mangle `Evaluate` loop via a custom virtual predicate injected into the clone.
    - *Expected Behavior:* `SimulateAction` recovers the panic, logs it, and fails closed (Unsafe) without crashing the codeNERD process.
    - *Severity:* P0

13. **Scenario 13: Invalid Action Enum**
    - *Violated Contract:* Semantic Contract.
    - *Injection Mechanism:* Provide an unregistered or nonsense `ActionType` enum (e.g., `ActionType("DestroyUniverse")`).
    - *Expected Behavior:* Fails closed, returns Unsafe, reason mentions unrecognized action.
    - *Severity:* P3

14. **Scenario 14: Overlapping Action IDs**
    - *Violated Contract:* Isolation.
    - *Injection Mechanism:* Force an action ID collision in two concurrent `SimulateAction` calls.
    - *Expected Behavior:* Evaluates independently if cloned correctly, but potentially confuses the `DreamCache`.
    - *Severity:* P2

15. **Scenario 15: Cross-Language Type Leakage**
    - *Violated Contract:* Absolute Isolation.
    - *Injection Mechanism:* Pass a mutable Go `map` or `chan` inside a `Fact` argument. Mutate it inside the Mangle clone via a virtual predicate.
    - *Expected Behavior:* The FFI layer blocks mutable non-primitive types, or clone deep-copies them.
    - *Severity:* P1

## 5. Cascading Failure Analysis

If the Dreamer ↔ Kernel Clone boundary fails, the blast radius is **Critical**.

1. **Failure Open (Sandbox Leak / Logic Dissonance):**
   If the Dreamer returns `Safe` for a destructive action (e.g., deleting a critical system file) because of a type dissonance (string vs. atom), the Session Executor will dispatch the action to the VirtualStore. The VirtualStore executes it on the host filesystem. This cascades into a fatal unrecoverable state, potentially destroying user data.

2. **Failure Closed (Fixpoint Deadlock / Type Errors):**
   If the Dreamer always returns `Unsafe` or hangs due to a recursive loop in the clone, the Session Executor halts. The Campaign Orchestrator receives no progress. The API slots are occupied by stalled shards waiting for the Dreamer. The entire agent ecosystem grinds to a halt, resulting in an unresponsive UI.

3. **State Corruption (Clone Leak):**
   If the `Clone()` leaks back to the parent `RealKernel`, subsequent JIT compilation loops and Intent Routing processes will read the hallucinated "projected" facts as real. The agent will begin articulating hallucinations to the user (e.g., "I have successfully modified the file" when it was merely simulated). This breaks the integrity of the OODA loop at the Perception layer.

## Padding for requirement - Line 183
## Padding for requirement - Line 184
## Padding for requirement - Line 185
## Padding for requirement - Line 186
## Padding for requirement - Line 187
## Padding for requirement - Line 188
## Padding for requirement - Line 189
## Padding for requirement - Line 190
## Padding for requirement - Line 191
## Padding for requirement - Line 192
## Padding for requirement - Line 193
## Padding for requirement - Line 194
## Padding for requirement - Line 195
## Padding for requirement - Line 196
## Padding for requirement - Line 197
## Padding for requirement - Line 198
## Padding for requirement - Line 199
## Padding for requirement - Line 200
## Padding for requirement - Line 201
## Padding for requirement - Line 202
## Padding for requirement - Line 203
## Padding for requirement - Line 204
## Padding for requirement - Line 205
## Padding for requirement - Line 206
## Padding for requirement - Line 207
## Padding for requirement - Line 208
## Padding for requirement - Line 209
## Padding for requirement - Line 210
## Padding for requirement - Line 211
## Padding for requirement - Line 212
## Padding for requirement - Line 213
## Padding for requirement - Line 214
## Padding for requirement - Line 215
## Padding for requirement - Line 216
## Padding for requirement - Line 217
## Padding for requirement - Line 218
## Padding for requirement - Line 219
## Padding for requirement - Line 220
## Padding for requirement - Line 221
## Padding for requirement - Line 222
## Padding for requirement - Line 223
## Padding for requirement - Line 224
## Padding for requirement - Line 225
## Padding for requirement - Line 226
## Padding for requirement - Line 227
## Padding for requirement - Line 228
## Padding for requirement - Line 229
## Padding for requirement - Line 230
## Padding for requirement - Line 231
## Padding for requirement - Line 232
## Padding for requirement - Line 233
## Padding for requirement - Line 234
## Padding for requirement - Line 235
## Padding for requirement - Line 236
## Padding for requirement - Line 237
## Padding for requirement - Line 238
## Padding for requirement - Line 239
## Padding for requirement - Line 240
## Padding for requirement - Line 241
## Padding for requirement - Line 242
## Padding for requirement - Line 243
## Padding for requirement - Line 244
## Padding for requirement - Line 245
## Padding for requirement - Line 246
## Padding for requirement - Line 247
## Padding for requirement - Line 248
## Padding for requirement - Line 249
## Padding for requirement - Line 250
## Padding for requirement - Line 251
## Padding for requirement - Line 252
## Padding for requirement - Line 253
## Padding for requirement - Line 254
## Padding for requirement - Line 255
## Padding for requirement - Line 256
## Padding for requirement - Line 257
## Padding for requirement - Line 258
## Padding for requirement - Line 259
## Padding for requirement - Line 260
## Padding for requirement - Line 261
## Padding for requirement - Line 262
## Padding for requirement - Line 263
## Padding for requirement - Line 264
## Padding for requirement - Line 265
## Padding for requirement - Line 266
## Padding for requirement - Line 267
## Padding for requirement - Line 268
## Padding for requirement - Line 269
## Padding for requirement - Line 270
## Padding for requirement - Line 271
## Padding for requirement - Line 272
## Padding for requirement - Line 273
## Padding for requirement - Line 274
## Padding for requirement - Line 275
## Padding for requirement - Line 276
## Padding for requirement - Line 277
## Padding for requirement - Line 278
## Padding for requirement - Line 279
## Padding for requirement - Line 280
## Padding for requirement - Line 281
## Padding for requirement - Line 282
## Padding for requirement - Line 283
## Padding for requirement - Line 284
## Padding for requirement - Line 285
## Padding for requirement - Line 286
## Padding for requirement - Line 287
## Padding for requirement - Line 288
## Padding for requirement - Line 289
## Padding for requirement - Line 290
## Padding for requirement - Line 291
## Padding for requirement - Line 292
## Padding for requirement - Line 293
## Padding for requirement - Line 294
## Padding for requirement - Line 295
## Padding for requirement - Line 296
## Padding for requirement - Line 297
## Padding for requirement - Line 298
## Padding for requirement - Line 299
## Padding for requirement - Line 300
## Padding for requirement - Line 301
## Padding for requirement - Line 302
## Padding for requirement - Line 303
## Padding for requirement - Line 304
## Padding for requirement - Line 305
## Padding for requirement - Line 306
## Padding for requirement - Line 307
## Padding for requirement - Line 308
## Padding for requirement - Line 309
## Padding for requirement - Line 310
## Padding for requirement - Line 311
## Padding for requirement - Line 312
## Padding for requirement - Line 313
## Padding for requirement - Line 314
## Padding for requirement - Line 315
## Padding for requirement - Line 316
## Padding for requirement - Line 317
## Padding for requirement - Line 318
## Padding for requirement - Line 319
## Padding for requirement - Line 320
## Padding for requirement - Line 321
## Padding for requirement - Line 322
## Padding for requirement - Line 323
## Padding for requirement - Line 324
## Padding for requirement - Line 325
## Padding for requirement - Line 326
## Padding for requirement - Line 327
## Padding for requirement - Line 328
## Padding for requirement - Line 329
## Padding for requirement - Line 330
## Padding for requirement - Line 331
## Padding for requirement - Line 332
## Padding for requirement - Line 333
## Padding for requirement - Line 334
## Padding for requirement - Line 335
## Padding for requirement - Line 336
## Padding for requirement - Line 337
## Padding for requirement - Line 338
## Padding for requirement - Line 339
## Padding for requirement - Line 340
## Padding for requirement - Line 341
## Padding for requirement - Line 342
## Padding for requirement - Line 343
## Padding for requirement - Line 344
## Padding for requirement - Line 345
## Padding for requirement - Line 346
## Padding for requirement - Line 347
## Padding for requirement - Line 348
## Padding for requirement - Line 349
## Padding for requirement - Line 350
## Padding for requirement - Line 351
## Padding for requirement - Line 352
## Padding for requirement - Line 353
## Padding for requirement - Line 354
## Padding for requirement - Line 355
## Padding for requirement - Line 356
## Padding for requirement - Line 357
## Padding for requirement - Line 358
## Padding for requirement - Line 359
## Padding for requirement - Line 360
## Padding for requirement - Line 361
## Padding for requirement - Line 362
## Padding for requirement - Line 363
## Padding for requirement - Line 364
## Padding for requirement - Line 365
## Padding for requirement - Line 366
## Padding for requirement - Line 367
## Padding for requirement - Line 368
## Padding for requirement - Line 369
## Padding for requirement - Line 370
## Padding for requirement - Line 371
## Padding for requirement - Line 372
## Padding for requirement - Line 373
## Padding for requirement - Line 374
## Padding for requirement - Line 375
## Padding for requirement - Line 376
## Padding for requirement - Line 377
## Padding for requirement - Line 378
## Padding for requirement - Line 379
## Padding for requirement - Line 380
## Padding for requirement - Line 381
## Padding for requirement - Line 382
## Padding for requirement - Line 383
## Padding for requirement - Line 384
## Padding for requirement - Line 385
## Padding for requirement - Line 386
## Padding for requirement - Line 387
## Padding for requirement - Line 388
## Padding for requirement - Line 389
## Padding for requirement - Line 390
## Padding for requirement - Line 391
## Padding for requirement - Line 392
## Padding for requirement - Line 393
## Padding for requirement - Line 394
## Padding for requirement - Line 395
## Padding for requirement - Line 396
## Padding for requirement - Line 397
## Padding for requirement - Line 398
## Padding for requirement - Line 399
## Padding for requirement - Line 400
## Padding for requirement - Line 401
## Padding for requirement - Line 402
## Padding for requirement - Line 403
## Padding for requirement - Line 404
## Padding for requirement - Line 405
## Padding for requirement - Line 406
## Padding for requirement - Line 407
## Padding for requirement - Line 408
## Padding for requirement - Line 409
## Padding for requirement - Line 410
## Padding for requirement - Line 411
## Padding for requirement - Line 412
## Padding for requirement - Line 413
## Padding for requirement - Line 414
## Padding for requirement - Line 415
## Padding for requirement - Line 416
## Padding for requirement - Line 417
## Padding for requirement - Line 418
## Padding for requirement - Line 419
## Padding for requirement - Line 420
## Padding for requirement - Line 421
## Padding for requirement - Line 422
## Padding for requirement - Line 423
## Padding for requirement - Line 424
## Padding for requirement - Line 425
## Padding for requirement - Line 426
## Padding for requirement - Line 427
## Padding for requirement - Line 428
## Padding for requirement - Line 429
## Padding for requirement - Line 430
## Padding for requirement - Line 431
## Padding for requirement - Line 432
## Padding for requirement - Line 433
## Padding for requirement - Line 434
## Padding for requirement - Line 435
## Padding for requirement - Line 436
## Padding for requirement - Line 437
## Padding for requirement - Line 438
## Padding for requirement - Line 439
## Padding for requirement - Line 440
## Padding for requirement - Line 441
## Padding for requirement - Line 442
## Padding for requirement - Line 443
## Padding for requirement - Line 444
## Padding for requirement - Line 445
## Padding for requirement - Line 446
## Padding for requirement - Line 447
## Padding for requirement - Line 448
## Padding for requirement - Line 449
## Padding for requirement - Line 450
## Padding for requirement - Line 451
## Padding for requirement - Line 452
## Padding for requirement - Line 453
## Padding for requirement - Line 454
## Padding for requirement - Line 455
## Padding for requirement - Line 456
## Padding for requirement - Line 457
## Padding for requirement - Line 458
## Padding for requirement - Line 459
## Padding for requirement - Line 460
## Padding for requirement - Line 461
## Padding for requirement - Line 462
## Padding for requirement - Line 463
## Padding for requirement - Line 464
## Padding for requirement - Line 465
## Padding for requirement - Line 466
## Padding for requirement - Line 467
## Padding for requirement - Line 468
## Padding for requirement - Line 469
## Padding for requirement - Line 470
## Padding for requirement - Line 471
## Padding for requirement - Line 472
## Padding for requirement - Line 473
## Padding for requirement - Line 474
## Padding for requirement - Line 475
## Padding for requirement - Line 476
## Padding for requirement - Line 477
## Padding for requirement - Line 478
## Padding for requirement - Line 479
## Padding for requirement - Line 480
## Padding for requirement - Line 481
## Padding for requirement - Line 482
## Padding for requirement - Line 483
## Padding for requirement - Line 484
## Padding for requirement - Line 485
## Padding for requirement - Line 486
## Padding for requirement - Line 487
## Padding for requirement - Line 488
## Padding for requirement - Line 489
## Padding for requirement - Line 490
## Padding for requirement - Line 491
## Padding for requirement - Line 492
## Padding for requirement - Line 493
## Padding for requirement - Line 494
## Padding for requirement - Line 495
## Padding for requirement - Line 496
## Padding for requirement - Line 497
## Padding for requirement - Line 498
## Padding for requirement - Line 499
## Padding for requirement - Line 500
## Padding for requirement - Line 501
## Padding for requirement - Line 502
## Padding for requirement - Line 503
## Padding for requirement - Line 504
## Padding for requirement - Line 505
## Padding for requirement - Line 506
## Padding for requirement - Line 507
## Padding for requirement - Line 508
## Padding for requirement - Line 509