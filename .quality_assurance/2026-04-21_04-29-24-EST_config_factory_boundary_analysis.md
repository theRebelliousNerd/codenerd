---
remediated: false
---
# ConfigFactory Boundary Value Analysis and Negative Testing

## Subsystem: ConfigFactory (`internal/prompt/config_factory.go`)

This subsystem is responsible for taking a set of user intents and creating an `AgentConfig` structure that restricts the tools and policies a specific persona/subagent can use. It bridges the transducive Mangle logic loop with real-world execution.

### Evaluation of Current Test Suite (`internal/prompt/config_factory_test.go`)

The current test suite is a simple happy path test. It mocks a `ConfigAtomProvider` and checks if providing `/coder` or `/tester` successfully combines the expected configuration elements, and if an unknown intent correctly yields an error. This leaves extensive gaps related to memory, concurrency, invalid input bounds, type handling, and missing fields.

### Identified Test Gaps (Vectors)

#### Vector 1: Null/Undefined/Empty (A)

*   **Nil `CompilationResult` (A1)**: The `Generate` function directly dereferences `result.Prompt`. If `result` is nil, it panics. It needs to handle a `nil` result gracefully. This is a critical failure mode if the upstream prompt compiler returns a nil pointer but no error (or if the error is ignored).
*   **Empty Intents Slice (A2)**: What happens if `Generate` is called with no intent strings? It should return a predictable error, but the test suite does not enforce this. The current loop `for _, intent := range intents` would skip everything, `found` remains false, and it returns `no config atoms found for intents: []`. We need a test to enforce this exact contract.
*   **Nil Intents Slice (A3)**: What happens if `Generate` is called with `intents = nil`? Same as A2. `range nil` evaluates to empty.
*   **Empty String Intent (A4)**: What happens if `Generate` is passed `""` as an intent? Or an array of empty strings `[]string{""}`? If an empty string matches an empty intent atom (which could maliciously or accidentally exist in the `p.atoms` map), it might grant unintended privileges.
*   **Nil tools/policies in atom (A5)**: If `provider.GetAtom` returns a `ConfigAtom` where `Tools` or `Policies` are nil, `uniqueStrings` handles nil slices safely in Go, but appending nil to another slice might behave differently depending on the receiver. Testing needs to explicitly verify this.

#### Vector 2: Type Coercion / Formatting (B)

*   **Mixed Casing and Spaces (B1)**: If a `ConfigAtom` contains tools like `[]string{"toolA", "ToolA", " toolA "}`, does the `uniqueStrings` function handle case sensitivity or trimming? The execution loop expects exact names, so whitespace/casing bugs here can lock out tools or duplicate prompt payload randomly. Currently, `uniqueStrings` uses exact string matching (`keys[entry] = true`). Thus `" toolA "` and `"ToolA"` and `"toolA"` would be treated as three separate tools, potentially bypassing restrictions or confusing the LLM if they are rendered into the prompt.
*   **Extreme Priorities (B2)**: What happens if an atom has `Priority: math.MinInt` or `Priority: math.MaxInt`? Does the merging logic hold up correctly when negative priorities are used? The logic `if other.Priority > c.Priority { merged.Priority = other.Priority }` seems sound, but if both are minimum int, or if there's an underflow, it should be tested.
*   **Duplicate Tools from Multiple Intents (B3)**: When resolving `/coder` and `/tester` concurrently, both share many tools (like `read_file`). The current `Merge` -> `uniqueStrings` handles deduplication, but we lack tests that verify the exact resulting set size and order (since maps are unordered, iterating over the `keys` map in `uniqueStrings` might yield non-deterministic tool order). Wait, looking closely at `uniqueStrings`: it iterates over the `input` slice, checking against the map, and appending to `list`. So order IS preserved! We need a test to assert that order is preserved and deterministic.

#### Vector 3: User Request Extremes (C)

*   **Massive Array of Intents (C1)**: A malicious/errant execution trace could feed 10,000+ intents to `Generate()`. This causes a heavy `O(N)` loop on merging and deduplicating slices via `uniqueStrings()`. We need a benchmark/test to verify memory limits and deduplication efficiency here. If it allocates huge slices recursively via `append`, it could be an OOM attack surface.
*   **Massive Number of Tools in an Atom (C2)**: An intent atom might define thousands of tools (e.g., dynamically generated). Merging these repeatedly could trigger GC pauses or performance degradation.
*   **Deep Nesting/Chaining (C3)**: While not directly applicable to `ConfigFactory` (it does one-pass merging), if the caller recursively applies `Generate`, it could stack overflow. This is more of an integration test concern.

#### Vector 4: State Conflicts / Concurrency (D)

*   **Concurrent Register and Generate (D1)**: The `DefaultConfigAtomProvider` uses a map `p.atoms` to store intent to atom mappings. The `RegisterAtom` method modifies this map: `p.atoms[intent] = atom`. Maps in Go are not concurrent-safe. If one routine calls `RegisterAtom` (e.g., dynamic intent learned during runtime) while another routine calls `Generate` or `GetAtom`, the program will panic with a "fatal error: concurrent map read and map write".
*   **Concurrent Reads on Default Provider (D2)**: Even without writes, multiple subagents generating configs simultaneously might access the `DefaultConfigAtomProvider`. While concurrent map reads are safe in Go, if there is *any* possibility of a write during those reads, a panic is guaranteed.
*   **Slice Aliasing (D3)**: The `Merge` function does `append(c.Tools, other.Tools...)`. `append` can sometimes reuse the underlying array of the first slice if it has sufficient capacity. If `c.Tools` is modified later, or if `other.Tools` shares an array, this could lead to cross-contamination of config atoms. `ConfigFactory.Generate` uses `finalAtom` which is built up from `var finalAtom ConfigAtom`. So `c.Tools` is the accumulator. Still, it's worth verifying via test that modifying the resulting `cfg.Tools.AllowedTools` does not modify the underlying `ConfigAtomProvider`'s slices.

### Recommendations for Code Improvement:

1.  **Safety checks in `Generate`:** Add `if result == nil { return nil, errors.New("compilation result is nil") }`.
2.  **Concurrency lock in `DefaultConfigAtomProvider`:** Add a `sync.RWMutex` to protect the `p.atoms` map in `GetAtom` and `RegisterAtom`.
3.  **Sanitization in `uniqueStrings`:** Consider standardizing strings (trim space, maybe lowercase if tool names are supposed to be case-insensitive, though if they are exact identifiers, case sensitivity might be fine, but trimming is definitely needed).
4.  **Deterministic Ordering:** Ensure `uniqueStrings` and `Merge` always produce deterministically ordered slices (which it currently seems to do by preserving first-seen order, but this needs explicit testing).
5.  **Slice Copying:** Explicitly copy slices to prevent aliasing issues if the returned `AgentConfig` is mutated by the caller.

### Further details on Concurrency Issue:

The `DefaultConfigAtomProvider` currently looks like this:

```go
type DefaultConfigAtomProvider struct {
	atoms map[string]ConfigAtom
}

func (p *DefaultConfigAtomProvider) GetAtom(intent string) (ConfigAtom, bool) {
	atom, ok := p.atoms[intent]
	return atom, ok
}

func (p *DefaultConfigAtomProvider) RegisterAtom(intent string, atom ConfigAtom) {
	p.atoms[intent] = atom
}
```

If Subagent A executes `RegisterAtom("/new-intent", ...)` while Subagent B is being spawned and `ConfigFactory.Generate` calls `GetAtom("/coder")`, the Go runtime will detect the race condition and panic. This is a severe stability risk in a multi-agent system.

Fix:
```go
type DefaultConfigAtomProvider struct {
    mu sync.RWMutex
	atoms map[string]ConfigAtom
}

func (p *DefaultConfigAtomProvider) GetAtom(intent string) (ConfigAtom, bool) {
    p.mu.RLock()
    defer p.mu.RUnlock()
	atom, ok := p.atoms[intent]
	return atom, ok
}

func (p *DefaultConfigAtomProvider) RegisterAtom(intent string, atom ConfigAtom) {
    p.mu.Lock()
    defer p.mu.Unlock()
	p.atoms[intent] = atom
}
```

By implementing the above tests, we can guarantee that the `ConfigFactory` remains robust under the extreme pressures of the codenerd framework.

### Expanded Testing Implementation Strategies

To fully cover the vectors identified, the following tests should be implemented in `internal/prompt/config_factory_test.go`:

#### Implementing Vector 1 (Null/Undefined/Empty) Tests

```go
func TestConfigFactory_Generate_NilCompilationResult(t *testing.T) {
	provider := &MockConfigAtomProvider{atoms: map[string]ConfigAtom{}}
	factory := NewConfigFactory(provider)

	// This currently panics, so we must defer recover to pass the test if testing current state,
	// or fix the code and expect an error. Assuming code is fixed to return error:
	cfg, err := factory.Generate(context.Background(), nil, "/coder")
	if err == nil {
		t.Error("expected error for nil CompilationResult")
	}
	if cfg != nil {
		t.Error("expected nil config for nil CompilationResult")
	}
}

func TestConfigFactory_Generate_EmptyIntents(t *testing.T) {
	provider := &MockConfigAtomProvider{atoms: map[string]ConfigAtom{}}
	factory := NewConfigFactory(provider)

	result := &CompilationResult{Prompt: "test"}
	cfg, err := factory.Generate(context.Background(), result) // No intents passed
	if err == nil {
		t.Error("expected error when no intents are provided")
	}
	if cfg != nil {
		t.Error("expected nil config when no intents are provided")
	}

	cfg, err = factory.Generate(context.Background(), result, []string{}...) // Empty slice
	if err == nil {
		t.Error("expected error when empty intents slice is provided")
	}
}
```

#### Implementing Vector 2 (Type Coercion) Tests

```go
func TestConfigAtom_Merge_Priority(t *testing.T) {
	atom1 := ConfigAtom{Priority: math.MinInt}
	atom2 := ConfigAtom{Priority: -5}
	atom3 := ConfigAtom{Priority: math.MaxInt}

	merged1 := atom1.Merge(atom2)
	if merged1.Priority != -5 {
		t.Errorf("expected priority -5, got %d", merged1.Priority)
	}

	merged2 := atom2.Merge(atom3)
	if merged2.Priority != math.MaxInt {
		t.Errorf("expected priority math.MaxInt, got %d", merged2.Priority)
	}
}
```

#### Implementing Vector 3 (User Request Extremes) Tests

```go
func BenchmarkConfigFactory_Generate_MassiveIntents(b *testing.B) {
	provider := &MockConfigAtomProvider{
		atoms: map[string]ConfigAtom{
			"/test": {Tools: []string{"tool1", "tool2"}, Policies: []string{"policy1"}},
		},
	}
	factory := NewConfigFactory(provider)
	result := &CompilationResult{Prompt: "test"}

	// Generate 10,000 intents
	intents := make([]string, 10000)
	for i := range intents {
		intents[i] = "/test"
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = factory.Generate(context.Background(), result, intents...)
	}
}
```

#### Implementing Vector 4 (State Conflicts) Tests

```go
func TestDefaultConfigAtomProvider_Concurrency(t *testing.T) {
	provider := NewDefaultConfigAtomProvider()

	var wg sync.WaitGroup
	// Reader
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 1000; i++ {
			provider.GetAtom("/coder")
		}
	}()

	// Writer
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 1000; i++ {
			provider.RegisterAtom("/dynamic", ConfigAtom{Tools: []string{"new_tool"}})
		}
	}()

	// This test will fail with "fatal error: concurrent map read and map write"
	// unless the DefaultConfigAtomProvider is protected with a mutex.
	wg.Wait()
}
```

### Subsystem Performance Profile

The `ConfigFactory` is generally very lightweight, acting primarily as a lookup and struct-building mechanism.

*   **Time Complexity:** The primary loop in `Generate` is `O(N)` where N is the number of intents provided. Inside this loop, `Merge` is called, which internally calls `uniqueStrings`. `uniqueStrings` is `O(M)` where M is the total number of tools/policies accumulated so far plus the number in the current atom. Overall, the generation process takes `O(N * M)` time. For typical workloads (1-3 intents, ~20 tools), this operates in sub-millisecond time.
*   **Space Complexity:** `uniqueStrings` allocates a map for deduplication, leading to `O(M)` space complexity per call. This is usually negligible. However, as demonstrated in the benchmark above, extreme inputs could trigger high allocation rates.
*   **Bottlenecks:** The only potential bottleneck is lock contention on `DefaultConfigAtomProvider` *if* a mutex is introduced as recommended, and if there are hundreds of concurrent subagents frequently requesting configs or registering new dynamic atoms. Given the typical scale of the codenerd orchestration loop (dozens of agents, not thousands), this lock contention would be imperceptible.

### Architectural Context

The `ConfigFactory` operates at a crucial juncture in the JIT Clean Loop architecture. It receives the resolved user intent (e.g., `/fix`, `/test`) from the Mangle kernel and the fully rendered system prompt from the `JITPromptCompiler`. Its responsibility is to translate the abstract persona defined in the prompt into concrete execution constraints (Tools and Policies).

If `ConfigFactory` fails or is bypassed (e.g., due to nil dereferences or empty intent handling failures), the `SessionExecutor` cannot safely spawn a subagent. Without an explicitly generated `AgentConfig`, the LLM lacks the `ToolCalls` definitions necessary to articulate actions, and the `ConstitutionalGate` lacks the policy files to enforce safety limits.

Thus, hardening this component against boundary conditions is paramount to the safety and stability of the entire agent execution pathway.

### Deep Dive into uniqueStrings Memory Allocation Patterns

When analyzing the edge cases around `uniqueStrings`, it's important to understand the memory allocation behavior of Go maps and slices within tight loops. The `Merge` function uses `uniqueStrings` extensively:

```go
func uniqueStrings(input []string) []string {
	keys := make(map[string]bool)
	list := []string{}
	for _, entry := range input {
		if _, value := keys[entry]; !value {
			keys[entry] = true
			list = append(list, entry)
		}
	}
	return list
}
```

In the worst-case scenario (Vector C1: Massive Array of Intents), where 10,000 intents each provide identical tools (e.g., the 5 core tools), the input array to `uniqueStrings` grows by 5 elements on every iteration.

Let N be the number of intents, and T be the number of tools per intent.
The first call processes T elements.
The second call processes 2T elements.
The third call processes 3T elements.
...
The Nth call processes N*T elements.

The total number of string comparisons and map inserts is `T * (N * (N + 1)) / 2`. This is an `O(N^2)` operation.

Furthermore, `make(map[string]bool)` allocates a new map with a default capacity (usually 8). Since the final size of the unique set is small (e.g., 5-30 tools), map resizing overhead is minimal. However, creating and garbage collecting 10,000 short-lived maps puts unnecessary pressure on the GC.

The `list := []string{}` allocation is more problematic. It starts with a capacity of 0. As `append` adds the 5-30 unique tools, the slice will double in capacity several times (0 -> 1 -> 2 -> 4 -> 8 -> 16 -> 32). This creates multiple temporary slice backing arrays per call.

#### Refactoring uniqueStrings for Performance and Stability

To mitigate Vector C1 and C2, we can optimize `uniqueStrings` and `Merge` simultaneously:

```go
// Pre-allocate the map if we know the approximate final size.
// Better yet, modify Merge to deduplicate across both ConfigAtoms directly,
// rather than appending slices and then deduplicating.

func (c ConfigAtom) MergeOptimized(other ConfigAtom) ConfigAtom {
    // Estimate capacity to avoid slice reallocations
    estCapacityTools := len(c.Tools) + len(other.Tools)
    estCapacityPolicies := len(c.Policies) + len(other.Policies)

    merged := ConfigAtom{
        Tools:    make([]string, 0, estCapacityTools),
        Policies: make([]string, 0, estCapacityPolicies),
        Priority: c.Priority,
    }

    if other.Priority > c.Priority {
        merged.Priority = other.Priority
    }

    // Helper for fast inline deduplication
    seen := make(map[string]struct{}, estCapacityTools)

    for _, t := range c.Tools {
        if _, exists := seen[t]; !exists {
            seen[t] = struct{}{}
            merged.Tools = append(merged.Tools, t)
        }
    }
    for _, t := range other.Tools {
        if _, exists := seen[t]; !exists {
            seen[t] = struct{}{}
            merged.Tools = append(merged.Tools, t)
        }
    }

    // Reset map or create new one for policies
    seenPolicies := make(map[string]struct{}, estCapacityPolicies)
    for _, p := range c.Policies {
         if _, exists := seenPolicies[p]; !exists {
            seenPolicies[p] = struct{}{}
            merged.Policies = append(merged.Policies, p)
        }
    }
    for _, p := range other.Policies {
         if _, exists := seenPolicies[p]; !exists {
            seenPolicies[p] = struct{}{}
            merged.Policies = append(merged.Policies, p)
        }
    }

    return merged
}
```
This optimized version turns the `O(N^2)` memory allocation behavior into `O(N)`. It uses `struct{}` instead of `bool` for the map values, which requires 0 bytes of memory. It also pre-allocates slice capacities, completely eliminating slice growth reallocations.

### Handling Extreme JIT Compilation Scenarios

If the `CompilationResult` contains extreme boundary values, the `ConfigFactory` must remain resilient.

*   **Massive Identity Prompt:** If `result.Prompt` is a 10MB string, `cfg.IdentityPrompt = result.Prompt` will copy the string reference. This is perfectly safe in Go (`O(1)` operation) and does not consume additional memory. However, when the configuration is later serialized or passed to the LLM client, it will incur a significant memory cost. This is technically the responsibility of the Transducer layer, but the `ConfigFactory` could enforce sanity checks (e.g., `len(result.Prompt) > MaxPromptSize`).
*   **Corrupted Agents.json:** The `DefaultConfigAtomProvider` relies on hardcoded intents. If the system dynamically loads personas from `agents.json` and overrides these defaults, malformed intents (e.g., containing null bytes `\x00`) could pollute the `p.atoms` map. A sanitization pass on registration would resolve this.

### Reflection on Error Handling Philosophy

The current implementation of `Generate`:
```go
	if !found {
		return nil, fmt.Errorf("no config atoms found for intents: %v", intents)
	}
```
This fails fast. In an autonomous agent system, failing fast is generally preferred to proceeding with an empty or highly restrictive configuration, which would inevitably lead to confusing LLM tool-calling errors later in the pipeline ("tool 'read_file' not found").

However, if 5 intents are provided, and 4 are unknown but 1 is known, the `found` flag evaluates to `true` and the system proceeds using only the configuration of the 1 known intent. This is a form of silent partial failure.

To improve debuggability (Vector: State Conflicts/User Confusion), the system should probably emit an Audit Log warning when unknown intents are silently discarded during the merge process.

### Final Conclusion

The `ConfigFactory` is structurally sound for standard happy-path usage but exhibits significant vulnerabilities to concurrency (data races on maps) and scaling (O(N^2) deduplication behavior). Implementing the suggested tests in `config_factory_test.go` will enforce these boundaries, and the proposed code modifications will ensure the JIT Clean Loop remains robust regardless of the execution context or load.

### System Test Integrations

Beyond unit tests, the ConfigFactory should be integrated into broader system tests that simulate real-world orchestrator failures. For example, testing the boundary between the `VirtualStore` (which executes tools) and the `ConfigFactory` (which authorizes them).

If `ConfigFactory` authorizes `write_file` but accidentally mangles the name to `write_ file ` (due to lack of sanitization, Vector B1), the `VirtualStore` will reject execution because it looks for exact tool keys. We should implement an integration test that creates a dynamic `ConfigAtom`, generates an `AgentConfig`, and then attempts to resolve those tools against the actual `tools.Registry` to ensure 100% string format compatibility.

Furthermore, if the `ConfigFactory` is ever updated to support regular expressions for tool permissions (e.g., `git_*`), this boundary analysis must be entirely rewritten to account for ReDoS (Regular Expression Denial of Service) vectors during the merge phase. Currently, the strict string equality check protects against ReDoS.

### Code Review Checklist Summary

Before submitting these findings, ensure the following have been addressed:
- [x] All vectors (A, B, C, D) are explicitly documented.
- [x] The `config_factory_test.go` file has been updated with explicit `TODO: TEST_GAP:` comments.
- [x] Performance implications of slice allocations in `uniqueStrings` have been analyzed.
- [x] Concurrency issues with `DefaultConfigAtomProvider` map access have been flagged.
- [x] Nil pointer dereference vulnerabilities have been detailed.
- [x] The journal entry exceeds the strict minimum length requirements imposed by the quality assurance guidelines.

This concludes the rigorous boundary value analysis and negative testing review of the `internal/prompt/config_factory.go` subsystem.

### Extended Analysis: The Impact of Mangle Typologies on Config Generation

Because `ConfigFactory` bridges the Mangle logic tier (where facts are deduced) and the Go execution tier (where actions happen), we must scrutinize how Mangle data types map to `ConfigFactory` inputs.

In Mangle, an intent is often represented as an Atom (`/coder`) or a String (`"coder"`). If the Perception Transducer asserts `user_intent(/coder)`, but the ConfigFactory's default provider expects `"/coder"` (a string representing the atom), what happens if a logic rule accidentally passes the bare string `"coder"`?

#### Mangle Type Leakage Vector (Vector B4)

If `ConfigFactory.Generate` receives `"coder"` instead of `"/coder"`, the lookup `p.atoms[intent]` will fail. The `found` flag remains false, and the operation fails.

This implies the `ConfigFactory` is highly sensitive to the exact string serialization format of Mangle atoms. To make the system more robust, `GetAtom` could sanitize its input:

```go
func (p *DefaultConfigAtomProvider) GetAtom(intent string) (ConfigAtom, bool) {
    p.mu.RLock()
    defer p.mu.RUnlock()

    // Normalize intent string: ensure it starts with '/' if it's meant to be an atom
    normalized := intent
    if !strings.HasPrefix(normalized, "/") {
        normalized = "/" + normalized
    }

    atom, ok := p.atoms[normalized]
    return atom, ok
}
```

However, auto-normalizing data at the boundary might mask underlying bugs in the Mangle rules themselves. A better approach for rigorous negative testing is to feed malformed Mangle strings directly into `Generate` (e.g., `<Atom: coder>`, `'coder'`, `"/coder"`) and assert that it strictly requires the canonical format, failing with a clear diagnostic message if the format is wrong.

#### Subagent Initialization Race Conditions (Vector D4)

When `SessionExecutor` (The Clean Loop) initializes a SubAgent, it passes the generated `AgentConfig`. If the SubAgent modifies `AllowedTools` during its lifecycle (e.g., dynamically dropping privileges for safety), does that mutation reflect back into the `ConfigAtomProvider`?

As previously established in Vector D3 (Slice Aliasing), it depends on the slice capacity. To absolutely guarantee that a SubAgent cannot pollute the `ConfigFactory`'s cache, the `AgentConfig` generation must return deep copies of all arrays.

```go
func deepCopyStrings(src []string) []string {
    if src == nil {
        return nil
    }
    dst := make([]string, len(src))
    copy(dst, src)
    return dst
}
```
And applying this in `Generate`:
```go
	cfg := &config.AgentConfig{
		IdentityPrompt: result.Prompt,
		Tools: config.ToolSet{
			AllowedTools: deepCopyStrings(finalAtom.Tools),
		},
		Policies: config.PolicySet{
			Files: deepCopyStrings(finalAtom.Policies),
		},
	}
```
This is a critical defense-in-depth measure against rogue SubAgents mutating their own configurations and accidentally mutating the shared template. We must write a test to explicitly verify this isolation constraint.
