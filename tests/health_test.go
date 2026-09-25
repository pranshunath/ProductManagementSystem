package tests

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"producthub/internal/api"
	"producthub/pkg/response"
)

func TestHealthEndpoint_DegradedWhenNoDB(t *testing.T) {
	// Initialize router without DB connection
	app := api.SetupRouter(api.RouterConfig{
		DB: nil,
	})

	// Construct HTTP request
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)

	// Execute through Fiber in-memory test engine
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("Failed to execute request: %v", err)
	}

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("Expected status code %d, got %d", http.StatusServiceUnavailable, resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	var res response.Response
	if err := json.Unmarshal(bodyBytes, &res); err != nil {
		t.Fatalf("Failed to parse JSON response: %v", err)
	}

	if !res.Success {
		t.Errorf("Expected res.Success to be true, got %v", res.Success)
	}

	dataMap, ok := res.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected res.Data to be a map, got %T", res.Data)
	}

	if dataMap["service"] != "ProductHub API" {
		t.Errorf("Expected service 'ProductHub API', got '%v'", dataMap["service"])
	}

	if dataMap["database"] != "disconnected" {
		t.Errorf("Expected database 'disconnected', got '%v'", dataMap["database"])
	}
}

func TestNotFoundEndpoint(t *testing.T) {
	app := api.SetupRouter(api.RouterConfig{
		DB: nil,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/non-existent-endpoint", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("Failed to execute request: %v", err)
	}

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected status code %d, got %d", http.StatusNotFound, resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	var res response.Response
	if err := json.Unmarshal(bodyBytes, &res); err != nil {
		t.Fatalf("Failed to parse JSON response: %v", err)
	}

	if res.Success {
		t.Errorf("Expected res.Success to be false for 404, got %v", res.Success)
	}

	if res.Error == nil || res.Error.Code != "NOT_FOUND" {
		t.Errorf("Expected error code NOT_FOUND, got %v", res.Error)
	}
}
