package world

import (
	"testing"
)

// TestNewGoCodeParser verifies the constructor behavior for GoCodeParser.
func TestNewGoCodeParser(t *testing.T) {
	tests := []struct {
		name        string
		projectRoot string
	}{
		{
			name:        "valid project root",
			projectRoot: "/path/to/project",
		},
		{
			name:        "empty project root",
			projectRoot: "",
		},
		{
			name:        "relative project root",
			projectRoot: "./src",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewGoCodeParser(tt.projectRoot)
			if parser == nil {
				t.Fatal("NewGoCodeParser() returned nil")
			}
			if parser.projectRoot != tt.projectRoot {
				t.Errorf("NewGoCodeParser() projectRoot = %v, want %v", parser.projectRoot, tt.projectRoot)
			}
		})
	}
}

// TestGoCodeParser_ImplementsInterface verifies that *GoCodeParser implements the CodeParser interface.
func TestGoCodeParser_ImplementsInterface(t *testing.T) {
	var _ CodeParser = (*GoCodeParser)(nil)
}
