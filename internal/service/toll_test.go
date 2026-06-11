package service

import (
	"math"
	"testing"

	"github.com/filogic/micro-services-valhalla/internal/model"
)

// newTestCalculator returns a calculator backed by the hard-coded
// default rates (NL 0.201, DE 0.348, BE 0.204, FR 0.20).
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
		{distance: 8000, duration: 300, cost: 1.61, ratePerKm: 0.201, begin: 0, end: 20},
		{distance: 2000, duration: 60, cost: 0.4, ratePerKm: 0.201, begin: 25, end: 30},
		{distance: 4000, duration: 150, cost: 1.39, ratePerKm: 0.348, begin: 30, end: 40},
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
	// 1.608 + 0.402 + 1.392 = 3.402 → 3.4 (rounded once at the end)
	if summary.TotalCost != 3.4 {
		t.Errorf("totalCost: want 3.4, got %v", summary.TotalCost)
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

// A short non-tolled stretch (unnamed ramp/connector) inside a tolled
// run must be bridged into the segment; a long one must split it.
func TestCalculateBridgesShortGapsOnly(t *testing.T) {
	makeRoute := func(gapMeters float64) *ValhallaResult {
		return &ValhallaResult{
			Legs: []ValhallaLeg{{
				Points: makePoints(31),
				Maneuvers: []ValhallaManeuver{
					{Length: 5000, Time: 180, CountryCode: "NL", StreetNames: []string{"A2"}, BeginShapeIndex: 0, EndShapeIndex: 10},
					{Length: gapMeters, Time: 20, CountryCode: "NL", StreetNames: nil, BeginShapeIndex: 10, EndShapeIndex: 15},
					{Length: 4000, Time: 150, CountryCode: "NL", StreetNames: []string{"A12"}, BeginShapeIndex: 15, EndShapeIndex: 30},
				},
			}},
		}
	}

	bridged := newTestCalculator().Calculate(makeRoute(300), truckSpec(40))
	if len(bridged.Segments) != 1 {
		t.Fatalf("300m gap: expected 1 bridged segment, got %d", len(bridged.Segments))
	}
	if bridged.Segments[0].Distance != 9300 {
		t.Errorf("bridged distance: want 9300 (incl. gap), got %v", bridged.Segments[0].Distance)
	}

	split := newTestCalculator().Calculate(makeRoute(1500), truckSpec(40))
	if len(split.Segments) != 2 {
		t.Fatalf("1500m gap: expected 2 segments, got %d", len(split.Segments))
	}
}

// Edge-level data must map to maneuvers with scaled times and country codes.
func TestBuildEdgeManeuvers(t *testing.T) {
	points := makePoints(21)
	edges := []traceEdge{
		{Names: []string{"A2"}, Length: 6, Speed: 100, BeginShapeIndex: 0, EndShapeIndex: 10},
		{Names: nil, Length: 0.3, Speed: 60, BeginShapeIndex: 10, EndShapeIndex: 12},
		{Names: []string{"A12"}, Length: 4, Speed: 100, BeginShapeIndex: 12, EndShapeIndex: 20},
	}

	maneuvers := buildEdgeManeuvers(edges, points, 400)
	if len(maneuvers) != 3 {
		t.Fatalf("expected 3 maneuvers, got %d", len(maneuvers))
	}

	totalTime, totalLength := 0.0, 0.0
	for _, m := range maneuvers {
		totalTime += m.Time
		totalLength += m.Length
		if m.CountryCode != "NL" {
			t.Errorf("expected NL country, got %q", m.CountryCode)
		}
	}
	if math.Abs(totalTime-400) > 0.01 {
		t.Errorf("times must sum to the leg duration: got %v", totalTime)
	}
	if math.Abs(totalLength-10300) > 0.5 {
		t.Errorf("lengths: want 10300m, got %v", totalLength)
	}
}

// Water crossings (Afsluitdijk) resolve to no country from the land
// polygons and must inherit the country of the preceding stretch.
func TestFillManeuverCountries(t *testing.T) {
	maneuvers := []ValhallaManeuver{
		{CountryCode: ""},
		{CountryCode: "NL"},
		{CountryCode: ""},
		{CountryCode: ""},
		{CountryCode: "NL"},
		{CountryCode: "DE"},
		{CountryCode: ""},
	}
	fillManeuverCountries(maneuvers)

	want := []string{"NL", "NL", "NL", "NL", "NL", "DE", "DE"}
	for i, m := range maneuvers {
		if m.CountryCode != want[i] {
			t.Errorf("maneuver %d: want %s, got %s", i, want[i], m.CountryCode)
		}
	}
}

// Real OSM data writes German (and some other) road refs with a space
// ("A 3", "B 43") while Dutch refs have none ("A2"). Both must match.
func TestIsTollRoadHandlesSpacedRefs(t *testing.T) {
	cases := []struct {
		country string
		names   []string
		want    bool
	}{
		{"DE", []string{"A 3"}, true},
		{"DE", []string{"B 43"}, true},
		{"DE", []string{"A3"}, true},
		{"DE", []string{"Hans-Thoma-Straße"}, false},
		{"NL", []string{"A2"}, true},
		{"NL", []string{"Rijksweg A74"}, true},
		{"NL", []string{"E19 E 19"}, true}, // A16 stretches carry only their E-number in OSM
		{"NL", []string{"N629"}, false},
		{"BE", []string{"E 40"}, true},
	}

	for _, c := range cases {
		if got := IsTollRoad(c.country, c.names); got != c.want {
			t.Errorf("IsTollRoad(%s, %v): want %v, got %v", c.country, c.names, c.want, got)
		}
	}
}

// A single maneuver crossing the NL/BE border (E19 Breda → Antwerp) must
// be split per country instead of being attributed to its midpoint —
// which can fall in a gap between the simplified border polygons.
func TestSplitManeuverByCountryAtBorder(t *testing.T) {
	const n = 21
	points := make([][2]float64, n)
	for i := 0; i < n; i++ {
		f := float64(i) / float64(n-1)
		points[i] = [2]float64{51.59 - (51.59-51.26)*f, 4.78 - (4.78-4.43)*f} // Breda → Antwerp
	}

	parts := splitManeuverByCountry(ValhallaManeuver{
		Length:          34000,
		Time:            1200,
		StreetNames:     []string{"E19"},
		BeginShapeIndex: 0,
		EndShapeIndex:   n - 1,
	}, points)

	if len(parts) < 2 {
		t.Fatalf("expected the border-crossing maneuver to split, got %d part(s): %+v", len(parts), parts)
	}
	if parts[0].CountryCode != "NL" {
		t.Errorf("first part: want NL, got %q", parts[0].CountryCode)
	}
	if parts[len(parts)-1].CountryCode != "BE" {
		t.Errorf("last part: want BE, got %q", parts[len(parts)-1].CountryCode)
	}

	totalLength, totalTime := 0.0, 0.0
	for i, part := range parts {
		if part.CountryCode == "" {
			t.Errorf("part %d has no country", i)
		}
		totalLength += part.Length
		totalTime += part.Time
		if i > 0 && parts[i-1].EndShapeIndex != part.BeginShapeIndex {
			t.Errorf("parts %d/%d not contiguous: %d != %d", i-1, i, parts[i-1].EndShapeIndex, part.BeginShapeIndex)
		}
	}
	if math.Abs(totalLength-34000) > 1 {
		t.Errorf("lengths must sum to the original: got %v", totalLength)
	}
	if math.Abs(totalTime-1200) > 0.01 {
		t.Errorf("times must sum to the original: got %v", totalTime)
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
