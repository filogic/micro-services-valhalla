package service

import (
	"encoding/json"
	"fmt"
	"math"
	"os"

	"github.com/filogic/micro-services-valhalla/internal/model"
)

// ── Data structures ─────────────────────────────────────────────────

// TollConfig holds the toll configuration for a single country.
type TollConfig struct {
	Country            string            `json:"country"`
	Operator           string            `json:"operator"`
	System             string            `json:"system"`
	MinWeightTonnes    float64           `json:"minWeightTonnes"`
	TolledRoadFraction float64           `json:"tolledRoadFraction"`
	WeightClasses      []WeightClassRate `json:"weightClasses"`
	// CO2ClassRates holds flat rates per weight bracket for CO₂ classes 2-5.
	// Used by NL and DE where trucks with better CO₂ efficiency get lower rates
	// that no longer depend on Euro emission class.
	// Key = CO₂ class as string ("2".."5").
	CO2ClassRates map[string][]CO2WeightRate `json:"co2ClassRates,omitempty"`
}

// WeightClassRate maps Euro emission classes to rates (EUR/km) for
// a specific gross vehicle weight range.
type WeightClassRate struct {
	Min   float64            `json:"min"`  // inclusive, tonnes
	Max   float64            `json:"max"`  // exclusive, tonnes
	Rates map[string]float64 `json:"rates"` // euroClass → EUR/km
}

// CO2WeightRate is a flat rate per weight bracket, used when a truck's
// CO₂ emission class qualifies for reduced tolls (class 2-5).
type CO2WeightRate struct {
	Min  float64 `json:"min"`  // inclusive, tonnes
	Max  float64 `json:"max"`  // exclusive, tonnes
	Rate float64 `json:"rate"` // EUR/km
}

// ── Calculator ──────────────────────────────────────────────────────

type TollCalculator struct {
	configs map[string]TollConfig
}

func NewTollCalculator(dataPath string) *TollCalculator {
	tc := &TollCalculator{}
	tc.configs = tc.loadRates(dataPath)
	return tc
}

func (tc *TollCalculator) Calculate(route *ValhallaResult, vehicle *model.VehicleSpec) model.TollSummary {
	summary := model.TollSummary{
		Segments: []model.TollSegment{},
	}

	if vehicle == nil {
		return summary
	}

	weight := vehicle.EffectiveWeight()
	euroClass := vehicle.EffectiveEuroClass()
	co2Class := vehicle.EffectiveCO2Class()

	// Resolve the EUR/km rate per country once; 0 means "not tolled
	// for this vehicle" (no config, below MinWeightTonnes, or no rate).
	rateCache := make(map[string]float64)
	rateFor := func(country string) float64 {
		if rate, ok := rateCache[country]; ok {
			return rate
		}

		rate := 0.0
		if config, ok := tc.configs[country]; ok && weight >= config.MinWeightTonnes {
			// 1. Try CO₂ class rates (NL/DE class 2-5: flat rate per weight bracket)
			if co2Class > 1 && config.CO2ClassRates != nil {
				classKey := fmt.Sprintf("%d", co2Class)
				if classRates, ok := config.CO2ClassRates[classKey]; ok {
					rate = findCO2Rate(classRates, weight)
				}
			}

			// 2. Fall back to Euro-class rate from the matching weight class
			if rate <= 0 {
				rate = findWeightClassRate(config.WeightClasses, weight, euroClass)
			}
			if rate < 0 {
				rate = 0
			}
		}

		rateCache[country] = rate
		return rate
	}

	// Walk the maneuvers and accumulate contiguous tolled stretches.
	// A segment ends when a non-tolled maneuver, a country change or a
	// leg boundary is encountered.
	totalCost := 0.0

	type openSegment struct {
		country  string
		rate     float64
		distance float64 // meters
		duration float64 // seconds
		begin    int     // shape index into the leg polyline
		end      int
	}

	for _, leg := range route.Legs {
		var cur *openSegment

		flush := func() {
			if cur == nil {
				return
			}
			seg := *cur
			cur = nil
			if seg.distance <= 0 {
				return
			}

			cost := (seg.distance / 1000.0) * seg.rate
			totalCost += cost

			rate := seg.rate
			out := model.TollSegment{
				Cost:      math.Round(cost*100) / 100,
				Distance:  math.Round(seg.distance),
				Duration:  math.Round(seg.duration*1000) / 1000,
				RatePerKm: &rate,
			}
			if pts := sliceShape(leg.Points, seg.begin, seg.end); len(pts) >= 2 {
				out.Polyline = encodePolyline(pts)
			}

			summary.TotalDistance += out.Distance
			summary.Segments = append(summary.Segments, out)
		}

		// Short non-tolled stretches inside a tolled run (unnamed ramps,
		// junction connectors — common with edge-level data) are bridged
		// into the segment instead of splitting it.
		const maxBridgeGapMeters = 600.0
		gapLength, gapDuration := 0.0, 0.0
		gapEnd := 0

		for _, m := range leg.Maneuvers {
			rate := 0.0
			if m.CountryCode != "" && IsTollRoad(m.CountryCode, m.StreetNames) {
				rate = rateFor(m.CountryCode)
			}

			if rate <= 0 {
				if cur != nil {
					gapLength += m.Length
					gapDuration += m.Time
					gapEnd = m.EndShapeIndex
					if gapLength >= maxBridgeGapMeters {
						flush()
						gapLength, gapDuration = 0, 0
					}
				}
				continue
			}
			if cur != nil && cur.country != m.CountryCode {
				flush()
				gapLength, gapDuration = 0, 0
			}
			if cur != nil && gapLength > 0 {
				cur.distance += gapLength
				cur.duration += gapDuration
				cur.end = gapEnd
				gapLength, gapDuration = 0, 0
			}
			if cur == nil {
				cur = &openSegment{
					country: m.CountryCode,
					rate:    rate,
					begin:   m.BeginShapeIndex,
					end:     m.EndShapeIndex,
				}
			}
			cur.distance += m.Length
			cur.duration += m.Time
			cur.end = m.EndShapeIndex
		}
		flush()
	}

	summary.TotalCost = math.Round(totalCost*100) / 100

	return summary
}

// sliceShape returns the polyline points from begin to end (inclusive),
// clamped to valid bounds.
func sliceShape(points [][2]float64, begin, end int) [][2]float64 {
	if begin < 0 {
		begin = 0
	}
	if end > len(points)-1 {
		end = len(points) - 1
	}
	if begin >= end {
		return nil
	}
	return points[begin : end+1]
}

// ── Rate lookup helpers ─────────────────────────────────────────────

// findWeightClassRate returns the EUR/km rate for the given vehicle
// weight and Euro emission class. If no bracket matches exactly it
// falls back to the heaviest bracket.
func findWeightClassRate(classes []WeightClassRate, weight float64, euroClass string) float64 {
	for _, wc := range classes {
		if weight >= wc.Min && weight < wc.Max {
			if rate, ok := wc.Rates[euroClass]; ok {
				return rate
			}
		}
	}
	// Fallback: use the heaviest weight class
	if len(classes) > 0 {
		last := classes[len(classes)-1]
		if rate, ok := last.Rates[euroClass]; ok {
			return rate
		}
	}
	return 0
}

// findCO2Rate returns the flat EUR/km rate for a CO₂-class-qualified
// vehicle at the given weight. Falls back to the heaviest bracket.
func findCO2Rate(rates []CO2WeightRate, weight float64) float64 {
	for _, r := range rates {
		if weight >= r.Min && weight < r.Max {
			return r.Rate
		}
	}
	if len(rates) > 0 {
		return rates[len(rates)-1].Rate
	}
	return 0
}

// ── Rate loading ────────────────────────────────────────────────────

func (tc *TollCalculator) loadRates(dataPath string) map[string]TollConfig {
	path := dataPath + "/toll_rates.json"
	data, err := os.ReadFile(path)
	if err != nil {
		return tc.defaultRates()
	}

	var configs map[string]TollConfig
	if err := json.Unmarshal(data, &configs); err != nil {
		return tc.defaultRates()
	}

	return configs
}

// defaultRates provides a minimal hard-coded fallback so the service
// still works if toll_rates.json cannot be loaded.
func (tc *TollCalculator) defaultRates() map[string]TollConfig {
	allEuro := func(rate float64) map[string]float64 {
		return map[string]float64{
			"EURO_0": rate, "EURO_I": rate, "EURO_II": rate,
			"EURO_III": rate, "EURO_IV": rate,
			"EURO_V": rate, "EURO_VI": rate, "EURO_VI_E": rate,
		}
	}

	return map[string]TollConfig{
		"DE": {
			Country: "DE", Operator: "Toll Collect", System: "distance",
			MinWeightTonnes: 7.5, TolledRoadFraction: 0.75,
			WeightClasses: []WeightClassRate{
				{Min: 7.5, Max: 9999, Rates: allEuro(0.348)},
			},
		},
		"NL": {
			Country: "NL", Operator: "Vrachtwagenheffing", System: "distance",
			MinWeightTonnes: 3.5, TolledRoadFraction: 1.0,
			WeightClasses: []WeightClassRate{
				{Min: 3.5, Max: 9999, Rates: allEuro(0.201)},
			},
		},
		"BE": {
			Country: "BE", Operator: "Viapass", System: "distance",
			MinWeightTonnes: 3.5, TolledRoadFraction: 0.70,
			WeightClasses: []WeightClassRate{
				{Min: 3.5, Max: 9999, Rates: allEuro(0.204)},
			},
		},
		"FR": {
			Country: "FR", Operator: "Autoroutes", System: "distance",
			MinWeightTonnes: 3.5, TolledRoadFraction: 0.55,
			WeightClasses: []WeightClassRate{
				{Min: 3.5, Max: 9999, Rates: allEuro(0.20)},
			},
		},
	}
}
