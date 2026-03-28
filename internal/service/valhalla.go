package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
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
			Timeout: 15 * time.Second,
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
	Legs             []ValhallaLeg
}

type ValhallaLeg struct {
	Distance   float64
	Duration   float64
	HasToll    bool
	Maneuvers  []ValhallaManeuver
}

type ValhallaManeuver struct {
	Length          float64
	Time            float64
	HasToll         bool
	CountryCode     string // ISO 3166-1 alpha-2 (e.g., "NL", "DE")
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

	return c.parseResponse(respBody, req)
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
					Length          float64 `json:"length"`
					Time            float64 `json:"time"`
					TollBooth       bool    `json:"toll_booth"`
					Toll            bool    `json:"toll"`
					BeginShapeIndex int     `json:"begin_shape_index"`
					EndShapeIndex   int     `json:"end_shape_index"`
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
		points := decodePolyline(leg.Shape)

		for _, m := range leg.Maneuvers {
			// Determine country from the midpoint of this maneuver's shape segment
			countryCode := ""
			midIdx := (m.BeginShapeIndex + m.EndShapeIndex) / 2
			if midIdx < len(points) {
				countryCode = countryFromCoord(points[midIdx][0], points[midIdx][1])
			}
			vl.Maneuvers = append(vl.Maneuvers, ValhallaManeuver{
				Length:          m.Length * 1000,
				Time:            m.Time,
				HasToll:         m.Toll || m.TollBooth,
				CountryCode:     countryCode,
				BeginShapeIndex: m.BeginShapeIndex,
				EndShapeIndex:   m.EndShapeIndex,
			})
		}

		result.Legs = append(result.Legs, vl)
	}

	return result, nil
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

// countryFromCoord returns ISO 3166-1 alpha-2 country code for a coordinate.
// Uses a simplified point-in-polygon approach with key boundary latitudes/longitudes
// to distinguish neighboring countries in the Benelux/DACH region.
func countryFromCoord(lat, lon float64) string {
	// Luxembourg (small, check first)
	if lat >= 49.4 && lat <= 50.2 && lon >= 5.7 && lon <= 6.55 {
		return "LU"
	}
	// Netherlands vs Belgium boundary: ~51.35°N is roughly the NL-BE border
	// NL extends south to ~51.35 in Zeeuws-Vlaanderen and Limburg
	if lat >= 51.35 && lat <= 53.6 && lon >= 3.3 && lon <= 7.25 {
		return "NL"
	}
	// Southern Netherlands: Limburg and Noord-Brabant (east of 4.0°E, above 51.0°N)
	if lat >= 51.0 && lat < 51.35 && lon >= 4.0 && lon <= 7.25 {
		return "NL"
	}
	// Zeeland part of NL (west, above 51.2°N)
	if lat >= 51.2 && lat < 51.35 && lon >= 3.3 && lon < 4.0 {
		return "NL"
	}
	// Belgium
	if lat >= 49.5 && lat <= 51.55 && lon >= 2.5 && lon <= 6.4 {
		return "BE"
	}
	// Germany — exclude Benelux overlap by requiring lon > 5.8 only above 51°N
	if lat >= 47.2 && lat <= 55.1 && lon >= 5.8 && lon <= 15.1 {
		// Avoid NL/BE overlap in the west
		if lon < 6.2 && lat > 50.0 && lat < 52.0 {
			// This area could be NL or BE — already handled above
			return "DE"
		}
		return "DE"
	}
	// France
	if lat >= 42.3 && lat <= 51.1 && lon >= -5.2 && lon <= 8.3 {
		return "FR"
	}
	// Switzerland
	if lat >= 45.8 && lat <= 47.85 && lon >= 5.9 && lon <= 10.5 {
		return "CH"
	}
	// Austria
	if lat >= 46.3 && lat <= 49.05 && lon >= 9.5 && lon <= 17.2 {
		return "AT"
	}
	// Italy
	if lat >= 36.6 && lat <= 47.1 && lon >= 6.6 && lon <= 18.6 {
		return "IT"
	}
	// Spain
	if lat >= 36.0 && lat <= 43.8 && lon >= -9.3 && lon <= 3.4 {
		return "ES"
	}
	// Poland
	if lat >= 49.0 && lat <= 54.85 && lon >= 14.1 && lon <= 24.2 {
		return "PL"
	}
	// Czech Republic
	if lat >= 48.55 && lat <= 51.06 && lon >= 12.1 && lon <= 18.9 {
		return "CZ"
	}
	// Denmark
	if lat >= 54.5 && lat <= 57.8 && lon >= 8.0 && lon <= 15.2 {
		return "DK"
	}
	return ""
}
