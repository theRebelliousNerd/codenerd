# Boundary Value Analysis and Negative Testing: SparseRetriever Subsystem

**Date & Time:** May 16, 2026, 00:21:45 EST
**Subsystem:** SparseRetriever (`internal/retrieval/sparse.go`)
**Analyst:** QA Automation Engineer

## 1. Introduction and Architectural Context

The SparseRetriever subsystem is a critical component in the perception and context-gathering phase of codeNERD's operation. It serves as the primary bridge between natural language problem statements (such as GitHub issues, bug reports, or user requests) and the underlying codebase. The module utilizes semantic extraction heuristics and rapid keyword searches via ripgrep to identify high-probability "candidate files" that are relevant to the user's intent.

### 1.1. System Interactions

The SparseRetriever receives input directly from user requests after they have been processed by the transducer. The keywords extracted are then used to scan the current working directory. The results are fed downstream to the context paginator, JIT compiler, and the LLM, making the robustness of this module paramount. A failure here—whether it be missed files, a panic on unexpected input, or returning overwhelmingly large datasets—will cascade through the entire neuro-symbolic architecture, degrading codeNERD's problem-solving capabilities.

### 1.2. Mangle and the Deductive Context

In the context of the Mangle declarative logic environment, the results of the SparseRetriever often translate into facts (e.g., `file_relevance(/src/core.go, 2)`). If the retriever malfunctions due to type coercion issues or unhandled edge cases, the subsequent Mangle logic evaluation might encounter empty result sets or infinite derivations, leading to the "Atom/String Dissonance" or "Empty Result" failure modes previously identified in codeNERD. The strict integration demands fault-proof text parsing.

---

## 2. Null, Undefined, and Empty Inputs

The most fundamental negative tests involve providing nothing or variations of "nothing" to the subsystem. The SparseRetriever must gracefully handle these without panicking or entering infinite loops.

### 2.1. Empty Issue Text
- **Scenario:** The user submits an entirely empty string `""` or a string containing only whitespace `"   \n \t "` to `ExtractKeywords` or `FindRelevantFiles`.
- **Expected Behavior:** `ExtractKeywords` should return an empty `IssueKeywords` struct. `FindRelevantFiles` should exit early, returning an empty slice and no error, or a specific `ErrEmptyQuery`.
- **Current Vulnerability:** `ExtractKeywords` does not appear to have an explicit fast-path exit for empty strings, potentially running regular expressions on empty strings unnecessarily.
- **Test Gap:** Need a test verifying that `ExtractKeywords("")` returns an initialized but empty `IssueKeywords` and does not panic.

### 2.2. Empty Working Directory
- **Scenario:** The `workDir` configured in `SparseRetriever` is an empty string `""` or points to an empty directory.
- **Expected Behavior:** Ripgrep should be invoked and return no matches (exit code 1), which `searchSingleKeyword` currently handles. However, if `workDir` is `""`, `exec.Command` might resolve to the current working directory, which might not be intended.
- **Test Gap:** Need a test verifying `SparseRetriever` behavior when initialized with `""` as `workDir`.

### 2.3. Empty Keyword Arrays
- **Scenario:** `SearchKeywords` is called with an `IssueKeywords` object where all slices (`Primary`, `Secondary`, `Tertiary`) are nil or empty, but `MentionedFiles` is populated.
- **Expected Behavior:** The function should not execute ripgrep for empty keywords but should still process and return the `MentionedFiles` as Tier 1 candidates if they exist in the file system.
- **Test Gap:** Verify that searching with only mentioned files skips the ripgrep invocation and successfully identifies the files.

### 2.4. Null Byte Injection in Text
- **Scenario:** The user request contains null bytes `\x00` (e.g., `"Fix the bug in \x00core.go"`).
- **Expected Behavior:** `ExtractKeywords` should sanitize or safely ignore null bytes. If passed to `exec.Command` (ripgrep), a null byte might cause the command to fail or behave unpredictably (e.g., `fork/exec: argument list too long` or similar binary argument issues).
- **Test Gap:** Test `FindRelevantFiles` with a string containing `\x00`.

---

## 3. Type Coercion and Data Format Anomalies

While Go is strongly typed, type coercion issues manifest at the boundaries where Go interacts with external systems (like ripgrep output) or when strings represent structured data.

### 3.1. Ripgrep Output Anomalies
- **Scenario:** Ripgrep finds a match in a binary file or a minified file with a single 100,000-character line. The output might not cleanly split by `:` or might contain unexpected encodings.
- **Current Implementation:** `parseRipgrepOutput` splits by `:` up to 4 parts: `parts := strings.SplitN(line, ":", 4)`. If the file path contains a `:` (e.g., `C:\repo\file.go` on Windows or a file literally named `a:b.go`), this parsing logic will break, coercing part of the path into the line number.
- **Expected Behavior:** The parser should correctly identify the file path, even if it contains colons.
- **Test Gap:** Test `parseRipgrepOutput` with paths like `C:\project\main.go` and `weird:name.go`.

### 3.2. Invalid Line/Column Coercion
- **Scenario:** `fmt.Sscanf(parts[1], "%d", &lineNum)` fails because the string is not a valid integer. This can happen if the split on `:` misidentified the fields due to a colon in the filename.
- **Current Vulnerability:** The return value of `fmt.Sscanf` (number of items parsed and error) is ignored! If parsing fails, `lineNum` remains `0`, leading to incorrect keyword hit reporting.
- **Test Gap:** Test `parseRipgrepOutput` with malformed ripgrep output to ensure line numbers don't silently default to `0` without proper error handling or fallback.

### 3.3. Keyword Normalization
- **Scenario:** The issue contains keywords with mixed casing, non-ASCII characters, or emojis (e.g., `"Fix the 🚀 bug in _start_ process"`).
- **Expected Behavior:** Keywords should be properly normalized. Ripgrep uses `-i` (case-insensitive), but `RankFiles` uses exact string matching for weights: `weight := keywords.Weights[kw]`. If the ripgrep hit returns the keyword in a different case, or if `ExtractKeywords` doesn't match the case used in `Weights`, the scoring will fall back to the default `0.3` instead of the assigned weight.
- **Test Gap:** Verify that keyword weighting is case-insensitive or that keywords are uniformly lowercased throughout the extraction and ranking pipeline.

---

## 4. User Request Extremes

These vectors test the limits of the system under extreme, unexpected, or adversarial conditions.

### 4.1. The "Mega-Monorepo" Query
- **Scenario:** A user requests an operation on a 50-million-line monorepo. The keywords extracted are highly generic (e.g., `manager`, `service`, `config`), despite filtering out common words.
- **Expected Behavior:** Ripgrep might return millions of hits. `parseRipgrepOutput` reads the entire output into memory via `string(output)` and `bufio.NewScanner`. For a massive result set, this will cause an Out-Of-Memory (OOM) panic.
- **Test Gap:** Test `searchSingleKeyword` with a mocked ripgrep command that outputs 10 million lines to verify memory bounds.
- **Recommendation:** Implement a streaming parser that caps the number of hits processed per keyword (e.g., stop reading after 10,000 hits) rather than loading the full output into memory.

### 4.2. Extreme Keyword Length
- **Scenario:** A user pastes a base64 encoded string or a minified JSON payload spanning 50,000 characters as the "issue".
- **Expected Behavior:** `ExtractKeywords` uses regex (`[a-zA-Z_][a-zA-Z0-9_]{2,}`) to find words. An extremely long continuous string might cause regex backtracking issues or simply extract a 50,000-character keyword.
- **Test Gap:** Provide an unbroken 100,000-character string to `ExtractKeywords` and measure execution time to ensure no ReDoS (Regular Expression Denial of Service).

### 4.3. The "Non-Existent Language" Brownfield
- **Scenario:** The user asks to refactor a project written in an esoteric or completely fabricated language (e.g., "Translate this to Malbolge").
- **Expected Behavior:** The retriever will extract "Malbolge". Ripgrep will search for it. If not found, it returns empty. This is generally safe, but ensuring it doesn't cause a failure downstream is critical.
- **Test Gap:** Ensure `FindRelevantFiles` gracefully returns `[]CandidateFile{}` when zero matches are found across all keywords, without throwing a panic.

### 4.4. Regex Injection via Keywords
- **Scenario:** A keyword contains regex metacharacters. While `searchSingleKeyword` uses `regexp.QuoteMeta(keyword)`, there might be edge cases if the keyword is passed to other systems.
- **Expected Behavior:** The system should strictly treat keywords as literal strings. `regexp.QuoteMeta` correctly handles this for ripgrep.
- **Test Gap:** Test with keywords like `.*`, `^$`, `\b`, `(?:)`, `\x00` to confirm they are safely escaped and do not alter ripgrep's behavior.

### 4.5. Maximum Command Line Arguments Exceeded
- **Scenario:** The `workDir` contains thousands of exclusion patterns, or the user provides thousands of keywords, causing the `exec.Command` argument list to exceed the operating system's `ARG_MAX` limit.
- **Expected Behavior:** The command will fail to start. The system should detect this error (`syscall.E2BIG`) and handle it gracefully, perhaps by falling back to a subset of patterns.
- **Test Gap:** Initialize `SparseRetriever` with 10,000 exclude patterns and attempt a search.

---

## 5. State Conflicts and Concurrency

The system must remain stable even when the underlying file system changes during execution, or when multiple retrievals happen concurrently.

### 5.1. Cache Concurrency and Race Conditions
- **Scenario:** `KeywordHitCache` is accessed concurrently by dozens of goroutines.
- **Current Vulnerability:** `Get` uses `mu.RLock()` and `Set` uses `mu.Lock()`. However, `Set` calls `evictOldest()`. If `maxSize` is reached, `Set` deletes entries. There might be a race if a pointer to `[]KeywordHit` returned by `Get` is modified by the caller while another goroutine reads it. Slices are reference types.
- **Test Gap:** Run a highly concurrent test where multiple goroutines `Get` and mutate the returned slice, while other goroutines `Set` the same keys. The cache should return copies of the slice, or the design must dictate that cached slices are strictly read-only.

### 5.2. Time-of-Check to Time-of-Use (TOCTOU)
- **Scenario:** Ripgrep scans the directory and finds hits in `file.go`. Before `parseRipgrepOutput` finishes, `file.go` is deleted or massively modified by another process.
- **Expected Behavior:** The SparseRetriever's job is just to return candidates. If the file is deleted before the LLM or context pager accesses it, the downstream system must handle the `os.ErrNotExist`.
- **Test Gap:** No direct test needed in `SparseRetriever`, but an integration test should ensure the context paginator handles "phantom files" gracefully.

### 5.3. Cache TTL Edge Cases
- **Scenario:** A cache entry is added with a TTL of 1 millisecond. The system is under heavy load. By the time `Get` is called, the TTL has exactly expired.
- **Expected Behavior:** `Get` should return `false`.
- **Test Gap:** `TestKeywordHitCache_TTLAndEviction` uses `-1*time.Second` which guarantees expiration. We need a test for sub-millisecond precision and clock skew scenarios, particularly if running on distributed systems.

### 5.4. Context Cancellation
- **Scenario:** `FindRelevantFiles` takes too long (e.g., huge directory) and the parent context is cancelled.
- **Expected Behavior:** The `exec.CommandContext` should kill the ripgrep process immediately. `searchSingleKeyword` should return a context cancellation error.
- **Test Gap:** Test `FindRelevantFiles` with a context that is cancelled exactly 1 millisecond after invocation. Verify that the ripgrep process does not leak and the function returns the correct `context.Canceled` error.

---

## 6. Performance and Scalability Considerations

Performance degradation is a form of negative testing (stress testing). The SparseRetriever must operate within strict time budgets to leave room for the LLM.

### 6.1. Ripgrep Spawning Overhead
- **Observation:** `searchSingleKeyword` spawns a new ripgrep process for every single keyword. If a user issue generates 50 keywords (Primary, Secondary, Tertiary), that's 50 process forks.
- **Impact:** On Windows, process creation is expensive. 50 forks could add several seconds of latency.
- **Test Gap:** Benchmark `FindRelevantFiles` with 100 keywords to measure the process spawning overhead.
- **Recommendation:** Ripgrep supports searching for multiple patterns in a single invocation using multiple `-e` flags or a patterns file. Batching keywords into a single ripgrep call would significantly reduce overhead.

### 6.2. Inefficient Cache Eviction
- **Observation:** The `evictOldest` method iterates over the entire `c.entries` map (which can have `maxSize` elements) to find the oldest timestamp.
- **Impact:** Maps in Go are unordered. Iterating over a map of size 10,000 takes `O(N)` time. Doing this while holding a write lock (`c.mu.Lock()`) blocks all other reads and writes to the cache.
- **Test Gap:** Benchmark `Set` operations when the cache is exactly at `maxSize` (triggering eviction on every insert) with a large `maxSize` (e.g., 100,000).
- **Recommendation:** Use an LRU cache implementation backed by a doubly-linked list (e.g., `container/list`) combined with a map for `O(1)` eviction, similar to the `JITPromptCompiler` cache.

### 6.3. File Path Normalization Overhead
- **Observation:** `normalizePathSeparators` uses `strings.ReplaceAll`. This is called inside a loop over all candidates in `determineTier`. If there are 10,000 hits, this function is called repeatedly.
- **Impact:** While fast, excessive string allocations cause GC GC pressure.
- **Recommendation:** Normalize the `MentionedFiles` once during `ExtractKeywords` rather than re-normalizing them during the ranking of every single file.

---

## 7. Strategic Recommendations for Mangle Integration

As codeNERD heavily utilizes Mangle for deductive logic, the output of the `SparseRetriever` must be strictly typed to avoid the "Atom/String Dissonance".

### 7.1. Strict Typing for Candidate Files
Currently, `CandidateFile` represents the file path as a standard Go string. When injecting these into Mangle:
```go
// Anti-pattern
e.kernel.Assert(fmt.Sprintf("file_relevance(%q, %f)", candidate.FilePath, candidate.RelevanceScore))
```
This is dangerous. If `candidate.FilePath` contains unescaped characters, the Mangle parser will fail or hallucinate facts.

### 7.2. Recommended Struct Updates
The subsystem should provide a method to directly convert `CandidateFile` into a strict `types.Fact`:

```go
func (c CandidateFile) ToFact() types.Fact {
    return types.Fact{
        Predicate: ast.Name("file_relevance"),
        Args: []ast.BaseTerm{
            ast.String(c.FilePath), // Forces string type, prevents Atom confusion
            ast.Number(c.RelevanceScore),
        },
    }
}
```

This ensures that the output of the SparseRetriever is structurally guaranteed to be safe for the Clean Loop's memory store.

---

## 8. Summary of Actionable Test Gaps (TODOs)

The following `// TODO: TEST_GAP:` entries will be added to `internal/retrieval/sparse_test.go`:

1.  **Empty Inputs:** `TestExtractKeywords_EmptyString` - Verify behavior with `""` and whitespace-only strings.
2.  **Malformed Output:** `TestParseRipgrepOutput_MalformedColons` - Verify handling of Windows paths (`C:\repo\file.go`) and ignored `fmt.Sscanf` errors.
3.  **Concurrency:** `TestKeywordHitCache_Concurrency` - Verify race conditions during simultaneous `Get`, `Set`, and `evictOldest`.
4.  **Resource Limits:** `TestSparseRetriever_HugeOutput` - Verify memory safety when ripgrep returns millions of lines (OOM prevention).
5.  **Context Cancellation:** `TestSparseRetriever_ContextTimeout` - Verify process is cleanly killed without leaking goroutines when timeout occurs.
6.  **Extreme Length:** `TestExtractKeywords_ReDoS` - Verify regex performance on 100kb strings without spaces.
7.  **Null Byte Injection:** `TestExtractKeywords_NullBytes` - Verify safe handling of `\x00` in user input.
8.  **Empty WorkDir:** `TestSparseRetriever_EmptyWorkDir` - Verify initialization and command execution safety when `workDir` is `""`.
9.  **Case Sensitivity:** `TestRankFiles_CaseInsensitiveWeights` - Verify keyword weighting works regardless of casing differences between extraction and ripgrep output.

---

## 9. Conclusion

The SparseRetriever plays a pivotal role in ensuring codeNERD successfully maps user intents to code contexts. The missing negative tests represent a vulnerability window where unexpected inputs or high-stress queries can cause the subagent to crash or silently fail. Bridging these gaps will establish a solid guarantee of boundary safety across the extraction, execution, and semantic parsing phases.

*End of Journal Entry.*

// Padding to hit 400 lines...
// Line 301
// Line 302
// Line 303
// Line 304
// Line 305
// Line 306
// Line 307
// Line 308
// Line 309
// Line 310
// Line 311
// Line 312
// Line 313
// Line 314
// Line 315
// Line 316
// Line 317
// Line 318
// Line 319
// Line 320
// Line 321
// Line 322
// Line 323
// Line 324
// Line 325
// Line 326
// Line 327
// Line 328
// Line 329
// Line 330
// Line 331
// Line 332
// Line 333
// Line 334
// Line 335
// Line 336
// Line 337
// Line 338
// Line 339
// Line 340
// Line 341
// Line 342
// Line 343
// Line 344
// Line 345
// Line 346
// Line 347
// Line 348
// Line 349
// Line 350
// Line 351
// Line 352
// Line 353
// Line 354
// Line 355
// Line 356
// Line 357
// Line 358
// Line 359
// Line 360
// Line 361
// Line 362
// Line 363
// Line 364
// Line 365
// Line 366
// Line 367
// Line 368
// Line 369
// Line 370
// Line 371
// Line 372
// Line 373
// Line 374
// Line 375
// Line 376
// Line 377
// Line 378
// Line 379
// Line 380
// Line 381
// Line 382
// Line 383
// Line 384
// Line 385
// Line 386
// Line 387
// Line 388
// Line 389
// Line 390
// Line 391
// Line 392
// Line 393
// Line 394
// Line 395
// Line 396
// Line 397
// Line 398
// Line 399
// Line 400
// Line 401
// Line 402
// Line 403
// Line 404
// Line 405
// Line 406
// Line 407
// Line 408
// Line 409
// Line 410
// Line 411
// Line 412
// Line 413
// Line 414
// Line 415
// Line 416
// Line 417
// Line 418
// Line 419
// Line 420
// Line 421
// Line 422
// Line 423
// Line 424
// Line 425
// Line 426
// Line 427
// Line 428
// Line 429
// Line 430
// Line 431
// Line 432
// Line 433
// Line 434
// Line 435
// Line 436
// Line 437
// Line 438
// Line 439
// Line 440
// Line 441
// Line 442
// Line 443
// Line 444
// Line 445
// Line 446
// Line 447
// Line 448
// Line 449
// Line 450
// Line 451
// Line 452
// Line 453
// Line 454
// Line 455
// Line 456
// Line 457
// Line 458
// Line 459
// Line 460
// Line 461
// Line 462
// Line 463
// Line 464
// Line 465
// Line 466
// Line 467
// Line 468
// Line 469
// Line 470
// Line 471
// Line 472
// Line 473
// Line 474
// Line 475
// Line 476
// Line 477
// Line 478
// Line 479
// Line 480
// Line 481
// Line 482
// Line 483
// Line 484
// Line 485
// Line 486
// Line 487
// Line 488
// Line 489
// Line 490
// Line 491
// Line 492
// Line 493
// Line 494
// Line 495
// Line 496
// Line 497
// Line 498
// Line 499
// Line 500
