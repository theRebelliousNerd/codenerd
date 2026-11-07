package mangle

import (
	"strings"
	"testing"
)

func TestSchemaDeclCache_ReusesSchemasScan(t *testing.T) {
	schemas := "Decl foo_bar(X, Y)\nDecl baz_qux(X)\nfoo_bar(1, 2).\n"
	// First validator populates the process cache.
	first := NewSchemaValidator(schemas, "learned_head_one(X) :- foo_bar(X, _).")
	if err := first.LoadDeclaredPredicates(); err != nil {
		t.Fatalf("first load failed: %v", err)
	}
	if !first.IsDeclared("foo_bar") || !first.IsDeclared("baz_qux") {
		t.Fatalf("first validator missing schemas decls: %v", first.GetDeclaredPredicates())
	}

	key := schemas
	if _, _, ok := cachedSchemaDecls(key); !ok {
		t.Fatalf("expected schemas scan to be cached")
	}

	// Second validator with identical schemas but different learned text must
	// reuse the cached schemas scan and still pick up its own learned heads.
	second := NewSchemaValidator(schemas, "other_learned_head(X) :- baz_qux(X).")
	if err := second.LoadDeclaredPredicates(); err != nil {
		t.Fatalf("second load failed: %v", err)
	}
	if !second.IsDeclared("foo_bar") || !second.IsDeclared("baz_qux") {
		t.Fatalf("cached load missing schemas decls: %v", second.GetDeclaredPredicates())
	}
	if !second.IsDeclared("other_learned_head") {
		t.Fatalf("second validator missing its own learned head")
	}
	if second.IsDeclared("learned_head_one") {
		t.Fatalf("learned heads leaked across validators via schemas cache")
	}
}

func TestSchemaDeclCache_DistinctSchemasDoNotCollide(t *testing.T) {
	a := "Decl alpha_pred(X)\n"
	b := "Decl beta_pred(X)\n"
	va := NewSchemaValidator(a, "")
	if err := va.LoadDeclaredPredicates(); err != nil {
		t.Fatalf("load a failed: %v", err)
	}
	vb := NewSchemaValidator(b, "")
	if err := vb.LoadDeclaredPredicates(); err != nil {
		t.Fatalf("load b failed: %v", err)
	}
	if !va.IsDeclared("alpha_pred") || va.IsDeclared("beta_pred") {
		t.Fatalf("schemas cache collision for a: %v", va.GetDeclaredPredicates())
	}
	if !vb.IsDeclared("beta_pred") || vb.IsDeclared("alpha_pred") {
		t.Fatalf("schemas cache collision for b: %v", vb.GetDeclaredPredicates())
	}
}

func TestSchemaDeclCache_SchemasParseErrorReturnsWrapped(t *testing.T) {
	bad := "Decl broken_cache_err_pred(X, Y\n"
	sv := NewSchemaValidator(bad, "")
	err := sv.LoadDeclaredPredicates()
	if err == nil {
		t.Fatalf("expected schemas parse error for unclosed Decl, got nil")
	}
	if _, _, ok := cachedSchemaDecls(bad); ok {
		t.Fatalf("failed schemas scan must not be cached")
	}
	sv2 := NewSchemaValidator(bad, "")
	if err2 := sv2.LoadDeclaredPredicates(); err2 == nil {
		t.Fatalf("expected retry of bad schemas to fail, got nil")
	}
}

func BenchmarkSchemaDeclCache_ColdVsWarm(b *testing.B) {
	buildSchemas := func(tag string, n int) string {
		var sb strings.Builder
		for i := 0; i < n; i++ {
			sb.WriteString("Decl bench_pred_" + tag + "_" + string(rune('a'+i%26)) + "_" + string(rune('a'+(i/26)%26)) + "(X, Y)\n")
		}
		return sb.String()
	}
	b.Run("cold_miss", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			schemas := buildSchemas("cold"+string(rune('0'+i%10))+string(rune('0'+(i/10)%10))+string(rune('0'+(i/100)%10)), 500)
			sv := NewSchemaValidator(schemas, "")
			if err := sv.LoadDeclaredPredicates(); err != nil {
				b.Fatalf("cold load failed: %v", err)
			}
		}
	})
	b.Run("warm_hit", func(b *testing.B) {
		schemas := buildSchemas("warm", 500)
		seed := NewSchemaValidator(schemas, "")
		if err := seed.LoadDeclaredPredicates(); err != nil {
			b.Fatalf("seed load failed: %v", err)
		}
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			sv := NewSchemaValidator(schemas, "warm_learned_head(X) :- bench_pred_warm_a_a(X, _).")
			if err := sv.LoadDeclaredPredicates(); err != nil {
				b.Fatalf("warm load failed: %v", err)
			}
		}
	})
}
