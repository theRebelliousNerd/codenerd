package mcp

import (
	"math"
	"testing"
)

// --- float32SliceToBytes / bytesToFloat32Slice ---

func TestFloat32SliceToBytes_WhenValid_ShouldRoundTrip(t *testing.T) {
	input := []float32{1.0, 2.0, 3.0, -0.5}
	bytes := float32SliceToBytes(input)
	if len(bytes) != len(input)*4 {
		t.Fatalf("expected %d bytes, got %d", len(input)*4, len(bytes))
	}

	result := bytesToFloat32Slice(bytes)
	if len(result) != len(input) {
		t.Fatalf("expected %d floats, got %d", len(input), len(result))
	}
	for i, v := range input {
		if result[i] != v {
			t.Errorf("result[%d] = %v, want %v", i, result[i], v)
		}
	}
}

func TestFloat32SliceToBytes_WhenEmpty_ShouldReturnEmpty(t *testing.T) {
	bytes := float32SliceToBytes(nil)
	if len(bytes) != 0 {
		t.Errorf("expected empty bytes, got %d", len(bytes))
	}
}

func TestBytesToFloat32Slice_WhenEmpty_ShouldReturnEmpty(t *testing.T) {
	result := bytesToFloat32Slice(nil)
	if len(result) != 0 {
		t.Errorf("expected empty result, got %d", len(result))
	}
}

// --- cosineSimilarity ---

func TestCosineSimilarity_WhenIdentical_ShouldReturnOne(t *testing.T) {
	a := []float32{1.0, 2.0, 3.0}
	got := cosineSimilarity(a, a)
	if math.Abs(got-1.0) > 0.0001 {
		t.Errorf("cosineSimilarity(identical) = %v, want ~1.0", got)
	}
}

func TestCosineSimilarity_WhenOrthogonal_ShouldReturnZero(t *testing.T) {
	a := []float32{1.0, 0.0}
	b := []float32{0.0, 1.0}
	got := cosineSimilarity(a, b)
	if math.Abs(got) > 0.0001 {
		t.Errorf("cosineSimilarity(orthogonal) = %v, want ~0.0", got)
	}
}

func TestCosineSimilarity_WhenOpposite_ShouldReturnNegOne(t *testing.T) {
	a := []float32{1.0, 0.0}
	b := []float32{-1.0, 0.0}
	got := cosineSimilarity(a, b)
	if math.Abs(got+1.0) > 0.0001 {
		t.Errorf("cosineSimilarity(opposite) = %v, want ~-1.0", got)
	}
}

func TestCosineSimilarity_WhenZeroVector_ShouldReturnZero(t *testing.T) {
	a := []float32{0.0, 0.0}
	b := []float32{1.0, 2.0}
	got := cosineSimilarity(a, b)
	if got != 0.0 {
		t.Errorf("cosineSimilarity(zero, nonzero) = %v, want 0", got)
	}
}

func TestCosineSimilarity_WhenDifferentLengths_ShouldHandleGracefully(t *testing.T) {
	a := []float32{1.0, 2.0, 3.0}
	b := []float32{1.0, 2.0}
	// Should not panic, just compute partial
	got := cosineSimilarity(a, b)
	_ = got // Just ensure no panic
}
