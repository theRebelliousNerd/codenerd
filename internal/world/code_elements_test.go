package world

import (
	"testing"

	"codenerd/internal/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

)

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
