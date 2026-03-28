package model

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

type VehicleSpec struct {
	Height          *float64 `json:"height,omitempty"`          // meters
	Weight          *float64 `json:"weight,omitempty"`          // tonnes
	Length          *float64 `json:"length,omitempty"`          // meters
	Width           *float64 `json:"width,omitempty"`           // meters
	Axles           *int     `json:"axles,omitempty"`
	AxleLoad        *float64 `json:"axleLoad,omitempty"`        // tonnes per axle
	EuroClass       *string  `json:"euroClass,omitempty"`       // EURO_0..EURO_VI_E
	FuelType        *string  `json:"fuelType,omitempty"`        // Diesel, LNG, Electric, etc.
	Hazmat          bool     `json:"hazmat,omitempty"`
	FuelConsumption *float64 `json:"fuelConsumption,omitempty"` // l/km override (GLEC Tier 2)
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
		return *v.EuroClass
	}
	return "EURO_VI"
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
