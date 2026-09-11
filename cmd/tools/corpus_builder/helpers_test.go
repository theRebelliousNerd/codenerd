package main

import (
	"encoding/binary"
	"math"
	"strings"
	"testing"
)

func TestContainsAndCleanAtom(t *testing.T) {
	if !contains([]string{"a", "b"}, "a") || contains([]string{"a"}, "z") {
		t.Error("contains misbehaves")
	}
	if cleanAtom("/intent") != "intent" {
		t.Errorf("cleanAtom(/intent)=%q, want intent", cleanAtom("/intent"))
	}
	if cleanAtom("plain") != "plain" {
		t.Errorf("cleanAtom(plain)=%q, want plain", cleanAtom("plain"))
	}
}

func TestNullableStringCorpus(t *testing.T) {
	if nullableString("") != nil {
		t.Error("empty -> nil")
	}
	if nullableString("x") != "x" {
		t.Error("non-empty -> itself")
	}
}

func TestEncodeFloat32SliceCorpus(t *testing.T) {
	vec := []float32{0.5, -1.25}
	buf := encodeFloat32Slice(vec)
	if len(buf) != 8 {
		t.Fatalf("len=%d, want 8", len(buf))
	}
	for i, want := range vec {
		if got := math.Float32frombits(binary.LittleEndian.Uint32(buf[i*4:])); got != want {
			t.Errorf("vec[%d]=%v decoded %v", i, want, got)
		}
	}
}

func TestFindMGFiles(t *testing.T) {
	files, err := findMGFiles()
	if err != nil {
		t.Fatalf("findMGFiles: %v", err)
	}
	// Three files are listed by hand (taxonomy, doc_taxonomy, build_topology);
	// the schema directory is walked, and the intent monolith that used to be
	// the fourth hand-listed entry is deleted — its facts live in the modular
	// schema/intent_*.mg files the walk finds.
	if len(files) < 3 {
		t.Fatalf("expected at least the known .mg files, got %d: %v", len(files), files)
	}
	for _, f := range files {
		if strings.HasSuffix(f, "schema/intent.mg") || strings.HasSuffix(f, `schema\intent.mg`) {
			t.Errorf("the deleted intent monolith is still listed: %v", files)
		}
	}
	var hasTaxonomy bool
	for _, f := range files {
		if strings.HasSuffix(f, "taxonomy.mg") {
			hasTaxonomy = true
		}
	}
	if !hasTaxonomy {
		t.Errorf("expected taxonomy.mg among known files: %v", files)
	}
}
