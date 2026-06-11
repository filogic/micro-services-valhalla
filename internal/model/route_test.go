package model

import (
	"encoding/json"
	"testing"
)

// TestTollSummaryJSONShape locks the toll wire format consumed by
// OpenTMS (TollKilometersService) and other clients.
func TestTollSummaryJSONShape(t *testing.T) {
	rate := 0.197
	summary := TollSummary{
		TotalCost:     36.91,
		TotalDistance: 187383,
		Segments: []TollSegment{{
			Cost:      36.91,
			Distance:  187383,
			Duration:  7316.884,
			RatePerKm: &rate,
			Polyline:  "_p~iF~ps|U",
		}},
	}

	data, err := json.Marshal(summary)
	if err != nil {
		t.Fatal(err)
	}

	want := `{"totalCost":36.91,"totalDistance":187383,"segments":[{"cost":36.91,"distance":187383,"duration":7316.884,"ratePerKm":0.197,"polyline":"_p~iF~ps|U"}]}`
	if string(data) != want {
		t.Errorf("toll payload shape changed:\nwant %s\ngot  %s", want, string(data))
	}
}
