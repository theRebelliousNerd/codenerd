package init

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"codenerd/internal/perception"
	"codenerd/internal/store"
	"codenerd/internal/types"
)

// Domain research: a vision expert's knowledge base is built from its research
// questions, answered by grounded web search on the main client. An answer
// with no source is not knowledge and is discarded, as a "no documentation"
// answer is. Every atom carries its sources and the date it was retrieved, so
// a fast-moving answer (model releases, agent techniques) can be recognised
// as stale and researched again by a later init.

const domainRetrievedLayout = "2006-01-02"

var domainRetrievedLine = regexp.MustCompile(`(?m)^Retrieved: (\d{4}-\d{2}-\d{2})$`)

// domainConceptPrefix is the concept prefix one question's atoms share.
func domainConceptPrefix(question domainQuestion) string {
	volatility := "stable"
	if question.FastMoving {
		volatility = "fast"
	}
	slug := sanitizeToolName(question.Question)
	if runes := []rune(slug); len(runes) > 80 {
		slug = string(runes[:80])
	}
	return "domain/" + volatility + "/" + slug + "/"
}

// domainResearchStale reports whether a question's stored atoms are past the
// staleness bound: only fast-moving questions expire, and only when a bound
// is configured.
func domainResearchStale(atoms []store.KnowledgeAtom, question domainQuestion, staleDays int, now time.Time) bool {
	if !question.FastMoving || staleDays <= 0 || len(atoms) == 0 {
		return false
	}
	var newest time.Time
	for _, atom := range atoms {
		retrieved := atom.CreatedAt
		if match := domainRetrievedLine.FindStringSubmatch(atom.Content); match != nil {
			if parsed, err := time.Parse(domainRetrievedLayout, match[1]); err == nil {
				retrieved = parsed
			}
		}
		if retrieved.After(newest) {
			newest = retrieved
		}
	}
	return now.Sub(newest) > time.Duration(staleDays)*24*time.Hour
}

// groundedSources are the distinct URLs a grounded answer cited or retrieved.
func groundedSources(result *types.GroundedWebSearchResult) []string {
	if result == nil {
		return nil
	}
	seen := map[string]bool{}
	var urls []string
	add := func(url string) {
		url = strings.TrimSpace(url)
		if url == "" || seen[url] {
			return
		}
		seen[url] = true
		urls = append(urls, url)
	}
	for _, citation := range result.Citations {
		add(citation.URL)
	}
	for _, page := range result.Results {
		add(page.URL)
	}
	sort.Strings(urls)
	return urls
}

// researchDomainQuestions answers each question that has no fresh atoms and
// stores the grounded answers. It returns the atoms added and notes for the
// init warnings.
func (i *Initializer) researchDomainQuestions(ctx context.Context, db *store.LocalStore, agent RecommendedAgent, questions []domainQuestion, staleDays int, existingHashes map[string]bool) (int, []string) {
	route := i.route(slotMain)
	searcher, ok := route.client.(types.GroundedWebSearcher)
	if !ok || !searcher.SupportsGroundedWebSearch() {
		return 0, []string{fmt.Sprintf(
			"%s: its %d research question(s) need grounded web search on the main client (%s), which offers none; nothing ungrounded was stored",
			agent.Name, len(questions), formatLLMIdentity(route.provider, route.model))}
	}
	stored, err := db.GetKnowledgeAtomsByPrefix("domain/")
	if err != nil {
		return 0, []string{fmt.Sprintf("%s: stored domain research unreadable: %v", agent.Name, err)}
	}
	byPrefix := map[string][]store.KnowledgeAtom{}
	for _, atom := range stored {
		if cut := strings.LastIndex(atom.Concept, "/"); cut > 0 {
			byPrefix[atom.Concept[:cut+1]] = append(byPrefix[atom.Concept[:cut+1]], atom)
		}
	}

	now := time.Now()
	added := 0
	var notes []string
	for _, question := range questions {
		if ctx.Err() != nil {
			break
		}
		prefix := domainConceptPrefix(question)
		existing := byPrefix[prefix]
		stale := domainResearchStale(existing, question, staleDays, now)
		if len(existing) > 0 && !stale {
			continue
		}
		var result *types.GroundedWebSearchResult
		_, err := i.withJITPrompt(ctx, initCall{slot: slotMain, phase: "domain_research", task: question.Question, query: true},
			func(ctx context.Context, client perception.LLMClient, _ string) (string, error) {
				searcher, ok := client.(types.GroundedWebSearcher)
				if !ok {
					return "", fmt.Errorf("client %T has no grounded web search", client)
				}
				r, err := searcher.GroundedWebSearch(ctx, question.Question)
				if err != nil {
					return "", err
				}
				result = r
				return r.Text, nil
			})
		if err != nil {
			notes = append(notes, fmt.Sprintf("%s: research question %q failed: %v", agent.Name, question.Question, err))
			continue
		}
		sources := groundedSources(result)
		if len(sources) == 0 {
			notes = append(notes, fmt.Sprintf("%s: the answer to %q cited no source and was not stored", agent.Name, question.Question))
			continue
		}
		var sections []string
		for _, section := range strings.Split(result.Text, "\n\n") {
			if section = strings.TrimSpace(section); len(section) >= 50 {
				sections = append(sections, section)
			}
		}
		if len(sections) == 0 {
			notes = append(notes, fmt.Sprintf("%s: the answer to %q had no substantive text", agent.Name, question.Question))
			continue
		}
		// The stale answer is retired only now that a grounded replacement
		// is in hand, so a failed refresh keeps the old knowledge.
		if stale {
			if _, err := db.DeleteKnowledgeAtomsByPrefix(prefix); err != nil {
				notes = append(notes, fmt.Sprintf("%s: stale research for %q was not retired: %v", agent.Name, question.Question, err))
				continue
			}
		}
		footer := fmt.Sprintf("\n\nQuestion: %s\nSources: %s\nRetrieved: %s", question.Question, strings.Join(sources, " ; "), now.Format(domainRetrievedLayout))
		for index, section := range sections {
			ok, err := appendKnowledgeAtom(ctx, db, fmt.Sprintf("%ssection_%d", prefix, index), section+footer, 0.85, existingHashes)
			if err != nil {
				notes = append(notes, fmt.Sprintf("%s: storing research for %q failed: %v", agent.Name, question.Question, err))
				break
			}
			if ok {
				added++
			}
		}
	}
	return added, notes
}
