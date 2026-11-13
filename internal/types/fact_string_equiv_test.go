package types

import (
	"fmt"
	"math"
	"strings"
	"testing"
	"time"
)

// referenceFactString is Fact.String as it was: Sprintf per argument, joined.
func referenceFactString(f Fact) string {
	var args []string
	for _, arg := range f.Args {
		switch v := arg.(type) {
		case MangleAtom:
			args = append(args, string(v))
		case MangleString:
			args = append(args, fmt.Sprintf("%q", v))
		case string:
			// Handle valid Mangle name constants (start with /).
			// NOTE: Many normal strings can start with "/" (e.g., Go comments "//", Unix paths),
			// so we only treat it as a name constant if it parses as one.
			if isValidMangleNameConstant(v) {
				args = append(args, v)
			} else {
				args = append(args, fmt.Sprintf("%q", v))
			}
		case int:
			args = append(args, fmt.Sprintf("%d", v))
		case int64:
			args = append(args, fmt.Sprintf("%d", v))
		case float64:
			// %f, not %v or the AST renderer: mangle-go prints Float64(2.0) as
			// "2", which re-parses as an int64. Keeping the decimal point is
			// what makes a whole float survive a round trip through a .mg file.
			args = append(args, fmt.Sprintf("%f", v))
		case float32:
			args = append(args, fmt.Sprintf("%f", float64(v)))
		case time.Time:
			// Quoted RFC3339Nano matches what ExtractString produces when the
			// same argument is read back, so writer and reader agree.
			args = append(args, fmt.Sprintf("%q", v.Format(time.RFC3339Nano)))
		case time.Duration:
			args = append(args, fmt.Sprintf("%q", v.String()))
		case bool:
			if v {
				args = append(args, "/true")
			} else {
				args = append(args, "/false")
			}
		case map[string]any, []any, []string, []int, []int64, []float64:
			// Containers must render the way ToAtom encodes them: as a quoted
			// JSON string. The previous %v fallback emitted a bare `map[a:b]`,
			// which is not merely lossy — it is not lexically valid Mangle, and
			// this output is not display-only. northstar.RenderVisionMangle
			// writes Fact.String() into a .mg file that the kernel loads at
			// boot, so one container-valued fact would have made the whole
			// generated file fail to parse.
			args = append(args, quoteJSON(v))
		default:
			// Same reasoning as the container branch: whatever this is, it has
			// to leave here as a single valid Mangle token. ToAtom rejects
			// unknown types outright, but String cannot return an error, so it
			// quotes instead of emitting a bare pointer address.
			args = append(args, quoteJSONOrValue(v))
		}
	}
	return fmt.Sprintf("%s(%s).", f.Predicate, strings.Join(args, ", "))
}

type unknownArg struct{ A int }

func TestFactString_MatchesSprintfForm(t *testing.T) {
	when := time.Date(2026, 11, 12, 19, 6, 5, 123456789, time.UTC)
	facts := []Fact{
		{Predicate: "session_active"},
		{Predicate: "modified", Args: []any{"internal/core/kernel.go"}},
		{Predicate: "p", Args: []any{"/name", "/not a name", "//comment", "/", "", `quote " and \\ backslash`, "tab\there", "unicode é 世界 \u2028", "\x00\x7f"}},
		{Predicate: "p", Args: []any{MangleAtom("/fix"), MangleString("a \"string\""), MangleString("/looks/like/a/name")}},
		{Predicate: "n", Args: []any{0, -7, int64(1) << 62, int64(math.MinInt64), 2.0, -0.5, 1e21, 1e-9, math.Inf(1), math.Inf(-1), math.NaN(), math.Copysign(0, -1), float32(1.5)}},
		{Predicate: "t", Args: []any{when, 90 * time.Second, time.Duration(0), true, false}},
		{Predicate: "c", Args: []any{map[string]any{"a": 1, "b": []any{"x"}}, []any{1, "two"}, []string{"a", "b"}, []int{1, 2}, []int64{3}, []float64{1.5}, []string(nil)}},
		{Predicate: "u", Args: []any{unknownArg{3}, &unknownArg{4}, nil, struct{}{}, uint8(9), []byte("raw")}},
		{Predicate: "code_defines", Args: []any{"internal/core/kernel_query.go", "core.RealKernel.Query", "/function", int64(48), int64(52)}},
	}
	for _, f := range facts {
		if got, want := f.String(), referenceFactString(f); got != want {
			t.Errorf("%s:\n got %s\nwant %s", f.Predicate, got, want)
		}
	}
}

func BenchmarkFactString(b *testing.B) {
	f := Fact{Predicate: "code_defines", Args: []any{"internal/core/kernel_query.go", "core.RealKernel.Query", "/function", int64(48), int64(52)}}
	b.Run("sprintf", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			_ = referenceFactString(f)
		}
	})
	b.Run("buffer", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			_ = f.String()
		}
	})
}
