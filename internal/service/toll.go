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
		Currency: "EUR",
		Segments: []model.TollSegment{},
	}

	if vehicle == nil {
		return summary
	}

	weight := vehicle.EffectiveWeight()
	euroClass := vehicle.EffectiveEuroClass()
	co2Class := vehicle.EffectiveCO2Class()

	countryInfo := tc.estimateCountryDistancesDetailed(route)

	for country, info := range countryInfo {
		config, ok := tc.configs[country]
		if !ok {
			continue
		}

		if weight < config.MinWeightTonnes {
			continue
		}

		// 1. Try CO₂ class rates (NL/DE class 2-5: flat rate per weight bracket)
		rate := 0.0
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

		if rate <= 0 {
			continue
		}

		tollDistance := info.TolledDistance
		totalDistance := info.TotalDistance
		cost := (tollDistance / 1000.0) * rate

		tollFraction := 0.0
		if totalDistance > 0 {
			tollFraction = math.Round(tollDistance/totalDistance*100) / 100
		}

		ratePtr := rate
		summary.Segments = append(summary.Segments, model.TollSegment{
			Country:       country,
			Operator:      config.Operator,
			System:        config.System,
			Distance:      math.Round(tollDistance),
			TotalDistance:  math.Round(totalDistance),
			TollFraction:  tollFraction,
			Cost:          math.Round(cost*100) / 100,
			RatePerKm:     &ratePtr,
		})
	}

	for _, seg := range summary.Segments {
		summary.TotalCost += seg.Cost
	}
	summary.TotalCost = math.Round(summary.TotalCost*100) / 100

	return summary
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

// ── Country-distance estimation ─────────────────────────────────────

// countryDistanceInfo holds both total and tolled distances for a country.
type countryDistanceInfo struct {
	TotalDistance  float64 // total meters in this country
	TolledDistance float64 // meters on tolled roads
}

// estimateCountryDistancesDetailed returns per-country distance info.
// For NL, it uses the official toll road registry to match individual
// maneuver street names against the ~80 tolled road numbers.
// For other countries, it applies the tolledRoadFraction estimate.
func (tc *TollCalculator) estimateCountryDistancesDetailed(route *ValhallaResult) map[string]countryDistanceInfo {
	result := make(map[string]countryDistanceInfo)

	for _, leg := range route.Legs {
		for _, m := range leg.Maneuvers {
			cc := m.CountryCode
			if cc == "" {
				continue
			}

			info := result[cc]
			info.TotalDistance += m.Length

			// Match each maneuver against the country's toll road registry.
			// This uses exact road-number matching per country:
			// - NL: official vrachtwagenheffing road list (80 roads)
			// - DE: all Autobahn (A) + Bundesstraßen (B)
			// - BE: motorways + selected N-roads (Viapass)
			// - FR: all autoroutes (A)
			// - CH: all roads (LSVA = 100%)
			// - etc.
			if IsTollRoad(cc, m.StreetNames) {
				info.TolledDistance += m.Length
			}

			result[cc] = info
		}
	}

	return result
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
				{Min: 7.5, Max: 9999, Rates: allEuro(0.269)},
			},
		},
		"NL": {
			Country: "NL", Operator: "Vrachtwagenheffing", System: "distance",
			MinWeightTonnes: 3.5, TolledRoadFraction: 1.0,
			WeightClasses: []WeightClassRate{
				{Min: 3.5, Max: 9999, Rates: allEuro(0.197)},
			},
		},
		"BE": {
			Country: "BE", Operator: "Viapass", System: "distance",
			MinWeightTonnes: 3.5, TolledRoadFraction: 0.70,
			WeightClasses: []WeightClassRate{
				{Min: 3.5, Max: 9999, Rates: allEuro(0.074)},
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
