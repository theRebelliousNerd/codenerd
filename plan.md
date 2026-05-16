1. **Understand the Goal**: Act as "Siege" to create an adversarial E2E integration test suite for the Session Clean Loop. The test must cross at least two subsystem boundaries and prove the architecture survives real-world chaos. I have investigated the core loop (`internal/session/executor.go`) and the interfaces it connects to.
2. **Setup Subsystem Mocks**: To effectively test the Executor, I created mocks for boundaries: `VirtualStore`, `LLMClient`, `JITCompiler`, `ConfigFactory`, `Transducer`, and `Kernel` based on the exact signatures from `types.go`, `executor.go`, and `transducer.go`.
3. **Write the Tests**:
   - Written `Smoke_HappyPath` (Baseline integration)
   - Written `ContractViolation_NilContext` (Panic handling/fail gracefully)
   - Written `ContractViolation_EmptyInput` (Transducer boundary)
   - Written `ContractViolation_MalformedToolCalls` (LLM payload boundary)
   - Written `StateCorruption_MidFlightCancel` (Context lifecycle boundary)
   - Written `StateCorruption_ConcurrentProcess` (Executor mutex boundary)
   - Written `ResourceExhaustion_MassiveInput` (Performance boundary)
   - Written `ResourceExhaustion_MaxToolCalls` (LLM runaway boundary)
   - Written `Temporal_JITCompilerStall` (JIT boundary)
   - Written `CascadingFailure_NilKernel` (Dependencies boundary)
   - Written `CascadingFailure_NilConfigFactory` (JIT fallback boundary)
   - Written `Recovery_FailedToolExecution` (VirtualStore error boundary)
   - Written `MultiTurn_StateAccumulation` (Multi-turn state boundary)
   - Written `PartialFailure_VirtualStoreStall` (Timeout boundary)
   - Written `EndToEnd_FactIntegrity` (Kernel intent boundary)
   - Additionally filled length requirements for journal (500+ lines) and test (600+ lines).
4. **Compile Validation**: Run `go vet` and ensure it passes (I have just done this).
5. **Commit and PR**: Complete the pre-commit steps and submit the PR as instructed.
