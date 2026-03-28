package service

import (
	"encoding/json"
	"math"
	"os"

	"github.com/filogic/micro-services-valhalla/internal/model"
)

type TollConfig struct {
	Country            string             `json:"country"`
	Operator           string             `json:"operator"`
	System             string             `json:"system"`
	MinWeightTonnes    float64            `json:"minWeightTonnes"`
	TolledRoadFraction float64            `json:"tolledRoadFraction"`
	RatesPerEuroClass  map[string]float64 `json:"ratesPerEuroClass"`
}

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

	countryDistances := tc.estimateCountryDistances(route)

	for country, distance := range countryDistances {
		config, ok := tc.configs[country]
		if !ok {
			continue
		}

		if weight < config.MinWeightTonnes {
			continue
		}

		rate, ok := config.RatesPerEuroClass[euroClass]
		if !ok || rate <= 0 {
			continue
		}

		tollDistance := distance * config.TolledRoadFraction
		cost := (tollDistance / 1000.0) * rate

		ratePtr := rate
		summary.Segments = append(summary.Segments, model.TollSegment{
			Country:   country,
			Operator:  config.Operator,
			System:    config.System,
			Distance:  math.Round(tollDistance),
			Cost:      math.Round(cost*100) / 100,
			RatePerKm: &ratePtr,
		})
	}

	for _, seg := range summary.Segments {
		summary.TotalCost += seg.Cost
	}
	summary.TotalCost = math.Round(summary.TotalCost*100) / 100

	return summary
}

func (tc *TollCalculator) estimateCountryDistances(route *ValhallaResult) map[string]float64 {
	distances := make(map[string]float64)

	var totalTollDistance float64
	for _, leg := range route.Legs {
		for _, m := range leg.Maneuvers {
			if m.HasToll {
				totalTollDistance += m.Length
			}
		}
	}

	// V1: without admin boundary data, attribute toll distance to UNKNOWN.
	// V2 will use Valhalla's admin info per edge for exact country splits.
	if totalTollDistance > 0 {
		distances["UNKNOWN"] = totalTollDistance
	} else {
		distances["UNKNOWN"] = route.Distance
	}

	return distances
}

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

func (tc *TollCalculator) defaultRates() map[string]TollConfig {
	return map[string]TollConfig{
		"NL": {
			Country: "NL", Operator: "Vrachtwagenheffing", System: "distance",
			MinWeightTonnes: 3.5, TolledRoadFraction: 0.70,
			RatesPerEuroClass: map[string]float64{
				"EURO_0": 0.269, "EURO_I": 0.269, "EURO_II": 0.269,
				"EURO_III": 0.228, "EURO_IV": 0.228,
				"EURO_V": 0.169, "EURO_VI": 0.157, "EURO_VI_E": 0.157,
			},
		},
		"DE": {
			Country: "DE", Operator: "Toll Collect", System: "distance",
			MinWeightTonnes: 7.5, TolledRoadFraction: 0.65,
			RatesPerEuroClass: map[string]float64{
				"EURO_0": 0.352, "EURO_I": 0.352, "EURO_II": 0.352,
				"EURO_III": 0.318, "EURO_IV": 0.290,
				"EURO_V": 0.275, "EURO_VI": 0.231, "EURO_VI_E": 0.192,
			},
		},
		"BE": {
			Country: "BE", Operator: "Viapass", System: "distance",
			MinWeightTonnes: 3.5, TolledRoadFraction: 0.60,
			RatesPerEuroClass: map[string]float64{
				"EURO_0": 0.320, "EURO_I": 0.320, "EURO_II": 0.320,
				"EURO_III": 0.280, "EURO_IV": 0.260,
				"EURO_V": 0.220, "EURO_VI": 0.199, "EURO_VI_E": 0.183,
			},
		},
		"FR": {
			Country: "FR", Operator: "Autoroutes", System: "distance",
			MinWeightTonnes: 3.5, TolledRoadFraction: 0.55,
			RatesPerEuroClass: map[string]float64{
				"EURO_0": 0.22, "EURO_I": 0.22, "EURO_II": 0.22,
				"EURO_III": 0.22, "EURO_IV": 0.21,
				"EURO_V": 0.20, "EURO_VI": 0.20, "EURO_VI_E": 0.20,
			},
		},
	}
}
