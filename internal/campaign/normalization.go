package campaign

import (
	"path/filepath"
	"strings"

	"codenerd/internal/logging"
)

var (
	// allowedPhaseCategories must stay in lockstep with build_phase_type/2 in
	// internal/core/defaults/build_topology.mg. A category permitted here but absent
	// there derives no phase_precedence, so has_phase_category/1 is false and the
	// kernel reports "missing_category" against a perfectly valid plan.
	// TestPhaseCategoryTablesMatchKernel pins the two together.
	allowedPhaseCategories = map[string]struct{}{
		"/scaffold":    {},
		"/domain_core": {},
		"/data_layer":  {},
		"/service":     {},
		"/transport":   {},
		"/integration": {},
		"/research":    {},
		"/test":        {},
		"/ops":         {},
	}

	// phaseCategorySynonyms mirrors phase_synonym/2 in build_topology.mg. Those facts
	// exist to make LLM classification resilient, but they can never fire on the
	// campaign path: normalization runs in Go before any fact reaches the kernel, so an
	// alias like "testing" was collapsed to the /service fallback -- a mid-layer -- and
	// the phase silently sorted into the wrong build stratum. Resolving aliases here is
	// what makes the kernel's table reachable.
	phaseCategorySynonyms = map[string]string{
		"planning":  "/research",
		"discovery": "/research",
		"analysis":  "/research",
		// assault_campaign.go emits /analysis, /review and /remediation directly; without
		// these three the assault plan's four phases collapse onto one layer.
		"implementation": "/service",
		"remediation":    "/service",
		"review":         "/test",
		"setup":          "/scaffold",
		"config":         "/scaffold",
		"bootstrap":      "/scaffold",
		"types":          "/domain_core",
		"interfaces":     "/domain_core",
		"entities":       "/domain_core",
		"database":       "/data_layer",
		"storage":        "/data_layer",
		"logic":          "/service",
		"processor":      "/service",
		"api":            "/transport",
		"frontend":       "/transport",
		"wiring":         "/integration",
		"main":           "/integration",
		"testing":        "/test",
		"qa":             "/test",
		"verification":   "/test",
		"deploy":         "/ops",
		"release":        "/ops",
		"monitoring":     "/ops",
	}
	allowedComplexities = map[string]struct{}{
		"/low":      {},
		"/medium":   {},
		"/high":     {},
		"/critical": {},
	}
)

func normalizePhaseCategory(category string) string {
	normalized := strings.TrimSpace(strings.ToLower(category))
	if normalized == "" {
		return "/service"
	}
	if canonical, ok := phaseCategorySynonyms[strings.TrimPrefix(normalized, "/")]; ok {
		return canonical
	}
	if !strings.HasPrefix(normalized, "/") {
		normalized = "/" + normalized
	}
	if _, ok := allowedPhaseCategories[normalized]; ok {
		return normalized
	}
	// Falling back is not free: /service is a mid-layer, so an unrecognized category
	// silently sorts the phase between data_layer and transport. Say so rather than
	// letting the plan's build topology be wrong in silence.
	logging.CampaignWarn("phase category %q is neither a canonical build layer nor a known alias; "+
		"falling back to /service, so build-topology ordering for this phase is a guess", category)
	return "/service"
}

func normalizeComplexity(value string) string {
	return normalizeEnum(value, allowedComplexities, "/medium")
}

func normalizeEnum(value string, allowed map[string]struct{}, fallback string) string {
	normalized := strings.TrimSpace(strings.ToLower(value))
	if normalized == "" {
		return fallback
	}
	if !strings.HasPrefix(normalized, "/") {
		normalized = "/" + normalized
	}
	if _, ok := allowed[normalized]; ok {
		return normalized
	}
	return fallback
}

func campaignSlug(campaignID string) string {
	slug := strings.TrimPrefix(strings.TrimSpace(campaignID), "/campaign_")
	if slug == "" || slug == campaignID {
		slug = sanitizeCampaignID(campaignID)
	}
	if slug == "" {
		return "campaign"
	}
	return slug
}

func sanitizeTaskArtifactPath(workspace, rawPath string) string {
	path := strings.TrimSpace(rawPath)
	if path == "" {
		return ""
	}

	if workspace == "" {
		clean := strings.TrimPrefix(normalizePath(path), "./")
		if clean == "." || clean == "" || clean == ".." || strings.HasPrefix(clean, "../") {
			return ""
		}
		return clean
	}

	baseAbs, err := filepath.Abs(workspace)
	if err != nil {
		baseAbs = filepath.Clean(workspace)
	}

	target := path
	if !filepath.IsAbs(target) {
		target = filepath.Join(baseAbs, target)
	}

	targetAbs, err := filepath.Abs(target)
	if err != nil {
		targetAbs = filepath.Clean(target)
	}

	rel, err := filepath.Rel(baseAbs, targetAbs)
	if err != nil {
		return ""
	}

	normalized := strings.TrimPrefix(filepath.ToSlash(filepath.Clean(rel)), "./")
	if normalized == "." || normalized == "" || normalized == ".." || strings.HasPrefix(normalized, "../") {
		return ""
	}

	return normalized
}
