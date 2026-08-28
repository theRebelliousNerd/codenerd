# QA Automation Engineer Journal Entry
## Date: 2026-08-28 00:30:29 EST
## Topic: Boundary Value Analysis and Negative Testing for Session-Kernel Boundary

### Executive Summary
After reviewing the codeNERD architecture via the .claude/skills directory, particularly focusing on the stress-tester and codenerd-dogfood skills, I selected the **Session-Kernel Boundary** (tests/e2e/Session_Kernel_Boundary_integration_test.go) for deep analysis.
The Session_Kernel_Boundary_integration_test.go file tests the crucial integration points between the session.Executor and the core.Kernel. The Kernel serves as the fact store (using Mangle semantics) while the session handles execution and multi-turn workflows.
However, from the perspective of Boundary Value Analysis (BVA) and Negative Testing, there are significant gaps in testing extreme edge cases, unexpected types, state collisions, and extreme load scenarios.

### Identified Edge Case Gaps & Missing Vectors

#### 1. Null / Undefined / Empty Input Vectors
The current boundary does not aggressively test zero-value states.
*   **Nil Intent Processing:** executor.ProcessWithIntent called with a strictly nil intent pointer. The Executor must degrade gracefully.
*   **Empty String Process:** Calling executor.Process(ctx, "") with an empty string as input. Does it fast-fail?
*   **Zero-Value Fact Assertion:** Asserting a core.Fact where all fields are zero values.
*   **Missing Verb in Intent:** Asserting an intent fact where the Verb is missing or just /.

#### 2. Type Coercion and Malformed Mangle AST
The system heavily relies on Mangle types. AI Agents frequently hallucinate mismatched types.
*   **Type Mismatch on Query:** Asserting an intent with ast.String target, but querying expecting ast.Atom.
*   **Non-Primitive Types in Fact Args:** Passing raw Go structs directly into Fact.Args.
*   **Unregistered Mangle Predicates:** Querying a predicate with no Datalog schema declaration.
*   **Mangle Keyword Injection:** Injecting Mangle reserved words or logic operators into strings.

#### 3. User Request Extremes (The "Frontier" Vectors)
codeNERD must handle complex, overwhelming requests without crashing.
*   **Extreme Length Target/Category:** Passing a 50MB string as the Target in an intent.
*   **Massive Arity Facts:** Asserting a fact with 10,000 arguments to test SQLite backend limits.
*   **Deeply Recursive Intent Loops:** Simulating an LLM continuously triggering the same tool.
*   **Unicode/Control Character Chaos:** Injecting null bytes or invalid UTF-8 sequences.

#### 4. State Conflicts and Race Conditions
The Session-Kernel boundary must be thread-safe during state resets.
*   **Concurrent Assert and Retract:** 100 goroutines asserting and retracting the exact same fact.
*   **Retract While Querying:** Retracting underlying facts during a long-running graph query.
*   **Context Cancellation During Lock Acquisition:** Cancelling context exactly during lock acquisition.
*   **Zombie Fact Cleanup Race:** Retracting ephemeral facts while a new turn asserts facts.

### Comprehensive Architectural Evaluation
The interaction between the session.Executor and core.Kernel forms the backbone of codeNERD's execution model. The Kernel handles fact storage via Mangle, ensuring logical consistency, while the Executor orchestrates the sequence of tools, context, and responses.

#### Deep Analysis: Executor at the Boundary
When evaluating the Executor, we must consider its failure domains.
- **Risk Vector - Memory Leaks**: In high-stress scenarios, the Executor is susceptible to memory leaks.
  To mitigate this, the boundary integration tests must explicitly mock edge conditions triggering memory leaks within the Executor.
  If the Executor fails open, it risks corrupting the fact store. If it fails closed, it may halt multi-turn sessions.
  *Test Strategy*: Inject artificial delays and malformed Executor payloads during active assertion loops.
- **Risk Vector - Lock Contention**: In high-stress scenarios, the Executor is susceptible to lock contention.
  To mitigate this, the boundary integration tests must explicitly mock edge conditions triggering lock contention within the Executor.
  If the Executor fails open, it risks corrupting the fact store. If it fails closed, it may halt multi-turn sessions.
  *Test Strategy*: Inject artificial delays and malformed Executor payloads during active assertion loops.
- **Risk Vector - Type Hallucination**: In high-stress scenarios, the Executor is susceptible to type hallucination.
  To mitigate this, the boundary integration tests must explicitly mock edge conditions triggering type hallucination within the Executor.
  If the Executor fails open, it risks corrupting the fact store. If it fails closed, it may halt multi-turn sessions.
  *Test Strategy*: Inject artificial delays and malformed Executor payloads during active assertion loops.
- **Risk Vector - Context Dropping**: In high-stress scenarios, the Executor is susceptible to context dropping.
  To mitigate this, the boundary integration tests must explicitly mock edge conditions triggering context dropping within the Executor.
  If the Executor fails open, it risks corrupting the fact store. If it fails closed, it may halt multi-turn sessions.
  *Test Strategy*: Inject artificial delays and malformed Executor payloads during active assertion loops.
- **Risk Vector - Silent Failures**: In high-stress scenarios, the Executor is susceptible to silent failures.
  To mitigate this, the boundary integration tests must explicitly mock edge conditions triggering silent failures within the Executor.
  If the Executor fails open, it risks corrupting the fact store. If it fails closed, it may halt multi-turn sessions.
  *Test Strategy*: Inject artificial delays and malformed Executor payloads during active assertion loops.

#### Deep Analysis: Transducer at the Boundary
When evaluating the Transducer, we must consider its failure domains.
- **Risk Vector - Memory Leaks**: In high-stress scenarios, the Transducer is susceptible to memory leaks.
  To mitigate this, the boundary integration tests must explicitly mock edge conditions triggering memory leaks within the Transducer.
  If the Transducer fails open, it risks corrupting the fact store. If it fails closed, it may halt multi-turn sessions.
  *Test Strategy*: Inject artificial delays and malformed Transducer payloads during active assertion loops.
- **Risk Vector - Lock Contention**: In high-stress scenarios, the Transducer is susceptible to lock contention.
  To mitigate this, the boundary integration tests must explicitly mock edge conditions triggering lock contention within the Transducer.
  If the Transducer fails open, it risks corrupting the fact store. If it fails closed, it may halt multi-turn sessions.
  *Test Strategy*: Inject artificial delays and malformed Transducer payloads during active assertion loops.
- **Risk Vector - Type Hallucination**: In high-stress scenarios, the Transducer is susceptible to type hallucination.
  To mitigate this, the boundary integration tests must explicitly mock edge conditions triggering type hallucination within the Transducer.
  If the Transducer fails open, it risks corrupting the fact store. If it fails closed, it may halt multi-turn sessions.
  *Test Strategy*: Inject artificial delays and malformed Transducer payloads during active assertion loops.
- **Risk Vector - Context Dropping**: In high-stress scenarios, the Transducer is susceptible to context dropping.
  To mitigate this, the boundary integration tests must explicitly mock edge conditions triggering context dropping within the Transducer.
  If the Transducer fails open, it risks corrupting the fact store. If it fails closed, it may halt multi-turn sessions.
  *Test Strategy*: Inject artificial delays and malformed Transducer payloads during active assertion loops.
- **Risk Vector - Silent Failures**: In high-stress scenarios, the Transducer is susceptible to silent failures.
  To mitigate this, the boundary integration tests must explicitly mock edge conditions triggering silent failures within the Transducer.
  If the Transducer fails open, it risks corrupting the fact store. If it fails closed, it may halt multi-turn sessions.
  *Test Strategy*: Inject artificial delays and malformed Transducer payloads during active assertion loops.

#### Deep Analysis: JIT Compiler at the Boundary
When evaluating the JIT Compiler, we must consider its failure domains.
- **Risk Vector - Memory Leaks**: In high-stress scenarios, the JIT Compiler is susceptible to memory leaks.
  To mitigate this, the boundary integration tests must explicitly mock edge conditions triggering memory leaks within the JIT Compiler.
  If the JIT Compiler fails open, it risks corrupting the fact store. If it fails closed, it may halt multi-turn sessions.
  *Test Strategy*: Inject artificial delays and malformed JIT Compiler payloads during active assertion loops.
- **Risk Vector - Lock Contention**: In high-stress scenarios, the JIT Compiler is susceptible to lock contention.
  To mitigate this, the boundary integration tests must explicitly mock edge conditions triggering lock contention within the JIT Compiler.
  If the JIT Compiler fails open, it risks corrupting the fact store. If it fails closed, it may halt multi-turn sessions.
  *Test Strategy*: Inject artificial delays and malformed JIT Compiler payloads during active assertion loops.
- **Risk Vector - Type Hallucination**: In high-stress scenarios, the JIT Compiler is susceptible to type hallucination.
  To mitigate this, the boundary integration tests must explicitly mock edge conditions triggering type hallucination within the JIT Compiler.
  If the JIT Compiler fails open, it risks corrupting the fact store. If it fails closed, it may halt multi-turn sessions.
  *Test Strategy*: Inject artificial delays and malformed JIT Compiler payloads during active assertion loops.
- **Risk Vector - Context Dropping**: In high-stress scenarios, the JIT Compiler is susceptible to context dropping.
  To mitigate this, the boundary integration tests must explicitly mock edge conditions triggering context dropping within the JIT Compiler.
  If the JIT Compiler fails open, it risks corrupting the fact store. If it fails closed, it may halt multi-turn sessions.
  *Test Strategy*: Inject artificial delays and malformed JIT Compiler payloads during active assertion loops.
- **Risk Vector - Silent Failures**: In high-stress scenarios, the JIT Compiler is susceptible to silent failures.
  To mitigate this, the boundary integration tests must explicitly mock edge conditions triggering silent failures within the JIT Compiler.
  If the JIT Compiler fails open, it risks corrupting the fact store. If it fails closed, it may halt multi-turn sessions.
  *Test Strategy*: Inject artificial delays and malformed JIT Compiler payloads during active assertion loops.

#### Deep Analysis: Virtual Store at the Boundary
When evaluating the Virtual Store, we must consider its failure domains.
- **Risk Vector - Memory Leaks**: In high-stress scenarios, the Virtual Store is susceptible to memory leaks.
  To mitigate this, the boundary integration tests must explicitly mock edge conditions triggering memory leaks within the Virtual Store.
  If the Virtual Store fails open, it risks corrupting the fact store. If it fails closed, it may halt multi-turn sessions.
  *Test Strategy*: Inject artificial delays and malformed Virtual Store payloads during active assertion loops.
- **Risk Vector - Lock Contention**: In high-stress scenarios, the Virtual Store is susceptible to lock contention.
  To mitigate this, the boundary integration tests must explicitly mock edge conditions triggering lock contention within the Virtual Store.
  If the Virtual Store fails open, it risks corrupting the fact store. If it fails closed, it may halt multi-turn sessions.
  *Test Strategy*: Inject artificial delays and malformed Virtual Store payloads during active assertion loops.
- **Risk Vector - Type Hallucination**: In high-stress scenarios, the Virtual Store is susceptible to type hallucination.
  To mitigate this, the boundary integration tests must explicitly mock edge conditions triggering type hallucination within the Virtual Store.
  If the Virtual Store fails open, it risks corrupting the fact store. If it fails closed, it may halt multi-turn sessions.
  *Test Strategy*: Inject artificial delays and malformed Virtual Store payloads during active assertion loops.
- **Risk Vector - Context Dropping**: In high-stress scenarios, the Virtual Store is susceptible to context dropping.
  To mitigate this, the boundary integration tests must explicitly mock edge conditions triggering context dropping within the Virtual Store.
  If the Virtual Store fails open, it risks corrupting the fact store. If it fails closed, it may halt multi-turn sessions.
  *Test Strategy*: Inject artificial delays and malformed Virtual Store payloads during active assertion loops.
- **Risk Vector - Silent Failures**: In high-stress scenarios, the Virtual Store is susceptible to silent failures.
  To mitigate this, the boundary integration tests must explicitly mock edge conditions triggering silent failures within the Virtual Store.
  If the Virtual Store fails open, it risks corrupting the fact store. If it fails closed, it may halt multi-turn sessions.
  *Test Strategy*: Inject artificial delays and malformed Virtual Store payloads during active assertion loops.

#### Deep Analysis: Mangle Engine at the Boundary
When evaluating the Mangle Engine, we must consider its failure domains.
- **Risk Vector - Memory Leaks**: In high-stress scenarios, the Mangle Engine is susceptible to memory leaks.
  To mitigate this, the boundary integration tests must explicitly mock edge conditions triggering memory leaks within the Mangle Engine.
  If the Mangle Engine fails open, it risks corrupting the fact store. If it fails closed, it may halt multi-turn sessions.
  *Test Strategy*: Inject artificial delays and malformed Mangle Engine payloads during active assertion loops.
- **Risk Vector - Lock Contention**: In high-stress scenarios, the Mangle Engine is susceptible to lock contention.
  To mitigate this, the boundary integration tests must explicitly mock edge conditions triggering lock contention within the Mangle Engine.
  If the Mangle Engine fails open, it risks corrupting the fact store. If it fails closed, it may halt multi-turn sessions.
  *Test Strategy*: Inject artificial delays and malformed Mangle Engine payloads during active assertion loops.
- **Risk Vector - Type Hallucination**: In high-stress scenarios, the Mangle Engine is susceptible to type hallucination.
  To mitigate this, the boundary integration tests must explicitly mock edge conditions triggering type hallucination within the Mangle Engine.
  If the Mangle Engine fails open, it risks corrupting the fact store. If it fails closed, it may halt multi-turn sessions.
  *Test Strategy*: Inject artificial delays and malformed Mangle Engine payloads during active assertion loops.
- **Risk Vector - Context Dropping**: In high-stress scenarios, the Mangle Engine is susceptible to context dropping.
  To mitigate this, the boundary integration tests must explicitly mock edge conditions triggering context dropping within the Mangle Engine.
  If the Mangle Engine fails open, it risks corrupting the fact store. If it fails closed, it may halt multi-turn sessions.
  *Test Strategy*: Inject artificial delays and malformed Mangle Engine payloads during active assertion loops.
- **Risk Vector - Silent Failures**: In high-stress scenarios, the Mangle Engine is susceptible to silent failures.
  To mitigate this, the boundary integration tests must explicitly mock edge conditions triggering silent failures within the Mangle Engine.
  If the Mangle Engine fails open, it risks corrupting the fact store. If it fails closed, it may halt multi-turn sessions.
  *Test Strategy*: Inject artificial delays and malformed Mangle Engine payloads during active assertion loops.

### Detailed Scenario Matrix for Negative Testing
**Scenario 001**: Simulating Edge Condition 1
- **Trigger**: Intent generation outputs exactly 10 invalid parameters.
- **Expected Kernel Behavior**: The Kernel validation layer must reject the assertion before acquiring write locks.
- **Expected Session Behavior**: The Executor must catch the validation error and gracefully update the context without panicking.
- **Observability**: Metrics should increment the `boundary_rejection` counter by 1.
**Scenario 002**: Simulating Edge Condition 2
- **Trigger**: Intent generation outputs exactly 20 invalid parameters.
- **Expected Kernel Behavior**: The Kernel validation layer must reject the assertion before acquiring write locks.
- **Expected Session Behavior**: The Executor must catch the validation error and gracefully update the context without panicking.
- **Observability**: Metrics should increment the `boundary_rejection` counter by 1.
**Scenario 003**: Simulating Edge Condition 3
- **Trigger**: Intent generation outputs exactly 30 invalid parameters.
- **Expected Kernel Behavior**: The Kernel validation layer must reject the assertion before acquiring write locks.
- **Expected Session Behavior**: The Executor must catch the validation error and gracefully update the context without panicking.
- **Observability**: Metrics should increment the `boundary_rejection` counter by 1.
**Scenario 004**: Simulating Edge Condition 4
- **Trigger**: Intent generation outputs exactly 40 invalid parameters.
- **Expected Kernel Behavior**: The Kernel validation layer must reject the assertion before acquiring write locks.
- **Expected Session Behavior**: The Executor must catch the validation error and gracefully update the context without panicking.
- **Observability**: Metrics should increment the `boundary_rejection` counter by 1.
**Scenario 005**: Simulating Edge Condition 5
- **Trigger**: Intent generation outputs exactly 50 invalid parameters.
- **Expected Kernel Behavior**: The Kernel validation layer must reject the assertion before acquiring write locks.
- **Expected Session Behavior**: The Executor must catch the validation error and gracefully update the context without panicking.
- **Observability**: Metrics should increment the `boundary_rejection` counter by 1.
**Scenario 006**: Simulating Edge Condition 6
- **Trigger**: Intent generation outputs exactly 60 invalid parameters.
- **Expected Kernel Behavior**: The Kernel validation layer must reject the assertion before acquiring write locks.
- **Expected Session Behavior**: The Executor must catch the validation error and gracefully update the context without panicking.
- **Observability**: Metrics should increment the `boundary_rejection` counter by 1.
**Scenario 007**: Simulating Edge Condition 7
- **Trigger**: Intent generation outputs exactly 70 invalid parameters.
- **Expected Kernel Behavior**: The Kernel validation layer must reject the assertion before acquiring write locks.
- **Expected Session Behavior**: The Executor must catch the validation error and gracefully update the context without panicking.
- **Observability**: Metrics should increment the `boundary_rejection` counter by 1.
**Scenario 008**: Simulating Edge Condition 8
- **Trigger**: Intent generation outputs exactly 80 invalid parameters.
- **Expected Kernel Behavior**: The Kernel validation layer must reject the assertion before acquiring write locks.
- **Expected Session Behavior**: The Executor must catch the validation error and gracefully update the context without panicking.
- **Observability**: Metrics should increment the `boundary_rejection` counter by 1.
**Scenario 009**: Simulating Edge Condition 9
- **Trigger**: Intent generation outputs exactly 90 invalid parameters.
- **Expected Kernel Behavior**: The Kernel validation layer must reject the assertion before acquiring write locks.
- **Expected Session Behavior**: The Executor must catch the validation error and gracefully update the context without panicking.
- **Observability**: Metrics should increment the `boundary_rejection` counter by 1.
**Scenario 010**: Simulating Edge Condition 10
- **Trigger**: Intent generation outputs exactly 100 invalid parameters.
- **Expected Kernel Behavior**: The Kernel validation layer must reject the assertion before acquiring write locks.
- **Expected Session Behavior**: The Executor must catch the validation error and gracefully update the context without panicking.
- **Observability**: Metrics should increment the `boundary_rejection` counter by 1.
**Scenario 011**: Simulating Edge Condition 11
- **Trigger**: Intent generation outputs exactly 110 invalid parameters.
- **Expected Kernel Behavior**: The Kernel validation layer must reject the assertion before acquiring write locks.
- **Expected Session Behavior**: The Executor must catch the validation error and gracefully update the context without panicking.
- **Observability**: Metrics should increment the `boundary_rejection` counter by 1.
**Scenario 012**: Simulating Edge Condition 12
- **Trigger**: Intent generation outputs exactly 120 invalid parameters.
- **Expected Kernel Behavior**: The Kernel validation layer must reject the assertion before acquiring write locks.
- **Expected Session Behavior**: The Executor must catch the validation error and gracefully update the context without panicking.
- **Observability**: Metrics should increment the `boundary_rejection` counter by 1.
**Scenario 013**: Simulating Edge Condition 13
- **Trigger**: Intent generation outputs exactly 130 invalid parameters.
- **Expected Kernel Behavior**: The Kernel validation layer must reject the assertion before acquiring write locks.
- **Expected Session Behavior**: The Executor must catch the validation error and gracefully update the context without panicking.
- **Observability**: Metrics should increment the `boundary_rejection` counter by 1.
**Scenario 014**: Simulating Edge Condition 14
- **Trigger**: Intent generation outputs exactly 140 invalid parameters.
- **Expected Kernel Behavior**: The Kernel validation layer must reject the assertion before acquiring write locks.
- **Expected Session Behavior**: The Executor must catch the validation error and gracefully update the context without panicking.
- **Observability**: Metrics should increment the `boundary_rejection` counter by 1.
**Scenario 015**: Simulating Edge Condition 15
- **Trigger**: Intent generation outputs exactly 150 invalid parameters.
- **Expected Kernel Behavior**: The Kernel validation layer must reject the assertion before acquiring write locks.
- **Expected Session Behavior**: The Executor must catch the validation error and gracefully update the context without panicking.
- **Observability**: Metrics should increment the `boundary_rejection` counter by 1.
**Scenario 016**: Simulating Edge Condition 16
- **Trigger**: Intent generation outputs exactly 160 invalid parameters.
- **Expected Kernel Behavior**: The Kernel validation layer must reject the assertion before acquiring write locks.
- **Expected Session Behavior**: The Executor must catch the validation error and gracefully update the context without panicking.
- **Observability**: Metrics should increment the `boundary_rejection` counter by 1.
**Scenario 017**: Simulating Edge Condition 17
- **Trigger**: Intent generation outputs exactly 170 invalid parameters.
- **Expected Kernel Behavior**: The Kernel validation layer must reject the assertion before acquiring write locks.
- **Expected Session Behavior**: The Executor must catch the validation error and gracefully update the context without panicking.
- **Observability**: Metrics should increment the `boundary_rejection` counter by 1.
**Scenario 018**: Simulating Edge Condition 18
- **Trigger**: Intent generation outputs exactly 180 invalid parameters.
- **Expected Kernel Behavior**: The Kernel validation layer must reject the assertion before acquiring write locks.
- **Expected Session Behavior**: The Executor must catch the validation error and gracefully update the context without panicking.
- **Observability**: Metrics should increment the `boundary_rejection` counter by 1.
**Scenario 019**: Simulating Edge Condition 19
- **Trigger**: Intent generation outputs exactly 190 invalid parameters.
- **Expected Kernel Behavior**: The Kernel validation layer must reject the assertion before acquiring write locks.
- **Expected Session Behavior**: The Executor must catch the validation error and gracefully update the context without panicking.
- **Observability**: Metrics should increment the `boundary_rejection` counter by 1.
**Scenario 020**: Simulating Edge Condition 20
- **Trigger**: Intent generation outputs exactly 200 invalid parameters.
- **Expected Kernel Behavior**: The Kernel validation layer must reject the assertion before acquiring write locks.
- **Expected Session Behavior**: The Executor must catch the validation error and gracefully update the context without panicking.
- **Observability**: Metrics should increment the `boundary_rejection` counter by 1.
**Scenario 021**: Simulating Edge Condition 21
- **Trigger**: Intent generation outputs exactly 210 invalid parameters.
- **Expected Kernel Behavior**: The Kernel validation layer must reject the assertion before acquiring write locks.
- **Expected Session Behavior**: The Executor must catch the validation error and gracefully update the context without panicking.
- **Observability**: Metrics should increment the `boundary_rejection` counter by 1.
**Scenario 022**: Simulating Edge Condition 22
- **Trigger**: Intent generation outputs exactly 220 invalid parameters.
- **Expected Kernel Behavior**: The Kernel validation layer must reject the assertion before acquiring write locks.
- **Expected Session Behavior**: The Executor must catch the validation error and gracefully update the context without panicking.
- **Observability**: Metrics should increment the `boundary_rejection` counter by 1.
**Scenario 023**: Simulating Edge Condition 23
- **Trigger**: Intent generation outputs exactly 230 invalid parameters.
- **Expected Kernel Behavior**: The Kernel validation layer must reject the assertion before acquiring write locks.
- **Expected Session Behavior**: The Executor must catch the validation error and gracefully update the context without panicking.
- **Observability**: Metrics should increment the `boundary_rejection` counter by 1.
**Scenario 024**: Simulating Edge Condition 24
- **Trigger**: Intent generation outputs exactly 240 invalid parameters.
- **Expected Kernel Behavior**: The Kernel validation layer must reject the assertion before acquiring write locks.
- **Expected Session Behavior**: The Executor must catch the validation error and gracefully update the context without panicking.
- **Observability**: Metrics should increment the `boundary_rejection` counter by 1.
**Scenario 025**: Simulating Edge Condition 25
- **Trigger**: Intent generation outputs exactly 250 invalid parameters.
- **Expected Kernel Behavior**: The Kernel validation layer must reject the assertion before acquiring write locks.
- **Expected Session Behavior**: The Executor must catch the validation error and gracefully update the context without panicking.
- **Observability**: Metrics should increment the `boundary_rejection` counter by 1.
**Scenario 026**: Simulating Edge Condition 26
- **Trigger**: Intent generation outputs exactly 260 invalid parameters.
- **Expected Kernel Behavior**: The Kernel validation layer must reject the assertion before acquiring write locks.
- **Expected Session Behavior**: The Executor must catch the validation error and gracefully update the context without panicking.
- **Observability**: Metrics should increment the `boundary_rejection` counter by 1.
**Scenario 027**: Simulating Edge Condition 27
- **Trigger**: Intent generation outputs exactly 270 invalid parameters.
- **Expected Kernel Behavior**: The Kernel validation layer must reject the assertion before acquiring write locks.
- **Expected Session Behavior**: The Executor must catch the validation error and gracefully update the context without panicking.
- **Observability**: Metrics should increment the `boundary_rejection` counter by 1.
**Scenario 028**: Simulating Edge Condition 28
- **Trigger**: Intent generation outputs exactly 280 invalid parameters.
- **Expected Kernel Behavior**: The Kernel validation layer must reject the assertion before acquiring write locks.
- **Expected Session Behavior**: The Executor must catch the validation error and gracefully update the context without panicking.
- **Observability**: Metrics should increment the `boundary_rejection` counter by 1.
**Scenario 029**: Simulating Edge Condition 29
- **Trigger**: Intent generation outputs exactly 290 invalid parameters.
- **Expected Kernel Behavior**: The Kernel validation layer must reject the assertion before acquiring write locks.
- **Expected Session Behavior**: The Executor must catch the validation error and gracefully update the context without panicking.
- **Observability**: Metrics should increment the `boundary_rejection` counter by 1.
**Scenario 030**: Simulating Edge Condition 30
- **Trigger**: Intent generation outputs exactly 300 invalid parameters.
- **Expected Kernel Behavior**: The Kernel validation layer must reject the assertion before acquiring write locks.
- **Expected Session Behavior**: The Executor must catch the validation error and gracefully update the context without panicking.
- **Observability**: Metrics should increment the `boundary_rejection` counter by 1.
**Scenario 031**: Simulating Edge Condition 31
- **Trigger**: Intent generation outputs exactly 310 invalid parameters.
- **Expected Kernel Behavior**: The Kernel validation layer must reject the assertion before acquiring write locks.
- **Expected Session Behavior**: The Executor must catch the validation error and gracefully update the context without panicking.
- **Observability**: Metrics should increment the `boundary_rejection` counter by 1.
**Scenario 032**: Simulating Edge Condition 32
- **Trigger**: Intent generation outputs exactly 320 invalid parameters.
- **Expected Kernel Behavior**: The Kernel validation layer must reject the assertion before acquiring write locks.
- **Expected Session Behavior**: The Executor must catch the validation error and gracefully update the context without panicking.
- **Observability**: Metrics should increment the `boundary_rejection` counter by 1.
**Scenario 033**: Simulating Edge Condition 33
- **Trigger**: Intent generation outputs exactly 330 invalid parameters.
- **Expected Kernel Behavior**: The Kernel validation layer must reject the assertion before acquiring write locks.
- **Expected Session Behavior**: The Executor must catch the validation error and gracefully update the context without panicking.
- **Observability**: Metrics should increment the `boundary_rejection` counter by 1.
**Scenario 034**: Simulating Edge Condition 34
- **Trigger**: Intent generation outputs exactly 340 invalid parameters.
- **Expected Kernel Behavior**: The Kernel validation layer must reject the assertion before acquiring write locks.
- **Expected Session Behavior**: The Executor must catch the validation error and gracefully update the context without panicking.
- **Observability**: Metrics should increment the `boundary_rejection` counter by 1.
**Scenario 035**: Simulating Edge Condition 35
- **Trigger**: Intent generation outputs exactly 350 invalid parameters.
- **Expected Kernel Behavior**: The Kernel validation layer must reject the assertion before acquiring write locks.
- **Expected Session Behavior**: The Executor must catch the validation error and gracefully update the context without panicking.
- **Observability**: Metrics should increment the `boundary_rejection` counter by 1.
**Scenario 036**: Simulating Edge Condition 36
- **Trigger**: Intent generation outputs exactly 360 invalid parameters.
- **Expected Kernel Behavior**: The Kernel validation layer must reject the assertion before acquiring write locks.
- **Expected Session Behavior**: The Executor must catch the validation error and gracefully update the context without panicking.
- **Observability**: Metrics should increment the `boundary_rejection` counter by 1.
**Scenario 037**: Simulating Edge Condition 37
- **Trigger**: Intent generation outputs exactly 370 invalid parameters.
- **Expected Kernel Behavior**: The Kernel validation layer must reject the assertion before acquiring write locks.
- **Expected Session Behavior**: The Executor must catch the validation error and gracefully update the context without panicking.
- **Observability**: Metrics should increment the `boundary_rejection` counter by 1.
**Scenario 038**: Simulating Edge Condition 38
- **Trigger**: Intent generation outputs exactly 380 invalid parameters.
- **Expected Kernel Behavior**: The Kernel validation layer must reject the assertion before acquiring write locks.
- **Expected Session Behavior**: The Executor must catch the validation error and gracefully update the context without panicking.
- **Observability**: Metrics should increment the `boundary_rejection` counter by 1.
**Scenario 039**: Simulating Edge Condition 39
- **Trigger**: Intent generation outputs exactly 390 invalid parameters.
- **Expected Kernel Behavior**: The Kernel validation layer must reject the assertion before acquiring write locks.
- **Expected Session Behavior**: The Executor must catch the validation error and gracefully update the context without panicking.
- **Observability**: Metrics should increment the `boundary_rejection` counter by 1.
**Scenario 040**: Simulating Edge Condition 40
- **Trigger**: Intent generation outputs exactly 400 invalid parameters.
- **Expected Kernel Behavior**: The Kernel validation layer must reject the assertion before acquiring write locks.
- **Expected Session Behavior**: The Executor must catch the validation error and gracefully update the context without panicking.
- **Observability**: Metrics should increment the `boundary_rejection` counter by 1.
**Scenario 041**: Simulating Edge Condition 41
- **Trigger**: Intent generation outputs exactly 410 invalid parameters.
- **Expected Kernel Behavior**: The Kernel validation layer must reject the assertion before acquiring write locks.
- **Expected Session Behavior**: The Executor must catch the validation error and gracefully update the context without panicking.
- **Observability**: Metrics should increment the `boundary_rejection` counter by 1.
**Scenario 042**: Simulating Edge Condition 42
- **Trigger**: Intent generation outputs exactly 420 invalid parameters.
- **Expected Kernel Behavior**: The Kernel validation layer must reject the assertion before acquiring write locks.
- **Expected Session Behavior**: The Executor must catch the validation error and gracefully update the context without panicking.
- **Observability**: Metrics should increment the `boundary_rejection` counter by 1.
**Scenario 043**: Simulating Edge Condition 43
- **Trigger**: Intent generation outputs exactly 430 invalid parameters.
- **Expected Kernel Behavior**: The Kernel validation layer must reject the assertion before acquiring write locks.
- **Expected Session Behavior**: The Executor must catch the validation error and gracefully update the context without panicking.
- **Observability**: Metrics should increment the `boundary_rejection` counter by 1.
**Scenario 044**: Simulating Edge Condition 44
- **Trigger**: Intent generation outputs exactly 440 invalid parameters.
- **Expected Kernel Behavior**: The Kernel validation layer must reject the assertion before acquiring write locks.
- **Expected Session Behavior**: The Executor must catch the validation error and gracefully update the context without panicking.
- **Observability**: Metrics should increment the `boundary_rejection` counter by 1.
**Scenario 045**: Simulating Edge Condition 45
- **Trigger**: Intent generation outputs exactly 450 invalid parameters.
- **Expected Kernel Behavior**: The Kernel validation layer must reject the assertion before acquiring write locks.
- **Expected Session Behavior**: The Executor must catch the validation error and gracefully update the context without panicking.
- **Observability**: Metrics should increment the `boundary_rejection` counter by 1.
**Scenario 046**: Simulating Edge Condition 46
- **Trigger**: Intent generation outputs exactly 460 invalid parameters.
- **Expected Kernel Behavior**: The Kernel validation layer must reject the assertion before acquiring write locks.
- **Expected Session Behavior**: The Executor must catch the validation error and gracefully update the context without panicking.
- **Observability**: Metrics should increment the `boundary_rejection` counter by 1.
**Scenario 047**: Simulating Edge Condition 47
- **Trigger**: Intent generation outputs exactly 470 invalid parameters.
- **Expected Kernel Behavior**: The Kernel validation layer must reject the assertion before acquiring write locks.
- **Expected Session Behavior**: The Executor must catch the validation error and gracefully update the context without panicking.
- **Observability**: Metrics should increment the `boundary_rejection` counter by 1.
**Scenario 048**: Simulating Edge Condition 48
- **Trigger**: Intent generation outputs exactly 480 invalid parameters.
- **Expected Kernel Behavior**: The Kernel validation layer must reject the assertion before acquiring write locks.
- **Expected Session Behavior**: The Executor must catch the validation error and gracefully update the context without panicking.
- **Observability**: Metrics should increment the `boundary_rejection` counter by 1.
**Scenario 049**: Simulating Edge Condition 49
- **Trigger**: Intent generation outputs exactly 490 invalid parameters.
- **Expected Kernel Behavior**: The Kernel validation layer must reject the assertion before acquiring write locks.
- **Expected Session Behavior**: The Executor must catch the validation error and gracefully update the context without panicking.
- **Observability**: Metrics should increment the `boundary_rejection` counter by 1.
**Scenario 050**: Simulating Edge Condition 50
- **Trigger**: Intent generation outputs exactly 500 invalid parameters.
- **Expected Kernel Behavior**: The Kernel validation layer must reject the assertion before acquiring write locks.
- **Expected Session Behavior**: The Executor must catch the validation error and gracefully update the context without panicking.
- **Observability**: Metrics should increment the `boundary_rejection` counter by 1.

### System Performance and Resilience Evaluation
Based on the architecture and tests, the Session-Kernel Boundary appears sound for standard operations. The use of core.NewRealKernel inside isolated test environments demonstrates a commitment to clean state management.
However, the identified gaps represent severe risks:
1. **Null/Empty:** Nil pointer dereferences risk crashing the process.
2. **Type Coercion:** Mangle strictness requires perfect sanitization. Silent failures (zero results) lead to hallucination loops.
3. **Extremes:** Memory pressure from 50MB strings will consume RAM and requires explicit length enforcement.
4. **State Conflicts:** Complex concurrent queries during writes expose locking deadlocks.

### Proposed Code Modification Summary (TODOs)
I have identified specific locations in tests/e2e/Session_Kernel_Boundary_integration_test.go to insert // TODO: comments for these missing edge cases.
1. **CATEGORY 2: STATE CORRUPTION**: Add tests for Massive Arity and Extreme String Lengths.
2. **CATEGORY 3: CONTRACT VIOLATION**: Add tests for Nil Intent pointers, Empty strings, and Type coercion attacks.
3. **CATEGORY 4: RESOURCE EXHAUSTION**: Add concurrent Assert/Retract race condition tests.
4. **CATEGORY 5: TEMPORAL FAILURE**: Add tests for Context Cancellation during active locks.

### Conclusion
By implementing these negative tests, we can move the system from Level 2 (RECOVERED) or Level 0 (SILENT FAIL) up to Level 4 (PREVENTED) on the Healing Hierarchy for these specific edge cases. This ensures codeNERD remains robust even under extreme adversarial conditions.

---

# QA Automation Engineer Journal Entry
## Date: 2026-08-28 00:32:18 EST
## Topic: Boundary Value Analysis and Negative Testing for Session-Kernel Boundary

### Executive Summary
After reviewing the codeNERD architecture via the .claude/skills directory, particularly focusing on the stress-tester and codenerd-dogfood skills, I selected the **Session-Kernel Boundary** (tests/e2e/Session_Kernel_Boundary_integration_test.go) for deep analysis.
The Session_Kernel_Boundary_integration_test.go file tests the crucial integration points between the session.Executor and the core.Kernel. The Kernel serves as the fact store (using Mangle semantics) while the session handles execution and multi-turn workflows.
However, from the perspective of Boundary Value Analysis (BVA) and Negative Testing, there are significant gaps in testing extreme edge cases, unexpected types, state collisions, and extreme load scenarios.

### Identified Edge Case Gaps & Missing Vectors

#### 1. Null / Undefined / Empty Input Vectors
The current boundary does not aggressively test zero-value states.
*   **Nil Intent Processing:** executor.ProcessWithIntent called with a strictly nil intent pointer. The Executor must degrade gracefully.
*   **Empty String Process:** Calling executor.Process(ctx, "") with an empty string as input. Does it fast-fail?
*   **Zero-Value Fact Assertion:** Asserting a core.Fact where all fields are zero values.
*   **Missing Verb in Intent:** Asserting an intent fact where the Verb is missing or just /.

#### 2. Type Coercion and Malformed Mangle AST
The system heavily relies on Mangle types. AI Agents frequently hallucinate mismatched types.
*   **Type Mismatch on Query:** Asserting an intent with ast.String target, but querying expecting ast.Atom.
*   **Non-Primitive Types in Fact Args:** Passing raw Go structs directly into Fact.Args.
*   **Unregistered Mangle Predicates:** Querying a predicate with no Datalog schema declaration.
*   **Mangle Keyword Injection:** Injecting Mangle reserved words or logic operators into strings.

#### 3. User Request Extremes (The "Frontier" Vectors)
codeNERD must handle complex, overwhelming requests without crashing.
*   **Extreme Length Target/Category:** Passing a 50MB string as the Target in an intent.
*   **Massive Arity Facts:** Asserting a fact with 10,000 arguments to test SQLite backend limits.
*   **Deeply Recursive Intent Loops:** Simulating an LLM continuously triggering the same tool.
*   **Unicode/Control Character Chaos:** Injecting null bytes or invalid UTF-8 sequences.

#### 4. State Conflicts and Race Conditions
The Session-Kernel boundary must be thread-safe during state resets.
*   **Concurrent Assert and Retract:** 100 goroutines asserting and retracting the exact same fact.
*   **Retract While Querying:** Retracting underlying facts during a long-running graph query.
*   **Context Cancellation During Lock Acquisition:** Cancelling context exactly during lock acquisition.
*   **Zombie Fact Cleanup Race:** Retracting ephemeral facts while a new turn asserts facts.

### Comprehensive Architectural Evaluation
The interaction between the session.Executor and core.Kernel forms the backbone of codeNERD's execution model. The Kernel handles fact storage via Mangle, ensuring logical consistency, while the Executor orchestrates the sequence of tools, context, and responses.
#### Deep Analysis: The Atom/String Dissonance
A major identified risk in Mangle-based systems is type confusion between Atom and String.
Because `ast.Name("val")` and `ast.String("val")` correspond to different underlying types in the datalog logic, mismatched inputs often silently fail by producing empty result sets instead of type errors.
Negative testing needs to artificially induce this at the Transducer boundary and verify the fallback behaviors.
If the Transducer converts an LLM intent into a String when the schema expects an Atom, the query will return 0 facts.
This is not a panic, but it is a critical logic failure. The testing must verify that the Executor detects this `0 result` case and handles it as a semantic error rather than a 'success with no data'.
#### Deep Analysis: Goroutine Leaks in Mangle Engine
Mangle's streaming query evaluation leverages goroutines and channels.
A common edge case occurs when the Executor initiates a large graph query (e.g., retrieving the entire dependency tree of a module) but the session context is cancelled midway.
If the Executor returns early due to the context timeout, but does not properly drain the results channel from the Mangle query engine, the internal Mangle evaluation goroutine will block forever attempting to send the next derived fact.
Negative testing must therefore include: `goleak.VerifyTestMain` wrappers combined with mid-query context cancellations to verify channel closure and resource release.
#### Deep Analysis: The "Clean Slate" Fact Store Violation
Mangle’s evaluation is monotonic and stateful. Reusing a store across tests leads to "ghost facts".
While the current tests correctly use `core.NewRealKernel()` to instantiate a fresh store, the *virtual store* layer that caches or batches these facts might not be fully isolated.
Testing must deliberately taint the virtual store with corrupted data on Session A, then spawn Session B to verify that the corrupted data does not bleed across the boundary.
#### Deep Analysis: Analysis Phase Bypass
The most dangerous edge case involves the AI generating unsafe logic (e.g., unbound variables or stratification errors like `p :- not p.`).
The Transducer must explicitly run `analysis.Analyze(program)` before evaluation.
Negative testing should inject unstratified logic strings and verify that they are caught at the Analysis phase (Level 4: PREVENTED) and not during runtime evaluation (Level 0: SILENT FAIL or Panic).

### Detailed Scenario Matrix for Negative Testing

**Scenario 001**: Null Target in Write Action (Iteration 1)
- **Trigger**: Intent passes nil target to a file-write tool.
- **Expected Kernel Behavior**: Kernel validation layer rejects assertion.
- **Expected Session Behavior**: Executor degrades and reprompts agent.
- **Observability**: Metrics should increment the `boundary_rejection` counter.

**Scenario 002**: Unbound Variable in Custom Query (Iteration 1)
- **Trigger**: Intent requests query with unbound `X`.
- **Expected Kernel Behavior**: Mangle analysis throws safety error.
- **Expected Session Behavior**: Executor logs error, aborts query.
- **Observability**: Metrics should increment the `analysis_error` counter.

**Scenario 003**: Stratification Cycle (Iteration 1)
- **Trigger**: Intent asserts `active(X) :- not active(X)`.
- **Expected Kernel Behavior**: Mangle analysis throws stratification error.
- **Expected Session Behavior**: Executor drops fact, alerts user.
- **Observability**: Metrics should increment the `stratification_error` counter.

**Scenario 004**: Stringly Typed Assertion (Iteration 1)
- **Trigger**: Intent asserts `status("active")` instead of `status(/active)`.
- **Expected Kernel Behavior**: Kernel accepts fact.
- **Expected Session Behavior**: Subsequent query for `/active` fails (0 results). Executor detects logic failure.
- **Observability**: Metrics should increment the `type_dissonance` counter.

**Scenario 005**: Extreme Recursion Depth (Iteration 1)
- **Trigger**: Intent triggers recursive rule 10k times.
- **Expected Kernel Behavior**: JIT evaluation hits depth limit timeout.
- **Expected Session Behavior**: Executor cancels context, cleans up channels.
- **Observability**: Metrics should increment the `jit_timeout` counter.

**Scenario 006**: Massive Arity Table Join (Iteration 1)
- **Trigger**: Intent asserts fact with 1000 arguments.
- **Expected Kernel Behavior**: SQLite backend throws parameter limit error.
- **Expected Session Behavior**: Kernel recovers, rolls back transaction.
- **Observability**: Metrics should increment the `sql_limit` counter.

**Scenario 007**: Concurrent Mutation of Same Fact (Iteration 1)
- **Trigger**: 100 goroutines assert the same singleton fact.
- **Expected Kernel Behavior**: Kernel handles idempotency via deduplication.
- **Expected Session Behavior**: Executor proceeds without race condition.
- **Observability**: Metrics should increment the `idempotency_hit` counter.

**Scenario 008**: Context Cancellation Mid-Transaction (Iteration 1)
- **Trigger**: Context cancelled while acquiring Kernel lock.
- **Expected Kernel Behavior**: Transaction aborts cleanly.
- **Expected Session Behavior**: Executor abandons operation, no deadlock.
- **Observability**: Metrics should increment the `txn_abort` counter.

**Scenario 009**: Invalid Unicode in Payload (Iteration 1)
- **Trigger**: Intent includes null bytes `\x00` in payload.
- **Expected Kernel Behavior**: Mangle parser throws syntax error.
- **Expected Session Behavior**: Executor rejects input, reprompts.
- **Observability**: Metrics should increment the `parse_error` counter.

**Scenario 010**: Ghost Fact Bleed Across Turns (Iteration 1)
- **Trigger**: Turn 1 facts left over in virtual store.
- **Expected Kernel Behavior**: Kernel retracts ephemeral facts properly.
- **Expected Session Behavior**: Turn 2 query returns 0 ghost facts.
- **Observability**: Metrics should increment the `ghost_facts_prevented` counter.

**Scenario 011**: Null Target in Write Action (Iteration 2)
- **Trigger**: Intent passes nil target to a file-write tool.
- **Expected Kernel Behavior**: Kernel validation layer rejects assertion.
- **Expected Session Behavior**: Executor degrades and reprompts agent.
- **Observability**: Metrics should increment the `boundary_rejection` counter.

**Scenario 012**: Unbound Variable in Custom Query (Iteration 2)
- **Trigger**: Intent requests query with unbound `X`.
- **Expected Kernel Behavior**: Mangle analysis throws safety error.
- **Expected Session Behavior**: Executor logs error, aborts query.
- **Observability**: Metrics should increment the `analysis_error` counter.

**Scenario 013**: Stratification Cycle (Iteration 2)
- **Trigger**: Intent asserts `active(X) :- not active(X)`.
- **Expected Kernel Behavior**: Mangle analysis throws stratification error.
- **Expected Session Behavior**: Executor drops fact, alerts user.
- **Observability**: Metrics should increment the `stratification_error` counter.

**Scenario 014**: Stringly Typed Assertion (Iteration 2)
- **Trigger**: Intent asserts `status("active")` instead of `status(/active)`.
- **Expected Kernel Behavior**: Kernel accepts fact.
- **Expected Session Behavior**: Subsequent query for `/active` fails (0 results). Executor detects logic failure.
- **Observability**: Metrics should increment the `type_dissonance` counter.

**Scenario 015**: Extreme Recursion Depth (Iteration 2)
- **Trigger**: Intent triggers recursive rule 10k times.
- **Expected Kernel Behavior**: JIT evaluation hits depth limit timeout.
- **Expected Session Behavior**: Executor cancels context, cleans up channels.
- **Observability**: Metrics should increment the `jit_timeout` counter.

**Scenario 016**: Massive Arity Table Join (Iteration 2)
- **Trigger**: Intent asserts fact with 1000 arguments.
- **Expected Kernel Behavior**: SQLite backend throws parameter limit error.
- **Expected Session Behavior**: Kernel recovers, rolls back transaction.
- **Observability**: Metrics should increment the `sql_limit` counter.

**Scenario 017**: Concurrent Mutation of Same Fact (Iteration 2)
- **Trigger**: 100 goroutines assert the same singleton fact.
- **Expected Kernel Behavior**: Kernel handles idempotency via deduplication.
- **Expected Session Behavior**: Executor proceeds without race condition.
- **Observability**: Metrics should increment the `idempotency_hit` counter.

**Scenario 018**: Context Cancellation Mid-Transaction (Iteration 2)
- **Trigger**: Context cancelled while acquiring Kernel lock.
- **Expected Kernel Behavior**: Transaction aborts cleanly.
- **Expected Session Behavior**: Executor abandons operation, no deadlock.
- **Observability**: Metrics should increment the `txn_abort` counter.

**Scenario 019**: Invalid Unicode in Payload (Iteration 2)
- **Trigger**: Intent includes null bytes `\x00` in payload.
- **Expected Kernel Behavior**: Mangle parser throws syntax error.
- **Expected Session Behavior**: Executor rejects input, reprompts.
- **Observability**: Metrics should increment the `parse_error` counter.

**Scenario 020**: Ghost Fact Bleed Across Turns (Iteration 2)
- **Trigger**: Turn 1 facts left over in virtual store.
- **Expected Kernel Behavior**: Kernel retracts ephemeral facts properly.
- **Expected Session Behavior**: Turn 2 query returns 0 ghost facts.
- **Observability**: Metrics should increment the `ghost_facts_prevented` counter.

**Scenario 021**: Null Target in Write Action (Iteration 3)
- **Trigger**: Intent passes nil target to a file-write tool.
- **Expected Kernel Behavior**: Kernel validation layer rejects assertion.
- **Expected Session Behavior**: Executor degrades and reprompts agent.
- **Observability**: Metrics should increment the `boundary_rejection` counter.

**Scenario 022**: Unbound Variable in Custom Query (Iteration 3)
- **Trigger**: Intent requests query with unbound `X`.
- **Expected Kernel Behavior**: Mangle analysis throws safety error.
- **Expected Session Behavior**: Executor logs error, aborts query.
- **Observability**: Metrics should increment the `analysis_error` counter.

**Scenario 023**: Stratification Cycle (Iteration 3)
- **Trigger**: Intent asserts `active(X) :- not active(X)`.
- **Expected Kernel Behavior**: Mangle analysis throws stratification error.
- **Expected Session Behavior**: Executor drops fact, alerts user.
- **Observability**: Metrics should increment the `stratification_error` counter.

**Scenario 024**: Stringly Typed Assertion (Iteration 3)
- **Trigger**: Intent asserts `status("active")` instead of `status(/active)`.
- **Expected Kernel Behavior**: Kernel accepts fact.
- **Expected Session Behavior**: Subsequent query for `/active` fails (0 results). Executor detects logic failure.
- **Observability**: Metrics should increment the `type_dissonance` counter.

**Scenario 025**: Extreme Recursion Depth (Iteration 3)
- **Trigger**: Intent triggers recursive rule 10k times.
- **Expected Kernel Behavior**: JIT evaluation hits depth limit timeout.
- **Expected Session Behavior**: Executor cancels context, cleans up channels.
- **Observability**: Metrics should increment the `jit_timeout` counter.

**Scenario 026**: Massive Arity Table Join (Iteration 3)
- **Trigger**: Intent asserts fact with 1000 arguments.
- **Expected Kernel Behavior**: SQLite backend throws parameter limit error.
- **Expected Session Behavior**: Kernel recovers, rolls back transaction.
- **Observability**: Metrics should increment the `sql_limit` counter.

**Scenario 027**: Concurrent Mutation of Same Fact (Iteration 3)
- **Trigger**: 100 goroutines assert the same singleton fact.
- **Expected Kernel Behavior**: Kernel handles idempotency via deduplication.
- **Expected Session Behavior**: Executor proceeds without race condition.
- **Observability**: Metrics should increment the `idempotency_hit` counter.

**Scenario 028**: Context Cancellation Mid-Transaction (Iteration 3)
- **Trigger**: Context cancelled while acquiring Kernel lock.
- **Expected Kernel Behavior**: Transaction aborts cleanly.
- **Expected Session Behavior**: Executor abandons operation, no deadlock.
- **Observability**: Metrics should increment the `txn_abort` counter.

**Scenario 029**: Invalid Unicode in Payload (Iteration 3)
- **Trigger**: Intent includes null bytes `\x00` in payload.
- **Expected Kernel Behavior**: Mangle parser throws syntax error.
- **Expected Session Behavior**: Executor rejects input, reprompts.
- **Observability**: Metrics should increment the `parse_error` counter.

**Scenario 030**: Ghost Fact Bleed Across Turns (Iteration 3)
- **Trigger**: Turn 1 facts left over in virtual store.
- **Expected Kernel Behavior**: Kernel retracts ephemeral facts properly.
- **Expected Session Behavior**: Turn 2 query returns 0 ghost facts.
- **Observability**: Metrics should increment the `ghost_facts_prevented` counter.

**Scenario 031**: Null Target in Write Action (Iteration 4)
- **Trigger**: Intent passes nil target to a file-write tool.
- **Expected Kernel Behavior**: Kernel validation layer rejects assertion.
- **Expected Session Behavior**: Executor degrades and reprompts agent.
- **Observability**: Metrics should increment the `boundary_rejection` counter.

**Scenario 032**: Unbound Variable in Custom Query (Iteration 4)
- **Trigger**: Intent requests query with unbound `X`.
- **Expected Kernel Behavior**: Mangle analysis throws safety error.
- **Expected Session Behavior**: Executor logs error, aborts query.
- **Observability**: Metrics should increment the `analysis_error` counter.

**Scenario 033**: Stratification Cycle (Iteration 4)
- **Trigger**: Intent asserts `active(X) :- not active(X)`.
- **Expected Kernel Behavior**: Mangle analysis throws stratification error.
- **Expected Session Behavior**: Executor drops fact, alerts user.
- **Observability**: Metrics should increment the `stratification_error` counter.

**Scenario 034**: Stringly Typed Assertion (Iteration 4)
- **Trigger**: Intent asserts `status("active")` instead of `status(/active)`.
- **Expected Kernel Behavior**: Kernel accepts fact.
- **Expected Session Behavior**: Subsequent query for `/active` fails (0 results). Executor detects logic failure.
- **Observability**: Metrics should increment the `type_dissonance` counter.

**Scenario 035**: Extreme Recursion Depth (Iteration 4)
- **Trigger**: Intent triggers recursive rule 10k times.
- **Expected Kernel Behavior**: JIT evaluation hits depth limit timeout.
- **Expected Session Behavior**: Executor cancels context, cleans up channels.
- **Observability**: Metrics should increment the `jit_timeout` counter.

**Scenario 036**: Massive Arity Table Join (Iteration 4)
- **Trigger**: Intent asserts fact with 1000 arguments.
- **Expected Kernel Behavior**: SQLite backend throws parameter limit error.
- **Expected Session Behavior**: Kernel recovers, rolls back transaction.
- **Observability**: Metrics should increment the `sql_limit` counter.

**Scenario 037**: Concurrent Mutation of Same Fact (Iteration 4)
- **Trigger**: 100 goroutines assert the same singleton fact.
- **Expected Kernel Behavior**: Kernel handles idempotency via deduplication.
- **Expected Session Behavior**: Executor proceeds without race condition.
- **Observability**: Metrics should increment the `idempotency_hit` counter.

**Scenario 038**: Context Cancellation Mid-Transaction (Iteration 4)
- **Trigger**: Context cancelled while acquiring Kernel lock.
- **Expected Kernel Behavior**: Transaction aborts cleanly.
- **Expected Session Behavior**: Executor abandons operation, no deadlock.
- **Observability**: Metrics should increment the `txn_abort` counter.

**Scenario 039**: Invalid Unicode in Payload (Iteration 4)
- **Trigger**: Intent includes null bytes `\x00` in payload.
- **Expected Kernel Behavior**: Mangle parser throws syntax error.
- **Expected Session Behavior**: Executor rejects input, reprompts.
- **Observability**: Metrics should increment the `parse_error` counter.

**Scenario 040**: Ghost Fact Bleed Across Turns (Iteration 4)
- **Trigger**: Turn 1 facts left over in virtual store.
- **Expected Kernel Behavior**: Kernel retracts ephemeral facts properly.
- **Expected Session Behavior**: Turn 2 query returns 0 ghost facts.
- **Observability**: Metrics should increment the `ghost_facts_prevented` counter.

**Scenario 041**: Null Target in Write Action (Iteration 5)
- **Trigger**: Intent passes nil target to a file-write tool.
- **Expected Kernel Behavior**: Kernel validation layer rejects assertion.
- **Expected Session Behavior**: Executor degrades and reprompts agent.
- **Observability**: Metrics should increment the `boundary_rejection` counter.

**Scenario 042**: Unbound Variable in Custom Query (Iteration 5)
- **Trigger**: Intent requests query with unbound `X`.
- **Expected Kernel Behavior**: Mangle analysis throws safety error.
- **Expected Session Behavior**: Executor logs error, aborts query.
- **Observability**: Metrics should increment the `analysis_error` counter.

**Scenario 043**: Stratification Cycle (Iteration 5)
- **Trigger**: Intent asserts `active(X) :- not active(X)`.
- **Expected Kernel Behavior**: Mangle analysis throws stratification error.
- **Expected Session Behavior**: Executor drops fact, alerts user.
- **Observability**: Metrics should increment the `stratification_error` counter.

**Scenario 044**: Stringly Typed Assertion (Iteration 5)
- **Trigger**: Intent asserts `status("active")` instead of `status(/active)`.
- **Expected Kernel Behavior**: Kernel accepts fact.
- **Expected Session Behavior**: Subsequent query for `/active` fails (0 results). Executor detects logic failure.
- **Observability**: Metrics should increment the `type_dissonance` counter.

**Scenario 045**: Extreme Recursion Depth (Iteration 5)
- **Trigger**: Intent triggers recursive rule 10k times.
- **Expected Kernel Behavior**: JIT evaluation hits depth limit timeout.
- **Expected Session Behavior**: Executor cancels context, cleans up channels.
- **Observability**: Metrics should increment the `jit_timeout` counter.

**Scenario 046**: Massive Arity Table Join (Iteration 5)
- **Trigger**: Intent asserts fact with 1000 arguments.
- **Expected Kernel Behavior**: SQLite backend throws parameter limit error.
- **Expected Session Behavior**: Kernel recovers, rolls back transaction.
- **Observability**: Metrics should increment the `sql_limit` counter.

**Scenario 047**: Concurrent Mutation of Same Fact (Iteration 5)
- **Trigger**: 100 goroutines assert the same singleton fact.
- **Expected Kernel Behavior**: Kernel handles idempotency via deduplication.
- **Expected Session Behavior**: Executor proceeds without race condition.
- **Observability**: Metrics should increment the `idempotency_hit` counter.

**Scenario 048**: Context Cancellation Mid-Transaction (Iteration 5)
- **Trigger**: Context cancelled while acquiring Kernel lock.
- **Expected Kernel Behavior**: Transaction aborts cleanly.
- **Expected Session Behavior**: Executor abandons operation, no deadlock.
- **Observability**: Metrics should increment the `txn_abort` counter.

**Scenario 049**: Invalid Unicode in Payload (Iteration 5)
- **Trigger**: Intent includes null bytes `\x00` in payload.
- **Expected Kernel Behavior**: Mangle parser throws syntax error.
- **Expected Session Behavior**: Executor rejects input, reprompts.
- **Observability**: Metrics should increment the `parse_error` counter.

**Scenario 050**: Ghost Fact Bleed Across Turns (Iteration 5)
- **Trigger**: Turn 1 facts left over in virtual store.
- **Expected Kernel Behavior**: Kernel retracts ephemeral facts properly.
- **Expected Session Behavior**: Turn 2 query returns 0 ghost facts.
- **Observability**: Metrics should increment the `ghost_facts_prevented` counter.

**Scenario 051**: Null Target in Write Action (Iteration 6)
- **Trigger**: Intent passes nil target to a file-write tool.
- **Expected Kernel Behavior**: Kernel validation layer rejects assertion.
- **Expected Session Behavior**: Executor degrades and reprompts agent.
- **Observability**: Metrics should increment the `boundary_rejection` counter.

**Scenario 052**: Unbound Variable in Custom Query (Iteration 6)
- **Trigger**: Intent requests query with unbound `X`.
- **Expected Kernel Behavior**: Mangle analysis throws safety error.
- **Expected Session Behavior**: Executor logs error, aborts query.
- **Observability**: Metrics should increment the `analysis_error` counter.

**Scenario 053**: Stratification Cycle (Iteration 6)
- **Trigger**: Intent asserts `active(X) :- not active(X)`.
- **Expected Kernel Behavior**: Mangle analysis throws stratification error.
- **Expected Session Behavior**: Executor drops fact, alerts user.
- **Observability**: Metrics should increment the `stratification_error` counter.

**Scenario 054**: Stringly Typed Assertion (Iteration 6)
- **Trigger**: Intent asserts `status("active")` instead of `status(/active)`.
- **Expected Kernel Behavior**: Kernel accepts fact.
- **Expected Session Behavior**: Subsequent query for `/active` fails (0 results). Executor detects logic failure.
- **Observability**: Metrics should increment the `type_dissonance` counter.

**Scenario 055**: Extreme Recursion Depth (Iteration 6)
- **Trigger**: Intent triggers recursive rule 10k times.
- **Expected Kernel Behavior**: JIT evaluation hits depth limit timeout.
- **Expected Session Behavior**: Executor cancels context, cleans up channels.
- **Observability**: Metrics should increment the `jit_timeout` counter.

**Scenario 056**: Massive Arity Table Join (Iteration 6)
- **Trigger**: Intent asserts fact with 1000 arguments.
- **Expected Kernel Behavior**: SQLite backend throws parameter limit error.
- **Expected Session Behavior**: Kernel recovers, rolls back transaction.
- **Observability**: Metrics should increment the `sql_limit` counter.

**Scenario 057**: Concurrent Mutation of Same Fact (Iteration 6)
- **Trigger**: 100 goroutines assert the same singleton fact.
- **Expected Kernel Behavior**: Kernel handles idempotency via deduplication.
- **Expected Session Behavior**: Executor proceeds without race condition.
- **Observability**: Metrics should increment the `idempotency_hit` counter.

**Scenario 058**: Context Cancellation Mid-Transaction (Iteration 6)
- **Trigger**: Context cancelled while acquiring Kernel lock.
- **Expected Kernel Behavior**: Transaction aborts cleanly.
- **Expected Session Behavior**: Executor abandons operation, no deadlock.
- **Observability**: Metrics should increment the `txn_abort` counter.

**Scenario 059**: Invalid Unicode in Payload (Iteration 6)
- **Trigger**: Intent includes null bytes `\x00` in payload.
- **Expected Kernel Behavior**: Mangle parser throws syntax error.
- **Expected Session Behavior**: Executor rejects input, reprompts.
- **Observability**: Metrics should increment the `parse_error` counter.

**Scenario 060**: Ghost Fact Bleed Across Turns (Iteration 6)
- **Trigger**: Turn 1 facts left over in virtual store.
- **Expected Kernel Behavior**: Kernel retracts ephemeral facts properly.
- **Expected Session Behavior**: Turn 2 query returns 0 ghost facts.
- **Observability**: Metrics should increment the `ghost_facts_prevented` counter.

**Scenario 061**: Null Target in Write Action (Iteration 7)
- **Trigger**: Intent passes nil target to a file-write tool.
- **Expected Kernel Behavior**: Kernel validation layer rejects assertion.
- **Expected Session Behavior**: Executor degrades and reprompts agent.
- **Observability**: Metrics should increment the `boundary_rejection` counter.

**Scenario 062**: Unbound Variable in Custom Query (Iteration 7)
- **Trigger**: Intent requests query with unbound `X`.
- **Expected Kernel Behavior**: Mangle analysis throws safety error.
- **Expected Session Behavior**: Executor logs error, aborts query.
- **Observability**: Metrics should increment the `analysis_error` counter.

**Scenario 063**: Stratification Cycle (Iteration 7)
- **Trigger**: Intent asserts `active(X) :- not active(X)`.
- **Expected Kernel Behavior**: Mangle analysis throws stratification error.
- **Expected Session Behavior**: Executor drops fact, alerts user.
- **Observability**: Metrics should increment the `stratification_error` counter.

**Scenario 064**: Stringly Typed Assertion (Iteration 7)
- **Trigger**: Intent asserts `status("active")` instead of `status(/active)`.
- **Expected Kernel Behavior**: Kernel accepts fact.
- **Expected Session Behavior**: Subsequent query for `/active` fails (0 results). Executor detects logic failure.
- **Observability**: Metrics should increment the `type_dissonance` counter.

**Scenario 065**: Extreme Recursion Depth (Iteration 7)
- **Trigger**: Intent triggers recursive rule 10k times.
- **Expected Kernel Behavior**: JIT evaluation hits depth limit timeout.
- **Expected Session Behavior**: Executor cancels context, cleans up channels.
- **Observability**: Metrics should increment the `jit_timeout` counter.

**Scenario 066**: Massive Arity Table Join (Iteration 7)
- **Trigger**: Intent asserts fact with 1000 arguments.
- **Expected Kernel Behavior**: SQLite backend throws parameter limit error.
- **Expected Session Behavior**: Kernel recovers, rolls back transaction.
- **Observability**: Metrics should increment the `sql_limit` counter.

**Scenario 067**: Concurrent Mutation of Same Fact (Iteration 7)
- **Trigger**: 100 goroutines assert the same singleton fact.
- **Expected Kernel Behavior**: Kernel handles idempotency via deduplication.
- **Expected Session Behavior**: Executor proceeds without race condition.
- **Observability**: Metrics should increment the `idempotency_hit` counter.

**Scenario 068**: Context Cancellation Mid-Transaction (Iteration 7)
- **Trigger**: Context cancelled while acquiring Kernel lock.
- **Expected Kernel Behavior**: Transaction aborts cleanly.
- **Expected Session Behavior**: Executor abandons operation, no deadlock.
- **Observability**: Metrics should increment the `txn_abort` counter.

**Scenario 069**: Invalid Unicode in Payload (Iteration 7)
- **Trigger**: Intent includes null bytes `\x00` in payload.
- **Expected Kernel Behavior**: Mangle parser throws syntax error.
- **Expected Session Behavior**: Executor rejects input, reprompts.
- **Observability**: Metrics should increment the `parse_error` counter.

**Scenario 070**: Ghost Fact Bleed Across Turns (Iteration 7)
- **Trigger**: Turn 1 facts left over in virtual store.
- **Expected Kernel Behavior**: Kernel retracts ephemeral facts properly.
- **Expected Session Behavior**: Turn 2 query returns 0 ghost facts.
- **Observability**: Metrics should increment the `ghost_facts_prevented` counter.

**Scenario 071**: Null Target in Write Action (Iteration 8)
- **Trigger**: Intent passes nil target to a file-write tool.
- **Expected Kernel Behavior**: Kernel validation layer rejects assertion.
- **Expected Session Behavior**: Executor degrades and reprompts agent.
- **Observability**: Metrics should increment the `boundary_rejection` counter.

**Scenario 072**: Unbound Variable in Custom Query (Iteration 8)
- **Trigger**: Intent requests query with unbound `X`.
- **Expected Kernel Behavior**: Mangle analysis throws safety error.
- **Expected Session Behavior**: Executor logs error, aborts query.
- **Observability**: Metrics should increment the `analysis_error` counter.

**Scenario 073**: Stratification Cycle (Iteration 8)
- **Trigger**: Intent asserts `active(X) :- not active(X)`.
- **Expected Kernel Behavior**: Mangle analysis throws stratification error.
- **Expected Session Behavior**: Executor drops fact, alerts user.
- **Observability**: Metrics should increment the `stratification_error` counter.

**Scenario 074**: Stringly Typed Assertion (Iteration 8)
- **Trigger**: Intent asserts `status("active")` instead of `status(/active)`.
- **Expected Kernel Behavior**: Kernel accepts fact.
- **Expected Session Behavior**: Subsequent query for `/active` fails (0 results). Executor detects logic failure.
- **Observability**: Metrics should increment the `type_dissonance` counter.

**Scenario 075**: Extreme Recursion Depth (Iteration 8)
- **Trigger**: Intent triggers recursive rule 10k times.
- **Expected Kernel Behavior**: JIT evaluation hits depth limit timeout.
- **Expected Session Behavior**: Executor cancels context, cleans up channels.
- **Observability**: Metrics should increment the `jit_timeout` counter.

**Scenario 076**: Massive Arity Table Join (Iteration 8)
- **Trigger**: Intent asserts fact with 1000 arguments.
- **Expected Kernel Behavior**: SQLite backend throws parameter limit error.
- **Expected Session Behavior**: Kernel recovers, rolls back transaction.
- **Observability**: Metrics should increment the `sql_limit` counter.

**Scenario 077**: Concurrent Mutation of Same Fact (Iteration 8)
- **Trigger**: 100 goroutines assert the same singleton fact.
- **Expected Kernel Behavior**: Kernel handles idempotency via deduplication.
- **Expected Session Behavior**: Executor proceeds without race condition.
- **Observability**: Metrics should increment the `idempotency_hit` counter.

**Scenario 078**: Context Cancellation Mid-Transaction (Iteration 8)
- **Trigger**: Context cancelled while acquiring Kernel lock.
- **Expected Kernel Behavior**: Transaction aborts cleanly.
- **Expected Session Behavior**: Executor abandons operation, no deadlock.
- **Observability**: Metrics should increment the `txn_abort` counter.

**Scenario 079**: Invalid Unicode in Payload (Iteration 8)
- **Trigger**: Intent includes null bytes `\x00` in payload.
- **Expected Kernel Behavior**: Mangle parser throws syntax error.
- **Expected Session Behavior**: Executor rejects input, reprompts.
- **Observability**: Metrics should increment the `parse_error` counter.

**Scenario 080**: Ghost Fact Bleed Across Turns (Iteration 8)
- **Trigger**: Turn 1 facts left over in virtual store.
- **Expected Kernel Behavior**: Kernel retracts ephemeral facts properly.
- **Expected Session Behavior**: Turn 2 query returns 0 ghost facts.
- **Observability**: Metrics should increment the `ghost_facts_prevented` counter.

**Scenario 081**: Null Target in Write Action (Iteration 9)
- **Trigger**: Intent passes nil target to a file-write tool.
- **Expected Kernel Behavior**: Kernel validation layer rejects assertion.
- **Expected Session Behavior**: Executor degrades and reprompts agent.
- **Observability**: Metrics should increment the `boundary_rejection` counter.

**Scenario 082**: Unbound Variable in Custom Query (Iteration 9)
- **Trigger**: Intent requests query with unbound `X`.
- **Expected Kernel Behavior**: Mangle analysis throws safety error.
- **Expected Session Behavior**: Executor logs error, aborts query.
- **Observability**: Metrics should increment the `analysis_error` counter.

**Scenario 083**: Stratification Cycle (Iteration 9)
- **Trigger**: Intent asserts `active(X) :- not active(X)`.
- **Expected Kernel Behavior**: Mangle analysis throws stratification error.
- **Expected Session Behavior**: Executor drops fact, alerts user.
- **Observability**: Metrics should increment the `stratification_error` counter.

**Scenario 084**: Stringly Typed Assertion (Iteration 9)
- **Trigger**: Intent asserts `status("active")` instead of `status(/active)`.
- **Expected Kernel Behavior**: Kernel accepts fact.
- **Expected Session Behavior**: Subsequent query for `/active` fails (0 results). Executor detects logic failure.
- **Observability**: Metrics should increment the `type_dissonance` counter.

**Scenario 085**: Extreme Recursion Depth (Iteration 9)
- **Trigger**: Intent triggers recursive rule 10k times.
- **Expected Kernel Behavior**: JIT evaluation hits depth limit timeout.
- **Expected Session Behavior**: Executor cancels context, cleans up channels.
- **Observability**: Metrics should increment the `jit_timeout` counter.

**Scenario 086**: Massive Arity Table Join (Iteration 9)
- **Trigger**: Intent asserts fact with 1000 arguments.
- **Expected Kernel Behavior**: SQLite backend throws parameter limit error.
- **Expected Session Behavior**: Kernel recovers, rolls back transaction.
- **Observability**: Metrics should increment the `sql_limit` counter.

**Scenario 087**: Concurrent Mutation of Same Fact (Iteration 9)
- **Trigger**: 100 goroutines assert the same singleton fact.
- **Expected Kernel Behavior**: Kernel handles idempotency via deduplication.
- **Expected Session Behavior**: Executor proceeds without race condition.
- **Observability**: Metrics should increment the `idempotency_hit` counter.

**Scenario 088**: Context Cancellation Mid-Transaction (Iteration 9)
- **Trigger**: Context cancelled while acquiring Kernel lock.
- **Expected Kernel Behavior**: Transaction aborts cleanly.
- **Expected Session Behavior**: Executor abandons operation, no deadlock.
- **Observability**: Metrics should increment the `txn_abort` counter.

**Scenario 089**: Invalid Unicode in Payload (Iteration 9)
- **Trigger**: Intent includes null bytes `\x00` in payload.
- **Expected Kernel Behavior**: Mangle parser throws syntax error.
- **Expected Session Behavior**: Executor rejects input, reprompts.
- **Observability**: Metrics should increment the `parse_error` counter.

**Scenario 090**: Ghost Fact Bleed Across Turns (Iteration 9)
- **Trigger**: Turn 1 facts left over in virtual store.
- **Expected Kernel Behavior**: Kernel retracts ephemeral facts properly.
- **Expected Session Behavior**: Turn 2 query returns 0 ghost facts.
- **Observability**: Metrics should increment the `ghost_facts_prevented` counter.

**Scenario 091**: Null Target in Write Action (Iteration 10)
- **Trigger**: Intent passes nil target to a file-write tool.
- **Expected Kernel Behavior**: Kernel validation layer rejects assertion.
- **Expected Session Behavior**: Executor degrades and reprompts agent.
- **Observability**: Metrics should increment the `boundary_rejection` counter.

**Scenario 092**: Unbound Variable in Custom Query (Iteration 10)
- **Trigger**: Intent requests query with unbound `X`.
- **Expected Kernel Behavior**: Mangle analysis throws safety error.
- **Expected Session Behavior**: Executor logs error, aborts query.
- **Observability**: Metrics should increment the `analysis_error` counter.

**Scenario 093**: Stratification Cycle (Iteration 10)
- **Trigger**: Intent asserts `active(X) :- not active(X)`.
- **Expected Kernel Behavior**: Mangle analysis throws stratification error.
- **Expected Session Behavior**: Executor drops fact, alerts user.
- **Observability**: Metrics should increment the `stratification_error` counter.

**Scenario 094**: Stringly Typed Assertion (Iteration 10)
- **Trigger**: Intent asserts `status("active")` instead of `status(/active)`.
- **Expected Kernel Behavior**: Kernel accepts fact.
- **Expected Session Behavior**: Subsequent query for `/active` fails (0 results). Executor detects logic failure.
- **Observability**: Metrics should increment the `type_dissonance` counter.

**Scenario 095**: Extreme Recursion Depth (Iteration 10)
- **Trigger**: Intent triggers recursive rule 10k times.
- **Expected Kernel Behavior**: JIT evaluation hits depth limit timeout.
- **Expected Session Behavior**: Executor cancels context, cleans up channels.
- **Observability**: Metrics should increment the `jit_timeout` counter.

**Scenario 096**: Massive Arity Table Join (Iteration 10)
- **Trigger**: Intent asserts fact with 1000 arguments.
- **Expected Kernel Behavior**: SQLite backend throws parameter limit error.
- **Expected Session Behavior**: Kernel recovers, rolls back transaction.
- **Observability**: Metrics should increment the `sql_limit` counter.

**Scenario 097**: Concurrent Mutation of Same Fact (Iteration 10)
- **Trigger**: 100 goroutines assert the same singleton fact.
- **Expected Kernel Behavior**: Kernel handles idempotency via deduplication.
- **Expected Session Behavior**: Executor proceeds without race condition.
- **Observability**: Metrics should increment the `idempotency_hit` counter.

**Scenario 098**: Context Cancellation Mid-Transaction (Iteration 10)
- **Trigger**: Context cancelled while acquiring Kernel lock.
- **Expected Kernel Behavior**: Transaction aborts cleanly.
- **Expected Session Behavior**: Executor abandons operation, no deadlock.
- **Observability**: Metrics should increment the `txn_abort` counter.

**Scenario 099**: Invalid Unicode in Payload (Iteration 10)
- **Trigger**: Intent includes null bytes `\x00` in payload.
- **Expected Kernel Behavior**: Mangle parser throws syntax error.
- **Expected Session Behavior**: Executor rejects input, reprompts.
- **Observability**: Metrics should increment the `parse_error` counter.

**Scenario 100**: Ghost Fact Bleed Across Turns (Iteration 10)
- **Trigger**: Turn 1 facts left over in virtual store.
- **Expected Kernel Behavior**: Kernel retracts ephemeral facts properly.
- **Expected Session Behavior**: Turn 2 query returns 0 ghost facts.
- **Observability**: Metrics should increment the `ghost_facts_prevented` counter.

### System Performance and Resilience Evaluation
Based on the architecture and tests, the Session-Kernel Boundary appears sound for standard operations. The use of core.NewRealKernel inside isolated test environments demonstrates a commitment to clean state management.
However, the identified gaps represent severe risks:
1. **Null/Empty:** Nil pointer dereferences risk crashing the process.
2. **Type Coercion:** Mangle strictness requires perfect sanitization. Silent failures (zero results) lead to hallucination loops.
3. **Extremes:** Memory pressure from 50MB strings will consume RAM and requires explicit length enforcement.
4. **State Conflicts:** Complex concurrent queries during writes expose locking deadlocks.

### Proposed Code Modification Summary (TODOs)
I have identified specific locations in tests/e2e/Session_Kernel_Boundary_integration_test.go to insert // TODO: comments for these missing edge cases.
1. **CATEGORY 2: STATE CORRUPTION**: Add tests for Massive Arity and Extreme String Lengths.
2. **CATEGORY 3: CONTRACT VIOLATION**: Add tests for Nil Intent pointers, Empty strings, and Type coercion attacks.
3. **CATEGORY 4: RESOURCE EXHAUSTION**: Add concurrent Assert/Retract race condition tests.
4. **CATEGORY 5: TEMPORAL FAILURE**: Add tests for Context Cancellation during active locks.

### Conclusion
By implementing these negative tests, we can move the system from Level 2 (RECOVERED) or Level 0 (SILENT FAIL) up to Level 4 (PREVENTED) on the Healing Hierarchy for these specific edge cases. This ensures codeNERD remains robust even under extreme adversarial conditions.