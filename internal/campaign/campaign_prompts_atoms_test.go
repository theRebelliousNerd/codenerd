package campaign

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/perception"
	"codenerd/internal/prompt"
	"codenerd/internal/types"
)

// Every campaign role's prompt lives in prompt atoms: there is no Go copy to
// fall back to. A role whose family has no atoms compiles to the shared
// substrate alone and produces a reply that looks normal and answers nothing.
//
// This reads the atom corpus and fails if a role has no atoms behind it;
// TestCampaignRolePrompts_AreCompiledFromTheirAtoms (campaign_test) compiles
// each role and checks its atoms reach the prompt.
func TestCampaignRoles_HaveAtomCoverage(t *testing.T) {
	atomsDir := filepath.Join(repoRoot(t), "internal", "prompt", "atoms", "campaign")
	if _, err := os.Stat(atomsDir); err != nil {
		t.Skipf("campaign atom corpus not present: %v", err)
	}

	corpus, err := loadCampaignAtomIDs(atomsDir)
	if err != nil {
		t.Fatalf("reading atom corpus: %v", err)
	}
	if len(corpus) < 20 {
		t.Fatalf("found only %d atom ids under %s; the scan is broken, not the corpus", len(corpus), atomsDir)
	}

	for _, role := range CampaignRoles() {
		family := CampaignRoleAtomFamily(role)
		if family == "" {
			t.Errorf("role %s has no atom family", role)
			continue
		}
		count := 0
		for _, id := range corpus {
			if strings.HasPrefix(id, family+"/") {
				count++
			}
		}
		if count == 0 {
			t.Errorf("role %s claims atom family %q but no atom in the corpus carries that prefix; "+
				"its prompt would carry nothing of its own", role, family)
		}
	}
}

// Every role names its family, its phase and its shard type, and the shard
// type is one whose reply is parsed by Go: a Piggyback envelope around a
// campaign reply is a second, competing output contract
// (prompt.IsStructuredOutputOnly).
func TestCampaignRoles_AreFullySpecified(t *testing.T) {
	phases := map[string]CampaignRole{}
	for _, role := range CampaignRoles() {
		if CampaignRoleAtomFamily(role) == "" || GetCampaignPhaseForRole(role) == "" || GetShardTypeForRole(role) == "" {
			t.Errorf("role %s: family %q, phase %q, shard %q; each must be named",
				role, CampaignRoleAtomFamily(role), GetCampaignPhaseForRole(role), GetShardTypeForRole(role))
		}
		if !prompt.IsStructuredOutputOnly(GetShardTypeForRole(role)) {
			t.Errorf("role %s compiles as shard %q, which keeps the Piggyback envelope", role, GetShardTypeForRole(role))
		}
		// A phase is the regime that selects a role's atoms; two roles on one
		// phase would each compile the other's contract.
		if other, dup := phases[GetCampaignPhaseForRole(role)]; dup {
			t.Errorf("roles %s and %s share campaign phase %s", other, role, GetCampaignPhaseForRole(role))
		}
		phases[GetCampaignPhaseForRole(role)] = role
	}
	if GetCampaignPhaseForRole(CampaignRole("nope")) != "" || GetShardTypeForRole(CampaignRole("nope")) != "" {
		t.Error("an unknown role resolved to a phase or a shard type")
	}
}

type stubPromptProvider struct {
	prompt string
	err    error
}

func (s stubPromptProvider) GetPrompt(context.Context, CampaignRole, string, perception.LLMClient) (string, error) {
	return s.prompt, s.err
}

// Without a provider, after a failed compile, or with an empty compile, a
// campaign model call has no prompt to send and says so.
func TestRolePrompt_UnavailableIsAnError(t *testing.T) {
	for name, provider := range map[string]PromptProvider{
		"no provider":   nil,
		"compile error": stubPromptProvider{err: errors.New("compile failed")},
		"empty compile": stubPromptProvider{prompt: "  \n"},
	} {
		t.Run(name, func(t *testing.T) {
			got, err := rolePrompt(context.Background(), provider, RolePlanner, "/c", nil)
			if !errors.Is(err, types.ErrRolePromptUnavailable) || got != "" {
				t.Fatalf("rolePrompt = %q, %v; want types.ErrRolePromptUnavailable", got, err)
			}
		})
	}
	got, err := rolePrompt(context.Background(), stubPromptProvider{prompt: "compiled"}, RolePlanner, "/c", nil)
	if err != nil || got != "compiled" {
		t.Fatalf("rolePrompt = %q, %v; want the provider's prompt", got, err)
	}
}

// loadCampaignAtomIDs reads `- id: "..."` entries out of the YAML corpus. A
// full YAML parse would pull a dependency into this package for a test; the ids
// are the only field this needs.
func loadCampaignAtomIDs(dir string) ([]string, error) {
	var ids []string
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !(strings.HasSuffix(path, ".yaml") || strings.HasSuffix(path, ".yml")) {
			return nil
		}
		data, rerr := os.ReadFile(path)
		if rerr != nil {
			return rerr
		}
		for _, line := range strings.Split(string(data), "\n") {
			trimmed := strings.TrimSpace(line)
			if !strings.HasPrefix(trimmed, "- id:") && !strings.HasPrefix(trimmed, "id:") {
				continue
			}
			value := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(trimmed, "- id:"), "id:"))
			value = strings.Trim(value, `"'`)
			if value != "" {
				ids = append(ids, value)
			}
		}
		return nil
	})
	return ids, err
}
