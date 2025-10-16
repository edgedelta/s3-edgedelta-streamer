package health

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/edgedelta/s3-edgedelta-streamer/internal/logging"
)

// HealthChecker defines the interface for health check components
type HealthChecker interface {
	Check(ctx context.Context) error
	Name() string
}

// HealthServer provides HTTP health check endpoints
type HealthServer struct {
	server   *http.Server
	checkers []HealthChecker
	mu       sync.RWMutex
}

// HealthStatus represents the health check response
type HealthStatus struct {
	Status    string            `json:"status"`
	Checks    map[string]string `json:"checks,omitempty"`
	Message   string            `json:"message,omitempty"`
	Timestamp string            `json:"timestamp"`
}

// NewHealthServer creates a new health check server
func NewHealthServer(address, path string, checkers ...HealthChecker) *HealthServer {
	hs := &HealthServer{
		checkers: checkers,
	}

	mux := http.NewServeMux()
	mux.HandleFunc(path, hs.healthHandler)
	mux.HandleFunc("/ready", hs.readyHandler)

	hs.server = &http.Server{
		Addr:    address,
		Handler: mux,
	}

	return hs
}

// Start starts the health check server
func (hs *HealthServer) Start() error {
	logger := logging.GetDefaultLogger()
	logger.Info("Starting health check server", "address", hs.server.Addr)

	go func() {
		if err := hs.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("Health check server failed", "error", err)
		}
	}()

	return nil
}

// Stop stops the health check server
func (hs *HealthServer) Stop(ctx context.Context) error {
	return hs.server.Shutdown(ctx)
}

// healthHandler handles /health requests
func (hs *HealthServer) healthHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	status := hs.performHealthChecks(ctx)

	w.Header().Set("Content-Type", "application/json")

	if status.Status == "healthy" {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
	}

	json.NewEncoder(w).Encode(status)
}

// readyHandler handles /ready requests (same as health for now)
func (hs *HealthServer) readyHandler(w http.ResponseWriter, r *http.Request) {
	hs.healthHandler(w, r)
}

// performHealthChecks runs all health checks
func (hs *HealthServer) performHealthChecks(ctx context.Context) HealthStatus {
	hs.mu.RLock()
	checkers := make([]HealthChecker, len(hs.checkers))
	copy(checkers, hs.checkers)
	hs.mu.RUnlock()

	status := HealthStatus{
		Status:    "healthy",
		Checks:    make(map[string]string),
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	for _, checker := range checkers {
		if err := checker.Check(ctx); err != nil {
			status.Status = "unhealthy"
			status.Checks[checker.Name()] = fmt.Sprintf("ERROR: %v", err)
		} else {
			status.Checks[checker.Name()] = "OK"
		}
	}

	if status.Status == "unhealthy" {
		status.Message = "One or more health checks failed"
	}

	return status
}

// AddChecker adds a health checker dynamically
func (hs *HealthServer) AddChecker(checker HealthChecker) {
	hs.mu.Lock()
	defer hs.mu.Unlock()
	hs.checkers = append(hs.checkers, checker)
}
