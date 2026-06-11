package service

import (
	"math"
	"testing"

	"github.com/filogic/micro-services-valhalla/internal/model"
)

// newTestCalculator returns a calculator backed by the hard-coded
// default rates (NL 0.197, DE 0.269, BE 0.074, FR 0.20).
func newTestCalculator() *TollCalculator {
	return NewTollCalculator("/nonexistent-path-forces-default-rates")
}

func truckSpec(weight float64) *model.VehicleSpec {
	return &model.VehicleSpec{Weight: &weight}
}

// makePoints generates n synthetic shape points along a diagonal line.
func makePoints(n int) [][2]float64 {
	points := make([][2]float64, n)
	for i := range points {
		points[i] = [2]float64{52.0 + float64(i)*0.01, 5.0 + float64(i)*0.01}
	}
	return points
}

func TestCalculateBuildsContiguousSegments(t *testing.T) {
	route := &ValhallaResult{
		Legs: []ValhallaLeg{{
			Points: makePoints(41),
			Maneuvers: []ValhallaManeuver{
				// Two contiguous NL toll maneuvers → merged into one segment
				{Length: 5000, Time: 180, CountryCode: "NL", StreetNames: []string{"A2"}, BeginShapeIndex: 0, EndShapeIndex: 10},
				{Length: 3000, Time: 120, CountryCode: "NL", StreetNames: []string{"A2"}, BeginShapeIndex: 10, EndShapeIndex: 20},
				// Non-tolled local road → breaks the segment
				{Length: 1000, Time: 90, CountryCode: "NL", StreetNames: []string{"Dorpsstraat"}, BeginShapeIndex: 20, EndShapeIndex: 25},
				// New NL toll stretch → second segment
				{Length: 2000, Time: 60, CountryCode: "NL", StreetNames: []string{"A12"}, BeginShapeIndex: 25, EndShapeIndex: 30},
				// Country change without gap → third segment
				{Length: 4000, Time: 150, CountryCode: "DE", StreetNames: []string{"A3"}, BeginShapeIndex: 30, EndShapeIndex: 40},
			},
		}},
	}

	summary := newTestCalculator().Calculate(route, truckSpec(40))

	if len(summary.Segments) != 3 {
		t.Fatalf("expected 3 segments, got %d: %+v", len(summary.Segments), summary.Segments)
	}

	expected := []struct {
		distance  float64
		duration  float64
		cost      float64
		ratePerKm float64
		begin     int
		end       int
	}{
		{distance: 8000, duration: 300, cost: 1.58, ratePerKm: 0.197, begin: 0, end: 20},
		{distance: 2000, duration: 60, cost: 0.39, ratePerKm: 0.197, begin: 25, end: 30},
		{distance: 4000, duration: 150, cost: 1.08, ratePerKm: 0.269, begin: 30, end: 40},
	}

	leg := route.Legs[0]
	for i, want := range expected {
		seg := summary.Segments[i]
		if seg.Distance != want.distance {
			t.Errorf("segment %d distance: want %v, got %v", i, want.distance, seg.Distance)
		}
		if seg.Duration != want.duration {
			t.Errorf("segment %d duration: want %v, got %v", i, want.duration, seg.Duration)
		}
		if seg.Cost != want.cost {
			t.Errorf("segment %d cost: want %v, got %v", i, want.cost, seg.Cost)
		}
		if seg.RatePerKm == nil || *seg.RatePerKm != want.ratePerKm {
			t.Errorf("segment %d ratePerKm: want %v, got %v", i, want.ratePerKm, seg.RatePerKm)
		}

		// The segment polyline must decode back to the shape slice
		// covered by the merged maneuvers.
		decoded := decodePolyline(seg.Polyline)
		wantPoints := leg.Points[want.begin : want.end+1]
		if len(decoded) != len(wantPoints) {
			t.Fatalf("segment %d polyline: want %d points, got %d", i, len(wantPoints), len(decoded))
		}
		for j := range decoded {
			if math.Abs(decoded[j][0]-wantPoints[j][0]) > 1e-5 || math.Abs(decoded[j][1]-wantPoints[j][1]) > 1e-5 {
				t.Fatalf("segment %d polyline point %d: want %v, got %v", i, j, wantPoints[j], decoded[j])
			}
		}
	}

	if summary.TotalDistance != 14000 {
		t.Errorf("totalDistance: want 14000, got %v", summary.TotalDistance)
	}
	// 1.576 + 0.394 + 1.076 = 3.046 → 3.05 (rounded once at the end)
	if summary.TotalCost != 3.05 {
		t.Errorf("totalCost: want 3.05, got %v", summary.TotalCost)
	}
}

func TestCalculateWithoutVehicleReturnsEmptySummary(t *testing.T) {
	route := &ValhallaResult{
		Legs: []ValhallaLeg{{
			Points: makePoints(11),
			Maneuvers: []ValhallaManeuver{
				{Length: 5000, Time: 180, CountryCode: "NL", StreetNames: []string{"A2"}, BeginShapeIndex: 0, EndShapeIndex: 10},
			},
		}},
	}

	summary := newTestCalculator().Calculate(route, nil)

	if len(summary.Segments) != 0 || summary.TotalCost != 0 || summary.TotalDistance != 0 {
		t.Errorf("expected empty summary, got %+v", summary)
	}
}

func TestCalculateBelowMinWeightHasNoToll(t *testing.T) {
	route := &ValhallaResult{
		Legs: []ValhallaLeg{{
			Points: makePoints(11),
			Maneuvers: []ValhallaManeuver{
				{Length: 5000, Time: 180, CountryCode: "NL", StreetNames: []string{"A2"}, BeginShapeIndex: 0, EndShapeIndex: 10},
			},
		}},
	}

	// 2t is below the NL MinWeightTonnes of 3.5
	summary := newTestCalculator().Calculate(route, truckSpec(2))

	if len(summary.Segments) != 0 || summary.TotalCost != 0 {
		t.Errorf("expected no toll below minimum weight, got %+v", summary)
	}
}

func TestEncodePolylineRoundtrip(t *testing.T) {
	points := [][2]float64{
		{52.376234, 4.891567},
		{52.301112, 4.852201},
		{51.987654, 5.123456},
		{51.812345, 5.412345},
	}

	decoded := decodePolyline(encodePolyline(points))

	if len(decoded) != len(points) {
		t.Fatalf("want %d points, got %d", len(points), len(decoded))
	}
	for i := range points {
		if math.Abs(decoded[i][0]-points[i][0]) > 1e-6 || math.Abs(decoded[i][1]-points[i][1]) > 1e-6 {
			t.Errorf("point %d: want %v, got %v", i, points[i], decoded[i])
		}
	}
}
