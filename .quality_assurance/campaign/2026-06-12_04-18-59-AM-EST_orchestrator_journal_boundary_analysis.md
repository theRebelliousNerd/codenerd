# QA Journal: Boundary Value Analysis and Negative Testing
## Subsystem: orchestrator_journal.go (internal/campaign)
## Date: 2026-06-12
## Time: 04:18:59 AM EST

### 1. Introduction
This journal documents the boundary value analysis and negative testing evaluation for the `orchestrator_journal.go` module within the `campaign` subsystem of codeNERD.

The system is responsible for persisting a durable append-only event log (journaling) to ensure that campaign state updates (e.g. phases completed, tasks created) survive crashes or network interrupts, guaranteeing exactly-once semantics during recovery via sequence numbering and SHA256 checksum validation.

The testing evaluation aims to identify critical gaps primarily surrounding corrupted states, concurrency bounds, out-of-memory attack vectors via payloads, and edge cases related to cross-platform atomic renaming and directory syncing.

### 2. Missing Boundary / Negative Test Scenarios

#### 2.1 Null/Undefined/Empty vectors
1.  **Empty/Null Payloads**: If a payload passed into `appendJournalEventLocked` is technically an empty interface `any(nil)`, the JSON marshaling of `rawPayload` might result in literal `"null"` instead of being omitted. Wait, looking at the code: `if payload != nil { b, err := json.Marshal(payload); rawPayload = b }`. If payload is nil, `rawPayload` is nil. Because `json:"payload,omitzero"` is used in Go 1.22+, `nil` will be completely omitted from the JSON output. However, a test must explicitly verify that passing `nil` generates a journal line that does not contain a "payload" key, and more importantly, that `checksumJournalEvent` handles `ev.Payload` (which is `nil`) correctly by writing an empty byte slice to the hash.
2.  **Empty Strings**: What if `CampaignID`, `EventType`, or `SnapshotChecksum` are empty strings? An empty `CampaignID` would write a journal to `.nerd/campaigns/.journal.jsonl`. Does this break recovery?

#### 2.2 Type Coercion vectors
1.  **Corrupt JSON Payload**: If `recoverJournalSequence` encounters a line that is valid JSON structurally but the sequence number is a string `"1"` instead of an integer `1`, `json.Unmarshal([]byte(line), &ev)` will fail. The system will truncate the journal from that point. This needs to be explicitly tested.
2.  **Checksum Type Coercion**: A malicious or malformed log where the checksum is technically valid hex, but encodes non-utf8 garbage.

#### 2.3 User request Extremes vectors
1.  **Massive Payloads (OOM DOS)**: If a user runs a frontier coding benchmark that returns a 50MB string as a task result, and that task result is journaled as the `payload`. The `appendJournalEventLocked` will marshal a 50MB string.
    More dangerously, when `recoverJournalSequence` runs, it uses `scanner.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)`. The maximum buffer size for the scanner is set to **8MB**.
    If a single JSON journal line exceeds 8MB, `scanner.Scan()` will return `ErrTooLong`.
    The error handler:
    ```go
    if scanErr := scanner.Err(); scanErr != nil {
        needsTruncate = true
    }
    ```
    This means if a massive payload (e.g., >8MB code diff) is logged successfully by `appendJournalEventLocked`, upon reboot, `recoverJournalSequence` will fail to read it, **assume the file is corrupted, and permanently delete the 50MB payload line and everything after it!** This is a critical data-loss vector under extreme user inputs.
2.  **Massive Sequences**: If sequence numbers hit `math.MaxUint64`, does it wrap around and break the `lastSeq+1` logic in recovery? Unlikely for an agent, but theoretically possible.

#### 2.4 State Conflicts vectors
1.  **Concurrent Appends**: `appendJournalEventLocked` is meant to be called under a lock. However, there is no internal synchronization within `appendJournalEventLocked` itself. If two shards bypass the orchestrator lock (e.g. by accident in future refactors), it will interleave lines, corrupting JSON.
2.  **TOC/TOU in Atomic Rename**: In `writeCampaignSnapshotAtomic`, a temporary file is created, written, closed, verified, and renamed.
    ```go
    // Verify bytes before rename
    verifyBytes, err := os.ReadFile(tmpPath)
    if checksumBytes(verifyBytes) != checksumBytes(data) {
        return fmt.Errorf("snapshot temp verification checksum mismatch")
    }
    if err := renameAtomicReplace(tmpPath, path); err != nil {
        return fmt.Errorf("atomic snapshot rename: %w", err)
    }
    ```
    If another process modifies `tmpPath` *after* the `ReadFile` verification but *before* `renameAtomicReplace`, the system replaces the snapshot with corrupted data. This is a classic Time-Of-Check/Time-Of-Use race condition. While difficult to hit, a high-frequency adversary process could exploit it.
3.  **Sync Directory Failures (Cross Platform)**:
    ```go
    func syncDirIfSupported(dir string) error {
        if runtime.GOOS == "windows" { return nil }
        d, err := os.Open(dir)
        defer d.Close()
        return d.Sync()
    }
    ```
    Opening a directory in read-only mode to sync it is POSIX compliant, but what happens if the OS allows `os.Open` but rejects `d.Sync()` with `EINVAL`? The error propagates up, failing the journal append. The test suite needs to mock or test edge OS configurations (like mounted network drives, NFS, FUSE) where `d.Sync()` returns an error to ensure `appendJournalEventLocked` handles it correctly.

### 3. Actionable Items & Test Gaps
The following missing tests will be added directly into a new `internal/campaign/orchestrator_journal_test.go` file as TODOs:

1. **Scanner Limit Exceeded (Data Loss):** Verify that journal recovery does not inadvertently truncate valid events that exceed the `8MB` `bufio.Scanner` limit.
2. **Missing/Nil Payload Hash Verification:** Verify `checksumJournalEvent` deterministic hashing when `ev.Payload` is exactly `nil` vs `[]byte("")`.
3. **Sequence Mismatch Truncation Test:** Verify that if line `N` is corrupt, recovery correctly preserves `0..N-1` and safely overwrites the corrupt line.
4. **FUSE/NFS `syncDirIfSupported` Error:** Verify that the system handles `Sync()` returning an unsupported operation error gracefully without breaking campaign flow.

### 4. Conclusion
The most critical finding is the **8MB Scanner Limit Data-Loss Vector** (User Request Extremes). If `appendJournalEventLocked` has no maximum size limit on what it will marshal to disk, but `recoverJournalSequence` has a hard 8MB limit for reading, any payload >8MB will corrupt the campaign upon reboot. The system must either limit payload sizes *before* writing or stream-parse the JSONl file instead of using `bufio.Scanner` bounded by 8MB.
