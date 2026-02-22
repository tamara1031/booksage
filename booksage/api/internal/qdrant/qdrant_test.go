package qdrant

import (
	"testing"
)

func TestToFloat32Slice_Float32(t *testing.T) {
	input := []float32{1.0, 2.0, 3.0}
	result, err := toFloat32Slice(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 3 || result[0] != 1.0 || result[1] != 2.0 || result[2] != 3.0 {
		t.Errorf("unexpected result: %v", result)
	}
}

func TestToFloat32Slice_Float64(t *testing.T) {
	input := []float64{1.1, 2.2, 3.3}
	result, err := toFloat32Slice(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 3 {
		t.Errorf("expected 3 elements, got %d", len(result))
	}
	if result[0] != float32(1.1) {
		t.Errorf("expected %v, got %v", float32(1.1), result[0])
	}
}

func TestToFloat32Slice_AnyFloat64(t *testing.T) {
	input := []any{float64(1.0), float64(2.0), float64(3.0)}
	result, err := toFloat32Slice(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 3 {
		t.Errorf("expected 3 elements, got %d", len(result))
	}
}

func TestToFloat32Slice_AnyFloat32(t *testing.T) {
	input := []any{float32(1.0), float32(2.0)}
	result, err := toFloat32Slice(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("expected 2 elements, got %d", len(result))
	}
}

func TestToFloat32Slice_UnsupportedType(t *testing.T) {
	_, err := toFloat32Slice("not a slice")
	if err == nil {
		t.Fatal("expected error for unsupported type")
	}
}

func TestToFloat32Slice_AnyUnsupportedElement(t *testing.T) {
	input := []any{float64(1.0), "not a number"}
	_, err := toFloat32Slice(input)
	if err == nil {
		t.Fatal("expected error for unsupported element type")
	}
}

func TestToFloat32Slice_Empty(t *testing.T) {
	result, err := toFloat32Slice([]float32{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected empty slice, got %v", result)
	}
}

func TestDeterministicID(t *testing.T) {
	// Same input should produce same output
	id1 := deterministicID("doc-1-chunk-0")
	id2 := deterministicID("doc-1-chunk-0")
	if id1 != id2 {
		t.Errorf("expected deterministic: %d != %d", id1, id2)
	}

	// Different input should produce different output
	id3 := deterministicID("doc-1-chunk-1")
	if id1 == id3 {
		t.Error("expected different IDs for different inputs")
	}

	// Should produce non-zero value
	if id1 == 0 {
		t.Error("expected non-zero ID")
	}
}
