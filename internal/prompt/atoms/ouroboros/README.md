# Ouroboros Prompt Atoms

This directory contains prompt atoms for the Ouroboros/Autopoiesis system - codeNERD's self-modification engine.

## Directory Structure

```text
ouroboros/
├── tool_generator_identity.yaml  # Identity and mission for tool generator
├── detection.yaml                 # Tool need detection guidance
├── specification.yaml             # Tool code generation guidance
├── safety_check.yaml              # Safety requirements and checks
├── refinement.yaml                # Tool improvement guidance
└── README.md                      # This file
```

## Atom Files

### tool_generator_identity.yaml
Defines the identity and core mission for the Ouroboros tool generator.

**Atoms:**
- `ouroboros/identity/mission` (priority: 80, mandatory) - Core mission and execution context
- `ouroboros/identity/code_standards` (priority: 70, mandatory) - Go idioms and structure requirements

**Stages:** Detection, Specification, Refinement

### detection.yaml
Guides the detection of when new tools are needed.

**Atoms:**
- `ouroboros/detection/capability_gap_analysis` (priority: 70) - How to identify capability gaps
- `ouroboros/detection/confidence_calibration` (priority: 60) - How to set confidence scores

**Stages:** Detection

**Code Mapping:**
- Maps to `DetectToolNeed()` in `toolgen.go`
- Maps to `refineToolNeedWithLLM()` in `toolgen.go`

### specification.yaml
Guides the actual generation of tool code.

**Atoms:**
- `ouroboros/specification/generation_principles` (priority: 70, mandatory) - Core generation principles
- `ouroboros/specification/function_signature` (priority: 80, mandatory) - Required function signatures
- `ouroboros/specification/template_selection` (priority: 60) - Template-based generation patterns
- `ouroboros/specification/completeness_requirement` (priority: 90, mandatory) - Output completeness mandate

**Stages:** Specification

**Code Mapping:**
- Maps to `generateToolCode()` in `toolgen.go` (lines 456-491)
- Maps to system prompt starting at line 456:
  ```go
  systemPrompt := `You are a Go code generator for the codeNERD agent system.
  Generate clean, idiomatic Go code that follows these conventions:
  - Use standard library where possible
  - Include proper error handling
  - Add clear comments
  - Make functions testable
  - Follow Go naming conventions`
  ```

### safety_check.yaml
Defines safety requirements and forbidden patterns.

**Atoms:**
- `ouroboros/safety/critical_requirements` (priority: 100, mandatory) - Forbidden imports and patterns
- `ouroboros/safety/allowed_packages` (priority: 80, mandatory) - Explicitly allowed packages
- `ouroboros/safety/error_handling_mandates` (priority: 90, mandatory) - Error handling requirements
- `ouroboros/safety/static_analysis_checks` (priority: 70) - Post-generation analysis

**Stages:** Specification, Refinement, Safety Check

**Code Mapping:**
- Maps to `regenerateToolCodeWithFeedback()` in `toolgen.go` (lines 392-452)
- Maps to safety system prompt starting at line 399:
  ```go
  systemPrompt := `You are a Go code generator for the codeNERD agent system.
  Your previous code had safety violations. You must fix these issues.

  CRITICAL SAFETY REQUIREMENTS:
  - Do NOT use unsafe imports (os/exec, syscall, unsafe, plugin, runtime/cgo)
  - Do NOT use panic() - return errors instead
  - If using goroutines, always pass a cancelable context
  - Only use explicitly allowed packages (fmt, strings, bytes, context, encoding/*, errors, etc.)
  - Prefer error returns over panic
  - Use context.Context for cancellation`
  ```
- Maps to `SafetyChecker` in `ouroboros.go`

### refinement.yaml
Guides the improvement of tools based on execution feedback.

**Atoms:**
- `ouroboros/refinement/feedback_analysis` (priority: 70, mandatory) - Feedback-driven refinement focus
- `ouroboros/refinement/pattern_detection` (priority: 80, mandatory) - Recurring pattern detection
- `ouroboros/refinement/improvement_strategies` (priority: 70) - Specific improvement patterns
- `ouroboros/refinement/safety_violation_fixes` (priority: 95, mandatory) - How to fix safety violations
- `ouroboros/refinement/output_format` (priority: 60) - Expected output format

**Stages:** Refinement

**Code Mapping:**
- Maps to `Refine()` in `feedback.go` (lines 108-171)
- Maps to `refinementSystemPrompt` starting at line 237:
  ```go
  var refinementSystemPrompt = `You are a Go code optimizer specializing in improving tool reliability and completeness.

  When improving tools, focus on:
  1. PAGINATION - Always fetch all pages, not just the first
  2. LIMITS - Use maximum allowed limits, not defaults
  3. RETRIES - Add exponential backoff for transient failures
  4. ERROR HANDLING - Handle all error cases gracefully
  5. VALIDATION - Validate inputs and outputs

  Common anti-patterns to fix:
  - Only fetching first page of paginated results
  - Using default limit (10) instead of max (100+)
  - No retry logic for rate limits or network errors
  - Missing error handling for edge cases

  Generate clean, idiomatic Go code with proper error handling.`
  ```

## Ouroboros Stages

The Ouroboros Loop has the following stages:

1. **Detection** (`/detection`) - Detect missing capability
2. **Specification** (`/specification`) - Generate tool code via LLM
3. **Safety Check** (`/safety_check`) - Verify code safety
4. **Compilation** (`/compilation`) - Compile to binary
5. **Registration** (`/registration`) - Register in runtime
6. **Execution** (`/execution`) - Run tool with input
7. **Refinement** (`/refinement`) - Improve based on feedback

Each atom specifies which stages it applies to via the `ouroboros_stages` field.

## Priority Levels

- **100**: Critical safety requirements (must never be violated)
- **90-95**: Mandatory requirements (core to correctness)
- **80**: High-priority guidance (identity, standards)
- **70**: Important methodology (how to approach the task)
- **60**: Optional enhancements (nice-to-have guidance)

## Usage

The JIT prompt compiler will:
1. Select atoms based on the current Ouroboros stage
2. Filter by `ouroboros_stages` field
3. Sort by `priority` (highest first)
4. Inject mandatory atoms first
5. Resolve dependencies via `depends_on`
6. Compile into a unified system prompt

Example for Specification stage:
```text
Stage: /specification
Selected atoms:
  - ouroboros/specification/completeness_requirement (90, mandatory)
  - ouroboros/specification/function_signature (80, mandatory)
  - ouroboros/safety/critical_requirements (100, mandatory)
  - ouroboros/identity/mission (80, mandatory)
  - ouroboros/specification/generation_principles (70, mandatory)
  - ouroboros/identity/code_standards (70, mandatory)
  - ouroboros/specification/template_selection (60, optional)
```

## Integration Points

### In `toolgen.go`:
- `generateToolCode()` - Uses specification atoms
- `regenerateToolCodeWithFeedback()` - Uses safety + refinement atoms
- `refineToolNeedWithLLM()` - Uses detection atoms

### In `feedback.go`:
- `Refine()` - Uses refinement atoms
- `buildRefinementPrompt()` - Constructs prompt from refinement atoms

### In `ouroboros.go`:
- `Execute()` - Orchestrates all stages
- `SafetyChecker.Check()` - Uses safety check atoms

## Atom Dependencies

```text
ouroboros/identity/mission
  └── ouroboros/identity/code_standards (depends_on: mission)
      ├── ouroboros/specification/generation_principles
      │   └── ouroboros/specification/function_signature (depends_on: generation_principles)
      │   └── ouroboros/specification/template_selection
      └── ouroboros/safety/critical_requirements
          ├── ouroboros/safety/allowed_packages (depends_on: critical_requirements)
          └── ouroboros/safety/error_handling_mandates (depends_on: critical_requirements)

ouroboros/refinement/feedback_analysis
  └── ouroboros/refinement/pattern_detection (depends_on: feedback_analysis)
      └── ouroboros/refinement/improvement_strategies (depends_on: pattern_detection)
```

## Testing

To verify atom correctness:
1. Check YAML syntax: `yamllint ouroboros/*.yaml`
2. Verify all stages are valid: `/detection`, `/specification`, `/safety_check`, `/refinement`
3. Ensure priority levels are appropriate
4. Validate dependency chains don't have cycles

## Future Enhancements

Potential additions:
- `quality_expectations.yaml` - Quality profile generation guidance
- `test_generation.yaml` - Test code generation guidance
- `logging_injection.yaml` - Mandatory logging guidance
- `reasoning_trace.yaml` - Reasoning capture guidance


> *[Archived & Reviewed by The Librarian on 2026-01-25]*