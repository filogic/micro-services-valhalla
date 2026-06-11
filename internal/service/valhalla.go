package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/filogic/micro-services-valhalla/internal/model"
)

type ValhallaClient struct {
	baseURL    string
	httpClient *http.Client
}

func NewValhallaClient(baseURL string) *ValhallaClient {
	return &ValhallaClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// fetchIDToken gets a Google ID token from the metadata server for
// authenticating to internal Cloud Run services.
func fetchIDToken(ctx context.Context, audience string) (string, error) {
	url := "http://metadata.google.internal/computeMetadata/v1/instance/service-accounts/default/identity?audience=" + audience
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Metadata-Flavor", "Google")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("metadata server: %w", err)
	}
	defer resp.Body.Close()

	token, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(token)), nil
}

type ValhallaResult struct {
	Distance         float64
	Duration         float64
	Polyline         string
	UsedTruckCosting bool
	// EdgeEnriched is true when the legs carry edge-level road data from
	// /trace_attributes instead of maneuver-level data from /route.
	EdgeEnriched bool
	Legs         []ValhallaLeg
}

type ValhallaLeg struct {
	Distance   float64
	Duration   float64
	HasToll    bool
	Points     [][2]float64 // decoded shape, indexed by maneuver Begin/EndShapeIndex
	Maneuvers  []ValhallaManeuver
}

type ValhallaManeuver struct {
	Length          float64
	Time            float64
	HasToll         bool
	CountryCode     string // ISO 3166-1 alpha-2 (e.g., "NL", "DE")
	StreetNames     []string
	BeginShapeIndex int
	EndShapeIndex   int
}

func (c *ValhallaClient) GetRoute(ctx context.Context, req *model.RouteRequest) (*ValhallaResult, error) {
	body := c.buildRequest(req)

	jsonBytes, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/route", bytes.NewReader(jsonBytes))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	// Authenticate to internal Cloud Run service
	if token, err := fetchIDToken(ctx, c.baseURL); err == nil {
		httpReq.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("valhalla request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("valhalla returned %d: %s", resp.StatusCode, string(respBody))
	}

	result, err := c.parseResponse(respBody, req)
	if err != nil {
		return nil, err
	}

	// Maneuver street names only reflect the road where a maneuver starts:
	// a single "stay on" maneuver can cover 100+ km across several roads
	// (e.g. A31 → A7 over the Afsluitdijk), hiding tolled roads from the
	// toll matcher. Enrich the legs with edge-level road data; on failure
	// the maneuver-level data remains as fallback.
	costing := "auto"
	if result.UsedTruckCosting {
		costing = "truck"
	}
	result.EdgeEnriched = c.enrichLegsWithEdges(ctx, result, costing)

	return result, nil
}

// ── Edge-level enrichment via /trace_attributes ─────────────────────

type traceEdge struct {
	Names           []string `json:"names"`
	Length          float64  `json:"length"` // km
	Speed           float64  `json:"speed"`  // km/h
	BeginShapeIndex int      `json:"begin_shape_index"`
	EndShapeIndex   int      `json:"end_shape_index"`
}

type traceResponse struct {
	Edges []traceEdge `json:"edges"`
	Shape string      `json:"shape"`
}

// enrichLegsWithEdges replaces each leg's maneuvers with per-edge road
// data from /trace_attributes, giving exact road refs for every stretch.
// Returns true when every leg was enriched.
func (c *ValhallaClient) enrichLegsWithEdges(ctx context.Context, result *ValhallaResult, costing string) bool {
	allEnriched := true
	for i := range result.Legs {
		if !c.enrichLeg(ctx, &result.Legs[i], costing) {
			allEnriched = false
		}
	}
	return allEnriched
}

func (c *ValhallaClient) enrichLeg(ctx context.Context, leg *ValhallaLeg, costing string) bool {
	if len(leg.Points) < 2 {
		return false
	}

	shape := make([]map[string]float64, len(leg.Points))
	for i, p := range leg.Points {
		shape[i] = map[string]float64{"lat": p[0], "lon": p[1]}
	}

	traceReq := map[string]any{
		"shape":       shape,
		"costing":     costing,
		"shape_match": "walk_or_snap",
		"filters": map[string]any{
			"attributes": []string{
				"edge.names", "edge.length", "edge.speed",
				"edge.begin_shape_index", "edge.end_shape_index", "shape",
			},
			"action": "include",
		},
	}

	jsonBytes, err := json.Marshal(traceReq)
	if err != nil {
		return false
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/trace_attributes", bytes.NewReader(jsonBytes))
	if err != nil {
		return false
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if token, err := fetchIDToken(ctx, c.baseURL); err == nil {
		httpReq.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil || resp.StatusCode != http.StatusOK {
		return false
	}

	var trace traceResponse
	if err := json.Unmarshal(respBody, &trace); err != nil || len(trace.Edges) == 0 || trace.Shape == "" {
		return false
	}

	points := decodePolyline(trace.Shape)
	if len(points) < 2 {
		return false
	}

	maneuvers := buildEdgeManeuvers(trace.Edges, points, leg.Duration)
	if len(maneuvers) == 0 {
		return false
	}

	leg.Points = points
	leg.Maneuvers = maneuvers
	return true
}

// buildEdgeManeuvers converts trace edges into per-edge maneuvers with
// country codes, scaling edge travel times so they sum to the leg duration.
func buildEdgeManeuvers(edges []traceEdge, points [][2]float64, legDuration float64) []ValhallaManeuver {
	rawTimes := make([]float64, len(edges))
	totalTime, totalLength := 0.0, 0.0
	for i, e := range edges {
		if e.Speed > 0 {
			rawTimes[i] = e.Length / e.Speed * 3600
		}
		totalTime += rawTimes[i]
		totalLength += e.Length
	}

	var maneuvers []ValhallaManeuver
	for i, e := range edges {
		t := rawTimes[i]
		if totalTime > 0 {
			t = rawTimes[i] / totalTime * legDuration
		} else if totalLength > 0 {
			t = e.Length / totalLength * legDuration
		}

		m := ValhallaManeuver{
			Length:          e.Length * 1000,
			Time:            t,
			StreetNames:     e.Names,
			BeginShapeIndex: e.BeginShapeIndex,
			EndShapeIndex:   e.EndShapeIndex,
		}
		maneuvers = append(maneuvers, splitManeuverByCountry(m, points)...)
	}
	fillManeuverCountries(maneuvers)
	return maneuvers
}

// fillManeuverCountries assigns a country to maneuvers that resolved to
// none — typically water crossings like the Afsluitdijk (A7), which lie
// outside the simplified land polygons. They inherit the country of the
// preceding stretch; leading gaps take the first known country.
func fillManeuverCountries(maneuvers []ValhallaManeuver) {
	last := ""
	for i := range maneuvers {
		if maneuvers[i].CountryCode == "" {
			maneuvers[i].CountryCode = last
		} else {
			last = maneuvers[i].CountryCode
		}
	}
	first := ""
	for i := range maneuvers {
		if maneuvers[i].CountryCode != "" {
			first = maneuvers[i].CountryCode
			break
		}
	}
	for i := range maneuvers {
		if maneuvers[i].CountryCode != "" {
			break
		}
		maneuvers[i].CountryCode = first
	}
}

func (c *ValhallaClient) buildRequest(req *model.RouteRequest) map[string]any {
	locations := []map[string]any{
		{"lat": req.Origin.Lat, "lon": req.Origin.Lon, "type": "break"},
	}
	for _, wp := range req.Waypoints {
		locations = append(locations, map[string]any{
			"lat": wp.Lat, "lon": wp.Lon, "type": "through",
		})
	}
	locations = append(locations, map[string]any{
		"lat": req.Destination.Lat, "lon": req.Destination.Lon, "type": "break",
	})

	useTruck := req.Vehicle.RequiresTruckRouting()
	costing := "auto"
	if useTruck {
		costing = "truck"
	}

	costingOptions := c.buildCostingOptions(req, useTruck)

	valReq := map[string]any{
		"locations":          locations,
		"costing":            costing,
		"costing_options":    costingOptions,
		"directions_options": map[string]any{"units": "kilometers"},
	}

	if req.Options != nil && req.Options.DepartureTime != "" {
		valReq["date_time"] = map[string]any{
			"type":  1,
			"value": req.Options.DepartureTime,
		}
	}

	return valReq
}

func (c *ValhallaClient) buildCostingOptions(req *model.RouteRequest, useTruck bool) map[string]any {
	opts := make(map[string]any)

	if useTruck {
		truck := make(map[string]any)
		v := req.Vehicle

		if v.Height != nil {
			truck["height"] = *v.Height
		}
		if v.Width != nil {
			truck["width"] = *v.Width
		}
		if v.Length != nil {
			truck["length"] = *v.Length
		}
		if v.Weight != nil {
			truck["weight"] = *v.Weight
		}
		if v.Axles != nil {
			truck["axle_count"] = *v.Axles
		}
		if v.AxleLoad != nil {
			truck["axle_load"] = *v.AxleLoad
		}
		if v.Hazmat {
			truck["hazmat"] = true
		}
		if req.Options != nil {
			if req.Options.AvoidTolls {
				truck["use_tolls"] = 0.0
			}
			if req.Options.AvoidFerries {
				truck["use_ferries"] = 0.0
			}
			if req.Options.AvoidHighways {
				truck["use_highways"] = 0.0
			}
		}
		opts["truck"] = truck
	} else {
		auto := make(map[string]any)
		if req.Options != nil {
			if req.Options.AvoidTolls {
				auto["use_tolls"] = 0.0
			}
			if req.Options.AvoidFerries {
				auto["use_ferries"] = 0.0
			}
			if req.Options.AvoidHighways {
				auto["use_highways"] = 0.0
			}
		}
		opts["auto"] = auto
	}

	return opts
}

func (c *ValhallaClient) parseResponse(body []byte, req *model.RouteRequest) (*ValhallaResult, error) {
	var raw struct {
		Trip struct {
			Summary struct {
				Length float64 `json:"length"`
				Time   float64 `json:"time"`
			} `json:"summary"`
			Legs []struct {
				Shape   string `json:"shape"`
				Summary struct {
					Length  float64 `json:"length"`
					Time   float64 `json:"time"`
					HasToll bool    `json:"has_toll"`
				} `json:"summary"`
				Maneuvers []struct {
					Length           float64  `json:"length"`
					Time             float64  `json:"time"`
					TollBooth        bool     `json:"toll_booth"`
					Toll             bool     `json:"toll"`
					StreetNames      []string `json:"street_names"`
					BeginStreetNames []string `json:"begin_street_names"`
					BeginShapeIndex  int      `json:"begin_shape_index"`
					EndShapeIndex    int      `json:"end_shape_index"`
				} `json:"maneuvers"`
			} `json:"legs"`
		} `json:"trip"`
	}

	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("parse valhalla response: %w", err)
	}

	result := &ValhallaResult{
		Distance:         raw.Trip.Summary.Length * 1000, // km → m
		Duration:         raw.Trip.Summary.Time,
		UsedTruckCosting: req.Vehicle.RequiresTruckRouting(),
	}

	for _, leg := range raw.Trip.Legs {
		vl := ValhallaLeg{
			Distance: leg.Summary.Length * 1000,
			Duration: leg.Summary.Time,
			HasToll:  leg.Summary.HasToll,
		}

		if result.Polyline == "" {
			result.Polyline = leg.Shape
		}

		// Decode polyline to get coordinates for country detection
		// and per-segment polyline extraction in the toll calculator.
		points := decodePolyline(leg.Shape)
		vl.Points = points

		for _, m := range leg.Maneuvers {
			vl.Maneuvers = append(vl.Maneuvers, splitManeuverByCountry(ValhallaManeuver{
				Length:          m.Length * 1000,
				Time:            m.Time,
				HasToll:         m.Toll || m.TollBooth,
				StreetNames:     m.StreetNames,
				BeginShapeIndex: m.BeginShapeIndex,
				EndShapeIndex:   m.EndShapeIndex,
			}, points)...)
		}
		fillManeuverCountries(vl.Maneuvers)

		result.Legs = append(result.Legs, vl)
	}

	return result, nil
}

// splitManeuverByCountry assigns countries to a maneuver by walking its
// shape points and splits it where the country changes. A maneuver that
// crosses a border (e.g. 34 km on the E19 from Breda into Antwerp) would
// otherwise be attributed entirely to the country of its midpoint — or
// to no country at all when that midpoint falls in a gap between the
// simplified border polygons, silently dropping its toll.
func splitManeuverByCountry(m ValhallaManeuver, points [][2]float64) []ValhallaManeuver {
	begin, end := m.BeginShapeIndex, m.EndShapeIndex
	if begin < 0 {
		begin = 0
	}
	if end > len(points)-1 {
		end = len(points) - 1
	}
	if end <= begin {
		if begin >= 0 && begin < len(points) {
			m.CountryCode = countryFromCoord(points[begin][0], points[begin][1])
		}
		return []ValhallaManeuver{m}
	}

	// Fast path: short maneuver with matching endpoint countries.
	first := countryFromCoord(points[begin][0], points[begin][1])
	last := countryFromCoord(points[end][0], points[end][1])
	if first != "" && first == last && m.Length < 15000 {
		m.CountryCode = first
		return []ValhallaManeuver{m}
	}

	// Country per shape point. Points in gaps between the simplified
	// border polygons inherit the previous known country; leading gaps
	// take the first known country.
	codes := make([]string, end-begin+1)
	lastSeen := ""
	for i := range codes {
		code := countryFromCoord(points[begin+i][0], points[begin+i][1])
		if code == "" {
			code = lastSeen
		}
		codes[i] = code
		lastSeen = code
	}
	firstKnown := ""
	for _, code := range codes {
		if code != "" {
			firstKnown = code
			break
		}
	}
	for i := range codes {
		if codes[i] != "" {
			break
		}
		codes[i] = firstKnown
	}

	// Boundaries where the country changes.
	boundaries := []int{begin}
	for i := 1; i < len(codes); i++ {
		if codes[i] != codes[i-1] {
			boundaries = append(boundaries, begin+i)
		}
	}
	boundaries = append(boundaries, end)
	if len(boundaries) == 2 {
		m.CountryCode = codes[0]
		return []ValhallaManeuver{m}
	}

	// Apportion length and time over the parts by shape distance.
	shapeDist := func(from, to int) float64 {
		total := 0.0
		for i := from; i < to; i++ {
			dLat := points[i+1][0] - points[i][0]
			dLon := (points[i+1][1] - points[i][1]) * math.Cos(points[i][0]*math.Pi/180)
			total += math.Sqrt(dLat*dLat + dLon*dLon)
		}
		return total
	}
	totalDist := shapeDist(begin, end)

	var parts []ValhallaManeuver
	for b := 0; b < len(boundaries)-1; b++ {
		from, to := boundaries[b], boundaries[b+1]
		share := 1.0
		if totalDist > 0 {
			share = shapeDist(from, to) / totalDist
		}
		part := m
		part.CountryCode = codes[from-begin]
		part.Length = m.Length * share
		part.Time = m.Time * share
		part.BeginShapeIndex = from
		part.EndShapeIndex = to
		parts = append(parts, part)
	}
	return parts
}

// decodePolyline decodes a Google-style encoded polyline (precision 6 for Valhalla)
// into a slice of [lat, lon] pairs.
func decodePolyline(encoded string) [][2]float64 {
	var points [][2]float64
	index, lat, lng := 0, 0, 0

	for index < len(encoded) {
		// Decode latitude
		shift, result := 0, 0
		for {
			b := int(encoded[index]) - 63
			index++
			result |= (b & 0x1f) << shift
			shift += 5
			if b < 0x20 {
				break
			}
		}
		if result&1 != 0 {
			lat += ^(result >> 1)
		} else {
			lat += result >> 1
		}

		// Decode longitude
		shift, result = 0, 0
		for {
			b := int(encoded[index]) - 63
			index++
			result |= (b & 0x1f) << shift
			shift += 5
			if b < 0x20 {
				break
			}
		}
		if result&1 != 0 {
			lng += ^(result >> 1)
		} else {
			lng += result >> 1
		}

		points = append(points, [2]float64{float64(lat) / 1e6, float64(lng) / 1e6})
	}
	return points
}

// encodePolyline encodes [lat, lon] pairs into a Google-style encoded
// polyline at precision 6, the inverse of decodePolyline.
func encodePolyline(points [][2]float64) string {
	var sb strings.Builder
	prevLat, prevLng := 0, 0

	for _, p := range points {
		lat := int(math.Round(p[0] * 1e6))
		lng := int(math.Round(p[1] * 1e6))
		encodePolylineValue(&sb, lat-prevLat)
		encodePolylineValue(&sb, lng-prevLng)
		prevLat, prevLng = lat, lng
	}
	return sb.String()
}

func encodePolylineValue(sb *strings.Builder, v int) {
	u := v << 1
	if v < 0 {
		u = ^u
	}
	for u >= 0x20 {
		sb.WriteByte(byte(0x20|(u&0x1f)) + 63)
		u >>= 5
	}
	sb.WriteByte(byte(u) + 63)
}

