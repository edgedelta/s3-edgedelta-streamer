package health

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestBasicHealthChecker(t *testing.T) {
	checker := NewBasicHealthChecker()

	err := checker.Check(context.Background())
	if err != nil {
		t.Errorf("BasicHealthChecker.Check() returned error: %v", err)
	}

	if checker.Name() != "basic" {
		t.Errorf("Expected name 'basic', got '%s'", checker.Name())
	}
}

func TestHTTPHealthChecker(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	checker := NewHTTPHealthChecker(server.URL)

	err := checker.Check(context.Background())
	if err != nil {
		t.Errorf("HTTPHealthChecker.Check() returned error: %v", err)
	}

	if checker.Name() != "http" {
		t.Errorf("Expected name 'http', got '%s'", checker.Name())
	}
}

func TestHTTPHealthChecker_Unhealthy(t *testing.T) {
	// Create a test server that returns 500
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	checker := NewHTTPHealthChecker(server.URL)

	err := checker.Check(context.Background())
	if err == nil {
		t.Error("HTTPHealthChecker.Check() should have returned error for 500 status")
	}
}

func TestHTTPHealthChecker_Timeout(t *testing.T) {
	// Create a test server that sleeps
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	checker := NewHTTPHealthChecker(server.URL)

	// Use a very short timeout context
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	err := checker.Check(ctx)
	if err == nil {
		t.Error("HTTPHealthChecker.Check() should have timed out")
	}
}

func TestHealthServer_HealthHandler(t *testing.T) {
	checker := NewBasicHealthChecker()
	server := NewHealthServer(":0", "/health", checker)
	defer server.Stop(context.Background())

	// Create a test request
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	server.healthHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Check response content type
	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Expected content-type 'application/json', got '%s'", contentType)
	}
}

func TestHealthServer_ReadyHandler(t *testing.T) {
	checker := NewBasicHealthChecker()
	server := NewHealthServer(":0", "/health", checker)
	defer server.Stop(context.Background())

	// Create a test request
	req := httptest.NewRequest("GET", "/ready", nil)
	w := httptest.NewRecorder()

	server.readyHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}
