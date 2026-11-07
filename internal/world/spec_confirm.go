package world

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"codenerd/internal/specs"
	"codenerd/internal/tools"
	"codenerd/internal/types"
)

// SpecLinkConfirmer confirms spec links for one workspace: the reviewer's way
// of saying "I read this section and this element together, and they agree",
// which is what clears spec_drift and spec_stale_code for the link. It is the
// implementation behind the confirm_spec_link tool (internal/tools/codedom).
//
// It records what the two sides say now: the fingerprints the kernel holds,
// which the last scan measured. The confirmation is written to the workspace's
// store (specs.OpenStore) and the kernel's rows for the link are replaced in
// the same call, so the judgment clears without waiting for a scan: a scan only
// measures again when a file changed, and a confirmation changes none.
type SpecLinkConfirmer struct {
	kernel    types.Kernel
	workspace string
	now       func() int64
}

// NewSpecLinkConfirmer returns the confirmer of workspace's links, judged on
// the facts kernel holds.
func NewSpecLinkConfirmer(kernel types.Kernel, workspace string) *SpecLinkConfirmer {
	return &SpecLinkConfirmer{kernel: kernel, workspace: workspace, now: func() int64 { return time.Now().Unix() }}
}

// ConfirmSpecLink confirms the link between a section and an element and says
// what it confirmed.
func (c *SpecLinkConfirmer) ConfirmSpecLink(specRef, codeRef string) (string, error) {
	specRef, codeRef = strings.TrimSpace(specRef), strings.TrimSpace(codeRef)
	if specRef == "" || codeRef == "" {
		return "", fmt.Errorf("confirm_spec_link: spec_ref and code_ref are required")
	}
	root, err := tools.CanonicalWorkspaceRoot(c.workspace)
	if err != nil {
		return "", fmt.Errorf("confirm_spec_link: %w", err)
	}
	links, err := c.kernel.Query("spec_link")
	if err != nil {
		return "", fmt.Errorf("confirm_spec_link: read spec_link: %w", err)
	}
	var near []string
	linked := false
	for _, l := range links {
		if len(l.Args) < 2 {
			continue
		}
		s, e := specStr(l.Args[0]), specStr(l.Args[1])
		if s == specRef && e == codeRef {
			linked = true
		} else if s == specRef || e == codeRef {
			near = append(near, s+" <-> "+e)
		}
	}
	if !linked {
		msg := fmt.Sprintf("confirm_spec_link: no link between section %q and element %q", specRef, codeRef)
		if len(near) > 0 {
			sort.Strings(near)
			msg += "; links that share an end: " + strings.Join(near, "; ")
		}
		return "", fmt.Errorf("%s", msg)
	}
	specPrint, err := c.fingerprint("spec_section", specRef, 3)
	if err != nil {
		return "", fmt.Errorf("confirm_spec_link: the section %q is not in the world (the link dangles; fix the citation, there is nothing to confirm): %w", specRef, err)
	}
	codePrint, err := c.fingerprint("spec_element", codeRef, 5)
	if err != nil {
		return "", fmt.Errorf("confirm_spec_link: the element %q is not in the world (the link dangles; fix the citation, there is nothing to confirm): %w", codeRef, err)
	}
	was := c.verdict(specRef, codeRef)

	now := c.now()
	key := specs.LinkKey{Spec: specRef, Code: codeRef}
	if _, err := specs.OpenStore(root).Update(func(prev specs.Confirmations) (specs.Confirmations, error) {
		return specs.Confirm(prev, specRef, codeRef, specPrint, codePrint, now), nil
	}); err != nil {
		return "", fmt.Errorf("confirm_spec_link: %w", err)
	}
	if err := c.replaceRows(key, specs.Confirmation{SpecFingerprint: specPrint, CodeFingerprint: codePrint, ConfirmedAt: now}); err != nil {
		return "", fmt.Errorf("confirm_spec_link: the confirmation is saved but the kernel rows were not updated (a scan will reload them): %w", err)
	}
	return fmt.Sprintf("Confirmed: `%s` and `%s` agree as they now say (section %s, element %s). It was %s.", specRef, codeRef, specPrint, codePrint, was), nil
}

// fingerprint reads the fingerprint (the last argument, at index last) of the
// row of pred whose first argument is ref.
func (c *SpecLinkConfirmer) fingerprint(pred, ref string, last int) (string, error) {
	rows, err := c.kernel.Query(pred)
	if err != nil {
		return "", err
	}
	for _, r := range rows {
		if len(r.Args) > last && specStr(r.Args[0]) == ref {
			return specStr(r.Args[last]), nil
		}
	}
	return "", fmt.Errorf("no %s row", pred)
}

// verdict says what the kernel judged about the link before the confirmation.
func (c *SpecLinkConfirmer) verdict(specRef, codeRef string) string {
	holds := func(pred string) bool {
		rows, err := c.kernel.Query(pred)
		if err != nil {
			return false
		}
		for _, r := range rows {
			if len(r.Args) >= 2 && specStr(r.Args[0]) == specRef && specStr(r.Args[1]) == codeRef {
				return true
			}
		}
		return false
	}
	switch {
	case holds("spec_drift"):
		return "drifted (the code changed, the section did not)"
	case holds("spec_stale_code"):
		return "stale (the section changed, the code did not)"
	}
	return "not drifting"
}

// replaceRows swaps the kernel's confirmation of one link for the new one and
// drops the link's changed-since rows: the sides now match what is confirmed.
func (c *SpecLinkConfirmer) replaceRows(key specs.LinkKey, now specs.Confirmation) error {
	for _, pred := range []string{"spec_confirmed", "spec_changed_since"} {
		rows, err := c.kernel.Query(pred)
		if err != nil {
			return err
		}
		for _, r := range rows {
			if len(r.Args) >= 2 && specStr(r.Args[0]) == key.Spec && specStr(r.Args[1]) == key.Code {
				if err := c.kernel.RetractFact(r); err != nil {
					return err
				}
			}
		}
	}
	return c.kernel.Assert(types.Fact{Predicate: "spec_confirmed",
		Args: []any{key.Spec, key.Code, now.SpecFingerprint, now.CodeFingerprint, now.ConfirmedAt}})
}
