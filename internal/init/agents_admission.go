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
// written (WorkspaceAtomAdmission), resolved once per initializer.
func (i *Initializer) atomAdmission() atomjudge.Admission {
	i.admissionOnce.Do(func() {
		i.admission = WorkspaceAtomAdmission(i.config.Workspace, i.atomMaxChars())
	})
	return i.admission
}

// WorkspaceAtomAdmission resolves the generated-atom admission check
// (atomjudge.Admission) from the workspace's .nerd/config.json: within
// maxChars, one concern, not known by default, not padded, not covered by an
// atom already admitted for the agent. Jev reads each atom when the typesafe
// section is on with atom_checks; without it every atom within the bound is
// admitted unjudged, and the log says so.
func WorkspaceAtomAdmission(workspace string, maxChars int) atomjudge.Admission {
	check := atomjudge.Admission{MaxChars: maxChars}
	path := filepath.Join(workspace, ".nerd", "config.json")
	if _, err := os.Stat(path); err != nil {
		return check
	}
	uc, err := config.LoadUserConfig(path)
	if err != nil || uc == nil {
		return check
	}
	client, err := typesafe.FromConfig(uc.TypeSafe)
	if err != nil {
		logging.BootWarn("Generated agent atoms are admitted without Jev: %v", err)
		return check
	}
	var asker atomjudge.Asker
	if client != nil {
		asker = client
	}
	return atomjudge.AdmissionFor(asker, uc.TypeSafe, maxChars, admissionCompare)
}

// AgentDefinitionYAML renders the prompts.yaml of a user-defined or
// autopoiesis-authored agent the way init renders a specialist's: the
// template identity, then the methodology and domain bodies (model-written:
// the agent's system prompt, researched knowledge) split one atom per
// heading within jit.atom_max_chars and admitted by WorkspaceAtomAdmission.
// An empty body contributes no atom. Atom ids are prefixed with name as
// given: it is also the agent's directory name.
func AgentDefinitionYAML(ctx context.Context, workspace, name, role, topics, methodology, domain string) (string, error) {
	maxChars := WorkspaceAtomMaxChars(workspace)
	name = strings.TrimSpace(name)
	lower := name
	var topicList []string
	for _, t := range strings.Split(topics, ",") {
		if t = strings.TrimSpace(t); t != "" {
			topicList = append(topicList, t)
		}
	}
	atoms := agentPromptAtoms(lower, name, role, formatDomainExpertise(topicList), topics, methodology, domain, maxChars)
	atoms = admitAgentAtoms(ctx, WorkspaceAtomAdmission(workspace, maxChars), lower, atoms)
	yml := renderPromptsYAML(name, atoms)
	if err := validatePromptsYAML([]byte(yml), lower, maxChars); err != nil {
		return "", err
	}
	return yml, nil
}

// admitAgentAtoms runs the admission check over the model-written atoms of
// one agent and returns the ones it stores, in order. The identity atom is
// the template's, not the model's, and is not judged. A refused base piece
// (`<agent>/methodology`, `<agent>/domain`) hands its id to the next admitted
// piece of the same body, so references to the base keep resolving; a body
// with no admitted piece contributes no atom.
func admitAgentAtoms(ctx context.Context, check atomjudge.Admission, agentNameLower string, atoms []agentPromptAtom) []agentPromptAtom {
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
