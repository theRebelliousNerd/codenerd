package world

import (
	"fmt"
	"strings"

	"codenerd/internal/tools/codedom"
)

// loadElements joins the two production representations once per analysis.
// CodeDOM refs use fn:pkg.Name while Cartographer call edges use pkg.Name.
// Preserve CodeDOM identities and adapt holographic definitions when no parsed
// element exists yet; do not require a prior get_elements tool call.
func (b *TestDependencyBuilder) loadElements() error {
	elements, err := b.kernel.Query("code_element")
	if err != nil {
		return err
	}
	defines, err := b.kernel.Query("code_defines")
	if err != nil {
		return err
	}
	b.elements = nil
	b.aliases = make(map[string]string)
	seen := make(map[string]bool)
	add := func(f codedom.FactData) {
		if len(f.Args) != 5 {
			return
		}
		ref, file := fmt.Sprint(f.Args[0]), fmt.Sprint(f.Args[2])
		key := file + "\x00" + ref
		if seen[key] {
			return
		}
		seen[key] = true
		b.elements = append(b.elements, f)
		if i := strings.IndexByte(ref, ':'); i >= 0 {
			b.aliases[ref[i+1:]] = ref
		}
		if b.isTestFile(file) {
			b.testFiles[file] = true
		}
	}
	for _, f := range elements {
		add(f)
	}
	for _, f := range defines {
		if len(f.Args) != 5 {
			continue
		}
		symbol, kind := fmt.Sprint(f.Args[1]), fmt.Sprint(f.Args[2])
		prefix := strings.TrimPrefix(kind, "/")
		if prefix == "function" {
			prefix = "fn"
			if strings.Count(symbol, ".") > 1 {
				prefix = "method"
			}
		}
		add(codedom.FactData{Predicate: "code_element", Args: []any{prefix + ":" + symbol, kind, f.Args[0], f.Args[3], f.Args[4]}})
	}
	return nil
}
