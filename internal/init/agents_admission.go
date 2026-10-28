package init

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"codenerd/internal/atomjudge"
	"codenerd/internal/config"
	"codenerd/internal/logging"
	"codenerd/internal/prompt"
	"codenerd/internal/typesafe"
)

// admissionCompare bounds the earlier admitted atoms of the same agent one
// candidate is compared with for duplication.
const admissionCompare = 3

// atomAdmission is the check a generated shard-agent atom passes before it is
// written (atomjudge.Admission): within jit.atom_max_chars, one concern, not
// known by default, not padded, not covered by an atom already admitted for
// the agent. Jev reads the atom when the workspace's typesafe section is on
// with atom_checks; without it every atom within the bound is admitted
// unjudged, and the log says so. Resolved once per initializer.
func (i *Initializer) atomAdmission() atomjudge.Admission {
	i.admissionOnce.Do(func() {
		i.admission = atomjudge.Admission{MaxChars: i.atomMaxChars()}
		path := filepath.Join(i.config.Workspace, ".nerd", "config.json")
		if _, err := os.Stat(path); err != nil {
			return
		}
		uc, err := config.LoadUserConfig(path)
		if err != nil || uc == nil {
			return
		}
		client, err := typesafe.FromConfig(uc.TypeSafe)
		if err != nil {
			logging.BootWarn("Generated agent atoms are admitted without Jev: %v", err)
			return
		}
		var asker atomjudge.Asker
		if client != nil {
			asker = client
		}
		i.admission = atomjudge.AdmissionFor(asker, uc.TypeSafe, i.atomMaxChars(), admissionCompare)
	})
	return i.admission
}

// admitAgentAtoms runs the admission check over the model-written atoms of
// one agent and returns the ones it stores, in order. The identity atom is
// the template's, not the model's, and is not judged. A refused base piece
// (`<agent>/methodology`, `<agent>/domain`) hands its id to the next admitted
// piece of the same body, so references to the base keep resolving; a body
// with no admitted piece contributes no atom.
func (i *Initializer) admitAgentAtoms(ctx context.Context, agentNameLower string, atoms []agentPromptAtom) []agentPromptAtom {
	check := i.atomAdmission()
	identityID := agentNameLower + "/identity"
	var kept []agentPromptAtom
	var admitted []*prompt.PromptAtom
	orphanBase := map[string]agentPromptAtom{} // body base id -> refused base piece
	for _, a := range atoms {
		if a.ID == identityID {
			kept = append(kept, a)
			continue
		}
		cand := &prompt.PromptAtom{ID: a.ID, Category: prompt.AtomCategory(a.Category), Content: a.Content}
		verdict, err := check.Check(ctx, cand, admitted)
		if err != nil {
			logging.BootWarn("Agent atom %s not checked, kept: %v", a.ID, err)
		} else if !verdict.Admit() {
			logging.Boot("Agent atom %s refused: %s", a.ID, verdict)
			if base := bodyBase(agentNameLower, a.ID); base == a.ID {
				orphanBase[base] = a
			}
			continue
		} else if verdict.Decision == atomjudge.Unjudged {
			logging.Boot("Agent atom %s admitted unjudged: %s", a.ID, verdict.Reason)
		}
		if base := bodyBase(agentNameLower, a.ID); base != a.ID {
			if refused, ok := orphanBase[base]; ok {
				delete(orphanBase, base)
				a.ID, a.Concise, a.Min = refused.ID, refused.Concise, refused.Min
			}
		}
		kept = append(kept, a)
		admitted = append(admitted, cand)
	}
	return dropMissingDependencies(kept)
}

// bodyBase is the base id of the body an agent atom belongs to:
// `<agent>/methodology` for `<agent>/methodology/<slug>`.
func bodyBase(agentNameLower, id string) string {
	rest := strings.TrimPrefix(id, agentNameLower+"/")
	if head, _, ok := strings.Cut(rest, "/"); ok {
		return agentNameLower + "/" + head
	}
	return id
}

// dropMissingDependencies removes depends_on entries naming an atom the file
// no longer holds.
func dropMissingDependencies(atoms []agentPromptAtom) []agentPromptAtom {
	present := make(map[string]bool, len(atoms))
	for _, a := range atoms {
		present[a.ID] = true
	}
	for idx, a := range atoms {
		var deps []string
		for _, d := range a.DependsOn {
			if present[d] {
				deps = append(deps, d)
			}
		}
		atoms[idx].DependsOn = deps
	}
	return atoms
}
