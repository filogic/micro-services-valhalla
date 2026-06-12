package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/filogic/micro-services-valhalla/internal/model"
	"github.com/filogic/micro-services-valhalla/internal/service"
)

type RouteHandler struct {
	valhalla *service.ValhallaClient
	toll     *service.TollCalculator
	co2      *service.CO2Calculator
	cache    *responseCache
	logger   *slog.Logger
}

func NewRouteHandler(valhallaURL, dataPath string, logger *slog.Logger) *RouteHandler {
	return &RouteHandler{
		valhalla: service.NewValhallaClient(valhallaURL),
		toll:     service.NewTollCalculator(dataPath),
		co2:      service.NewCO2Calculator(),
		cache:    newResponseCache(256, 15*time.Minute),
		logger:   logger,
	}
}

// parseAndValidate decodes the request body and validates common fields.
// The raw body is returned as well; it is the cache identity of the request.
func (h *RouteHandler) parseAndValidate(w http.ResponseWriter, r *http.Request) (*model.RouteRequest, []byte, bool) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "POST required")
		return nil, nil, false
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "unable to read body: "+err.Error())
		return nil, nil, false
	}

	var req model.RouteRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return nil, nil, false
	}

	if req.Origin.Lat == 0 && req.Origin.Lon == 0 {
		writeError(w, http.StatusBadRequest, "origin is required")
		return nil, nil, false
	}
	if req.Destination.Lat == 0 && req.Destination.Lon == 0 {
		writeError(w, http.StatusBadRequest, "destination is required")
		return nil, nil, false
	}

	if req.Vehicle != nil {
		if err := req.Vehicle.ValidateEuroClass(); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return nil, nil, false
		}
	}

	return &req, body, true
}

// serveCached writes the cached response when the identical request was
// answered recently. Any change to the request (waypoints, vehicle, …)
// produces a different key and recomputes.
func (h *RouteHandler) serveCached(w http.ResponseWriter, endpoint string, body []byte) (string, bool) {
	key := h.cache.Key(endpoint, body)
	if cached, ok := h.cache.Get(key); ok {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Cache", "HIT")
		w.WriteHeader(http.StatusOK)
		w.Write(cached)
		return key, true
	}
	return key, false
}

func (h *RouteHandler) writeAndCache(w http.ResponseWriter, key string, v any) {
	bytes, err := json.Marshal(v)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "marshal response: "+err.Error())
		return
	}
	h.cache.Set(key, bytes)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(bytes)
}

// getRoute calls Valhalla and returns the result with timing.
func (h *RouteHandler) getRoute(w http.ResponseWriter, r *http.Request, req *model.RouteRequest) (*service.ValhallaResult, int64, bool) {
	start := time.Now()
	route, err := h.valhalla.GetRoute(r.Context(), req)
	if err != nil {
		h.logger.Error("valhalla error", "err", err)
		writeError(w, http.StatusBadGateway, "routing engine error: "+err.Error())
		return nil, 0, false
	}
	return route, time.Since(start).Milliseconds(), true
}

// buildRouteInfo converts a ValhallaResult into the API RouteInfo.
func buildRouteInfo(route *service.ValhallaResult, vehicle *model.VehicleSpec) model.RouteInfo {
	legs := make([]model.RouteLeg, len(route.Legs))
	for i, l := range route.Legs {
		d := time.Duration(l.Duration) * time.Second
		legs[i] = model.RouteLeg{
			Distance: l.Distance,
			Duration: l.Duration,
			Summary:  fmt.Sprintf("%.1f km, %s", l.Distance/1000, fmtDuration(d)),
		}
	}
	return model.RouteInfo{
		Distance: route.Distance,
		Duration: route.Duration,
		Polyline: route.Polyline,
		Vehicle:  vehicle,
		Legs:     legs,
	}
}

// ServeHTTP handles POST /api/v1/route — full response (route + toll + CO₂).
func (h *RouteHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	req, body, ok := h.parseAndValidate(w, r)
	if !ok {
		return
	}
	cacheKey, hit := h.serveCached(w, "route", body)
	if hit {
		return
	}

	route, valhallaMs, ok := h.getRoute(w, r, req)
	if !ok {
		return
	}

	toll := h.toll.Calculate(route, req.Vehicle)
	co2 := h.co2.Calculate(route.Distance, req.Vehicle, req.Cargo)

	h.logger.Info("route calculated",
		"endpoint", "route",
		"totalMs", time.Since(time.Now().Add(-time.Duration(valhallaMs)*time.Millisecond)).Milliseconds(),
		"valhallaMs", valhallaMs,
		"distance", route.Distance,
	)

	h.writeAndCache(w, cacheKey, model.RouteResponse{
		Route:           buildRouteInfo(route, req.Vehicle),
		CarbonFootprint: co2,
		Toll:            toll,
	})
}

// ServeToll handles POST /api/v1/toll — route + toll only.
func (h *RouteHandler) ServeToll(w http.ResponseWriter, r *http.Request) {
	req, body, ok := h.parseAndValidate(w, r)
	if !ok {
		return
	}
	cacheKey, hit := h.serveCached(w, "toll", body)
	if hit {
		return
	}

	route, valhallaMs, ok := h.getRoute(w, r, req)
	if !ok {
		return
	}

	toll := h.toll.Calculate(route, req.Vehicle)

	h.logger.Info("toll calculated",
		"endpoint", "toll",
		"valhallaMs", valhallaMs,
		"distance", route.Distance,
		"tollTotal", toll.TotalCost,
	)

	h.writeAndCache(w, cacheKey, model.TollResponse{
		Route: buildRouteInfo(route, req.Vehicle),
		Toll:  toll,
	})
}

// ServeCO2 handles POST /api/v1/co2 — route + CO₂ only.
func (h *RouteHandler) ServeCO2(w http.ResponseWriter, r *http.Request) {
	req, body, ok := h.parseAndValidate(w, r)
	if !ok {
		return
	}
	cacheKey, hit := h.serveCached(w, "co2", body)
	if hit {
		return
	}

	route, valhallaMs, ok := h.getRoute(w, r, req)
	if !ok {
		return
	}

	co2 := h.co2.Calculate(route.Distance, req.Vehicle, req.Cargo)

	h.logger.Info("co2 calculated",
		"endpoint", "co2",
		"valhallaMs", valhallaMs,
		"distance", route.Distance,
		"co2kg", co2.TotalKgCO2e,
	)

	h.writeAndCache(w, cacheKey, model.CO2Response{
		Route:           buildRouteInfo(route, req.Vehicle),
		CarbonFootprint: co2,
	})
}

func HealthHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "healthy",
		"service": "viatiq-routing",
		"version": "0.1.0",
		"time":    time.Now().UTC().Format(time.RFC3339),
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func fmtDuration(d time.Duration) string {
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	if h > 0 {
		return fmt.Sprintf("%dh%02dm", h, m)
	}
	return fmt.Sprintf("%dm", m)
}
