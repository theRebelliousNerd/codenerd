package prompt_evolution

import (
	"context"
	"sync"

	"codenerd/internal/atomjudge"
	"codenerd/internal/logging"
	"codenerd/internal/prompt"
)

// atomAdmission is the check every generated atom passes before it is stored
// as a candidate (atomjudge.Admission): within jit.atom_max_chars, one
// concern, not known by default, not padded, and not covered by a shipped or
// evolved atom that reaches the same compiles. Jev reads the atom; the
// decision is atomjudge's truth table. A missing or failed Jev reading admits
// the atom unjudged and logs why: it is never a "no".
type atomAdmission struct {
	check atomjudge.Admission
	// corpus yields the shipped atoms once, on the first generation, so boot
	// does not parse the corpus for a loop that may never run.
	corpusOnce sync.Once
	corpusFn   func() ([]*prompt.PromptAtom, error)
	corpus     []*prompt.PromptAtom
}

// SetAtomAdmission installs the admission check. corpus yields the shipped
// atoms the duplicate check compares with; nil compares with evolved atoms
// only. Boot calls it once, before any cycle.
func (pe *PromptEvolver) SetAtomAdmission(check atomjudge.Admission, corpus func() ([]*prompt.PromptAtom, error)) {
	pe.mu.Lock()
	pe.admission = &atomAdmission{check: check, corpusFn: corpus}
	pe.mu.Unlock()
}

func (a *atomAdmission) shipped() []*prompt.PromptAtom {
	a.corpusOnce.Do(func() {
		if a.corpusFn == nil {
			return
		}
		atoms, err := a.corpusFn()
		if err != nil {
			logging.Get(logging.CategoryAutopoiesis).Warn("Atom admission compares with evolved atoms only: shipped corpus unavailable: %v", err)
			return
		}
		a.corpus = atoms
	})
	return a.corpus
}

// admitGenerated returns the generated atoms the admission check stores and
// counts the rest as refused. Without an installed check every atom passes,
// and the log says the check is not configured.
func (pe *PromptEvolver) admitGenerated(ctx context.Context, atoms []*GeneratedAtom) (kept []*GeneratedAtom, refused int) {
	pe.mu.RLock()
	adm := pe.admission
	existing := make([]*prompt.PromptAtom, 0, len(pe.evolvedAtoms))
	for _, ga := range pe.evolvedAtoms {
		if ga != nil && ga.Atom != nil {
			existing = append(existing, ga.Atom)
		}
	}
	pe.mu.RUnlock()

	log := logging.Get(logging.CategoryAutopoiesis)
	if adm == nil {
		if len(atoms) > 0 {
			log.Warn("Atom admission is not configured: %d generated atoms stored unchecked", len(atoms))
		}
		return atoms, 0
	}
	existing = append(existing, adm.shipped()...)
	for _, ga := range atoms {
		if ga == nil || ga.Atom == nil {
			continue
		}
		verdict, err := adm.check.Check(ctx, ga.Atom, existing)
		switch {
		case err != nil:
			log.Warn("Generated atom %s not checked, kept: %v", ga.Atom.ID, err)
		case !verdict.Admit():
			log.Info("Generated atom %s refused: %s", ga.Atom.ID, verdict)
			refused++
			continue
		case verdict.Decision == atomjudge.Unjudged:
			log.Info("Generated atom %s admitted unjudged: %s", ga.Atom.ID, verdict.Reason)
		}
		kept = append(kept, ga)
		// A later candidate of the same batch must not repeat this one.
		existing = append(existing, ga.Atom)
	}
	return kept, refused
}

// AtomAdmissionBound reports the installed check's content bound
// (jit.atom_max_chars) and whether Jev reads candidates; zero and false when
// boot installed none.
func (pe *PromptEvolver) AtomAdmissionBound() (maxChars int, judged bool) {
	pe.mu.RLock()
	defer pe.mu.RUnlock()
	if pe.admission == nil {
		return 0, false
	}
	return pe.admission.check.MaxChars, pe.admission.check.Asker != nil
}
