package init

import (
	"context"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
)

// fixedInitEngine embeds everything as one short vector; the KB only needs an
// engine to exist.
type fixedInitEngine struct{}

func (fixedInitEngine) Embed(context.Context, string) ([]float32, error) {
	return []float32{0.1, 0.2, 0.3, 0.4}, nil
}
func (fixedInitEngine) EmbedBatch(_ context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i := range texts {
		out[i] = []float32{0.1, 0.2, 0.3, 0.4}
	}
	return out, nil
}
func (fixedInitEngine) Dimensions() int { return 4 }
func (fixedInitEngine) Name() string    { return "fixed" }

// topicRecorder answers every topic with two sections that mention it and
// records which topics were fetched.
type topicRecorder struct {
	mu      sync.Mutex
	fetched []string
}

func (r *topicRecorder) fetch(_ context.Context, topic string) (string, error) {
	r.mu.Lock()
	r.fetched = append(r.fetched, topic)
	r.mu.Unlock()
	section := "Notes on " + topic + ": " + strings.Repeat("the idiomatic way to use it, with examples. ", 3)
	return section + "\n\n" + section + " Second section.", nil
}

func (r *topicRecorder) take() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := r.fetched
	r.fetched = nil
	slices.Sort(out)
	return out
}

// An upgrade (`init --force`) researches only the topics the agent's KB does
// not already cover. filterTopicsNeedingResearch was written and tested for
// this and never called, so every upgrade re-fetched every topic.
func TestCreateAgentKnowledgeBase_UpgradeResearchesOnlyUncoveredTopics(t *testing.T) {
	rec := &topicRecorder{}
	ini := &Initializer{
		config:      InitConfig{Workspace: t.TempDir()},
		embedEngine: fixedInitEngine{},
		fetchTopic:  rec.fetch,
	}
	kbPath := filepath.Join(t.TempDir(), "goexpert_knowledge.db")
	agent := RecommendedAgent{Name: "GoExpert", Topics: []string{"go concurrency", "go generics"}}

	if _, err := ini.createAgentKnowledgeBase(context.Background(), kbPath, agent, false); err != nil {
		t.Fatalf("first build: %v", err)
	}
	if got := rec.take(); !slices.Equal(got, []string{"go concurrency", "go generics"}) {
		t.Fatalf("a fresh KB researched %v, want both topics", got)
	}

	agent.Topics = append(agent.Topics, "go testing")
	stats, err := ini.createAgentKnowledgeBase(context.Background(), kbPath, agent, true)
	if err != nil {
		t.Fatalf("upgrade: %v", err)
	}
	if got := rec.take(); !slices.Equal(got, []string{"go testing"}) {
		t.Fatalf("the upgrade researched %v, want only the uncovered topic", got)
	}
	if stats.SkippedTopics != 2 {
		t.Fatalf("SkippedTopics = %d, want 2", stats.SkippedTopics)
	}
}
