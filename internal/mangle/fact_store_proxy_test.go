package mangle

import (
	"github.com/google/mangle/ast"
	"github.com/google/mangle/factstore"
	"testing"
)

func TestFactStoreProxy_Initialization(t *testing.T) {
	baseStore := factstore.NewSimpleInMemoryStore()
	proxy := NewFactStoreProxy(baseStore)

	if proxy == nil {
		t.Fatal("Expected proxy to be initialized, got nil")
	}

	if proxy.FactStore == nil {
		t.Errorf("Expected base store to be set correctly")
	}

	if proxy.lazyLoaders == nil {
		t.Errorf("Expected lazyLoaders to be initialized")
	}
}

func TestFactStoreProxy_RegisterLoader(t *testing.T) {
	baseStore := factstore.NewSimpleInMemoryStore()
	proxy := NewFactStoreProxy(baseStore)

	predicate := "test_pred"
	called := false
	loader := func(atom ast.Atom) bool {
		called = true
		return true
	}

	proxy.RegisterLoader(predicate, loader)

	if proxy.lazyLoaders[predicate] == nil {
		t.Errorf("Expected loader to be registered")
	}

	proxy.lazyLoaders[predicate](ast.Atom{})

	if !called {
		t.Errorf("Expected registered loader to be callable and update state")
	}
}

func TestFactStoreProxy_GetFacts(t *testing.T) {
	baseStore := factstore.NewSimpleInMemoryStore()
	proxy := NewFactStoreProxy(baseStore)

	predicate := "test_pred"
	called := false
	loader := func(atom ast.Atom) bool {
		called = true
		return true
	}
	proxy.RegisterLoader(predicate, loader)

	// Create query atom: test_pred("arg")
	query := ast.NewAtom(predicate, ast.String("arg"))

	err := proxy.GetFacts(query, func(a ast.Atom) error {
		return nil
	})

	if err != nil {
		t.Errorf("GetFacts returned error: %v", err)
	}

	if !called {
		t.Errorf("Expected loader to be called by GetFacts")
	}
}
