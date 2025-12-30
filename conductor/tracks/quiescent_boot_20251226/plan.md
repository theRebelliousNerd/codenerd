# Plan: Quiescent Boot Mode (Architectural Fix)

**Goal:** Eliminate the need for a "boot guard" by fixing the architectural root cause that causes the system to automatically execute actions upon boot. The system should initialize in a naturally passive state.

**User Complaint:** "we shouldnt need a boot guard... its just a bandaid for some kind of architectural error... codenerd needs to just start on bootup, not rehydrate and just start taking a bunch of actions"

**Root Cause Analysis:**
- The "boot guard" was masking a problem where the system rehydrates state (likely `user_intent` or `next_action` facts) or auto-starts processes (like `WorldModelIngestor` scanning) that immediately trigger the execution loop.
- Simply clearing `user_intent` in `ExecutivePolicyShard` was apparently insufficient or happening at the wrong time/race condition.
- `hydrateNerdState` or `HydrateSessionContext` might be re-injecting triggers *after* the cleanup.
- System shards (like `WorldModelIngestor`) might be acting autonomously on boot.

**Revised Strategy:**
1.  **Remove Boot Guards:** Revert the boolean flags and checks in `ExecutivePolicyShard` and `VirtualStore`.
2.  **Audit Rehydration:** pinpoint exactly *what* facts are loaded during `performSystemBoot`.
3.  **Fix Lifecycle:**
    - Ensure `user_intent` and `next_action` are **NEVER** persisted to disk (or are filtered out on load).
    - Ensure `WorldModelIngestor` and other system shards start in a "Paused" or "Listening" mode, not an "Active Scan" mode, unless explicitly triggered.
    - Verify `MigrateOldSessionsToSQLite` does not side-effect into the active Kernel.
4.  **Verify Quiescence:** Boot the system and ensure `next_action` query returns empty without artificial blocks.

**Tasks:**
- [x] Revert "boot guard" logic in `executive.go` and `virtual_store.go`.
- [ ] Analyze `cmd/nerd/chat/session.go` -> `hydrateNerdState` / `LoadFactsFromFile` to see if `profile.mg` or other files contain ephemeral state.
- [ ] Analyze `internal/shards/system/world_model.go` for auto-scan behavior.
- [ ] Implement a clean separation between "Persisted Knowledge" (rules, long-term memory) and "Ephemeral Session State" (intents, actions).
- [ ] Verify that `performSystemBoot` does not trigger `processInput` logic.
