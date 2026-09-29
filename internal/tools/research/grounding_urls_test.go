package research

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
)

// The Gemini URL Context limit (maxURLContextURLs) is a real provider cap,
// so it stays — but it must never silently drop URLs. These tests pin that
// every entry point hands the remainder back to the caller (limits cleanup
// 2026-09-29).

func makeDocURLs(n int) []string {
	urls := make([]string, n)
	for i := range urls {
		urls[i] = fmt.Sprintf("https://doc%d.example.com/guide", i)
	}
	return urls
}

func TestEnableURLContext_WhenOverLimit_ReturnsDroppedURLs(t *testing.T) {
	client := &groundingClient{}
	helper := NewGroundingHelper(client)

	urls := makeDocURLs(maxURLContextURLs + 5)
	dropped := helper.EnableURLContext(urls)

	if len(client.urls) != maxURLContextURLs {
		t.Fatalf("expected first %d URLs sent, got %d", maxURLContextURLs, len(client.urls))
	}
	for i, u := range client.urls {
		if u != urls[i] {
			t.Fatalf("sent URLs are not the first %d: index %d = %q", maxURLContextURLs, i, u)
		}
	}
	if len(dropped) != 5 {
		t.Fatalf("expected 5 dropped URLs returned, got %d", len(dropped))
	}
	for i, u := range dropped {
		if u != urls[maxURLContextURLs+i] {
			t.Fatalf("dropped[%d] = %q, want %q", i, u, urls[maxURLContextURLs+i])
		}
	}
	if got := helper.GetLastDroppedURLs(); len(got) != 5 || got[0] != dropped[0] {
		t.Fatalf("GetLastDroppedURLs = %v, want the 5 withheld URLs", got)
	}
	stats := helper.GetStats()
	if stats.TotalURLsUsed != maxURLContextURLs || stats.TotalURLsDropped != 5 {
		t.Fatalf("stats = %+v, want used=%d dropped=5", stats, maxURLContextURLs)
	}
}

func TestEnableURLContext_WhenWithinLimit_SendsAllAndDropsNothing(t *testing.T) {
	client := &groundingClient{}
	helper := NewGroundingHelper(client)

	urls := makeDocURLs(3)
	if dropped := helper.EnableURLContext(urls); len(dropped) != 0 {
		t.Fatalf("expected no dropped URLs, got %v", dropped)
	}
	if len(client.urls) != 3 || len(helper.GetLastDroppedURLs()) != 0 {
		t.Fatalf("sent=%v dropped=%v", client.urls, helper.GetLastDroppedURLs())
	}
}

func TestSetURLContextURLs_WhenOverLimit_ReturnsDroppedURLs(t *testing.T) {
	client := &groundingClient{}
	helper := NewGroundingHelper(client)

	urls := makeDocURLs(maxURLContextURLs + 2)
	dropped := helper.SetURLContextURLs(urls)

	if len(client.urls) != maxURLContextURLs {
		t.Fatalf("expected %d URLs set, got %d", maxURLContextURLs, len(client.urls))
	}
	if len(dropped) != 2 || dropped[0] != urls[maxURLContextURLs] {
		t.Fatalf("expected the 2 withheld URLs returned, got %v", dropped)
	}
	if got := helper.GetLastDroppedURLs(); len(got) != 2 {
		t.Fatalf("GetLastDroppedURLs = %v", got)
	}
}

func TestSetURLContextURLs_WhenNilController_ReturnsNil(t *testing.T) {
	helper := NewGroundingHelper(plainClient{})
	if dropped := helper.SetURLContextURLs(makeDocURLs(25)); dropped != nil {
		t.Fatalf("expected nil for a client without grounding control, got %v", dropped)
	}
	if dropped := helper.EnableURLContext(makeDocURLs(25)); dropped != nil {
		t.Fatalf("expected nil for a client without grounding control, got %v", dropped)
	}
}

func TestGroundedResearch_WhenOverLimit_NamesURLsNotSent(t *testing.T) {
	client := &groundingClient{}
	helper := NewGroundingHelper(client)

	docURLs := makeDocURLs(maxURLContextURLs + 3)
	result, err := helper.GroundedResearch(context.Background(), "topic", docURLs)
	if err != nil {
		t.Fatalf("GroundedResearch: %v", err)
	}
	if len(result.DocURLs) != len(docURLs) {
		t.Fatalf("DocURLs should still name everything provided, got %d", len(result.DocURLs))
	}
	if len(result.DroppedDocURLs) != 3 || result.DroppedDocURLs[0] != docURLs[maxURLContextURLs] {
		t.Fatalf("DroppedDocURLs = %v, want the 3 withheld URLs", result.DroppedDocURLs)
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var wire map[string]any
	if err := json.Unmarshal(encoded, &wire); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	notSent, _ := wire["doc_urls_not_sent"].([]any)
	if len(notSent) != 3 || notSent[0] != docURLs[maxURLContextURLs] {
		t.Fatalf("doc_urls_not_sent = %v, want the 3 withheld URLs", wire["doc_urls_not_sent"])
	}
}

func TestSetURLContextURLs_WithinLimitClearsPriorDrops(t *testing.T) {
	client := &groundingClient{}
	helper := NewGroundingHelper(client)

	if dropped := helper.SetURLContextURLs(makeDocURLs(maxURLContextURLs + 4)); len(dropped) != 4 {
		t.Fatalf("expected 4 withheld URLs, got %d", len(dropped))
	}
	if dropped := helper.SetURLContextURLs(makeDocURLs(2)); len(dropped) != 0 || len(helper.GetLastDroppedURLs()) != 0 {
		t.Fatalf("a fitting call must clear the previous withhold, dropped=%v last=%v", dropped, helper.GetLastDroppedURLs())
	}
	if len(client.urls) != 2 {
		t.Fatalf("sent = %v, want the 2 fitting URLs", client.urls)
	}
	if stats := helper.GetStats(); stats.TotalURLsDropped != 4 {
		t.Fatalf("TotalURLsDropped = %d, want the earlier withhold kept in the counter", stats.TotalURLsDropped)
	}
}

func TestGroundedResearch_WhenWithinLimit_OmitsNotSent(t *testing.T) {
	helper := NewGroundingHelper(&groundingClient{})
	result, err := helper.GroundedResearch(context.Background(), "topic", makeDocURLs(2))
	if err != nil {
		t.Fatalf("GroundedResearch: %v", err)
	}
	if len(result.DroppedDocURLs) != 0 {
		t.Fatalf("expected no dropped URLs, got %v", result.DroppedDocURLs)
	}
}
