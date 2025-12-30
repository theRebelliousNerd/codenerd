package world

import (
	"testing"
)

type MockGraph struct{}

func (m *MockGraph) QueryGraph(queryType string, params map[string]interface{}) (interface{}, error) {
	if queryType == "dependencies" {
		return []string{"dep1", "dep2"}, nil
	}
	return nil, nil
}

func TestGraphQueryInterface(t *testing.T) {
	// This just verifies the interface exists and methods match signature.
	var _ GraphQuery = &MockGraph{}
}
