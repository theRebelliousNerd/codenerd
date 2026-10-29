package prompt

import (
	"strings"
)

// Provider and model pinning.
//
// An atom may declare `providers:` and/or `models:` selectors. When it does,
// the atom is admitted ONLY on a compile whose CompilationContext names a
// matching provider/model. This is what makes an atom learned against one
// vendor's failure modes stay with that vendor instead of leaking into every
// other model's system prompt.
//
// Both dimensions are fail-closed (see regime_dimension in
// internal/core/defaults/jit_compiler.mg): a pinned atom on a compile that
// never set a provider/model is blocked, not admitted. An atom that declares
// neither selector is unaffected and continues to match everywhere.
//
// # Why canonical tokens
//
// The atom's YAML says `models: [claude-opus-4]`; the runtime's model id is
// `anthropic/claude-opus-4-20260501`. Both sides have to land on the same
// Mangle name constant or the pin silently never fires -- and a pin that never
// fires is worse than no pin, because it reads as enforcement. factBuilder's
// writeAtom preserves '-' and '.' and treats '/' as a path separator, so
// "claude-opus-4" and "claude_opus_4" are two distinct constants to the
// kernel. Every value on both sides therefore goes through the normalizers
// below, which emit only [a-z0-9_] and are thus fixpoints of writeAtom.

// pinSanitize reduces s to a canonical Mangle-safe token: lowercase, with every
// run of non-alphanumeric characters collapsed to a single underscore and
// leading/trailing underscores removed. The result contains only [a-z0-9_],
// which factBuilder.writeAtom passes through unchanged apart from the leading
// slash.
func pinSanitize(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return ""
	}

	var b strings.Builder
	b.Grow(len(s))
	pendingSep := false
	wrote := false

	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9'):
			if pendingSep && wrote {
				b.WriteByte('_')
			}
			pendingSep = false
			b.WriteByte(c)
			wrote = true
		default:
			pendingSep = true
		}
	}

	return b.String()
}

// NormalizeProviderToken canonicalizes a provider name into the token used by
// both atom `providers:` selectors and compile_context(/provider, ...).
//
//	"Anthropic"   -> "anthropic"
//	"/openai"     -> "openai"
//	"vertex_ai"   -> "vertex_ai"
func NormalizeProviderToken(provider string) string {
	return pinSanitize(strings.TrimPrefix(strings.TrimSpace(provider), "/"))
}

// modelVersionSuffixes are trailing segments that identify a release of a model
// rather than the model itself, and are therefore dropped when deriving a
// family token.
var modelVersionSuffixes = map[string]struct{}{
	"latest":  {},
	"preview": {},
	"exp":     {},
	"beta":    {},
	"stable":  {},
}

// NormalizeModelToken canonicalizes a model identifier into its exact token.
//
// Routing prefixes, vendor dot-prefixes and Bedrock-style version suffixes are
// stripped first so that the many spellings of one model converge:
//
//	"anthropic/claude-opus-4-20260501"   -> "claude_opus_4_20260501"
//	"anthropic.claude-sonnet-4-v1:0"     -> "claude_sonnet_4_v1"
//	"gpt-4o"                             -> "gpt_4o"
func NormalizeModelToken(model string) string {
	name := strings.ToLower(strings.TrimSpace(model))
	name = strings.TrimPrefix(name, "/")
	if name == "" {
		return ""
	}

	// "openai/gpt-4o", "vertex_ai/gemini-2.5-pro" -> take the last path segment.
	if i := strings.LastIndex(name, "/"); i >= 0 {
		name = name[i+1:]
	}

	// "anthropic.claude-sonnet-4-v1:0" -> strip a leading vendor dot-prefix.
	for _, prov := range []string{"anthropic.", "openai.", "google.", "bedrock.", "meta.", "mistral."} {
		name = strings.TrimPrefix(name, prov)
	}

	// "...-v1:0" Bedrock-style version suffix.
	if i := strings.Index(name, ":"); i > 0 {
		name = name[:i]
	}

	return pinSanitize(name)
}

// ModelFamilyToken derives the release-independent family of an exact model
// token, so an atom can pin to a model across its dated snapshots.
//
//	"claude_opus_4_20260501"  -> "claude_opus_4"
//	"gpt_4o_2024_08_06"       -> "gpt_4o"
//	"gemini_3_pro_latest"     -> "gemini_3_pro"
//
// Only trailing segments that are unambiguously release markers are removed: a
// yyyymmdd stamp, a yyyy[_mm[_dd]] run, or one of modelVersionSuffixes. A bare
// numeric segment is never stripped on its own, because it is far more often
// part of the name ("claude_opus_4", "gpt_4o") than a version. When nothing is
// strippable the family equals the exact token, and both are emitted anyway --
// so a heuristic miss costs nothing beyond the pin being exact-only.
func ModelFamilyToken(exact string) string {
	segments := splitNonEmpty(exact, '_')
	if len(segments) == 0 {
		return ""
	}

	for len(segments) > 1 {
		last := segments[len(segments)-1]

		// Named release markers: "latest", "preview", ...
		if _, ok := modelVersionSuffixes[last]; ok {
			segments = segments[:len(segments)-1]
			continue
		}

		// A full yyyymmdd stamp in one segment.
		if isDateStamp(last) {
			segments = segments[:len(segments)-1]
			continue
		}

		// A yyyy[_mm[_dd]] run, matched right-to-left: peel the trailing
		// two-digit segments only once a leading year segment is proven to sit
		// behind them, so "gpt_4o" keeps its "4o" and "claude_opus_4" its "4".
		if n := trailingDateRun(segments); n > 0 {
			segments = segments[:len(segments)-n]
			continue
		}

		break
	}

	return strings.Join(segments, "_")
}

// trailingDateRun reports how many trailing segments form a yyyy[_mm[_dd]] run,
// or 0 if the tail is not such a run. It requires the year segment to be
// present so that a lone "06" is never mistaken for a version.
func trailingDateRun(segments []string) int {
	// Walk back over at most two 2-digit segments (mm, dd) looking for a year.
	for back := 0; back <= 2 && back < len(segments)-1; back++ {
		idx := len(segments) - 1 - back
		if isYear(segments[idx]) {
			return back + 1
		}
		if !isTwoDigit(segments[idx]) {
			return 0
		}
	}
	return 0
}

func isYear(s string) bool {
	return len(s) == 4 && strings.HasPrefix(s, "20") && allDigits(s)
}

func isTwoDigit(s string) bool {
	return len(s) == 2 && allDigits(s)
}

func isDateStamp(s string) bool {
	return len(s) == 8 && strings.HasPrefix(s, "20") && allDigits(s)
}

func allDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

func splitNonEmpty(s string, sep byte) []string {
	parts := strings.Split(s, string(sep))
	out := parts[:0]
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// ModelPinTokens returns the set of tokens a compile with the given model
// identifier satisfies: the exact token and, when it differs, the family token.
// An atom pinned at either granularity therefore matches.
func ModelPinTokens(model string) []string {
	exact := NormalizeModelToken(model)
	if exact == "" {
		return nil
	}
	family := ModelFamilyToken(exact)
	if family == "" || family == exact {
		return []string{exact}
	}
	return []string{exact, family}
}
