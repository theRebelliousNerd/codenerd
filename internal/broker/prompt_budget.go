package broker

// PromptBudgetFor is PromptBudget for a prompt that will be sent to a client
// capping its own input: inputWindow is that client's cap in tokens
// (InputWindow; 0 when it reports none). The share applies to whichever is
// smaller, the window the ledger enforces or the client's cap, so a planner or
// worker slot with a smaller max_input_tokens is compiled for the window it
// will actually be sent.
func (m *Meter) PromptBudgetFor(share float64, fallback, inputWindow int) int {
	budget := m.PromptBudget(share, fallback)
	if inputWindow <= 0 {
		return budget
	}
	if share <= 0 || share > 1 {
		share = 1
	}
	capped := int(float64(inputWindow) * share)
	if capped < minPromptBudget {
		capped = minPromptBudget
	}
	if capped < budget {
		return capped
	}
	return budget
}
