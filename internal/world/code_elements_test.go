package world

import (
	"reflect"
	"testing"
)

func TestNewCodeElementParserWithRoot_Basic(t *testing.T) {
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

// The function GetMethodsOfStruct is already tested in internal/world/code_elements_extra_test.go

func TestGetElement(t *testing.T) {
	elements := []CodeElement{
		{Ref: "fn:main", Type: ElementFunction, File: "a.go", StartLine: 1, EndLine: 5},
		{Ref: "struct:Foo", Type: ElementStruct, File: "a.go", StartLine: 7, EndLine: 10},
		{Ref: "method:Foo.Bar", Type: ElementMethod, Parent: "struct:Foo", File: "a.go", StartLine: 12, EndLine: 15},
	}

	tests := []struct {
		name     string
		elements []CodeElement
		ref      string
		wantIdx  int
	}{
		{
			name:     "get existing element (first)",
			elements: elements,
			ref:      "fn:main",
			wantIdx:  0,
		},
		{
			name:     "get existing element (last)",
			elements: elements,
			ref:      "method:Foo.Bar",
			wantIdx:  2,
		},
		{
			name:     "get non-existing element",
			elements: elements,
			ref:      "fn:missing",
			wantIdx:  -1,
		},
		{
			name:     "empty elements list",
			elements: []CodeElement{},
			ref:      "fn:main",
			wantIdx:  -1,
		},
		{
			name:     "nil elements list",
			elements: nil,
			ref:      "fn:main",
			wantIdx:  -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetElement(tt.elements, tt.ref)

			if tt.wantIdx == -1 {
				if got != nil {
					t.Errorf("GetElement() returned non-nil %v, want nil", got)
				}
				return
			}

			if got == nil {
				t.Fatalf("GetElement() returned nil, want element at index %d", tt.wantIdx)
			}

			expectedPtr := &tt.elements[tt.wantIdx]
			if got != expectedPtr {
				t.Errorf("GetElement() returned pointer %p, want %p (address in slice)", got, expectedPtr)
			}

			if !reflect.DeepEqual(*got, tt.elements[tt.wantIdx]) {
				t.Errorf("GetElement() content = %v, want %v", *got, tt.elements[tt.wantIdx])
			}
		})
	}
}
