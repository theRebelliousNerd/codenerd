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
// package the workspace pins for each of the agent's technology domains --
// the vocabulary's Go packages a go.mod requires (or the standard library's,
// in a Go workspace) and npm packages a package.json depends on -- each with
// the version its manifest pins, which the refresh policy compares with the
// version the knowledge was acquired at. A concept domain (ConceptDomains)
// contributes its research subjects; any other domain the workspace pins no
// package for is a plain subject. Init's knowledge build and the background
// refresh both ask it, with the workspace's ManifestPins.
func KnowledgeSubjects(workspace, agent string, domains, topics []string, pins []ManifestPin) []knowledge.Subject {
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
	goWorkspace := false
	if strings.TrimSpace(workspace) != "" {
		if _, err := os.Stat(filepath.Join(workspace, "go.mod")); err == nil {
			goWorkspace = true
		}
	}
	for _, pin := range pins {
		goWorkspace = goWorkspace || pin.Ecosystem == EcosystemGo
	}
	for _, domain := range domains {
		key := strings.ToLower(strings.TrimSpace(domain))
		if key == "" {
			continue
		}
		pinned := pinnedPackages(key, pins, goWorkspace)
		if len(pinned) == 0 {
			// A concept domain (security, testing) has no package to pin:
			// its subjects are the concept's own research topics.
			if concept := ConceptSubjects(key); len(concept) > 0 {
				for _, topic := range concept {
					if !seenTopic[strings.ToLower(topic)] {
						seenTopic[strings.ToLower(topic)] = true
						subjects = append(subjects, knowledge.Subject{Agent: agent, Topic: topic, Technology: key})
					}
				}
				continue
			}
			if !seenTopic[key] {
				seenTopic[key] = true
				subjects = append(subjects, knowledge.Subject{Agent: agent, Topic: key, Technology: key})
			}
			continue
		}
		for _, p := range pinned {
			p.Agent, p.Topic, p.Technology = agent, key, key
			subjects = append(subjects, p)
		}
	}
	return subjects
}

// pinnedPackages are the vocabulary's packages for a technology key that the
// pins name, as module subjects carrying their ecosystem and pinned version.
// A Go package inside a required module takes that module's version; a
// standard-library package counts in a Go workspace and pins no version.
func pinnedPackages(key string, pins []ManifestPin, goWorkspace bool) []knowledge.Subject {
	var out []knowledge.Subject
	for _, eco := range []Ecosystem{EcosystemGo, EcosystemNPM} {
		for _, pkg := range vocabularyPackages(eco) {
			if technologyVocabulary[eco][pkg] != key {
				continue
			}
			if eco == EcosystemGo && knowledge.IsStdPackage(pkg) {
				if goWorkspace {
					out = append(out, knowledge.Subject{Module: pkg, Ecosystem: string(eco)})
				}
				continue
			}
			for _, pin := range pins {
				if pin.Ecosystem != eco {
					continue
				}
				if pin.Module == pkg || (eco == EcosystemGo && strings.HasPrefix(pkg, pin.Module+"/")) {
					out = append(out, knowledge.Subject{Module: pkg, Ecosystem: string(eco), Version: pin.Version})
					break
				}
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Module < out[j].Module })
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
