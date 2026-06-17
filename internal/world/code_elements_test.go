package world

import (
	"reflect"
	"testing"
)

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
