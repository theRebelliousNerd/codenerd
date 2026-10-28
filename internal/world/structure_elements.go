package world

import (
	"context"

	"codenerd/internal/world/codemodel"
)

// ElementRefs maps every Go function and method to the code_element ref a
// session's turn_changed_element names for it (fn:<pkg>.<Name> or
// fn:<pkg>.<Recv>.<Name>, spelled from the package clause), and to the files
// that declare that ref. Two packages sharing a clause name and a function
// name collide on one ref: more than one file under a ref means the ref does
// not name one element, which a caller declaring contracts over refs must see.
func (s *StructureIndex) ElementRefs(ctx context.Context) (map[string][]string, StructureStats, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	stats, err := s.refreshLocked(ctx)
	if err != nil {
		return nil, stats, err
	}
	out := make(map[string][]string)
	for canonical, f := range s.files {
		if f.lang != codemodel.LangGo {
			continue
		}
		for _, sym := range f.symbols {
			if sym.Kind != string(codemodel.KindFunction) && sym.Kind != string(codemodel.KindMethod) {
				continue
			}
			ref := "fn:" + sym.pkg + "." + sym.Key
			if files := out[ref]; len(files) == 0 || files[len(files)-1] != canonical {
				out[ref] = append(out[ref], canonical)
			}
		}
	}
	return out, stats, nil
}
