package tests

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"producthub/internal/api"
)

func TestSwagger_SpecificationAndUI(t *testing.T) {
	app := api.SetupRouter(api.RouterConfig{
		DB: nil,
	})

	// 1. Verify GET /swagger/doc.json returns valid OpenAPI 3.0 specification
	reqDoc := httptest.NewRequest(http.MethodGet, "/swagger/doc.json", nil)
	respDoc, err := app.Test(reqDoc, -1)
	if err != nil {
		t.Fatalf("Failed to request /swagger/doc.json: %v", err)
	}
	if respDoc.StatusCode != http.StatusOK {
		t.Fatalf("Expected GET /swagger/doc.json to return 200 OK, got %d", respDoc.StatusCode)
	}

	bodyDoc, err := io.ReadAll(respDoc.Body)
	if err != nil {
		t.Fatalf("Failed to read body from /swagger/doc.json: %v", err)
	}

	var openAPISpec struct {
		OpenAPI string `json:"openapi"`
		Info    struct {
			Title   string `json:"title"`
			Version string `json:"version"`
		} `json:"info"`
		Paths map[string]interface{} `json:"paths"`
	}
	if err := json.Unmarshal(bodyDoc, &openAPISpec); err != nil {
		t.Fatalf("Failed to parse OpenAPI JSON document: %v", err)
	}
	if openAPISpec.OpenAPI != "3.0.3" {
		t.Errorf("Expected OpenAPI version '3.0.3', got '%s'", openAPISpec.OpenAPI)
	}
	if !strings.Contains(openAPISpec.Info.Title, "ProductHub") {
		t.Errorf("Expected Info.Title to contain 'ProductHub', got '%s'", openAPISpec.Info.Title)
	}
	if len(openAPISpec.Paths) < 10 {
		t.Errorf("Expected at least 10 documented endpoints in OpenAPI spec, got %d", len(openAPISpec.Paths))
	}

	// 2. Verify GET /docs/swagger.json alternate alias
	reqDocAlias := httptest.NewRequest(http.MethodGet, "/docs/swagger.json", nil)
	respDocAlias, _ := app.Test(reqDocAlias, -1)
	if respDocAlias.StatusCode != http.StatusOK {
		t.Errorf("Expected GET /docs/swagger.json to return 200 OK, got %d", respDocAlias.StatusCode)
	}

	// 3. Verify GET /swagger serves interactive Swagger UI HTML
	reqUI := httptest.NewRequest(http.MethodGet, "/swagger", nil)
	respUI, err := app.Test(reqUI, -1)
	if err != nil {
		t.Fatalf("Failed to request /swagger: %v", err)
	}
	if respUI.StatusCode != http.StatusOK {
		t.Fatalf("Expected GET /swagger to return 200 OK, got %d", respUI.StatusCode)
	}

	bodyUI, err := io.ReadAll(respUI.Body)
	if err != nil {
		t.Fatalf("Failed to read body from /swagger: %v", err)
	}
	if !strings.Contains(string(bodyUI), "swagger-ui") {
		t.Errorf("Expected Swagger UI HTML to contain 'swagger-ui'")
	}
	if !strings.Contains(string(bodyUI), "/swagger/doc.json") {
		t.Errorf("Expected Swagger UI HTML to reference '/swagger/doc.json'")
	}

	// 4. Verify GET /docs serves interactive documentation
	reqDocs := httptest.NewRequest(http.MethodGet, "/docs", nil)
	respDocs, _ := app.Test(reqDocs, -1)
	if respDocs.StatusCode != http.StatusOK {
		t.Errorf("Expected GET /docs to return 200 OK, got %d", respDocs.StatusCode)
	}
}
