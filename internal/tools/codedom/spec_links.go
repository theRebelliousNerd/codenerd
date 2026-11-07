package codedom

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"codenerd/internal/logging"
	"codenerd/internal/tools"
)

// SpecLinkConfirmer is implemented by world.SpecLinkConfirmer. It is an
// interface here because internal/world imports this package.
type SpecLinkConfirmer interface {
	// ConfirmSpecLink records that the section and the element agree as they
	// now say, and answers what it confirmed.
	ConfirmSpecLink(specRef, codeRef string) (string, error)
}

// Guarded like the structure provider: a process can hold more than one
// Cortex, and each boot registers.
var (
	specLinkConfirmerMu sync.RWMutex
	specLinkConfirmer   SpecLinkConfirmer
)

// RegisterSpecLinkConfirmer installs the confirmer confirm_spec_link calls.
func RegisterSpecLinkConfirmer(c SpecLinkConfirmer) {
	specLinkConfirmerMu.Lock()
	defer specLinkConfirmerMu.Unlock()
	specLinkConfirmer = c
}

// ConfirmSpecLinkTool records a reviewed judgment about a spec link: after the
// coder has read the section and the element and found them in agreement (the
// code moved and the section still says the right thing, or the reverse), the
// drift on that link is cleared. Editing the side that did not move confirms
// the link by itself; this is for the case where no edit is needed.
func ConfirmSpecLinkTool() *tools.Tool {
	return &tools.Tool{
		Name: "confirm_spec_link",
		Description: "Confirm that a spec section and the code element it is linked to agree, after you read both. " +
			"Use it when a link is reported as DRIFT (the code changed, the section did not) or STALE CODE (the section changed, the code did not) and the review shows nothing needs to change. " +
			"It records what each side says now, so the link stops being reported until one of them changes again. " +
			"Editing the section after the code, or the code after the section, confirms the link by itself; do not use this to wave away a drift you have not read.",
		Category: tools.CategoryCode,
		Priority: 40,
		// It writes the workspace's link confirmations (.nerd/specs/links.json),
		// codeNERD's own state like the research cache, and no workspace file.
		Effect:  tools.EffectWrite,
		Execute: executeConfirmSpecLink,
		Schema: tools.ToolSchema{
			Required: []string{"spec_ref", "code_ref"},
			Properties: map[string]tools.Property{
				"spec_ref": {Type: "string", Description: "The section ref as the spec block names it: path.md:Heading > Sub heading"},
				"code_ref": {Type: "string", Description: "The element ref as the spec block names it (internal/core.RealKernel.Evaluate)"},
			},
		},
	}
}

func executeConfirmSpecLink(_ context.Context, args map[string]any) (string, error) {
	get := func(k string) string { v, _ := args[k].(string); return strings.TrimSpace(v) }
	specRef, codeRef := get("spec_ref"), get("code_ref")
	if specRef == "" || codeRef == "" {
		return "", fmt.Errorf("confirm_spec_link: spec_ref and code_ref are required")
	}
	specLinkConfirmerMu.RLock()
	c := specLinkConfirmer
	specLinkConfirmerMu.RUnlock()
	if c == nil {
		return "", fmt.Errorf("confirm_spec_link: spec links are not tracked in this process (no workspace)")
	}
	out, err := c.ConfirmSpecLink(specRef, codeRef)
	if err != nil {
		return "", err
	}
	logging.Tools("confirm_spec_link: %s <-> %s", specRef, codeRef)
	return out, nil
}
