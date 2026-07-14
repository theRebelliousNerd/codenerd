package world

import (
	"reflect"
	"testing"

	"codenerd/internal/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestElementsToFacts(t *testing.T) {
	tests := []struct {
		name     string
		elements []CodeElement
		wantLen  int
		check    func(t *testing.T, facts []core.Fact)
	}{
		{
			name:     "nil elements",
			elements: nil,
			wantLen:  0,
		},
		{
			name:     "empty elements",
			elements: []CodeElement{},
			wantLen:  0,
		},
		{
			name: "single simple element",
			elements: []CodeElement{
				{
					Ref:        "fn:main",
					Type:       ElementFunction,
					File:       "main.go",
					StartLine:  10,
					EndLine:    20,
					Signature:  "func main()",
					Visibility: VisibilityPublic,
				},
			},
			wantLen: 3, // code_element, element_signature, element_visibility
			check: func(t *testing.T, facts []core.Fact) {
				assert.Equal(t, "code_element", facts[0].Predicate)
				assert.Equal(t, []any{"fn:main", "/function", "main.go", int64(10), int64(20)}, facts[0].Args)

				assert.Equal(t, "element_signature", facts[1].Predicate)
				assert.Equal(t, []any{"fn:main", "func main()"}, facts[1].Args)

				assert.Equal(t, "element_visibility", facts[2].Predicate)
				assert.Equal(t, []any{"fn:main", "/public"}, facts[2].Args)
			},
		},
		{
			name: "element with parent and actions",
			elements: []CodeElement{
				{
					Ref:        "method:MyStruct.DoWork",
					Type:       ElementMethod,
					File:       "work.go",
					StartLine:  30,
					EndLine:    40,
					Signature:  "func (m *MyStruct) DoWork()",
					Visibility: VisibilityPrivate,
					Parent:     "struct:MyStruct",
					Actions:    []ActionType{ActionReplace, ActionDelete},
				},
			},
			wantLen: 6, // 3 basic + 1 parent + 2 actions
			check: func(t *testing.T, facts []core.Fact) {
				// Assert basic existence of predicates to avoid order flakiness if ToFacts changes order
				var predicates []string
				for _, f := range facts {
					predicates = append(predicates, f.Predicate)
				}

				assert.Contains(t, predicates, "code_element")
				assert.Contains(t, predicates, "element_signature")
				assert.Contains(t, predicates, "element_visibility")
				assert.Contains(t, predicates, "element_parent")
				assert.Contains(t, predicates, "code_interactable")

				// Specifically check parent
				var parentFact *core.Fact
				for _, f := range facts {
					if f.Predicate == "element_parent" {
						parentFact = &f
						break
					}
				}
				require.NotNil(t, parentFact)
				assert.Equal(t, []any{"method:MyStruct.DoWork", "struct:MyStruct"}, parentFact.Args)

				// Check interactable actions
				var interactableActions []any
				for _, f := range facts {
					if f.Predicate == "code_interactable" {
						interactableActions = append(interactableActions, f.Args[1])
					}
				}
				assert.ElementsMatch(t, []any{"/replace", "/delete"}, interactableActions)
			},
		},
		{
			name: "multiple elements",
			elements: []CodeElement{
				{
					Ref:        "struct:A",
					Type:       ElementStruct,
					File:       "a.go",
					StartLine:  1,
					EndLine:    5,
					Visibility: VisibilityPublic,
				},
				{
					Ref:        "struct:B",
					Type:       ElementStruct,
					File:       "b.go",
					StartLine:  1,
					EndLine:    5,
					Visibility: VisibilityPrivate,
				},
			},
			wantLen: 6, // 3 facts per struct
			check: func(t *testing.T, facts []core.Fact) {
				assert.Equal(t, "code_element", facts[0].Predicate)
				assert.Equal(t, "struct:A", facts[0].Args[0])

				assert.Equal(t, "code_element", facts[3].Predicate)
				assert.Equal(t, "struct:B", facts[3].Args[0])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			facts := ElementsToFacts(tt.elements)
			require.Len(t, facts, tt.wantLen)
			if tt.check != nil {
				tt.check(t, facts)
			}
		})
	}
}

func TestGetElementsByType(t *testing.T) {
	elements := []CodeElement{
		{Ref: "fn:main", Type: ElementFunction, File: "a.go", StartLine: 1, EndLine: 5},
		{Ref: "struct:Foo", Type: ElementStruct, File: "a.go", StartLine: 7, EndLine: 10},
		{Ref: "method:Foo.Bar", Type: ElementMethod, Parent: "struct:Foo", File: "a.go", StartLine: 12, EndLine: 15},
		{Ref: "fn:helper", Type: ElementFunction, File: "a.go", StartLine: 17, EndLine: 20},
		{Ref: "const:MaxConn", Type: ElementConst, File: "b.go", StartLine: 2, EndLine: 2},
	}

	tests := []struct {
		name     string
		elements []CodeElement
		elemType ElementType
		wantRefs []string // Compare refs for simplicity and verification
	}{
		{
			name:     "get multiple functions",
			elements: elements,
			elemType: ElementFunction,
			wantRefs: []string{"fn:main", "fn:helper"},
		},
		{
			name:     "get single struct",
			elements: elements,
			elemType: ElementStruct,
			wantRefs: []string{"struct:Foo"},
		},
		{
			name:     "get non-existent type",
			elements: elements,
			elemType: ElementInterface,
			wantRefs: nil,
		},
		{
			name:     "empty elements list",
			elements: []CodeElement{},
			elemType: ElementFunction,
			wantRefs: nil,
		},
		{
			name:     "nil elements list",
			elements: nil,
			elemType: ElementFunction,
			wantRefs: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetElementsByType(tt.elements, tt.elemType)

			var gotRefs []string
			if got != nil {
				for _, e := range got {
					gotRefs = append(gotRefs, e.Ref)
				}
			}

			if !reflect.DeepEqual(gotRefs, tt.wantRefs) {
				t.Errorf("GetElementsByType() = %v, want %v", gotRefs, tt.wantRefs)
			}
		})
	}
}

func TestGetElementsInRange_New(t *testing.T) {
	elements := []CodeElement{
		{Ref: "1", StartLine: 1, EndLine: 5},
		{Ref: "2", StartLine: 6, EndLine: 10},
		{Ref: "3", StartLine: 11, EndLine: 15},
		{Ref: "4", StartLine: 16, EndLine: 20},
		{Ref: "5", StartLine: 5, EndLine: 15},
	}

	tests := []struct {
		name      string
		startLine int
		endLine   int
		wantRefs  []string
	}{
		{
			name:      "exact match one element",
			startLine: 6,
			endLine:   10,
			wantRefs:  []string{"2", "5"},
		},
		{
			name:      "overlap start",
			startLine: 4,
			endLine:   8,
			wantRefs:  []string{"1", "2", "5"},
		},
		{
			name:      "overlap end",
			startLine: 9,
			endLine:   12,
			wantRefs:  []string{"2", "3", "5"},
		},
		{
			name:      "encompassing",
			startLine: 1,
			endLine:   20,
			wantRefs:  []string{"1", "2", "3", "4", "5"},
		},
		{
			name:      "completely outside before",
			startLine: -5,
			endLine:   0,
			wantRefs:  nil,
		},
		{
			name:      "completely outside after",
			startLine: 21,
			endLine:   30,
			wantRefs:  nil,
		},
		{
			name:      "empty slice",
			startLine: 1,
			endLine:   5,
			wantRefs:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var input []CodeElement
			if tt.name != "empty slice" {
				input = elements
			}

			got := GetElementsInRange(input, tt.startLine, tt.endLine)

			var gotRefs []string
			for _, e := range got {
				gotRefs = append(gotRefs, e.Ref)
			}

			if len(gotRefs) == 0 && len(tt.wantRefs) == 0 {
				return
			}
			assert.ElementsMatch(t, tt.wantRefs, gotRefs)
		})
	}
}
