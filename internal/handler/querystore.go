package handler

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"cloud.google.com/go/storage"

	"github.com/filogic/micro-services-valhalla/internal/model"
)

// QueryStore captures computed route queries (and their toll result) to a
// GCS bucket so they can later be randomly sampled by the PTV benchmark.
//
// It is strictly best-effort: the storage client is initialized lazily, and
// every failure is logged but never propagated — capturing a query must
// never block, slow, or fail a routing request. Records are written under a
// date-partitioned `queries/` prefix and trimmed (no polylines) to stay tiny.
type QueryStore struct {
	bucket string
	logger *slog.Logger

	once    sync.Once
	client  *storage.Client
	initErr error
}

func NewQueryStore(bucket string, logger *slog.Logger) *QueryStore {
	return &QueryStore{bucket: bucket, logger: logger}
}

// storedQuery is the trimmed record persisted per query.
type storedQuery struct {
	TransactionId string             `json:"transactionId,omitempty"`
	Timestamp     string             `json:"timestamp"`
	Origin        model.Coordinate   `json:"origin"`
	Destination   model.Coordinate   `json:"destination"`
	Waypoints     []model.Coordinate `json:"waypoints,omitempty"`
	Vehicle       *model.VehicleSpec `json:"vehicle,omitempty"`
	Toll          storedToll         `json:"toll"`
}

type storedToll struct {
	TotalCost     float64                    `json:"totalCost"`
	TotalDistance float64                    `json:"totalDistance"`
	ByCountry     []model.TollCountrySummary `json:"byCountry,omitempty"`
}

func (s *QueryStore) clientOnce() (*storage.Client, error) {
	s.once.Do(func() {
		s.client, s.initErr = storage.NewClient(context.Background())
	})
	return s.client, s.initErr
}

// Put uploads a trimmed record of the query and its toll result. It is meant
// to be called in its own goroutine with a background context; it applies its
// own timeout and swallows all errors.
func (s *QueryStore) Put(transactionID string, req *model.RouteRequest, toll model.TollSummary) {
	if s == nil || s.bucket == "" || req == nil {
		return
	}

	rec := storedQuery{
		TransactionId: transactionID,
		Timestamp:     time.Now().UTC().Format(time.RFC3339),
		Origin:        req.Origin,
		Destination:   req.Destination,
		Waypoints:     req.Waypoints,
		Vehicle:       req.Vehicle,
		Toll: storedToll{
			TotalCost:     toll.TotalCost,
			TotalDistance: toll.TotalDistance,
			ByCountry:     toll.ByCountry,
		},
	}
	data, err := json.Marshal(rec)
	if err != nil {
		s.logger.Warn("querystore marshal failed", "err", err)
		return
	}

	client, err := s.clientOnce()
	if err != nil {
		s.logger.Warn("querystore client init failed", "err", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	now := time.Now().UTC()
	sum := sha256.Sum256(data)
	object := fmt.Sprintf("queries/%04d/%02d/%02d/%d-%s.json",
		now.Year(), int(now.Month()), now.Day(), now.UnixNano(), hex.EncodeToString(sum[:4]))

	w := client.Bucket(s.bucket).Object(object).NewWriter(ctx)
	w.ContentType = "application/json"
	if _, err := w.Write(data); err != nil {
		_ = w.Close()
		s.logger.Warn("querystore write failed", "object", object, "err", err)
		return
	}
	if err := w.Close(); err != nil {
		s.logger.Warn("querystore close failed", "object", object, "err", err)
	}
}
