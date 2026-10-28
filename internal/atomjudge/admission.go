package atomjudge

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"codenerd/internal/config"
	"codenerd/internal/prompt"
)

// Admission is the check every generator runs before it stores a prompt
// atom: init's shard-agent knowledge base, the marathon init, prompt
// evolution, Ouroboros and autopoiesis agents, the chat agent wizard. Jev
// reads the candidate (Atomicity); this file decides, as a truth table over
// those readings and a size bound from config (jit.atom_max_chars):
//
//	check                       reading                                   decision
//	size                        content over max_chars                    refused /oversize
//	duplicate                   an existing atom that reaches the same    refused /duplicate
//	                            calls covers it (Noul >= yes)
//	several concerns            concerns Score nearest "three to five"    refused /several_concerns
//	                            or more (>= 1.5)
//	known by default            known Score nearest "all of it" (>= 3.5)  refused /known_by_default
//	padding                     padding Score nearest "half or more"      refused /padding
//	                            (>= 2.5)
//	otherwise                                                             admitted
//	no Jev reading (off, failed, too large to ask)                       admitted /unjudged, the reason
//	                                                                      kept: the size check alone applied
//
// A reading between thresholds admits: only a clear reading refuses. Two
// separable instructions admit, since many single rules carry a condition
// Jev counts apart.
type Admission struct {
	// Asker is Jev; nil admits every candidate within the size bound as
	// unjudged.
	Asker Asker
	// MaxChars is the content bound (jit.atom_max_chars). Zero is refused:
	// the caller resolves it from config.
	MaxChars int
	// Yes is the probability at or above which a covers Noul reads yes
	// (typesafe judgment_yes_percent).
	Yes float64
	// Compare bounds how many existing atoms the duplicate check reads.
	Compare int
}

// AdmissionFor resolves the check from config: the content bound from
// jit.atom_max_chars, the yes threshold from the typesafe section, and Jev
// only when the section is on with atom_checks. A nil asker or atom_checks
// off leaves Jev out, so every candidate within the bound is unjudged.
func AdmissionFor(asker Asker, ts *config.TypeSafeConfig, maxChars, compare int) Admission {
	a := Admission{MaxChars: maxChars, Compare: compare}
	if asker != nil && ts != nil && ts.Uses().AtomChecks {
		yes, _ := ts.Thresholds()
		a.Asker, a.Yes = asker, float64(yes)/100
	}
	return a
}

// Admission decisions.
const (
	Admitted        = "/admitted"
	Unjudged        = "/unjudged"
	Oversize        = "/oversize"
	Duplicate       = "/duplicate"
	SeveralConcerns = "/several_concerns"
	KnownByDefault  = "/known_by_default"
	Padding         = "/padding"
)

// Score positions at which a reading refuses (see the table).
const (
	refuseConcerns = 1.5
	refuseKnown    = 3.5
	refusePadding  = 2.5
)

// AdmissionVerdict is the decision on one candidate.
type AdmissionVerdict struct {
	Decision string
	// Reason is the evidence: the reading that refused, the atom it
	// duplicates, or why Jev gave no reading.
	Reason    string
	Atomicity *Atomicity
}

// Admit reports whether the decision stores the atom.
func (v AdmissionVerdict) Admit() bool {
	return v.Decision == Admitted || v.Decision == Unjudged
}

func (v AdmissionVerdict) String() string {
	if v.Reason == "" {
		return v.Decision
	}
	return v.Decision + ": " + v.Reason
}

// Check decides one candidate against the existing atoms it may duplicate.
func (a Admission) Check(ctx context.Context, candidate *prompt.PromptAtom, existing []*prompt.PromptAtom) (AdmissionVerdict, error) {
	if candidate == nil || strings.TrimSpace(candidate.Content) == "" {
		return AdmissionVerdict{}, fmt.Errorf("atomjudge: admission needs an atom with content")
	}
	if a.MaxChars <= 0 {
		return AdmissionVerdict{}, fmt.Errorf("atomjudge: admission needs max_chars from jit.atom_max_chars")
	}
	if a.Asker != nil && (a.Yes <= 0 || a.Yes > 1) {
		return AdmissionVerdict{}, fmt.Errorf("atomjudge: admission needs the yes threshold from typesafe judgment_yes_percent")
	}
	if n := utf8.RuneCountInString(candidate.Content); n > a.MaxChars {
		return AdmissionVerdict{Decision: Oversize, Reason: fmt.Sprintf("%d chars, bound %d", n, a.MaxChars)}, nil
	}
	if a.Asker == nil {
		return AdmissionVerdict{Decision: Unjudged, Reason: "Jev is not configured"}, nil
	}
	compare := Nearest(candidate, existing, max(a.Compare, 0))
	at, err := JudgeAtomicity(ctx, a.Asker, candidate, compare)
	if err != nil {
		return AdmissionVerdict{Decision: Unjudged, Reason: "no Jev reading: " + err.Error()}, nil
	}
	return a.decide(candidate, compare, at), nil
}

func (a Admission) decide(candidate *prompt.PromptAtom, compare []*prompt.PromptAtom, at *Atomicity) AdmissionVerdict {
	v := AdmissionVerdict{Atomicity: at}
	for _, e := range compare {
		if at.Covered[e.ID] >= a.Yes && Reaches(e, candidate) {
			v.Decision, v.Reason = Duplicate, fmt.Sprintf("%s covers it (%.2f)", e.ID, at.Covered[e.ID])
			return v
		}
	}
	switch {
	case at.Concerns >= refuseConcerns:
		v.Decision, v.Reason = SeveralConcerns, fmt.Sprintf("concerns %.2f: %s", at.Concerns, nearestLevel(ConcernLevels, at.Concerns))
	case at.Known >= refuseKnown:
		v.Decision, v.Reason = KnownByDefault, fmt.Sprintf("known %.2f: %s", at.Known, nearestLevel(KnownLevels, at.Known))
	case at.Padding >= refusePadding:
		v.Decision, v.Reason = Padding, fmt.Sprintf("padding %.2f: %s", at.Padding, nearestLevel(PaddingLevels, at.Padding))
	default:
		v.Decision = Admitted
	}
	return v
}

func nearestLevel(levels []string, score float64) string {
	i := int(math.Round(score))
	i = min(max(i, 0), len(levels)-1)
	return levels[i]
}

// Reaches reports whether existing is selected wherever candidate would be:
// on every selector dimension the existing atom constrains, it admits every
// value the candidate admits. An existing atom scoped narrower than the
// candidate does not make it a duplicate, since the candidate also reaches
// calls the existing atom never does.
func Reaches(existing, candidate *prompt.PromptAtom) bool {
	dims := []struct{ e, c []string }{
		{existing.ShardTypes, candidate.ShardTypes},
		{existing.Languages, candidate.Languages},
		{existing.Frameworks, candidate.Frameworks},
		{existing.IntentVerbs, candidate.IntentVerbs},
		{existing.OperationalModes, candidate.OperationalModes},
		{existing.CampaignPhases, candidate.CampaignPhases},
		{existing.BuildLayers, candidate.BuildLayers},
		{existing.InitPhases, candidate.InitPhases},
		{existing.NorthstarPhases, candidate.NorthstarPhases},
		{existing.OuroborosStages, candidate.OuroborosStages},
		{existing.WorldStates, candidate.WorldStates},
	}
	for _, d := range dims {
		if len(d.e) == 0 {
			continue
		}
		if len(d.c) == 0 {
			return false
		}
		have := make(map[string]bool, len(d.e))
		for _, v := range d.e {
			have[strings.TrimPrefix(v, "/")] = true
		}
		for _, v := range d.c {
			if !have[strings.TrimPrefix(v, "/")] {
				return false
			}
		}
	}
	return true
}

// Nearest returns up to k existing atoms most lexically similar to the
// candidate (cosine over word counts), most similar first, the candidate's
// own ID excluded. It proposes duplicates for Jev to read; it decides
// nothing.
func Nearest(candidate *prompt.PromptAtom, existing []*prompt.PromptAtom, k int) []*prompt.PromptAtom {
	if k <= 0 || len(existing) == 0 {
		return nil
	}
	cv := wordVector(candidate.Content)
	type scored struct {
		atom *prompt.PromptAtom
		sim  float64
	}
	all := make([]scored, 0, len(existing))
	for _, e := range existing {
		if e == nil || e.ID == candidate.ID || strings.TrimSpace(e.Content) == "" {
			continue
		}
		if s := cosine(cv, wordVector(e.Content)); s > 0 {
			all = append(all, scored{e, s})
		}
	}
	sort.SliceStable(all, func(i, j int) bool {
		if all[i].sim != all[j].sim {
			return all[i].sim > all[j].sim
		}
		return all[i].atom.ID < all[j].atom.ID
	})
	out := make([]*prompt.PromptAtom, 0, min(k, len(all)))
	for _, s := range all[:min(k, len(all))] {
		out = append(out, s.atom)
	}
	return out
}

func wordVector(text string) map[string]float64 {
	v := map[string]float64{}
	for _, w := range strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_'
	}) {
		if len(w) >= 3 {
			v[w]++
		}
	}
	return v
}

func cosine(a, b map[string]float64) float64 {
	var dot, na, nb float64
	for w, x := range a {
		na += x * x
		dot += x * b[w]
	}
	for _, y := range b {
		nb += y * y
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / math.Sqrt(na*nb)
}
