package core

import "testing"

// X = Y unifies two variables and grounds neither. The engine therefore runs
// a following negation while X is still a variable. These are the minimized
// stores where planning that negation as if X were ground diverges.

func TestDRedNegationAfterVacuousEqualityMatchesTheFixpoint(t *testing.T) {
	k := newPolicyKernel(t, newRealKernelBase(), `
		Decl fv_p(Y).
		Decl fv_r(X).
		Decl fv_q(Z).
		Decl fv_h(Z).
		fv_h(Z) :- X = Y, fv_p(Y), !fv_r(X), fv_q(Z).
	`)
	axApply(t, k, "the grounding atom and a different block", "", func() {
		axAssert(t, k, axFactOf("fv_p", MangleAtom("/a")), axFactOf("fv_r", MangleAtom("/b")))
	})
	axApply(t, k, "the head argument arrives", "", func() {
		axAssert(t, k, axFactOf("fv_q", MangleAtom("/c")))
	})
	axApply(t, k, "the block hits the joined id", "", func() {
		axAssert(t, k, axFactOf("fv_r", MangleAtom("/a")))
	})
	axApply(t, k, "the block leaves the joined id", "", func() {
		axRetract(t, k, axFactOf("fv_r", MangleAtom("/a")))
	})
}

func TestDRedSecondNegationStaysBehindTheGroundingAtom(t *testing.T) {
	k := newPolicyKernel(t, newRealKernelBase(), `
		Decl fs_p(Y).
		Decl fs_r(X).
		Decl fs_s(X).
		Decl fs_q(Z).
		Decl fs_h(Z).
		fs_h(Z) :- X = Y, !fs_r(X), fs_p(Y), !fs_s(X), fs_q(Z).
	`)
	axApply(t, k, "no row of the first negation", "", func() {
		axAssert(t, k,
			axFactOf("fs_p", MangleAtom("/a")),
			axFactOf("fs_s", MangleAtom("/b")),
			axFactOf("fs_q", MangleAtom("/c")))
	})
	axApply(t, k, "the second negation hits the joined id", "", func() {
		axAssert(t, k, axFactOf("fs_s", MangleAtom("/a")))
	})
	axApply(t, k, "that hit leaves", "", func() {
		axRetract(t, k, axFactOf("fs_s", MangleAtom("/a")))
	})
}

func TestDRedEqualityBeforeNegationMatchesTheFixpoint(t *testing.T) {
	k := newPolicyKernel(t, newRealKernelBase(), `
		Decl fl_p(Y).
		Decl fl_r(X).
		Decl fl_q(Z).
		Decl fl_h(Z).
		fl_h(Z) :- X = Y, !fl_r(X), fl_p(Y), fl_q(Z).
	`)
	axApply(t, k, "the block is a different id", "", func() {
		axAssert(t, k,
			axFactOf("fl_p", MangleAtom("/a")),
			axFactOf("fl_r", MangleAtom("/b")),
			axFactOf("fl_q", MangleAtom("/c")))
	})
	axApply(t, k, "the block hits the joined id", "", func() {
		axAssert(t, k, axFactOf("fl_r", MangleAtom("/a")))
	})
	axApply(t, k, "the joined id leaves", "", func() {
		axRetract(t, k, axFactOf("fl_p", MangleAtom("/a")))
	})
}
