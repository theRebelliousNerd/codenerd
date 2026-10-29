package world

import (
	"context"
	"testing"
)

// A method is reached through values of types the index does not know: an
// interface that declares its name mentions it in production code, and a test
// that calls it mentions it only in a test. Declarations, the method's own and
// another type's of the same name, are not mentions.
func TestStructureIndexNameMentions(t *testing.T) {
	root := t.TempDir()
	writeStructFile(t, root, "go.mod", "module fixture\n\ngo 1.24\n")
	writeStructFile(t, root, "lib/lib.go", `package lib

type Runner interface{ Serve() error }

type A struct{}

func (A) Serve() error { return nil }

func (A) Lonely() {}

func (A) Tested() {}

type B struct{}

func (B) Lonely() {}
`)
	writeStructFile(t, root, "lib/lib_test.go", `package lib

import "testing"

func TestTested(t *testing.T) { A{}.Tested() }
`)
	idx := NewStructureIndex(root)
	ctx := context.Background()
	for _, tc := range []struct {
		name             string
		production, test int
	}{
		{"Serve", 1, 0},
		{"Lonely", 0, 0},
		{"Tested", 0, 1},
	} {
		m, stats, err := idx.NameMentions(ctx, tc.name)
		if err != nil {
			t.Fatalf("NameMentions(%s): %v", tc.name, err)
		}
		if len(stats.Deficits) > 0 {
			t.Fatalf("deficits: %v", stats.Deficits)
		}
		if m.Production != tc.production || m.Test != tc.test {
			t.Errorf("NameMentions(%s) = %+v, want production %d test %d", tc.name, m, tc.production, tc.test)
		}
	}
}
