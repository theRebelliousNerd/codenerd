package context

import (
	"fmt"
	"strings"
	"unicode"

	"codenerd/internal/articulation"
	"codenerd/internal/core"
	"codenerd/internal/logging"
	"codenerd/internal/store"
	"codenerd/internal/types"
)

// UserMemory is the saved long-term memory a person's "forget" can address:
// the preferences a "remember" instruction wrote. A row is read and removed by
// identity, never by predicate, because the generic "preference" key holds
// many unrelated values.
type UserMemory interface {
	UserPreferences() ([]store.StoredFact, error)
	DeleteStoredFact(id int64) (int64, error)
}

// SessionNoteReader lets a forget say whether the note it dropped existed.
type SessionNoteReader interface {
	Query(predicate string) ([]types.Fact, error)
}

// ExactRetractor retires a hydrated kernel fact exactly, by every argument.
// RetractFact would match on the first argument alone, which for a learned
// preference is the shared "preference" key: every preference at once.
type ExactRetractor interface {
	RetractExactFactsBatch(facts []types.Fact) error
}

const (
	learnedPreferencePredicate = "learned_preference"
	// minForgetMatchRunes is the shortest text a partial match may rest on, so
	// "a" or "to" cannot select a record by being inside it.
	minForgetMatchRunes = 4
)

// forgetSessionNote drops the session note under key and nothing else, and
// says so: durable memory was not addressed.
func forgetSessionNote(kernel MemoryKernel, op articulation.MemoryOperation, d articulation.ControlDisposition) articulation.ControlDisposition {
	if kernel == nil {
		logging.Get(logging.CategoryContext).Warn("Memory op forget FAILED, no kernel attached: key=%s", op.Key)
		d.Stage, d.Reason = articulation.StageFailed, "there is no kernel attached"
		return d
	}
	existed := true
	if reader, ok := kernel.(SessionNoteReader); ok {
		if notes, err := reader.Query(sessionNotePredicate); err == nil {
			existed = false
			for _, n := range notes {
				if len(n.Args) > 0 && fmt.Sprint(n.Args[0]) == op.Key {
					existed = true
					break
				}
			}
		}
	}
	if err := kernel.RetractFact(types.Fact{Predicate: sessionNotePredicate, Args: []any{op.Key}}); err != nil {
		logging.Get(logging.CategoryContext).Warn("Memory op forget could not drop the session note: key=%s: %v", op.Key, err)
		d.Stage, d.Reason = articulation.StageFailed, "the session note was not dropped: "+err.Error()
		return d
	}
	if existed {
		d.Receipt = fmt.Sprintf("dropped the session note %q; saved long-term memory was not touched", op.Key)
	} else {
		d.Receipt = fmt.Sprintf("there was no session note %q; saved long-term memory was not touched", op.Key)
	}
	return d
}

// forgetUserMemory removes the one saved preference the person's words name,
// and retires its live copy in the kernel. It changes nothing when the words
// name none or several: a record is never deleted on a guess.
func forgetUserMemory(kernel MemoryKernel, mem MemoryStore, op articulation.MemoryOperation, d articulation.ControlDisposition) articulation.ControlDisposition {
	target := strings.TrimSpace(op.Key)
	if target == "" {
		d.Stage, d.Reason = articulation.StageRejectedParse, "a forget needs to name what to forget"
		return d
	}
	um, ok := mem.(UserMemory)
	if !ok {
		d.Stage, d.Reason = articulation.StageUnsupported, "this path has no saved long-term memory that can remove a record; nothing was removed"
		d.Receipt = "saved long-term memory was not touched"
		return d
	}
	rows, err := um.UserPreferences()
	if err != nil {
		logging.Get(logging.CategoryContext).Warn("Memory op forget could not read saved preferences: %v", err)
		d.Stage, d.Reason = articulation.StageFailed, "saved long-term memory could not be read: "+err.Error()
		return d
	}
	matches := resolvePreference(target, rows)
	switch len(matches) {
	case 0:
		d.Stage = articulation.StageUnresolved
		d.Reason = fmt.Sprintf("no saved long-term preference matches %q, so nothing was removed (session notes are separate)", target)
		return d
	case 1:
	default:
		var values []string
		for _, m := range matches {
			values = append(values, fmt.Sprintf("%q", preferenceText(m)))
		}
		d.Stage = articulation.StageUnresolved
		d.Reason = fmt.Sprintf("%d saved preferences match %q (%s); say which one. Nothing was removed", len(matches), target, strings.Join(values, ", "))
		return d
	}
	row := matches[0]
	removed, err := um.DeleteStoredFact(row.ID)
	if err != nil {
		logging.Get(logging.CategoryContext).Warn("Memory op forget could not delete the saved preference: %v", err)
		d.Stage, d.Reason = articulation.StageFailed, "the saved preference was not removed: "+err.Error()
		return d
	}
	what := fmt.Sprintf("the saved long-term preference matching %q", target)
	if removed == 0 {
		d.Receipt = what + " was already absent"
		return d
	}
	d.Receipt = "removed " + what
	if kernel == nil {
		return d
	}
	live := core.LearnedProjection(learnedPreferencePredicate, core.Fact{Predicate: row.Predicate, Args: row.Args})
	retractor, ok := kernel.(ExactRetractor)
	if !ok {
		d.Stage = articulation.StagePartiallyApplied
		d.Reason = "removed from saved memory, but this session's live copy cannot be retired and stays until restart"
		return d
	}
	if err := retractor.RetractExactFactsBatch([]types.Fact{live}); err != nil {
		d.Stage = articulation.StagePartiallyApplied
		d.Reason = "removed from saved memory, but this session's live copy was not retired: " + err.Error()
	}
	return d
}

// genericPreferenceKey is the key every remembered sentence shares; it names
// no one record, so it cannot resolve a forget by itself.
const genericPreferenceKey = store.UserPreferencePredicate

// resolvePreference finds the saved preferences the words name, best tier
// first: the exact value, then a value containing the words (or contained in
// them), then the record's own key. The first tier with any candidate answers;
// more than one candidate in it is an ambiguity the caller must not guess at.
func resolvePreference(target string, rows []store.StoredFact) []store.StoredFact {
	want := foldMemoryText(target)
	if want == "" {
		return nil
	}
	var exact, partial, keyed []store.StoredFact
	for _, row := range rows {
		var isExact, isPartial bool
		for _, arg := range row.Args {
			have := foldMemoryText(fmt.Sprint(arg))
			switch {
			case have == "":
			case have == want:
				isExact = true
			case (runeLen(want) >= minForgetMatchRunes && strings.Contains(have, want)) ||
				(runeLen(have) >= minForgetMatchRunes && strings.Contains(want, have)):
				isPartial = true
			}
		}
		switch {
		case isExact:
			exact = append(exact, row)
		case isPartial:
			partial = append(partial, row)
		case row.Predicate != genericPreferenceKey && foldMemoryText(row.Predicate) == want:
			keyed = append(keyed, row)
		}
	}
	for _, tier := range [][]store.StoredFact{exact, partial, keyed} {
		if len(tier) > 0 {
			return tier
		}
	}
	return nil
}

func preferenceText(row store.StoredFact) string {
	parts := make([]string, 0, len(row.Args))
	for _, a := range row.Args {
		parts = append(parts, fmt.Sprint(a))
	}
	return strings.Join(parts, " ")
}

func runeLen(s string) int { return len([]rune(s)) }

// foldMemoryText lowercases and collapses everything but letters and digits
// to single spaces, so "Use TABS." and "use tabs" are the same words.
func foldMemoryText(s string) string {
	var b strings.Builder
	space := true
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			space = false
		} else if !space {
			b.WriteByte(' ')
			space = true
		}
	}
	return strings.TrimSpace(b.String())
}
