package handler

import (
	"encoding/json"
	"fmt"
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
	logger   *slog.Logger
}

func NewRouteHandler(valhallaURL, dataPath string, logger *slog.Logger) *RouteHandler {
	return &RouteHandler{
		valhalla: service.NewValhallaClient(valhallaURL),
		toll:     service.NewTollCalculator(dataPath),
		co2:      service.NewCO2Calculator(),
		logger:   logger,
	}
}

func (h *RouteHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "POST required")
		return
	}

	var req model.RouteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	if req.Origin.Lat == 0 && req.Origin.Lon == 0 {
		writeError(w, http.StatusBadRequest, "origin is required")
		return
	}
	if req.Destination.Lat == 0 && req.Destination.Lon == 0 {
		writeError(w, http.StatusBadRequest, "destination is required")
		return
	}

	start := time.Now()

	// Step 1: Valhalla route
	route, err := h.valhalla.GetRoute(r.Context(), &req)
	if err != nil {
		h.logger.Error("valhalla error", "err", err)
		writeError(w, http.StatusBadGateway, "routing engine error: "+err.Error())
		return
	}
	valhallaMs := time.Since(start).Milliseconds()

	// Step 2: Toll (in-memory)
	toll := h.toll.Calculate(route, req.Vehicle)
	tollMs := time.Since(start).Milliseconds() - valhallaMs

	// Step 3: CO₂ (pure math)
	co2 := h.co2.Calculate(route.Distance, req.Vehicle, req.Cargo)
	co2Ms := time.Since(start).Milliseconds() - valhallaMs - tollMs

	totalMs := time.Since(start).Milliseconds()

	costing := "auto"
	if route.UsedTruckCosting {
		costing = "truck"
	}

	h.logger.Info("route calculated",
		"totalMs", totalMs,
		"valhallaMs", valhallaMs,
		"tollMs", tollMs,
		"co2Ms", co2Ms,
		"distance", route.Distance,
		"costing", costing,
	)

	// Step 4: Assemble
	legs := make([]model.RouteLeg, len(route.Legs))
	for i, l := range route.Legs {
		d := time.Duration(l.Duration) * time.Second
		legs[i] = model.RouteLeg{
			Distance: l.Distance,
			Duration: l.Duration,
			Summary:  fmt.Sprintf("%.1f km, %s", l.Distance/1000, fmtDuration(d)),
		}
	}

	resp := model.RouteResponse{
		Route: model.RouteInfo{
			Distance: route.Distance,
			Duration: route.Duration,
			Polyline: route.Polyline,
			Vehicle:  req.Vehicle,
			Legs:     legs,
		},
		CarbonFootprint: co2,
		Toll:            toll,
	}

	writeJSON(w, http.StatusOK, resp)
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
