package tools_test

import (
	"codenerd/internal/tools"
	"codenerd/internal/tools/codedom"
	"codenerd/internal/tools/core"
	"codenerd/internal/tools/research"
	"codenerd/internal/tools/shell"
	"testing"
)

func TestProductionCatalogDeclaresEveryEffect(t *testing.T) {
	r := tools.NewRegistry()
	for _, register := range []func(*tools.Registry) error{core.RegisterAll, codedom.RegisterAll, research.RegisterAll, shell.RegisterAll} {
		if err := register(r); err != nil {
			t.Fatal(err)
		}
	}
	if len(r.All()) == 0 {
		t.Fatal("production catalog empty")
	}
	for _, tool := range r.All() {
		if _, err := tool.DeclaredEffect(); err != nil {
			t.Error(err)
		}
	}
	if got, err := (&tools.Tool{Name: "run_tests", Effect: tools.EffectRead}).DeclaredEffect(); err != nil || got != tools.EffectExecute {
		t.Fatal("builtin executable effect can be downgraded")
	}
}
