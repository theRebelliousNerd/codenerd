package perception

import (
	"reflect"
	"strings"
	"testing"

	"codenerd/internal/types"
)

// The provider counts a reply's reasoning once, for the whole reply. Each
// reasoning item the next turn replays carries its share of that count, which
// is what the broker measures an encrypted item by -- counted by its
// ciphertext instead, one replayed think made a request that fit read as
// 193,735 tokens and refused it (ladder run R1-4c).
func TestMetaToolResponseFromReply_EachReasoningItemCarriesItsTokens(t *testing.T) {
	u := &metaResponsesUsage{InputTokens: 100, OutputTokens: 500, TotalTokens: 600}
	u.OutputTokensDetails.ReasoningTokens = 400
	resp := metaToolResponseFromReply(&metaResponsesReply{
		Output: []metaResponsesItem{
			{ID: "rs_1", Type: "reasoning", EncryptedContent: strings.Repeat("a", 300)},
			{Type: "message", Content: []metaResponsesContent{{Type: "output_text", Text: "checking"}}},
			{ID: "rs_2", Type: "reasoning", EncryptedContent: strings.Repeat("b", 100)},
		},
		Usage: u,
	})
	var got []int
	for _, b := range resp.Blocks {
		if b.Kind == types.BlockThinking {
			got = append(got, b.Tokens)
		}
	}
	if want := []int{300, 100}; !reflect.DeepEqual(got, want) {
		t.Fatalf("reasoning items carry %v tokens, want %v -- the reply's 400, shared by ciphertext length", got, want)
	}

	// Shares that do not divide evenly still add up to the reply's count.
	u.OutputTokensDetails.ReasoningTokens = 401
	resp = metaToolResponseFromReply(&metaResponsesReply{
		Output: []metaResponsesItem{
			{ID: "rs_1", Type: "reasoning", EncryptedContent: strings.Repeat("a", 100)},
			{ID: "rs_2", Type: "reasoning", EncryptedContent: strings.Repeat("b", 100)},
			{ID: "rs_3", Type: "reasoning", EncryptedContent: strings.Repeat("c", 100)},
		},
		Usage: u,
	})
	sum := 0
	for _, b := range resp.Blocks {
		sum += b.Tokens
	}
	if sum != 401 {
		t.Fatalf("reasoning items carry %d tokens in all, want the reply's 401", sum)
	}

	// A reply without usage leaves the count unknown, not zero-cost: the
	// broker then falls back to the ciphertext's length.
	resp = metaToolResponseFromReply(&metaResponsesReply{
		Output: []metaResponsesItem{{ID: "rs_1", Type: "reasoning", EncryptedContent: "abc"}},
	})
	if len(resp.Blocks) != 1 || resp.Blocks[0].Tokens != 0 {
		t.Fatalf("with no usage the block's tokens must stay unknown, got %+v", resp.Blocks)
	}
}
