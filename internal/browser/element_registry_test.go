package browser

import "testing"

func TestElementRegistry_ReusesRefsWithinGeneration(t *testing.T) {
	registry := NewElementRegistry()
	first := registry.RegisterBatch([]ElementFingerprint{{TagName: "button", ID: "save", Selector: "#save"}})
	second := registry.RegisterBatch([]ElementFingerprint{{TagName: "button", ID: "save", Selector: "#save"}})
	if first[0].Ref == "" || second[0].Ref != first[0].Ref {
		t.Fatalf("ref was not stable: first=%q second=%q", first[0].Ref, second[0].Ref)
	}
	if first[0].Generation != 1 || second[0].Generation != 1 {
		t.Fatalf("unexpected generation: first=%d second=%d", first[0].Generation, second[0].Generation)
	}
}

func TestElementRegistry_ClearInvalidatesRefs(t *testing.T) {
	registry := NewElementRegistry()
	first := registry.RegisterBatch([]ElementFingerprint{{TagName: "input", Name: "email", Selector: "input[name=email]"}})[0]
	registry.Clear()
	if _, ok := registry.Get(first.Ref); ok {
		t.Fatalf("stale ref %q survived clear", first.Ref)
	}
	second := registry.RegisterBatch([]ElementFingerprint{{TagName: "input", Name: "email", Selector: "input[name=email]"}})[0]
	if second.Generation != first.Generation+1 || second.Ref == first.Ref {
		t.Fatalf("navigation generation was not reflected in ref: first=%+v second=%+v", first, second)
	}
}

func TestElementRegistry_RejectsSnapshotFromPriorGeneration(t *testing.T) {
	registry := NewElementRegistry()
	generation := registry.Generation()
	registry.Clear()
	registered, ok := registry.RegisterBatchForGeneration(generation, []ElementFingerprint{{TagName: "button", Selector: "#old"}})
	if ok || registered != nil || registry.Count() != 0 {
		t.Fatalf("stale observation repopulated registry: ok=%v registered=%v count=%d", ok, registered, registry.Count())
	}
}

func TestElementRegistry_GetReturnsCopy(t *testing.T) {
	registry := NewElementRegistry()
	registered := registry.RegisterBatch([]ElementFingerprint{{
		TagName: "button", Selector: "#save", Classes: []string{"primary"}, BoundingBox: map[string]float64{"width": 10},
	}})[0]
	copy, ok := registry.Get(registered.Ref)
	if !ok {
		t.Fatal("registered ref not found")
	}
	copy.Classes[0] = "mutated"
	copy.BoundingBox["width"] = 999
	again, _ := registry.Get(registered.Ref)
	if again.Classes[0] != "primary" || again.BoundingBox["width"] != 10 {
		t.Fatalf("registry state leaked through Get: %+v", again)
	}
}
