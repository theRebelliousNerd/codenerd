# Machine-checkable spec: the DRed evaluator (internal/core/kernel_eval_dred*.go).
#
# A spec file under Docs/specs/ or .nerd/specs/ holds ground facts of exactly
# three predicates and nothing else (internal/specs.ParseSpecFile refuses a
# rule, a Decl or any other predicate):
#
#   spec_requirement(Id, Text)               one stated obligation
#   requires(Id, CodeRef, Property)          Property: /exists /exported
#                                            /has_test /has_doc_comment
#   requires_calls(Id, CodeRef, OtherRef)    CodeRef calls OtherRef directly
#
# CodeRef is the element's workspace ref, as get_element prints it. The world
# scan measures the code model and the kernel derives
# requirement_violated(Id, CodeRef, Property) from the difference
# (policy/spec_pairing.mg); a requirement on an element that no longer exists is
# violated whatever its property. A call counts only when the index ties the
# callee name to exactly one element.

spec_requirement(/dred_entry, "The kernel has one DRed entry point. It is documented, and one evaluation stages the derived EDB, evaluates strata and re-derives the cone.").
requires(/dred_entry, "internal/core.RealKernel.evaluateDRedLocked", /exists).
requires(/dred_entry, "internal/core.RealKernel.evaluateDRedLocked", /has_doc_comment).
requires_calls(/dred_entry, "internal/core.RealKernel.evaluateDRedLocked", "internal/core.dredRun.stageDerivedEDB").
requires_calls(/dred_entry, "internal/core.RealKernel.evaluateDRedLocked", "internal/core.dredRun.stratum").
requires_calls(/dred_entry, "internal/core.RealKernel.evaluateDRedLocked", "internal/core.RealKernel.rederiveConeLocked").

spec_requirement(/dred_stratum, "A stratum honours the derived-fact limit, re-derives what lost its support, and recomputes aggregates group by group.").
requires_calls(/dred_stratum, "internal/core.dredRun.stratum", "internal/core.dredRun.checkLimit").
requires_calls(/dred_stratum, "internal/core.dredRun.stratum", "internal/core.dredRun.rederive").
requires_calls(/dred_stratum, "internal/core.dredRun.stratum", "internal/core.dredRun.aggregate").
requires_calls(/dred_stratum, "internal/core.dredRun.aggregate", "internal/core.dredRun.recomputeGroup").

spec_requirement(/dred_plan, "The delta plan and the join order are documented and tested: a plan that nothing exercises is a plan nobody trusts.").
requires(/dred_plan, "internal/core.dredIndex.plan", /has_doc_comment).
requires(/dred_plan, "internal/core.dredIndex.plan", /has_test).
requires(/dred_plan, "internal/core.planOrder", /has_doc_comment).
requires(/dred_plan, "internal/core.planOrder", /has_test).
