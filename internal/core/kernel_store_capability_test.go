package core

import (
	"reflect"
	"testing"

	"codeberg.org/TauCeti/mangle-go/ast"
)

// A clean kernel's evaluated store becomes the immutable base of every clone
// it hands out (kernel_overlay_store.go). Immutability holds only while every
// write goes through the kernel's own ownership check, so no exported method
// may hand out a writable store: a handle retained before Clone could add a
// row to the shared base and every clone would read it, with nothing in the
// clone's EDB, index or generation to say so. RealKernel.GetStore returned
// exactly that handle, with no caller (audit
// mangle-overlay-raw-store-ownership).
func TestKernels_ExposeNoWritableFactStore(t *testing.T) {
	// The write half of the engine's FactStore contract.
	type atomWriter interface {
		Add(ast.Atom) bool
	}
	writer := reflect.TypeOf((*atomWriter)(nil)).Elem()
	for _, typ := range []reflect.Type{
		reflect.TypeOf(&RealKernel{}),
		reflect.TypeOf(&CortexKernel{}),
	} {
		for i := 0; i < typ.NumMethod(); i++ {
			m := typ.Method(i)
			for j := 0; j < m.Type.NumOut(); j++ {
				out := m.Type.Out(j)
				if out.Implements(writer) {
					t.Errorf("%s.%s returns %s, a writable fact store: a retained handle bypasses the clone base's ownership check", typ, m.Name, out)
				}
			}
		}
	}
}
