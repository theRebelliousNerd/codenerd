package core

import (
	"errors"
	"strings"
	"testing"
	"time"

	"codenerd/internal/types"

	"codeberg.org/TauCeti/mangle-go/ast"
	"codeberg.org/TauCeti/mangle-go/builtin"
	"github.com/stretchr/testify/require"
)

func admissionKernel(t *testing.T, decls string) *RealKernel {
	t.Helper()
	k := setupMockKernel(t)
	k.AppendPolicy(decls)
	require.NoError(t, k.Evaluate())
	return k
}

// Bound alternatives are tuples, not per-column unions: (success, witness) or
// (failure, diagnostic) must not admit (success, diagnostic).
func TestAdmission_AlternativeTuplesAreNotFlattened(t *testing.T) {
	t.Parallel()
	k := admissionKernel(t, `Decl evidence_row(Status, Payload) bound [/name, /number] bound [/string, /string].`)

	require.NoError(t, k.Assert(Fact{Predicate: "evidence_row", Args: []any{MangleAtom("/success"), 7}}), "first alternative")
	require.NoError(t, k.Assert(Fact{Predicate: "evidence_row", Args: []any{"failed run", "diagnostic"}}), "second alternative")

	err := k.Assert(Fact{Predicate: "evidence_row", Args: []any{MangleAtom("/success"), "diagnostic"}})
	require.Error(t, err, "a crossed tuple must be rejected")
	require.Contains(t, err.Error(), "no declared bound alternative accepts it")
	require.Contains(t, err.Error(), "alternative 0: arg 1")
	require.Contains(t, err.Error(), "alternative 1: arg 0")

	rows, err := k.Query("evidence_row")
	require.NoError(t, err)
	require.Len(t, rows, 2)
}

// A plain Go string ToAtom read as a name ("/tmp") is a string to a /string
// slot. A MangleAtom there meant a name and is refused.
func TestAdmission_PlainStringInStringSlotStaysAString(t *testing.T) {
	t.Parallel()
	k := admissionKernel(t, `Decl path_seen(Path) bound [/string].
path_is_tmp() :- path_seen("/tmp").
Decl path_is_tmp() bound [].`)

	require.NoError(t, k.Assert(Fact{Predicate: "path_seen", Args: []any{"/tmp"}}))
	rows, err := k.Query("path_is_tmp")
	require.NoError(t, err)
	require.Len(t, rows, 1, "\"/tmp\" must reach the store as the string the Decl declares")

	require.Error(t, k.Assert(Fact{Predicate: "path_seen", Args: []any{MangleAtom("/etc")}}))
}

// A bare Go string is tolerated against a bare /name bound (documented in
// kernel_fact_decl.go), but an explicit MangleString is not.
func TestAdmission_BareStringNameToleranceIsNarrow(t *testing.T) {
	t.Parallel()
	k := admissionKernel(t, `Decl role_of(Who) bound [/name].`)
	require.NoError(t, k.Assert(Fact{Predicate: "role_of", Args: []any{"coder"}}))
	require.Error(t, k.Assert(Fact{Predicate: "role_of", Args: []any{types.MangleString("tester")}}))
	require.Error(t, k.Assert(Fact{Predicate: "role_of", Args: []any{42}}))
}

// Durations and times in a /number slot are their nanosecond counts, not a
// duration constant the int64-only comparisons would abort on.
func TestAdmission_DurationAndTimeNarrowToNumber(t *testing.T) {
	t.Parallel()
	k := admissionKernel(t, `Decl took(Op, Nanos) bound [/string, /number].
slow(Op) :- took(Op, N), N > 1000.
Decl slow(Op) bound [/string].`)
	at := time.Unix(0, 5000)
	require.NoError(t, k.Assert(Fact{Predicate: "took", Args: []any{"build", 2 * time.Microsecond}}))
	require.NoError(t, k.Assert(Fact{Predicate: "took", Args: []any{"stamp", at}}))
	rows, err := k.Query("slow")
	require.NoError(t, err)
	require.Len(t, rows, 2)
}

// Every write path runs the same admission: a transaction used to skip the
// Decl check entirely.
func TestAdmission_TransactionAssertsAreAdmitted(t *testing.T) {
	t.Parallel()
	k := admissionKernel(t, `Decl typed_tx(N) bound [/number].`)
	tx := k.Transaction()
	tx.Assert(Fact{Predicate: "typed_tx", Args: []any{"not a number"}})
	tx.Assert(Fact{Predicate: "typed_tx", Args: []any{3}})
	err := tx.Commit()
	require.Error(t, err)
	require.Contains(t, err.Error(), "arg 0")
	rows, err := k.Query("typed_tx")
	require.NoError(t, err)
	require.Len(t, rows, 1)
}

// The compiled handles must agree with the upstream runtime checker on every
// declared predicate of the production corpus: same verdict for a sample of
// well-typed and ill-typed tuples per Decl.
func TestTypedAdmissionAgreesWithUpstreamTypeChecker(t *testing.T) {
	t.Parallel()
	k, err := NewRealKernel()
	require.NoError(t, err)
	require.NoError(t, k.Evaluate())
	info := k.GetProgramInfo()
	upstream := builtin.NewTypeCheckerFromDesugared(info.Decls)
	samples := []ast.Constant{ast.Number(1), ast.String("s"), ast.Float64(1.5)}
	name, _ := ast.Name("/n")
	samples = append(samples, name)

	k.mu.Lock()
	defer k.mu.Unlock()
	checked := 0
	for sym := range info.Decls {
		if sym.Arity == 0 || sym.Arity > 6 {
			continue
		}
		pa, err := k.admissionForLocked(sym)
		require.NoError(t, err, sym)
		if pa == nil || len(pa.handles) == 0 {
			continue
		}
		for _, s := range samples {
			args := make([]ast.BaseTerm, sym.Arity)
			for i := range args {
				args[i] = s
			}
			atom := ast.Atom{Predicate: sym, Args: args}
			want := upstream.CheckTypeBounds(atom) == nil
			got := pa.check(atom, nil) == nil // nil raw: no bare-string tolerance
			require.Equal(t, want, got, "%s with %v", sym, s)
			checked++
		}
	}
	require.Greater(t, checked, 1000, "the corpus check must cover the declared predicates")
}

// Inclusion: a campaign plan whose dependency names a task the plan does not
// hold is quarantined, with the dangling edge named; a closed, consistent plan
// passes regardless of the order its rows arrive in.
func TestCheckBatchInclusion_CampaignPlanEdges(t *testing.T) {
	t.Parallel()
	k, err := NewRealKernel()
	require.NoError(t, err)

	plan := []Fact{
		{Predicate: "task_dependency", Args: []any{"t2", "t1"}}, // before its tasks: order is not a violation
		{Predicate: "campaign_phase", Args: []any{"p1", "c1", "Build", 1, MangleAtom("/pending"), "default"}},
		{Predicate: "campaign_task", Args: []any{"t1", "p1", "first", MangleAtom("/pending"), MangleAtom("/file_create")}},
		{Predicate: "campaign_task", Args: []any{"t2", "p1", "second", MangleAtom("/pending"), MangleAtom("/file_create")}},
	}
	report, err := k.InclusionReport("test", plan)
	require.NoError(t, err)
	require.Empty(t, report.Violations)
	require.Equal(t, 3, report.Checked)
	require.NoError(t, k.CheckBatchInclusion("test", plan))

	dangling := append(append([]Fact{}, plan...),
		Fact{Predicate: "task_dependency", Args: []any{"t2", "t_missing"}},
		Fact{Predicate: "campaign_task", Args: []any{"t3", "p_missing", "orphan", MangleAtom("/pending"), MangleAtom("/file_create")}},
	)
	err = k.CheckBatchInclusion("campaign c1", dangling)
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInclusionViolation))
	var incl *InclusionError
	require.True(t, errors.As(err, &incl))
	require.Len(t, incl.Report.Violations, 2)
	msg := err.Error()
	require.Contains(t, msg, "campaign c1")
	require.Contains(t, msg, "t_missing")
	require.Contains(t, msg, "p_missing")

	// The check publishes nothing.
	rows, err := k.Query("campaign_task")
	require.NoError(t, err)
	require.Empty(t, rows)
}

// A reference whose domain the batch does not hold is unresolved, not false.
func TestCheckBatchInclusion_OpenDomainIsUnresolved(t *testing.T) {
	t.Parallel()
	k := admissionKernel(t, `Decl known_check(C) bound [/string].
Decl check_result(C, Ok) bound [/string, /name] inclusion [known_check(C)].`)
	batch := []Fact{{Predicate: "check_result", Args: []any{"lint", MangleAtom("/true")}}}
	report, err := k.InclusionReport("probe", batch)
	require.NoError(t, err)
	require.Empty(t, report.Violations)
	require.Len(t, report.Unresolved, 1)
	require.True(t, strings.Contains(report.Unresolved[0].Reason, "not in this batch"), report.Unresolved[0].Reason)
	require.NoError(t, k.CheckBatchInclusion("probe", batch))

	// Once the batch carries the domain, the same row is decided.
	closed := append(batch, Fact{Predicate: "known_check", Args: []any{"vet"}})
	require.ErrorIs(t, k.CheckBatchInclusion("probe", closed), ErrInclusionViolation)
}
