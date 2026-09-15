package core

import (
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

// A missing constitution file must fail with the path named, never return
// empty bytes that boot a hollow kernel.
func TestReadRequiredEmbeddedFile(t *testing.T) {
	fsys := fstest.MapFS{
		"defaults/schemas.mg": {Data: []byte("Decl ok(X) bound [/name].")},
	}
	data, err := readRequiredEmbeddedFile(fsys, "defaults/schemas.mg")
	if err != nil {
		t.Fatalf("present file: %v", err)
	}
	if string(data) != "Decl ok(X) bound [/name]." {
		t.Fatalf("content = %q", data)
	}

	_, err = readRequiredEmbeddedFile(fsys, "defaults/schemas_missing.mg")
	if err == nil {
		t.Fatal("missing file returned nil error")
	}
	if !strings.Contains(err.Error(), "defaults/schemas_missing.mg") {
		t.Fatalf("error does not name the file: %v", err)
	}
}

// Every file the boot inventory lists must actually be embedded. This pins
// today's mapping so a typo'd filename fails the suite instead of silently
// dropping a schema or policy module at boot.
func TestBootInventory_AllListedFilesEmbedded(t *testing.T) {
	files := []string{"defaults/schemas.mg"}
	for _, f := range defaultSchemaFiles {
		files = append(files, "defaults/"+f)
	}
	for _, m := range DefaultCorePolicyModules() {
		files = append(files, "defaults/"+m)
	}
	policyFiles, err := DefaultPolicyFiles()
	if err != nil {
		t.Fatalf("DefaultPolicyFiles: %v", err)
	}
	for _, f := range policyFiles {
		files = append(files, "defaults/"+f)
	}
	for _, f := range files {
		if _, err := readRequiredEmbeddedFile(coreLogic, f); err != nil {
			t.Errorf("inventory lists %s but it is not embedded: %v", f, err)
		}
	}
	if len(files) < 100 {
		t.Errorf("inventory unexpectedly small (%d files); lists drifted?", len(files))
	}
}

// All three constructors must boot a kernel with a live event bus.
// NewRealKernelWithPath once skipped the bus, leaving shard kernels with a
// nil bus whose Subscribe would panic.
func TestConstructors_EventBusInitialized(t *testing.T) {
	kernels := map[string]*RealKernel{}
	var err error
	if kernels["NewRealKernel"], err = NewRealKernel(); err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	if kernels["WithWorkspace"], err = NewRealKernelWithWorkspace(t.TempDir()); err != nil {
		t.Fatalf("NewRealKernelWithWorkspace: %v", err)
	}
	if kernels["WithPath"], err = NewRealKernelWithPath(t.TempDir()); err != nil {
		t.Fatalf("NewRealKernelWithPath: %v", err)
	}
	for name, k := range kernels {
		if k.GetEventBus() == nil {
			t.Errorf("%s: nil event bus", name)
		}
	}
}

// End-to-end behavioral loop per constructor: subscribe, assert, receive.
// Proves the bus is not just non-nil but wired into the Assert path.
func TestConstructors_AssertPublishesEvent(t *testing.T) {
	build := map[string]func() (*RealKernel, error){
		"NewRealKernel": func() (*RealKernel, error) { return NewRealKernel() },
		"WithWorkspace": func() (*RealKernel, error) { return NewRealKernelWithWorkspace(t.TempDir()) },
		"WithPath":      func() (*RealKernel, error) { return NewRealKernelWithPath(t.TempDir()) },
	}
	for name, fn := range build {
		t.Run(name, func(t *testing.T) {
			k, err := fn()
			if err != nil {
				t.Fatalf("boot: %v", err)
			}
			const pred = "uplift_boot_probe"
			ch := k.GetEventBus().Subscribe([]string{pred})
			defer k.GetEventBus().Unsubscribe(ch)
			if err := k.Assert(Fact{Predicate: pred, Args: []any{"v"}}); err != nil {
				t.Fatalf("Assert: %v", err)
			}
			select {
			case ev := <-ch:
				if ev.Predicate != pred {
					t.Fatalf("event predicate = %q, want %q", ev.Predicate, pred)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("no event published for Assert")
			}
		})
	}
}
