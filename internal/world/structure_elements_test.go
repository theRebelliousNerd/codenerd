package world

import (
	"context"
	"slices"
	"testing"
)

// ElementRefs spells refs as a session's turn_changed_element does, and shows
// two files under one ref when a package clause name repeats.
func TestStructureElementRefs_SpellRefsLikeTurnChangedElement(t *testing.T) {
	root := t.TempDir()
	writeStructFile(t, root, "go.mod", "module fixture\n\ngo 1.22\n")
	writeStructFile(t, root, "a/util/u.go", "package util\n\ntype Box[T any] struct{}\n\nfunc (b *Box[T]) Get() {}\n\nfunc Do() {}\n")
	writeStructFile(t, root, "b/util/u.go", "package util\n\nfunc Do() {}\n")
	idx := NewStructureIndex(root)
	refs, _, err := idx.ElementRefs(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got := refs["fn:util.Box.Get"]; !slices.Equal(got, []string{"a/util/u.go"}) {
		t.Errorf("method ref files %v", got)
	}
	if got := refs["fn:util.Do"]; len(got) != 2 {
		t.Errorf("a repeated package clause and name must show both files, got %v", got)
	}
	if _, ok := refs["fn:util.Box"]; ok {
		t.Error("a type is not a function ref")
	}
}
