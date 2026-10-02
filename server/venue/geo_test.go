package venue

import "testing"

func TestHaversineDistanceMeters(t *testing.T) {
	// Same point
	if HaversineDistanceMeters(40.0, -74.0, 40.0, -74.0) != 0 {
		t.Fatal("expected zero distance")
	}
	// ~1 degree latitude ≈ 111km
	d := HaversineDistanceMeters(0, 0, 1, 0)
	if d < 110000 || d > 112000 {
		t.Fatalf("unexpected distance %v", d)
	}
}
