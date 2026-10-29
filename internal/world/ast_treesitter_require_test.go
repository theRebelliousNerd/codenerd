package world

import (
	"slices"
	"testing"
)

func importsOf(facts []Fact) []string {
	var out []string
	for _, f := range facts {
		if f.Predicate == "dependency_link" && len(f.Args) == 3 {
			if imp, ok := f.Args[2].(string); ok {
				out = append(out, imp)
			}
		}
	}
	slices.Sort(out)
	return out
}

// A CommonJS require("pkg") is an import: before, a JavaScript or TypeScript
// file using it recorded no dependency_link, so nothing downstream (impact,
// the shard-expert import match) knew what it used. A require of a computed
// name, or a call of something else named require-like, is not an import.
func TestRequireCallsAreImports(t *testing.T) {
	p := NewTreeSitterParser()
	defer p.Close()
	src := []byte("const express = require(\"express\");\nconst cfg = require('./config');\nconst dyn = require(name);\nloader.require(\"nope\");\nimport x from \"react\";\n")
	js, err := p.ParseJavaScript("server.js", src)
	if err != nil {
		t.Fatalf("ParseJavaScript: %v", err)
	}
	if got := importsOf(js); !slices.Equal(got, []string{"./config", "express", "react"}) {
		t.Errorf("JavaScript imports = %v", got)
	}
	ts, err := p.ParseTypeScript("server.ts", []byte("const express = require(\"express\");\n"))
	if err != nil {
		t.Fatalf("ParseTypeScript: %v", err)
	}
	if got := importsOf(ts); !slices.Equal(got, []string{"express"}) {
		t.Errorf("TypeScript imports = %v", got)
	}
}
