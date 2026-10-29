package broker

import (
	"context"
	"strings"
	"testing"

	"codenerd/internal/types"
)

// An encrypted reasoning signature is ciphertext: its length says nothing
// about what replaying it costs. Ladder run R1-4c (2026-09-19): after a
// 22,853-token think, the next request carried that think's encrypted_content
// -- about 13.6 characters per reasoning token -- and was counted as 193,735
// tokens, refused against the 188,000 a 200k window leaves after its output
// reserve. The turn died. Every earlier request of the day that replayed a
// think had grown by a fraction of what its ciphertext was counted as.

func TestMeasureARedactedThinkByWhatTheProviderCountedForIt(t *testing.T) {
	cipher := strings.Repeat("c", 310_000)
	base := &Request{Messages: []types.Message{types.NewAssistantMessage(types.TextBlock("done"))}}

	tokens := 22_853
	counted := types.RedactedThinkingBlock(cipher)
	counted.Tokens = tokens
	withCounted := &Request{Messages: []types.Message{types.NewAssistantMessage(counted, types.TextBlock("done"))}}
	got := measure(withCounted).History - measure(base).History
	if want := perThinkingOverheadChars + int(float64(tokens)*DefaultSeedRatio); got != want {
		t.Fatalf("a redacted think the provider counted measured %d chars, want %d (its tokens, not its ciphertext)", got, want)
	}

	// Where the provider did not say, the ciphertext's length stands: an
	// over-count, but never an under-count of something that is replayed.
	uncounted := &Request{Messages: []types.Message{types.NewAssistantMessage(types.RedactedThinkingBlock(cipher), types.TextBlock("done"))}}
	if got, want := measure(uncounted).History-measure(base).History, perThinkingOverheadChars+len(cipher); got != want {
		t.Fatalf("an uncounted redacted think measured %d chars, want %d", got, want)
	}

	// Plaintext reasoning is replayed as text and is counted as text.
	plain := types.ThinkingBlock(strings.Repeat("r", 400), "sig")
	plain.Tokens = 5
	withPlain := &Request{Messages: []types.Message{types.NewAssistantMessage(plain, types.TextBlock("done"))}}
	if got, want := measure(withPlain).History-measure(base).History, perThinkingOverheadChars+400+len("sig"); got != want {
		t.Fatalf("a plaintext think measured %d chars, want %d", got, want)
	}
}

// R1-4c's request, reproduced at its measured sizes: a 158,175-character
// system prompt, ordinary history, the replayed 22,853-token think carried as
// 317,000 characters of ciphertext, and the tool definitions -- counted at the
// 2.75 characters per token the calibrator had learned for the model. It fits
// the window; it must be admitted.
func TestAReplayedEncryptedThinkIsAdmittedAtWhatItCost(t *testing.T) {
	counter := NewEstimatingCounter(NewCalibratorWithSeed(2.75))
	ledger := NewLedger(LedgerConfig{Window: 200_000, OutputReserve: 12_000})

	think := types.RedactedThinkingBlock(strings.Repeat("c", 317_000))
	think.Tokens = 22_853
	req := &Request{
		Model:  "muse-spark-1.3-contributor",
		System: strings.Repeat("s", 158_175),
		Messages: []types.Message{
			types.NewUserMessage(types.TextBlock(strings.Repeat("u", 49_000))),
			types.NewAssistantMessage(think, types.TextBlock("reproducing"),
				types.ToolUseBlock("call_1", "write_file", map[string]any{"path": "repro_test.go"})),
		},
		Tools: []types.ToolDefinition{{Name: "write_file", Description: strings.Repeat("d", 7_300)}},
	}

	count, err := counter.Count(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if decision := ledger.Admit(PurposeSession, count); !decision.Allowed {
		t.Fatalf("a request that fits the window was refused: %s (counted %d tokens)", decision.Reason, count.Tokens)
	}
}
