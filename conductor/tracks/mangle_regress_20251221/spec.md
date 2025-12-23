# Track Spec: Mangle Logic Validation and Regression Suite

## Background
codeNERD relies on a neuro-symbolic architecture where the "Executive" is a deterministic logic kernel powered by Google Mangle. As the complexity of the policy rules (Mangle logic) increases, the risk of logic regressions, unintended side effects, and safety bypasses grows. Currently, logic validation is partially covered by Go unit tests, but there is no dedicated, high-level regression suite that validates complex multi-strata logic chains or constitutional safety invariants across the entire policy set.

## Objectives
- Create a specialized testing harness for Mangle `.mg` files that can be invoked from Go tests.
- Implement a suite of "Logic Assertions" to verify that specific intent/state combinations derive the correct `next_action`.
- Validate "Constitutional Invariants" (e.g., ensuring `permitted` is NEVER derived for specific dangerous actions).
- Integrate this suite into the standard project testing workflow (`go test ./...`).

## Technical Requirements
- **Harness:** A Go package (likely in `internal/mangle/testing`) that loads Mangle programs, asserts facts, and verifies derived facts.
- **Declarative Tests:** Support for defining logic tests as facts (e.g., `test_case(ID, ContextAtoms, ExpectedAction)`).
- **Adversarial Integration:** Link the harness to the Nemesis/PanicMaker components to verify that discovered vulnerabilities are permanently mitigated.

## Success Criteria
- [ ] Mangle logic changes can be verified with a single command.
- [ ] 100% coverage of core `permitted` logic rules.
- [ ] At least 10 regression tests for common "failure modes" (e.g., forgotten sender, nil deref).
- [ ] Automated detection of Mangle syntax errors during the build/test phase.
