package research

import (
	"codenerd/internal/browser"
	"codenerd/internal/config"
	"strings"
)

// Test seams: conveniences only tests call. They live here, not in
// production files, so the dead-code budget measures production code.

// headlessIfStarted is the headless manager once a read has started it.
func (rt *BrowserRuntime) headlessIfStarted() *browser.SessionManager {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	return rt.headless
}

// researchPageBrowser is the process the current policy would read with,
// if one has been started. Tests use it to count tabs. It does not launch.
func researchPageBrowser() *browser.SessionManager {
	if config.ResolvedResearchPolicy().BrowserHeadless {
		if rt := currentRuntime(); rt != nil {
			return rt.headlessIfStarted()
		}
		return nil
	}
	return getBrowserManager()
}

// GetDocURLsForTech returns documentation URLs for a technology.
func GetDocURLsForTech(tech string) []string {
	tech = strings.ToLower(tech)
	if urls, ok := CommonDocURLs[tech]; ok {
		return urls
	}
	return nil
}

// GetDocURLsForTechs returns documentation URLs for multiple technologies,
// deduplicated, in first-seen order. It does not apply Gemini's per-request
// URL cap. That limit belongs on the request (EnableURLContext and the
// Gemini client), which reports every URL it withholds. Cutting here hid
// the remainder from every caller, including one that would send the rest
// on a follow-up request (limits cleanup 2026-09-29).
func GetDocURLsForTechs(techs []string) []string {
	seen := make(map[string]bool)
	var urls []string

	for _, tech := range techs {
		for _, url := range GetDocURLsForTech(tech) {
			if seen[url] {
				continue
			}
			seen[url] = true
			urls = append(urls, url)
		}
	}

	return urls
}
