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

