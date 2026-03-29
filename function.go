package viatiq

import (
	"log/slog"
	"net/http"
	"os"
	"sync"

	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
	"github.com/filogic/micro-services-valhalla/internal/handler"
)

var (
	routeHandler *handler.RouteHandler
	once         sync.Once
)

func init() {
	functions.HTTP("Route", Route)

	once.Do(func() {
		logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

		valhallaURL := envOr("VALHALLA_BASE_URL", "http://localhost:8002")
		dataPath := envOr("DATA_PATH", "./internal/data")

		routeHandler = handler.NewRouteHandler(valhallaURL, dataPath, logger)

		logger.Info("viatiq function initialized", "valhallaURL", valhallaURL)
	})
}

// Route is the Cloud Function entry point.
// Deployed as: gcloud functions deploy viatiq-route --entry-point Route
func Route(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/route":
		routeHandler.ServeHTTP(w, r)
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/toll":
		routeHandler.ServeToll(w, r)
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/co2":
		routeHandler.ServeCO2(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/health":
		handler.HealthHandler(w, r)
	default:
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error":"not found","endpoints":["/api/v1/route","/api/v1/toll","/api/v1/co2","/api/v1/health"]}`))
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
