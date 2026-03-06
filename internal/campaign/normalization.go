package campaign

import (
	"path/filepath"
	"strings"
)

var (
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
	allowedComplexities = map[string]struct{}{
		"/low":      {},
		"/medium":   {},
		"/high":     {},
		"/critical": {},
	}
)

func normalizePhaseCategory(category string) string {
	return normalizeEnum(category, allowedPhaseCategories, "/service")
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
