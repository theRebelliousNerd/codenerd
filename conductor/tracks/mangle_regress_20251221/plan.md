# Track Plan: Mangle Logic Validation and Regression Suite

## Phase 1: Test Harness Development (Go)
Goal: Build the infrastructure to load, run, and assert facts against Mangle programs in a testing context.

- [~] Task: Create `internal/mangle/testing` package and base test runner
    - [ ] Sub-task: Write unit test for Mangle program loader
    - [ ] Sub-task: Implement `Loader` that reads `.mg` files and handles includes
- [ ] Task: Implement Assertion Engine
    - [ ] Sub-task: Write tests for `AssertFact` and `QueryExpect` methods
    - [ ] Sub-task: Implement logic to verify derived facts against expected results
- [ ] Task: Conductor - User Manual Verification 'Phase 1: Test Harness Development (Go)' (Protocol in workflow.md)

## Phase 2: Core Policy Regression Suite
Goal: Implement the first set of regression tests for existing core policies.

- [ ] Task: Implement Intent-to-Action mapping tests
    - [ ] Sub-task: Write Mangle "tests" for basic commands (init, run, scan)
    - [ ] Sub-task: Verify that `next_action` is correctly derived for standard intents
- [ ] Task: Implement Constitutional Safety Invariants
    - [ ] Sub-task: Write negative tests (e.g., ensure `rm` actions are NOT `permitted` by default)
    - [ ] Sub-task: Verify `admin_override` and `signed_approval` logic chains
- [ ] Task: Conductor - User Manual Verification 'Phase 2: Core Policy Regression Suite' (Protocol in workflow.md)

## Phase 3: Adversarial Mitigation & Regression
Goal: Link the harness to known failure modes to prevent re-emergence.

- [ ] Task: Encode "AI Failure Modes" into regression tests
    - [ ] Sub-task: Create tests for "Forgotten Sender" and "Nil Channel" detection rules
    - [ ] Sub-task: Verify that the "Commit Barrier" blocks commits on specific logic errors
- [ ] Task: Integrate with Nemesis results
    - [ ] Sub-task: Implement a way to load Nemesis-discovered vulnerabilities as regression cases
- [ ] Task: Conductor - User Manual Verification 'Phase 3: Adversarial Mitigation & Regression' (Protocol in workflow.md)

## Phase 4: Integration & CI/CD
Goal: Finalize the suite and ensure it runs automatically.

- [ ] Task: Integrate regression suite into `go test ./...`
- [ ] Task: Update project documentation and style guides with logic testing instructions
- [ ] Task: Conductor - User Manual Verification 'Phase 4: Integration & CI/CD' (Protocol in workflow.md)
