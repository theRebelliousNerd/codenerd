# reviewer/ - Code Review Enhancement Atoms

Comprehensive code review methodology atoms for the ReviewerShard.

## Files

| File | Purpose |
|------|---------|
| `enhancement_interrogator.yaml` | Socratic questioning for code improvements |
| `methodology_systematic_review.yaml` | Four-pass review process, PR size guidance, core checklists |
| `methodology_correctness.yaml` | Logic errors, off-by-one, null handling, boundaries, error handling |
| `methodology_security.yaml` | OWASP Top 10, injection, auth, data protection, input validation |
| `methodology_performance.yaml` | Algorithm complexity, N+1 queries, memory, caching, async |
| `methodology_maintainability.yaml` | Readability, coupling/cohesion, code smells, documentation |
| `methodology_test_coverage.yaml` | Test quality, mutation testing, edge cases, coverage gaps |
| `methodology_concurrency.yaml` | Race conditions, synchronization, deadlocks, resource cleanup |
| `methodology_ai_code_review.yaml` | AI hallucination detection, API verification, integration |
| `methodology_feedback.yaml` | Severity classification, constructive feedback, phrasing |

## Atom Count

- **Total Atoms:** 38 methodology atoms
- **Categories Covered:** 10 review dimensions

## Review Categories

### 1. Systematic Process
- Four-pass methodology (intent, correctness, edge cases, style)
- PR size calibration
- Core checklists
- Architecture assessment

### 2. Correctness
- Logic errors
- Off-by-one errors
- Null/nil handling
- Boundary conditions
- Error handling completeness

### 3. Security (OWASP)
- Injection (SQL, command, XSS)
- Authentication/authorization
- Data protection
- Input validation
- Dependency security

### 4. Performance
- Algorithm complexity (Big O)
- N+1 query detection
- Memory usage
- Caching strategies
- Async performance

### 5. Maintainability
- Readability and naming
- Coupling and cohesion
- Code smell detection
- Documentation quality

### 6. Test Coverage
- Coverage metrics
- Mutation testing concepts
- Test quality assessment
- Edge case coverage
- Test anti-patterns

### 7. Concurrency
- Race condition detection
- Synchronization primitives
- Deadlock prevention
- Resource cleanup
- Thread safety analysis

### 8. AI Code Review
- Hallucination detection
- API/package verification
- Logic verification
- Security review for AI code
- Integration review

### 9. Feedback
- Severity classification
- Conventional comments
- Feedback phrasing
- Structured output

## Selection

Selected via `shard_types: ["/reviewer"]` and `intent_verbs: ["/review", "/audit", "/check", "/analyze", "/inspect"]`.

## Dependencies

Main reviewer identity in `identity/reviewer.yaml` provides:
- `identity/reviewer/mission`
- `identity/reviewer/constraints`
- `identity/reviewer/focus_areas`

Hallucination guards in `hallucination/reviewer_hallucinations.yaml` provide:
- `hallucination/reviewer/false_positive`
- `hallucination/reviewer/outdated_pattern`
- `hallucination/reviewer/style_as_bug`
- `hallucination/reviewer/invented_vulnerability`
- `hallucination/reviewer/missing_context`

## Usage

These methodology atoms are selected alongside identity atoms when the JIT compiler encounters review intents. Priority ordering ensures foundational atoms load first, with specialized methodology atoms layered on top.

### Atom Hierarchy

1. **Identity atoms** (priority 100): Core mission and constraints
2. **Methodology fundamentals** (priority 95): Process foundations
3. **Specific methodologies** (priority 80-92): Domain-specific guidance
4. **Examples and anti-patterns** (priority 75-78): Supplementary guidance

## Research Sources

These atoms were developed from comprehensive research including:

- [OWASP Code Review Guide](https://owasp.org/www-project-code-review-guide/)
- [OWASP Secure Coding Practices](https://owasp.org/www-project-secure-coding-practices-quick-reference-guide/)
- [Google Code Coverage Best Practices](https://testing.googleblog.com/2020/08/code-coverage-best-practices.html)
- [Microsoft 30 Code Review Best Practices](https://www.michaelagreiler.com/code-review-best-practices/)
- [Netlify Feedback Ladders](https://www.netlify.com/blog/2020/03/05/feedback-ladders-how-we-encode-code-reviews-at-netlify/)
- [Korbit AI Hallucination Detection](https://www.korbit.ai/post/eliminating-hallucinations-in-ai-code-reviews-2)
- [Conventional Comments](https://conventionalcomments.org/)
