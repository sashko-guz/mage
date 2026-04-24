package health

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// Status represents health check status
type Status string

const (
	StatusHealthy   Status = "healthy"
	StatusUnhealthy Status = "unhealthy"
)

// CheckResult represents the result of a single health check
type CheckResult struct {
	Name    string `json:"name"`
	Status  Status `json:"status"`
	Message string `json:"message,omitempty"`
	Latency string `json:"latency,omitempty"`
}

// Response represents the full health check response
type Response struct {
	Status Status        `json:"status"`
	Checks []CheckResult `json:"checks,omitempty"`
}

// Checker performs a health check
type Checker interface {
	Name() string
	Check(ctx context.Context) CheckResult
}

// Handler handles health check HTTP endpoints
type Handler struct {
	readinessCheckers []Checker
	timeout           time.Duration
}

// NewHandler creates a new health handler
func NewHandler(timeout time.Duration, checkers ...Checker) *Handler {
	return &Handler{
		readinessCheckers: checkers,
		timeout:           timeout,
	}
}

// Liveness handles /health endpoint (liveness probe)
// Always returns 200 if process is running
func (h *Handler) Liveness(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(Response{Status: StatusHealthy})
}

// Readiness handles /ready endpoint (readiness probe)
// Checks all dependencies and returns 503 if any are unhealthy
func (h *Handler) Readiness(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), h.timeout)
	defer cancel()

	response := Response{Status: StatusHealthy}

	for _, checker := range h.readinessCheckers {
		result := checker.Check(ctx)
		response.Checks = append(response.Checks, result)
		if result.Status == StatusUnhealthy {
			response.Status = StatusUnhealthy
		}
	}

	status := http.StatusOK
	if response.Status == StatusUnhealthy {
		status = http.StatusServiceUnavailable
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(response)
}

// StorageChecker checks storage connectivity
type StorageChecker struct {
	storage Pingable
}

// Pingable is an interface for storage that can be pinged
type Pingable interface {
	Ping(ctx context.Context) error
}

// NewStorageChecker creates a new storage health checker
func NewStorageChecker(storage Pingable) *StorageChecker {
	return &StorageChecker{storage: storage}
}

func (s *StorageChecker) Name() string {
	return "storage"
}

func (s *StorageChecker) Check(ctx context.Context) CheckResult {
	start := time.Now()
	err := s.storage.Ping(ctx)
	latency := time.Since(start)

	if err != nil {
		return CheckResult{
			Name:    s.Name(),
			Status:  StatusUnhealthy,
			Message: err.Error(),
			Latency: latency.String(),
		}
	}

	return CheckResult{
		Name:    s.Name(),
		Status:  StatusHealthy,
		Latency: latency.String(),
	}
}
