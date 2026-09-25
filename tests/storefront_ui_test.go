package tests

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"producthub/internal/api"
)

func TestStorefrontUI_StaticAssetServing(t *testing.T) {
	app := api.SetupRouter(api.RouterConfig{
		DB: nil,
	})

	// 1. Verify GET / serves storefront index.html
	reqRoot := httptest.NewRequest(http.MethodGet, "/", nil)
	respRoot, err := app.Test(reqRoot, -1)
	if err != nil {
		t.Fatalf("Failed to request /: %v", err)
	}
	if respRoot.StatusCode != http.StatusOK {
		t.Fatalf("Expected GET / to return 200 OK, got %d", respRoot.StatusCode)
	}
	bodyRoot, err := io.ReadAll(respRoot.Body)
	if err != nil {
		t.Fatalf("Failed to read body from /: %v", err)
	}
	if !strings.Contains(string(bodyRoot), "ProductHub") {
		t.Errorf("Expected body to contain 'ProductHub', got: %s", string(bodyRoot[:min(len(bodyRoot), 150)]))
	}

	// 2. Verify GET /css/style.css serves valid stylesheet
	reqCSS := httptest.NewRequest(http.MethodGet, "/css/style.css", nil)
	respCSS, err := app.Test(reqCSS, -1)
	if err != nil {
		t.Fatalf("Failed to request /css/style.css: %v", err)
	}
	if respCSS.StatusCode != http.StatusOK {
		t.Fatalf("Expected GET /css/style.css to return 200 OK, got %d", respCSS.StatusCode)
	}
	bodyCSS, err := io.ReadAll(respCSS.Body)
	if err != nil {
		t.Fatalf("Failed to read body from /css/style.css: %v", err)
	}
	if !strings.Contains(string(bodyCSS), "--bg-primary") {
		t.Errorf("Expected CSS to contain design token '--bg-primary'")
	}

	// 3. Verify GET /js/app.js serves JavaScript application code
	reqJS := httptest.NewRequest(http.MethodGet, "/js/app.js", nil)
	respJS, err := app.Test(reqJS, -1)
	if err != nil {
		t.Fatalf("Failed to request /js/app.js: %v", err)
	}
	if respJS.StatusCode != http.StatusOK {
		t.Fatalf("Expected GET /js/app.js to return 200 OK, got %d", respJS.StatusCode)
	}
	bodyJS, err := io.ReadAll(respJS.Body)
	if err != nil {
		t.Fatalf("Failed to read body from /js/app.js: %v", err)
	}
	if !strings.Contains(string(bodyJS), "executeCheckout") {
		t.Errorf("Expected JS to contain 'executeCheckout'")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
