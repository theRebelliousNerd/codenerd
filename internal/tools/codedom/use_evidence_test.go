package codedom

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// S3 for a script rename: a file that did not reference the declaration
// gains a reference after the plan and before the write. The rename's sites
// are its read set, re-resolved under the commit lock.
func TestScriptRename_RefusesWhenANewReferenceAppearsBeforeTheWrite(t *testing.T) {
	dir := t.TempDir()
	hooks := writeFixture(t, dir, "hooks.tsx", "export function useAuth() {\n  return 1;\n}\n")
	writeFixture(t, dir, "card.tsx", "import { useAuth } from \"./hooks\";\nexport function Card() {\n  return useAuth();\n}\n")
	commitBeforeHook = func() {
		src := "import { useAuth } from \"./hooks\";\nexport function Late() {\n  return useAuth();\n}\n"
		_ = os.WriteFile(filepath.Join(dir, "late.tsx"), []byte(src), 0o644)
	}
	t.Cleanup(func() { commitBeforeHook = nil })
	from := StructureSymbol{Ref: "hooks.tsx:useAuth", Key: "useAuth", File: "hooks.tsx", Kind: "function"}
	_, err := executeScriptRepoint(ctxForDir(dir), from, "useSession", []string{"hooks.tsx", "card.tsx"})
	if err == nil || !strings.Contains(err.Error(), "optimistic conflict") {
		t.Fatalf("a rename must not leave a reference it never saw: %v", err)
	}
	if strings.Contains(readFile(t, hooks), "useSession") {
		t.Fatal("a refused rename must not write")
	}
}

func TestVerifyUseSource_RefusesOtherBytesAndOtherText(t *testing.T) {
	data := []byte("package a\n\nvar x = lib.Build()\n")
	src := string(data)
	start := strings.Index(src, "lib.Build")
	use := StructureUse{File: "a.go", Line: 3, Start: start, End: start + len("lib.Build"), Digest: sourceDigest(data), Text: "lib.Build"}
	if err := verifyUseSource("a.go", data, src, []StructureUse{use}); err != nil {
		t.Fatalf("the bytes the use was measured on verify: %v", err)
	}
	other := []byte(strings.Replace(src, "lib.Build", "lib.Bu1ld", 1))
	if err := verifyUseSource("a.go", other, string(other), []StructureUse{use}); err == nil || !strings.Contains(err.Error(), "changed after its uses were found") {
		t.Fatalf("a same-length substitution is another revision: %v", err)
	}
	stale := use
	stale.Digest = sourceDigest(other)
	if err := verifyUseSource("a.go", other, string(other), []StructureUse{stale}); err == nil || !strings.Contains(err.Error(), "no longer reads") {
		t.Fatalf("the span must still hold the text the use named: %v", err)
	}
	unrecorded := use
	unrecorded.Digest = ""
	if err := verifyUseSource("a.go", data, src, []StructureUse{unrecorded}); err == nil {
		t.Fatal("a use with no recorded revision is not applied")
	}
}
