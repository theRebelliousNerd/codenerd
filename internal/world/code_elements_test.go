package world

import (
	"testing"
)

func TestNewCodeElementParserWithRoot(t *testing.T) {
	rootPath := "/fake/project/root"
	parser := NewCodeElementParserWithRoot(rootPath)

	if parser == nil {
		t.Fatalf("Expected non-nil CodeElementParser")
	}

	if parser.Factory() == nil {
		t.Error("Expected non-nil ParserFactory inside CodeElementParser")
	}

	if parser.projectRoot != rootPath {
		t.Errorf("Expected projectRoot to be %q, got %q", rootPath, parser.projectRoot)
	}

	if parser.fileCache == nil {
		t.Error("Expected fileCache to be initialized")
	}
}
