package model

import "fmt"

// ── Request ──

type RouteRequest struct {
	Origin      Coordinate  `json:"origin"`
	Destination Coordinate  `json:"destination"`
	Waypoints   []Coordinate `json:"waypoints,omitempty"`
	Vehicle     *VehicleSpec `json:"vehicle,omitempty"`
	Cargo       *CargoSpec   `json:"cargo,omitempty"`
	Options     *RouteOptions `json:"options,omitempty"`
}

type Coordinate struct {
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
}

// EuroClass represents the Euro emission standard for a vehicle.
type EuroClass string

const (
	EuroClass0   EuroClass = "EURO_0"
	EuroClassI   EuroClass = "EURO_I"
	EuroClassII  EuroClass = "EURO_II"
	EuroClassIII EuroClass = "EURO_III"
	EuroClassIV  EuroClass = "EURO_IV"
	EuroClassV   EuroClass = "EURO_V"
	EuroClassVI  EuroClass = "EURO_VI"
	EuroClassVIE EuroClass = "EURO_VI_E"
)

var validEuroClasses = map[EuroClass]bool{
	EuroClass0: true, EuroClassI: true, EuroClassII: true,
	EuroClassIII: true, EuroClassIV: true, EuroClassV: true,
	EuroClassVI: true, EuroClassVIE: true,
}

// ValidEuroClassValues returns a sorted list of accepted enum values.
func ValidEuroClassValues() []string {
	return []string{"EURO_0", "EURO_I", "EURO_II", "EURO_III", "EURO_IV", "EURO_V", "EURO_VI", "EURO_VI_E"}
}

type VehicleSpec struct {
	Height          *float64   `json:"height,omitempty"`          // meters
	Weight          *float64   `json:"weight,omitempty"`          // tonnes
	Length          *float64   `json:"length,omitempty"`          // meters
	Width           *float64   `json:"width,omitempty"`           // meters
	Axles           *int       `json:"axles,omitempty"`
	AxleLoad        *float64   `json:"axleLoad,omitempty"`        // tonnes per axle
	EuroClass       *EuroClass `json:"euroClass,omitempty"`       // EURO_0..EURO_VI_E
	CO2Class        *int       `json:"co2Class,omitempty"`        // 1-5 (DE/NL), default 1
	FuelType        *string    `json:"fuelType,omitempty"`        // Diesel, LNG, Electric, etc.
	Hazmat          bool       `json:"hazmat,omitempty"`
	FuelConsumption *float64   `json:"fuelConsumption,omitempty"` // l/km override (GLEC Tier 2)
}

// RequiresTruckRouting returns true when any physical vehicle parameter
// is set, causing Valhalla to use truck costing with restriction checks.
func (v *VehicleSpec) RequiresTruckRouting() bool {
	if v == nil {
		return false
	}
	return v.Height != nil || v.Weight != nil || v.Length != nil ||
		v.Width != nil || v.Axles != nil || v.AxleLoad != nil || v.Hazmat
}

func (v *VehicleSpec) EffectiveEuroClass() string {
	if v != nil && v.EuroClass != nil {
		return string(*v.EuroClass)
	}
	return "EURO_VI"
}

// ValidateEuroClass returns an error if euroClass is set but not a valid enum value.
func (v *VehicleSpec) ValidateEuroClass() error {
	if v == nil || v.EuroClass == nil {
		return nil
	}
	if !validEuroClasses[*v.EuroClass] {
		return fmt.Errorf("invalid euroClass %q, valid values: %v", *v.EuroClass, ValidEuroClassValues())
	}
	return nil
}

// EffectiveCO2Class returns the vehicle's CO₂ emission class (1-5).
// Class 1 is the default (most polluting / highest toll rate).
// Classes 2-5 apply to newer vehicles with better CO₂ efficiency.
func (v *VehicleSpec) EffectiveCO2Class() int {
	if v != nil && v.CO2Class != nil && *v.CO2Class >= 1 && *v.CO2Class <= 5 {
		return *v.CO2Class
	}
	return 1
}

func (v *VehicleSpec) EffectiveFuelType() string {
	if v != nil && v.FuelType != nil {
		return *v.FuelType
	}
	return "Diesel"
}

func (v *VehicleSpec) EffectiveWeight() float64 {
	if v != nil && v.Weight != nil {
		return *v.Weight
	}
	return 1.5 // personenauto
}

type CargoSpec struct {
	WeightTonnes *float64 `json:"weightTonnes,omitempty"`
	LoadFactor   *float64 `json:"loadFactor,omitempty"` // 0-1
}

type RouteOptions struct {
	AvoidTolls    bool   `json:"avoidTolls,omitempty"`
	AvoidFerries  bool   `json:"avoidFerries,omitempty"`
	AvoidHighways bool   `json:"avoidHighways,omitempty"`
	DepartureTime string `json:"departureTime,omitempty"` // ISO 8601
}

// ── Response ──

type RouteResponse struct {
	Route           RouteInfo       `json:"route"`
	CarbonFootprint CarbonFootprint `json:"carbonFootprint"`
	Toll            TollSummary     `json:"toll"`
}

type RouteInfo struct {
	Distance float64      `json:"distance"`           // meters
	Duration float64      `json:"duration"`           // seconds
	Polyline string       `json:"polyline,omitempty"` // encoded
	Vehicle  *VehicleSpec `json:"vehicle,omitempty"`
	Legs     []RouteLeg   `json:"legs,omitempty"`
}

type RouteLeg struct {
	Distance float64 `json:"distance"`
	Duration float64 `json:"duration"`
	Summary  string  `json:"summary"`
}

type CarbonFootprint struct {
	TotalKgCO2e float64          `json:"totalKgCO2e"`
	GCO2ePerTkm *float64         `json:"gCO2ePerTkm,omitempty"`
	Methodology string           `json:"methodology"`
	Scope       string           `json:"scope"`
	Factors     EmissionFactors  `json:"factors"`
}

type EmissionFactors struct {
	EmissionFactor  float64 `json:"emissionFactor"`  // kg CO2e/l WTW
	FuelConsumption float64 `json:"fuelConsumption"` // l/km
	LoadFactor      float64 `json:"loadFactor"`
}

type TollSummary struct {
	TotalCost float64       `json:"totalCost"`
	Currency  string        `json:"currency"`
	Segments  []TollSegment `json:"segments"`
}

type TollSegment struct {
	Country   string   `json:"country"`
	Operator  string   `json:"operator"`
	System    string   `json:"system"` // distance, vignette, flat, bridge, tunnel
	Distance  float64  `json:"distance"`
	Cost      float64  `json:"cost"`
	RatePerKm *float64 `json:"ratePerKm,omitempty"`
}

// TollResponse is returned by POST /api/v1/toll.
type TollResponse struct {
	Route RouteInfo   `json:"route"`
	Toll  TollSummary `json:"toll"`
}

// CO2Response is returned by POST /api/v1/co2.
type CO2Response struct {
	Route           RouteInfo       `json:"route"`
	CarbonFootprint CarbonFootprint `json:"carbonFootprint"`
}
