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

// The function GetMethodsOfStruct is already tested in internal/world/code_elements_extra_test.go

func TestGetMethodsOfStruct_Extended(t *testing.T) {
	elements := []CodeElement{
		{Ref: "fn:main", Type: ElementFunction, File: "a.go", StartLine: 1, EndLine: 5},
		{Ref: "struct:Foo", Type: ElementStruct, File: "a.go", StartLine: 7, EndLine: 10},
		{Ref: "method:Foo.Bar", Type: ElementMethod, Parent: "struct:Foo", File: "a.go", StartLine: 12, EndLine: 15},
		{Ref: "method:Foo.Baz", Type: ElementMethod, Parent: "struct:Foo", File: "a.go", StartLine: 17, EndLine: 20},
		{Ref: "method:Bar.Qux", Type: ElementMethod, Parent: "struct:Bar", File: "b.go", StartLine: 22, EndLine: 25},
		{Ref: "const:MaxConn", Type: ElementConst, File: "b.go", StartLine: 2, EndLine: 2},
	}

	tests := []struct {
		name      string
		elements  []CodeElement
		structRef string
		wantRefs  []string
	}{
		{
			name:      "get multiple methods for struct",
			elements:  elements,
			structRef: "struct:Foo",
			wantRefs:  []string{"method:Foo.Bar", "method:Foo.Baz"},
		},
		{
			name:      "get single method for struct",
			elements:  elements,
			structRef: "struct:Bar",
			wantRefs:  []string{"method:Bar.Qux"},
		},
		{
			name:      "get methods for non-existent struct",
			elements:  elements,
			structRef: "struct:Unknown",
			wantRefs:  nil,
		},
		{
			name:      "empty elements list",
			elements:  []CodeElement{},
			structRef: "struct:Foo",
			wantRefs:  nil,
		},
		{
			name:      "nil elements list",
			elements:  nil,
			structRef: "struct:Foo",
			wantRefs:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetMethodsOfStruct(tt.elements, tt.structRef)

			var gotRefs []string
			if got != nil {
				for _, e := range got {
					gotRefs = append(gotRefs, e.Ref)
				}
			}

			if !reflect.DeepEqual(gotRefs, tt.wantRefs) {
				t.Errorf("GetMethodsOfStruct() = %v, want %v", gotRefs, tt.wantRefs)
			}
		})
	}
}

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

func TestGetElementsInRange_Extensive(t *testing.T) {
	elements := []CodeElement{
		{Ref: "fn:before", StartLine: 1, EndLine: 5},
		{Ref: "fn:overlap_start", StartLine: 4, EndLine: 10},
		{Ref: "fn:within", StartLine: 12, EndLine: 18},
		{Ref: "fn:encompassing", StartLine: 5, EndLine: 25},
		{Ref: "fn:overlap_end", StartLine: 15, EndLine: 22},
		{Ref: "fn:after", StartLine: 25, EndLine: 30},
		{Ref: "fn:exact_match", StartLine: 10, EndLine: 20},
	}

	tests := []struct {
		name      string
		elements  []CodeElement
		startLine int
		endLine   int
		wantRefs  []string
	}{
		{
			name:      "range 10 to 20",
			elements:  elements,
			startLine: 10,
			endLine:   20,
			wantRefs: []string{
				"fn:overlap_start", // 4-10 overlaps with 10-20
				"fn:within",        // 12-18 is within 10-20
				"fn:encompassing",  // 5-25 encompasses 10-20
				"fn:overlap_end",   // 15-22 overlaps with 10-20
				"fn:exact_match",   // 10-20 exactly matches 10-20
			},
		},
		{
			name:      "range 50 to 60 (no match)",
			elements:  elements,
			startLine: 50,
			endLine:   60,
			wantRefs:  nil,
		},
		{
			name:      "empty elements",
			elements:  []CodeElement{},
			startLine: 10,
			endLine:   20,
			wantRefs:  nil,
		},
		{
			name:      "nil elements",
			elements:  nil,
			startLine: 10,
			endLine:   20,
			wantRefs:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetElementsInRange(tt.elements, tt.startLine, tt.endLine)

			var gotRefs []string
			if got != nil {
				for _, e := range got {
					gotRefs = append(gotRefs, e.Ref)
				}
			}

			if !reflect.DeepEqual(gotRefs, tt.wantRefs) {
				t.Errorf("GetElementsInRange() = %v, want %v", gotRefs, tt.wantRefs)
			}
		})
	}
}
