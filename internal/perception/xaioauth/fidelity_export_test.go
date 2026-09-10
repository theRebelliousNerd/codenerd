package xaioauth

import (
	"encoding/json"

	"codenerd/internal/types"
)

// MapHistoryForFidelity exposes the real request mapper to the external test
// package, which is where the fidelity assertion has to live: it needs
// internal/perception's BlockFidelity table, and internal/perception imports
// this package, so only a xaioauth_test file can see both.
//
// It returns the encoded request rather than the messages because the fidelity
// question is about what xAI receives.
func MapHistoryForFidelity(history []types.Message) (string, error) {
	msgs, err := mapHistoryToChatMessages("", history)
	if err != nil {
		return "", err
	}
	encoded, err := json.Marshal(msgs)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}
