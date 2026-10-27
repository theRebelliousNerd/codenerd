package init

import (
	"context"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
)

// countingFetcher answers every topic with two storable sections and counts
// how often each topic is fetched and under what deadline.
type countingFetcher struct {
	mu        sync.Mutex
	fetches   map[string]int
	deadlined []string
	onFetch   func(topic string)
}

func (f *countingFetcher) fetch(ctx context.Context, topic string) (string, error) {
	f.mu.Lock()
	if f.fetches == nil {
		f.fetches = map[string]int{}
	}
	f.fetches[topic]++
	if _, ok := ctx.Deadline(); ok {
		f.deadlined = append(f.deadlined, topic)
	}
	hook := f.onFetch
	f.mu.Unlock()
	if hook != nil {
		hook(topic)
	}
	section := "Notes on " + topic + ": " + strings.Repeat("the idiomatic way to use it, with examples. ", 3)
	return section + "\n\n" + section + " Second section.", nil
}

// An agent's research pass has no clock of its own: each context7_fetch is
// bounded by research.context7_timeout. A two-minute pass deadline used to
// drop every topic after the first few.
func TestCreateAgentKnowledgeBase_ResearchHasNoPassDeadline(t *testing.T) {
	fetcher := &countingFetcher{}
	ini := &Initializer{
		config:      InitConfig{Workspace: t.TempDir()},
		embedEngine: fixedInitEngine{},
		fetchTopic:  fetcher.fetch,
	}
	agent := RecommendedAgent{Name: "GoExpert", Topics: []string{"go", "mangle", "sqlite"}}
	if _, err := ini.createAgentKnowledgeBase(context.Background(), filepath.Join(t.TempDir(), "goexpert_knowledge.db"), agent, false); err != nil {
		t.Fatal(err)
	}
	if len(fetcher.fetches) != 3 {
		t.Fatalf("fetched %v, want all three topics", fetcher.fetches)
	}
	if len(fetcher.deadlined) != 0 {
		t.Fatalf("topics %v ran under a pass deadline", fetcher.deadlined)
	}
}

// Init cancellation ends the research pass between topics.
func TestCreateAgentKnowledgeBase_ResearchStopsWhenCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	fetcher := &countingFetcher{onFetch: func(topic string) {
		if topic == "first" {
			cancel()
		}
	}}
	ini := &Initializer{
		config:      InitConfig{Workspace: t.TempDir()},
		embedEngine: fixedInitEngine{},
		fetchTopic:  fetcher.fetch,
	}
	agent := RecommendedAgent{Name: "GoExpert", Topics: []string{"first", "second"}}
	_, _ = ini.createAgentKnowledgeBase(ctx, filepath.Join(t.TempDir(), "goexpert_knowledge.db"), agent, false)
	if fetcher.fetches["second"] != 0 {
		t.Fatalf("fetched %v after cancellation, want only the first topic", fetcher.fetches)
	}
}

// A topic two specialists both research is fetched once per run.
func TestCreateType3Agents_SharedTopicIsFetchedOnce(t *testing.T) {
	ws := t.TempDir()
	nerdDir := filepath.Join(ws, ".nerd")
	fetcher := &countingFetcher{}
	ini := &Initializer{
		config:      InitConfig{Workspace: ws},
		embedEngine: fixedInitEngine{},
		fetchTopic:  fetcher.fetch,
	}
	agents := []RecommendedAgent{
		{Name: "GoExpert", Type: "persistent", Topics: []string{"go testing", "go concurrency"}},
		{Name: "TestArchitect", Type: "persistent", Topics: []string{"Go Testing", "coverage"}},
	}
	result := &InitResult{}
	created, _ := ini.createType3Agents(context.Background(), nerdDir, agents, result)
	if len(created) != 2 {
		t.Fatalf("created %d agents, want 2 (warnings %v)", len(created), result.Warnings)
	}
	topics := make([]string, 0, len(fetcher.fetches))
	for topic, n := range fetcher.fetches {
		if n != 1 {
			t.Fatalf("topic %q fetched %d times, want once", topic, n)
		}
		topics = append(topics, strings.ToLower(topic))
	}
	slices.Sort(topics)
	if !slices.Equal(topics, []string{"coverage", "go concurrency", "go testing"}) {
		t.Fatalf("fetched topics %v, want each distinct topic once", topics)
	}
}
