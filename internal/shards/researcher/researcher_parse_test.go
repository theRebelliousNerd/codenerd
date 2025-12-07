package researcher

import (
	"reflect"
	"testing"
)

func TestParseTaskMultiWordTopicAndKeywords(t *testing.T) {
	shard := NewResearcherShard()

	task := "topic:vector search ranking keywords:semantic search, recall, precision"
	topic, keywords, urls := shard.parseTask(task)

	if topic != "vector search ranking" {
		t.Fatalf("topic = %q, want %q", topic, "vector search ranking")
	}

	expectedKeywords := []string{"semantic search", "recall", "precision"}
	if !reflect.DeepEqual(keywords, expectedKeywords) {
		t.Fatalf("keywords = %#v, want %#v", keywords, expectedKeywords)
	}

	if len(urls) != 0 {
		t.Fatalf("urls = %#v, want none", urls)
	}
}

func TestParseTaskStripsUrlsAndKeepsTopic(t *testing.T) {
	shard := NewResearcherShard()

	task := "research openai embeddings https://example.com/docs keywords:embeddings, openai"
	topic, keywords, urls := shard.parseTask(task)

	if topic != "research openai embeddings" {
		t.Fatalf("topic = %q, want %q", topic, "research openai embeddings")
	}

	expectedKeywords := []string{"embeddings", "openai"}
	if !reflect.DeepEqual(keywords, expectedKeywords) {
		t.Fatalf("keywords = %#v, want %#v", keywords, expectedKeywords)
	}

	if len(urls) != 1 || urls[0] != "https://example.com/docs" {
		t.Fatalf("urls = %#v, want [https://example.com/docs]", urls)
	}
}

func TestParseTaskFallsBackToTopicWords(t *testing.T) {
	shard := NewResearcherShard()

	task := "vector search ranking signals"
	topic, keywords, urls := shard.parseTask(task)

	if topic != "vector search ranking signals" {
		t.Fatalf("topic = %q, want %q", topic, "vector search ranking signals")
	}

	expectedKeywords := []string{"vector", "search", "ranking", "signals"}
	if !reflect.DeepEqual(keywords, expectedKeywords) {
		t.Fatalf("keywords = %#v, want %#v", keywords, expectedKeywords)
	}

	if len(urls) != 0 {
		t.Fatalf("urls = %#v, want none", urls)
	}
}
