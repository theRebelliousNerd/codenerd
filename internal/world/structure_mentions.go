package world

import (
	"context"
	"strings"

	"codenerd/internal/world/codemodel"
)

// StructMentions counts the identifiers spelled one name in the Go files of
// the index that are not a declaration's own name, split by whether the file
// is a test.
type StructMentions struct {
	Production int
	Test       int
}

// NameMentions counts, over every Go file, the identifiers spelled name less
// the declarations of that name in the same file. It answers by name, the
// way Unreferenced does: a call, a selector, a value, a field of an interface
// type or a struct literal key all count. It is the reachability question for
// a method, whose calls go through values of types the index does not know:
// an interface that declares the name, or a call through a value of an
// interface type, mentions it where Uses (which looks only in files naming the
// receiver type) does not.
func (s *StructureIndex) NameMentions(ctx context.Context, name string) (StructMentions, StructureStats, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	stats, err := s.refreshLocked(ctx)
	if err != nil {
		return StructMentions{}, stats, err
	}
	var m StructMentions
	for canonical, f := range s.files {
		if f.lang != codemodel.LangGo {
			continue
		}
		n := f.idents[name]
		for _, sym := range f.symbols {
			if sym.name == name {
				n--
			}
		}
		if n <= 0 {
			continue
		}
		if strings.HasSuffix(canonical, "_test.go") {
			m.Test += n
		} else {
			m.Production += n
		}
	}
	return m, stats, nil
}
