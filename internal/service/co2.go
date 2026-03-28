package service

import (
	"math"

	"github.com/filogic/micro-services-valhalla/internal/model"
)

// GLEC v3 WTW emission factors (kg CO₂e per liter)
var wtwFactors = map[string]float64{
	"Diesel":   3.24,
	"Petrol":   2.94,
	"LNG":      3.00,
	"CNG":      2.96,
	"HVO":      0.49,
	"Electric": 0.00, // V2: country-specific grid mix
}

// GLEC v3 default fuel consumption (l/km) per weight class
var fuelConsumptionDefaults = map[string]float64{
	"LCV":             0.08, // <3.5t
	"MFT_RIGID_SMALL": 0.15, // 3.5-7.5t
	"MFT_RIGID_MED":   0.20, // 7.5-12t
	"HFT_RIGID_LARGE": 0.25, // 12-20t
	"HFT_RIGID_HEAVY": 0.28, // 20-26t
	"HFT_ARTICULATED": 0.30, // >26t
}

type CO2Calculator struct{}

func NewCO2Calculator() *CO2Calculator {
	return &CO2Calculator{}
}

func (c *CO2Calculator) Calculate(distanceM float64, vehicle *model.VehicleSpec, cargo *model.CargoSpec) model.CarbonFootprint {
	distanceKm := distanceM / 1000.0

	// 1. Fuel consumption
	fuelCons := c.getFuelConsumption(vehicle)

	// 2. WTW emission factor
	fuelType := "Diesel"
	if vehicle != nil {
		fuelType = vehicle.EffectiveFuelType()
	}
	emFactor := wtwFactors[fuelType]
	if emFactor == 0 && fuelType != "Electric" {
		emFactor = 3.24 // fallback diesel
	}

	// 3. Total CO₂e
	totalKg := distanceKm * fuelCons * emFactor

	// 4. Load factor
	isTruck := vehicle.RequiresTruckRouting()
	loadFactor := 1.0
	if isTruck {
		loadFactor = 0.6
	}
	if cargo != nil && cargo.LoadFactor != nil {
		loadFactor = *cargo.LoadFactor
	}

	// 5. Intensity (g CO₂e / tkm)
	var gPerTkm *float64
	if cargo != nil && cargo.WeightTonnes != nil && *cargo.WeightTonnes > 0 {
		v := (totalKg * 1000) / (distanceKm * *cargo.WeightTonnes)
		v = math.Round(v*10) / 10
		gPerTkm = &v
	}

	return model.CarbonFootprint{
		TotalKgCO2e: math.Round(totalKg*10) / 10,
		GCO2ePerTkm: gPerTkm,
		Methodology: "GLECv3/ISO14083",
		Scope:       "WTW",
		Factors: model.EmissionFactors{
			EmissionFactor:  emFactor,
			FuelConsumption: fuelCons,
			LoadFactor:      loadFactor,
		},
	}
}

func (c *CO2Calculator) getFuelConsumption(vehicle *model.VehicleSpec) float64 {
	// Tier 2: user override
	if vehicle != nil && vehicle.FuelConsumption != nil {
		return *vehicle.FuelConsumption
	}

	weight := 1.5 // personenauto default
	if vehicle != nil && vehicle.Weight != nil {
		weight = *vehicle.Weight
	}

	class := weightToClass(weight)
	if fc, ok := fuelConsumptionDefaults[class]; ok {
		return fc
	}

	return 0.07 // fallback personenauto
}

func weightToClass(tonnes float64) string {
	switch {
	case tonnes < 3.5:
		return "LCV"
	case tonnes < 7.5:
		return "MFT_RIGID_SMALL"
	case tonnes < 12:
		return "MFT_RIGID_MED"
	case tonnes < 20:
		return "HFT_RIGID_LARGE"
	case tonnes < 26:
		return "HFT_RIGID_HEAVY"
	default:
		return "HFT_ARTICULATED"
	}
}
