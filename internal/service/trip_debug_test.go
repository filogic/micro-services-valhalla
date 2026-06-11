package service

import (
	"fmt"
	"os"
	"testing"

	"github.com/filogic/micro-services-valhalla/internal/model"
)

// Temporary diagnostic: feed a saved Valhalla response through the real
// parse + toll matching pipeline and report per-maneuver verdicts.
// Run with: TRIP_JSON=/tmp/trip.json go test -run TestDebugTripTollMatching -v
func TestDebugTripTollMatching(t *testing.T) {
	path := os.Getenv("TRIP_JSON")
	if path == "" {
		t.Skip("TRIP_JSON not set")
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	client := NewValhallaClient("unused")
	weight := 40.0
	req := &model.RouteRequest{Vehicle: &model.VehicleSpec{Weight: &weight}}
	result, err := client.parseResponse(body, req)
	if err != nil {
		t.Fatal(err)
	}

	var lastVerdict string
	var runKm float64
	flush := func() {
		if lastVerdict != "" {
			fmt.Printf("%8.1f km  %s\n", runKm, lastVerdict)
		}
		runKm = 0
	}

	for li, leg := range result.Legs {
		fmt.Printf("== leg %d ==\n", li)
		for _, m := range leg.Maneuvers {
			toll := m.CountryCode != "" && IsTollRoad(m.CountryCode, m.StreetNames)
			verdict := fmt.Sprintf("country=%-3s toll=%-5v names=%v", m.CountryCode, toll, m.StreetNames)
			if verdict != lastVerdict {
				flush()
				lastVerdict = verdict
			}
			runKm += m.Length / 1000
		}
	}
	flush()
}
