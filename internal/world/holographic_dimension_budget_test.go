package world

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"codenerd/internal/config"
	working "codenerd/internal/context"
	"codenerd/internal/tools/codedom"
)

// The renderer passes the same atoms the policy keys on. A typo withholds
// the block, because the policy derives nothing for an unknown atom.
func TestHolographicDimensionAtomsMatchThePolicy(t *testing.T) {
	pairs := [][2]string{
		{holoCallers, working.HolographicDimCallers},
		{holoSignatures, working.HolographicDimSignatures},
		{holoTypes, working.HolographicDimTypes},
		{holoImporters, working.HolographicDimImporters},
		{holoOutline, working.HolographicDimOutline},
		{holoOutlineSignature, working.HolographicDimOutlineSignature},
	}
	for _, pair := range pairs {
		if pair[0] != pair[1] {
			t.Errorf("renderer atom %q, policy atom %q", pair[0], pair[1])
		}
	}
}

func writeFuncs(t *testing.T, n int) (dir, path string) {
	t.Helper()
	dir = t.TempDir()
	path = filepath.Join(dir, "s.go")
	var b strings.Builder
	b.WriteString("package p\n")
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, "\nfunc Fn%02d() {}\n", i)
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir, path
}

// Three signatures at 16 bytes need 48 of a 100-byte allowance (1000 bytes
// at a 10% share), so the pool renders whole and names no remainder.
func TestBudgetedSignatures_FewRenderWhole(t *testing.T) {
	dir, path := writeFuncs(t, 3)
	const wantLine = "- `func Fn00()`\n"
	if len(wantLine) != 16 {
		t.Fatalf("hand-computed signature line is %d bytes, want 16", len(wantLine))
	}
	section, seen := budgetedRender(t, NewHolographicProvider(nil, dir), path, func(c *config.WorkingConfig) {
		c.HolographicSignaturesSharePercent = 10
	}, 1000)
	m := dimensionMeasurement(t, seen, holoSignatures)
	if m.total != 3 || m.avg != len(wantLine) {
		t.Fatalf("measured signatures = (%d, %d bytes), want (3, %d)", m.total, m.avg, len(wantLine))
	}
	if got := strings.Count(section, "- `func "); got != 3 {
		t.Fatalf("rendered %d signatures, want all 3:\n%s", got, section)
	}
	if strings.Contains(section, "more exported") {
		t.Fatalf("a pool that fits states a remainder:\n%s", section)
	}
}

// Forty signatures at 16 bytes need 640 of a 100-byte allowance, so 100/16 = 6
// render (Fn00..Fn05, name order) and 34 remain. package_outline reads them.
func TestBudgetedSignatures_SmallBudgetRendersDerivedN(t *testing.T) {
	dir, path := writeFuncs(t, 40)
	section, seen := budgetedRender(t, NewHolographicProvider(nil, dir), path, func(c *config.WorkingConfig) {
		c.HolographicSignaturesSharePercent = 10
	}, 1000)
	m := dimensionMeasurement(t, seen, holoSignatures)
	if m.total != 40 || m.avg != 16 {
		t.Fatalf("measured signatures = (%d, %d bytes), want (40, 16)", m.total, m.avg)
	}
	if got := strings.Count(section, "- `func "); got != 6 {
		t.Fatalf("rendered %d signatures, want the derived 6:\n%s", got, section)
	}
	if !strings.Contains(section, "- `func Fn00()`") || !strings.Contains(section, "- `func Fn05()`") {
		t.Fatalf("the top of the name order must render:\n%s", section)
	}
	if strings.Contains(section, "- `func Fn06()`") {
		t.Fatalf("Fn06 renders past the derived count:\n%s", section)
	}
	if !strings.Contains(section, "and 34 more exported in package `p`") || !strings.Contains(section, "`package_outline`") {
		t.Fatalf("missing the true remainder and package_outline:\n%s", section)
	}
}

// A decider failure withholds the signature block behind its honest remainder
// instead of guessing a count.
func TestBudgetedSignatures_DeciderFailureWithholdsBehindRemainder(t *testing.T) {
	dir, path := writeFuncs(t, 5)
	section := NewHolographicProvider(nil, dir).PromptSectionWithBudget(context.Background(), path, 1000,
		func(context.Context, string, string, int, int, int) (int, error) {
			return 0, fmt.Errorf("engine unavailable")
		})
	if got := strings.Count(section, "- `func "); got != 0 {
		t.Fatalf("a failed decision rendered %d signature lines:\n%s", got, section)
	}
	if !strings.Contains(section, "and 5 more exported in package `p`") || !strings.Contains(section, "`package_outline`") {
		t.Fatalf("missing the withholding remainder:\n%s", section)
	}
}

func writeTypes(t *testing.T, n int) (dir, path string) {
	t.Helper()
	dir = t.TempDir()
	path = filepath.Join(dir, "s.go")
	var b strings.Builder
	b.WriteString("package p\n")
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, "\ntype T%02d struct{}\n", i)
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir, path
}

// Two types at 24 bytes need 48 of a 100-byte allowance, so both render.
func TestBudgetedTypes_FewRenderWhole(t *testing.T) {
	dir, path := writeTypes(t, 2)
	wantLine := "- `type T00` \u2014 struct\n"
	if len(wantLine) != 24 {
		t.Fatalf("hand-computed type line is %d bytes, want 24", len(wantLine))
	}
	section, seen := budgetedRender(t, NewHolographicProvider(nil, dir), path, func(c *config.WorkingConfig) {
		c.HolographicTypesSharePercent = 10
	}, 1000)
	m := dimensionMeasurement(t, seen, holoTypes)
	if m.total != 2 || m.avg != len(wantLine) {
		t.Fatalf("measured types = (%d, %d bytes), want (2, %d)", m.total, m.avg, len(wantLine))
	}
	if got := strings.Count(section, "- `type "); got != 2 {
		t.Fatalf("rendered %d types, want both:\n%s", got, section)
	}
	if strings.Contains(section, "more in package") || strings.Contains(section, "more among") {
		t.Fatalf("a pool that fits states a remainder:\n%s", section)
	}
}

// Twenty types at 24 bytes need 480 of a 100-byte allowance, so 100/24 = 4
// render and 16 remain. package_outline reads them.
func TestBudgetedTypes_SmallBudgetRendersDerivedN(t *testing.T) {
	dir, path := writeTypes(t, 20)
	section, seen := budgetedRender(t, NewHolographicProvider(nil, dir), path, func(c *config.WorkingConfig) {
		c.HolographicTypesSharePercent = 10
	}, 1000)
	m := dimensionMeasurement(t, seen, holoTypes)
	if m.total != 20 || m.avg != 24 {
		t.Fatalf("measured types = (%d, %d bytes), want (20, 24)", m.total, m.avg)
	}
	if got := strings.Count(section, "- `type "); got != 4 {
		t.Fatalf("rendered %d types, want the derived 4:\n%s", got, section)
	}
	if !strings.Contains(section, "- `type T00`") || !strings.Contains(section, "- `type T03`") {
		t.Fatalf("the top of the name order must render:\n%s", section)
	}
	if strings.Contains(section, "- `type T04`") {
		t.Fatalf("T04 renders past the derived count:\n%s", section)
	}
	if !strings.Contains(section, "and 16 more in package `p`") || !strings.Contains(section, "`package_outline`") {
		t.Fatalf("missing the true remainder and package_outline:\n%s", section)
	}
}

// Two short funcs under a 100-byte outline allowance render whole.
func TestBudgetedOutline_FewRenderWhole(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "s.go")
	src := "package p\n\nfunc F00() {}\n\nfunc F01() {}\n"
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	section, seen := budgetedRender(t, NewHolographicProvider(nil, dir), path, func(c *config.WorkingConfig) {
		c.HolographicOutlineSharePercent = 10
	}, 1000)
	m := dimensionMeasurement(t, seen, holoOutline)
	if m.total != 2 {
		t.Fatalf("measured outline entries = %d, want 2", m.total)
	}
	if m.avg*m.total > 100 {
		t.Fatalf("fixture does not fit the 100-byte allowance (mean %d)", m.avg)
	}
	if !strings.Contains(section, "func F00()") || !strings.Contains(section, "func F01()") {
		t.Fatalf("both entries must render:\n%s", section)
	}
	if strings.Contains(section, "more declarations") {
		t.Fatalf("a pool that fits states a remainder:\n%s", section)
	}
}

// outlinePool is the uncut entry lines the count decision measures, and the
// longest signature among them. The character tests derive the allowance
// from these bytes the same way the renderer does, then check the branch.
func outlinePool(t *testing.T, src string) (dir, path string, avg, n, longRunes int) {
	t.Helper()
	dir = t.TempDir()
	path = filepath.Join(dir, "o.go")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	elements := codedom.ElementsFromSource(path, src)
	if len(elements) == 0 {
		t.Fatal("fixture declared nothing")
	}
	sum := 0
	for _, el := range elements {
		sum += len(outlineEntryLine(el, outlineLabel(el), 0))
		if runes := utf8.RuneCountInString(outlineLabel(el)); runes > longRunes {
			longRunes = runes
		}
	}
	n = len(elements)
	avg = sum / n
	if avg < 1 {
		t.Fatal("outline lines measured empty")
	}
	return dir, path, avg, n, longRunes
}

// The floor raises the per-entry cut when the outline allowance divided by
// the entries kept falls under it. The long signature is cut to the floor,
// and the line names get_element.
func TestBudgetedOutline_SignatureFloorRaisesTheCut(t *testing.T) {
	long := strings.Repeat("a", 200)
	src := "package p\n\nfunc Long(" + long + " int) {}\n\nfunc F00() {}\n"
	dir, path, avg, n, longRunes := outlinePool(t, src)
	// Both entries fit: allowance = avg * n, share 10, so the budget is
	// allowance * 10. per = allowance / n = avg, and the floor is avg+5.
	allowance := avg * n
	const share = 10
	budget := allowance * (100 / share)
	floor := avg + 5
	if floor <= avg {
		t.Fatal("fixture does not put the floor above the per-entry allowance")
	}
	if floor >= longRunes {
		t.Fatalf("floor %d does not cut the long signature (%d runes)", floor, longRunes)
	}
	section, seen := budgetedRender(t, NewHolographicProvider(nil, dir), path, func(c *config.WorkingConfig) {
		c.HolographicOutlineSharePercent = share
		c.HolographicOutlineSignatureFloor = floor
	}, budget)
	m := dimensionMeasurement(t, seen, holoOutline)
	if m.total != n || m.avg != avg {
		t.Fatalf("measured outline = (%d, %d), want (%d, %d)", m.total, m.avg, n, avg)
	}
	sig := dimensionMeasurement(t, seen, holoOutlineSignature)
	if sig.total != n || sig.avg != 1 || sig.budget != budget {
		t.Fatalf("signature decision saw (%d entries, mean %d, budget %d), want (%d, 1, %d)", sig.total, sig.avg, sig.budget, n, budget)
	}
	rest := longRunes - floor
	want := fmt.Sprintf("%d more characters; `get_element` returns the signature whole", rest)
	if !strings.Contains(section, want) {
		t.Fatalf("missing the floor cut %q:\n%s", want, section)
	}
	if !strings.Contains(section, "func F00()") || strings.Count(section, "more characters") != 1 {
		t.Fatalf("the short signature was cut:\n%s", section)
	}
}

// The division wins when the per-entry allowance is at least the floor. The
// long signature is cut to that allowance, not to the floor and not to the
// old 100-rune constant.
func TestBudgetedOutline_SignatureDivisionCutsToTheAllowance(t *testing.T) {
	long := strings.Repeat("a", 200)
	src := "package p\n\nfunc Long(" + long + " int) {}\n\nfunc F00() {}\n"
	dir, path, avg, n, longRunes := outlinePool(t, src)
	allowance := avg * n
	const share = 10
	budget := allowance * (100 / share)
	const floor = 40
	if avg <= floor {
		t.Fatalf("per-entry allowance %d is not above the floor %d; this fixture does not exercise division", avg, floor)
	}
	if longRunes <= avg {
		t.Fatalf("division to %d runes does not cut the long signature (%d runes)", avg, longRunes)
	}
	section, seen := budgetedRender(t, NewHolographicProvider(nil, dir), path, func(c *config.WorkingConfig) {
		c.HolographicOutlineSharePercent = share
		c.HolographicOutlineSignatureFloor = floor
	}, budget)
	sig := dimensionMeasurement(t, seen, holoOutlineSignature)
	if sig.total != n {
		t.Fatalf("signature decision saw %d entries, want %d", sig.total, n)
	}
	rest := longRunes - avg
	want := fmt.Sprintf("%d more characters; `get_element` returns the signature whole", rest)
	if !strings.Contains(section, want) {
		t.Fatalf("missing the division cut %q:\n%s", want, section)
	}
	if strings.Count(section, "more characters") != 1 {
		t.Fatalf("expected one cut signature:\n%s", section)
	}
}
