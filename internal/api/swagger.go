package api

import (
	"os"

	"github.com/gofiber/fiber/v2"
)

// RegisterSwaggerRoutes registers the OpenAPI specification and interactive Swagger UI interface
func RegisterSwaggerRoutes(app *fiber.App) {
	// 1. Serve raw OpenAPI 3.0 specification JSON
	app.Get("/swagger/doc.json", serveSwaggerDoc)
	app.Get("/docs/swagger.json", serveSwaggerDoc)

	// 2. Serve interactive Swagger UI page
	swaggerUIHandler := func(c *fiber.Ctx) error {
		c.Set("Content-Type", "text/html; charset=utf-8")
		return c.SendString(swaggerUIHTML)
	}

	app.Get("/swagger", swaggerUIHandler)
	app.Get("/swagger/", swaggerUIHandler)
	app.Get("/docs", swaggerUIHandler)
	app.Get("/docs/", swaggerUIHandler)
}

func serveSwaggerDoc(c *fiber.Ctx) error {
	docPath := "./docs/swagger.json"
	if _, err := os.Stat(docPath); os.IsNotExist(err) {
		if _, err := os.Stat("../docs/swagger.json"); err == nil {
			docPath = "../docs/swagger.json"
		}
	}

	data, err := os.ReadFile(docPath)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"error": fiber.Map{
				"code":    "DOC_NOT_FOUND",
				"message": "OpenAPI specification document could not be located",
			},
		})
	}

	c.Set("Content-Type", "application/json")
	return c.Send(data)
}

const swaggerUIHTML = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>ProductHub — Interactive API Documentation</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css">
  <link rel="stylesheet" href="/css/style.css">
  <style>
    body {
      background-color: #0a0e17;
      color: #f9fafb;
      margin: 0;
      padding: 0;
      font-family: 'Inter', sans-serif;
    }
    .swagger-header-bar {
      background: rgba(17, 24, 39, 0.95);
      border-bottom: 1px solid rgba(255, 255, 255, 0.08);
      padding: 1rem 2rem;
      display: flex;
      justify-content: space-between;
      align-items: center;
      position: sticky;
      top: 0;
      z-index: 100;
      backdrop-filter: blur(12px);
    }
    .swagger-brand {
      display: flex;
      align-items: center;
      gap: 0.75rem;
      font-family: 'Outfit', sans-serif;
      font-size: 1.35rem;
      font-weight: 700;
      color: #ffffff;
      text-decoration: none;
    }
    .swagger-brand span {
      background: linear-gradient(135deg, #6366f1 0%, #06b6d4 100%);
      -webkit-background-clip: text;
      -webkit-text-fill-color: transparent;
    }
    .swagger-links {
      display: flex;
      gap: 0.75rem;
    }
    .swagger-btn {
      background: rgba(255, 255, 255, 0.06);
      border: 1px solid rgba(255, 255, 255, 0.12);
      color: #f9fafb;
      padding: 0.45rem 0.95rem;
      border-radius: 8px;
      font-size: 0.85rem;
      font-weight: 600;
      text-decoration: none;
      display: inline-flex;
      align-items: center;
      gap: 0.4rem;
      transition: all 0.2s ease;
    }
    .swagger-btn:hover {
      background: rgba(255, 255, 255, 0.12);
      border-color: #6366f1;
    }
    /* Swagger UI Dark Aesthetic Overrides */
    .swagger-ui {
      max-width: 1400px;
      margin: 0 auto;
      padding: 1.5rem 2rem 4rem;
    }
    .swagger-ui .info {
      margin: 1.5rem 0;
    }
    .swagger-ui .info .title {
      color: #f9fafb;
      font-family: 'Outfit', sans-serif;
      font-weight: 700;
    }
    .swagger-ui .info p, .swagger-ui .info li {
      color: #9ca3af;
    }
    .swagger-ui .scheme-container {
      background: rgba(17, 24, 39, 0.7);
      border: 1px solid rgba(255, 255, 255, 0.08);
      border-radius: 12px;
      box-shadow: none;
      padding: 1.25rem 1.5rem;
      margin-bottom: 2rem;
    }
    .swagger-ui .opblock {
      background: rgba(17, 24, 39, 0.55);
      border: 1px solid rgba(255, 255, 255, 0.08);
      border-radius: 10px;
      margin-bottom: 1rem;
      box-shadow: none;
    }
    .swagger-ui .opblock.opblock-get { border-color: rgba(59, 130, 246, 0.4); background: rgba(59, 130, 246, 0.05); }
    .swagger-ui .opblock.opblock-post { border-color: rgba(16, 185, 129, 0.4); background: rgba(16, 185, 129, 0.05); }
    .swagger-ui .opblock.opblock-put { border-color: rgba(245, 158, 11, 0.4); background: rgba(245, 158, 11, 0.05); }
    .swagger-ui .opblock.opblock-delete { border-color: rgba(239, 68, 68, 0.4); background: rgba(239, 68, 68, 0.05); }
    .swagger-ui .opblock .opblock-summary-method {
      border-radius: 6px;
      font-weight: 700;
    }
    .swagger-ui .opblock .opblock-summary-path {
      color: #f9fafb;
      font-weight: 600;
      font-size: 0.95rem;
    }
    .swagger-ui .opblock .opblock-summary-description {
      color: #9ca3af;
    }
    .swagger-ui section.models {
      background: rgba(17, 24, 39, 0.55);
      border: 1px solid rgba(255, 255, 255, 0.08);
      border-radius: 12px;
    }
    .swagger-ui section.models h4 {
      color: #f9fafb;
    }
    .swagger-ui .btn.authorize {
      background: #6366f1;
      border-color: #6366f1;
      color: #ffffff;
      border-radius: 8px;
    }
    .swagger-ui .btn.authorize svg {
      fill: #ffffff;
    }
  </style>
</head>
<body>
  <header class="swagger-header-bar">
    <a href="/" class="swagger-brand">
      <div style="background: rgba(99, 102, 241, 0.2); border: 1px solid rgba(99, 102, 241, 0.4); width: 36px; height: 36px; display: flex; align-items: center; justify-content: center; border-radius: 8px;">
        ⚡
      </div>
      <div>Product<span>Hub</span> Docs</div>
    </a>
    <div class="swagger-links">
      <a href="/" class="swagger-btn"><span>🏪</span> Storefront</a>
      <a href="/admin.html" class="swagger-btn"><span>🛠️</span> Admin Console</a>
      <a href="/swagger/doc.json" class="swagger-btn" target="_blank"><span>📄</span> OpenAPI Spec (JSON)</a>
    </div>
  </header>

  <div id="swagger-ui"></div>

  <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
  <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-standalone-preset.js"></script>
  <script>
    window.onload = function() {
      SwaggerUIBundle({
        url: "/swagger/doc.json",
        dom_id: '#swagger-ui',
        deepLinking: true,
        presets: [
          SwaggerUIBundle.presets.apis,
          SwaggerUIStandalonePreset
        ],
        layout: "BaseLayout",
        persistAuthorization: true,
        displayRequestDuration: true,
        docExpansion: "list"
      });
    };
  </script>
</body>
</html>
`
