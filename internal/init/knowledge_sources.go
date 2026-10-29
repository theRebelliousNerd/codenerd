package init

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"codenerd/internal/knowledge"
	"codenerd/internal/perception"
	"codenerd/internal/types"
)

// KnowledgeSubjects is what a shard agent's knowledge is acquired about: one
// subject per research topic (asked of web search and Context7), and one per
// Go package the workspace pins for each of the agent's technology domains
// (the vocabulary's packages for that key that a go.mod requires, or the
// standard library's), which the pinned-documentation and repository sources
// also read. Init's knowledge build and the background refresh both ask it.
func KnowledgeSubjects(workspace, agent string, domains, topics []string) []knowledge.Subject {
	var subjects []knowledge.Subject
	seenTopic := map[string]bool{}
	for _, topic := range topics {
		topic = strings.TrimSpace(topic)
		if topic == "" || seenTopic[strings.ToLower(topic)] {
			continue
		}
		seenTopic[strings.ToLower(topic)] = true
		subjects = append(subjects, knowledge.Subject{Agent: agent, Topic: topic})
	}
	gomods := goModContents(workspace)
	for _, domain := range domains {
		key := strings.ToLower(strings.TrimSpace(domain))
		if key == "" {
			continue
		}
		modules := pinnedGoPackages(key, gomods)
		if len(modules) == 0 {
			if !seenTopic[key] {
				seenTopic[key] = true
				subjects = append(subjects, knowledge.Subject{Agent: agent, Topic: key, Technology: key})
			}
			continue
		}
		for _, module := range modules {
			subjects = append(subjects, knowledge.Subject{Agent: agent, Topic: key, Technology: key, Module: module})
		}
	}
	return subjects
}

// goModContents is the text of every go.mod in the workspace (root first).
func goModContents(workspace string) []string {
	if strings.TrimSpace(workspace) == "" {
		return nil
	}
	paths := append([]string{filepath.Join(workspace, "go.mod")}, findManifestFiles(workspace, []string{"go.mod"}, maxManifestDepth)...)
	seen := map[string]bool{}
	var out []string
	for _, p := range paths {
		if seen[p] {
			continue
		}
		seen[p] = true
		if data, err := os.ReadFile(p); err == nil {
			out = append(out, string(data))
		}
	}
	return out
}

// pinnedGoPackages are the vocabulary's Go packages for a technology key that
// some go.mod requires; standard-library packages count when any go.mod
// exists.
func pinnedGoPackages(key string, gomods []string) []string {
	if len(gomods) == 0 {
		return nil
	}
	var out []string
	for _, pkg := range vocabularyPackages(EcosystemGo) {
		if technologyVocabulary[EcosystemGo][pkg] != key {
			continue
		}
		std := !strings.Contains(strings.SplitN(pkg, "/", 2)[0], ".")
		if std {
			out = append(out, pkg)
			continue
		}
		for _, mod := range gomods {
			if strings.Contains(mod, pkg+" ") || strings.Contains(mod, pkg+"/") {
				out = append(out, pkg)
				break
			}
		}
	}
	sort.Strings(out)
	return out
}

// KnowledgeSearchers are the configured clients that may offer grounded web
// search, each named by its provider: the web source's chain (one successful
// search per subject across them). A nil client is skipped.
func KnowledgeSearchers(clients ...perception.LLMClient) []knowledge.NamedSearcher {
	var out []knowledge.NamedSearcher
	for _, client := range clients {
		if client == nil {
			continue
		}
		searcher, ok := client.(types.GroundedWebSearcher)
		if !ok {
			continue
		}
		provider, _ := clientIdentity(client)
		out = append(out, knowledge.NamedSearcher{Provider: provider, Searcher: searcher})
	}
	return out
}

// knowledgeAcquirer is the acquirer init's knowledge build runs: grounded web
// search on the configured clients (main and planner, by provider), Context7
// (through the run's shared topic cache), the pinned Go documentation, and
// the repository's own use from init's scan.
func (i *Initializer) knowledgeAcquirer() knowledge.Acquirer {
	searchers := KnowledgeSearchers(i.route(slotMain).client, i.route(slotPlanner).client)
	var context7 func(ctx context.Context, topic string) (string, error)
	if fetch, err := i.topicFetcher(); err == nil {
		context7 = func(ctx context.Context, topic string) (string, error) {
			return i.research.fetch(ctx, topic, fetch)
		}
	}
	var kernel types.Kernel
	if i.kernel != nil {
		kernel = i.kernel
	}
	return knowledge.NewAcquirer(knowledge.Config{
		Searchers: searchers,
		Context7:  context7,
		Workspace: i.config.Workspace,
		Kernel:    kernel,
	})
}
