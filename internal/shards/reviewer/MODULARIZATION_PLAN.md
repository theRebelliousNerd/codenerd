# Reviewer Shard Modularization Plan

## Current State
The `reviewer.go` file is 2,056 lines, which exceeds the recommended 1,500-line threshold for maintainability.

## Proposed Structure

### File Breakdown

1. **reviewer.go** (~350 lines) - Core shard implementation
   - ReviewerShard struct definition
   - Execute method
   - Dependency injection methods
   - Shard interface implementation
   - Configuration types

2. **reviewer_task.go** (~300 lines) - Task parsing and file operations
   - parseTask function
   - readFile, shouldIgnore, detectLanguage helpers
   - parseDiffFiles
   - extractDiffInfo
   - File path manipulation utilities

3. **reviewer_reviews.go** (~450 lines) - Review type implementations
   - reviewFiles (full review with holographic context)
   - securityScan
   - styleCheck
   - complexityAnalysis
   - Helper functions: calculateOverallSeverity, shouldBlockCommit, generateSummary

4. **reviewer_diff.go** (~350 lines) - Diff review functionality
   - reviewDiff function
   - Diff parsing logic
   - Git integration via VirtualStore

5. **reviewer_neurosymbolic.go** (~400 lines) - Neuro-symbolic verification pipeline
   - executeNeuroSymbolicReview (7-step pipeline)
   - shouldUseNeuroSymbolic
   - assertModifiedFunctionFacts
   - extractAndAssertDataFlowFacts
   - generateNeuroSymbolicSummary
   - formatNeuroSymbolicResult
   - NeuroSymbolicConfig and NeuroSymbolicResult types

6. **reviewer_analysis.go** (~300 lines) - File analysis methods
   - analyzeFile, analyzeFileWithDeps, analyzeFileWithHolographic
   - analyzeArchitecture (holographic view)
   - assertFileFacts
   - filterFindingsWithMangle
   - persistFindings
   - Helper functions for Mangle integration

## Benefits

1. **Improved Maintainability**: Smaller files are easier to navigate and understand
2. **Clear Separation of Concerns**: Each file has a single, well-defined purpose
3. **Better Testability**: Individual components can be tested in isolation
4. **Easier Code Review**: Changes are localized to specific functional areas
5. **Reduced Cognitive Load**: Developers can focus on one aspect at a time

## Implementation Notes

- All files remain in the `reviewer` package
- No changes to public APIs or interfaces
- Existing functionality is preserved
- Build and test after modularization to ensure correctness

## File Sizes (Current)

- reviewer.go: 2,056 lines
- autopoiesis.go: 20,900 bytes
- llm.go: 42,646 bytes (needs modularization too)
- hypotheses.go: 21,490 bytes
- impact.go: 17,596 bytes

## Next Steps

1. Extract task parsing and file operations to reviewer_task.go
2. Extract review type implementations to reviewer_reviews.go
3. Extract diff review to reviewer_diff.go
4. Extract neuro-symbolic pipeline to reviewer_neurosymbolic.go
5. Extract analysis methods to reviewer_analysis.go
6. Update reviewer.go to keep only core shard implementation
7. Run full test suite to verify correctness
8. Build and test the complete system
