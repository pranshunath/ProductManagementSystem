package controllers

import (
	"fmt"
	"time"

	"producthub/pkg/response"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

// HealthController handles service liveness and readiness checks
type HealthController struct {
	db        *gorm.DB
	startTime time.Time
}

// NewHealthController creates a new instance of HealthController
func NewHealthController(db *gorm.DB) *HealthController {
	return &HealthController{
		db:        db,
		startTime: time.Now(),
	}
}

// Check returns the system health status, DB connectivity, and uptime
func (h *HealthController) Check(c *fiber.Ctx) error {
	dbStatus := "disconnected"
	dbLatency := "N/A"

	if h.db != nil {
		sqlDB, err := h.db.DB()
		if err == nil {
			start := time.Now()
			if err := sqlDB.Ping(); err == nil {
				dbStatus = "connected"
				dbLatency = fmt.Sprintf("%.2fms", float64(time.Since(start).Microseconds())/1000.0)
			}
		}
	}

	uptime := time.Since(h.startTime).Truncate(time.Second).String()

	status := "UP"
	if dbStatus != "connected" {
		status = "DEGRADED"
	}

	healthData := fiber.Map{
		"status":    status,
		"service":   "ProductHub API",
		"version":   "1.0.0",
		"uptime":    uptime,
		"database":  dbStatus,
		"db_latency": dbLatency,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}

	statusCode := fiber.StatusOK
	if status == "DEGRADED" {
		statusCode = fiber.StatusServiceUnavailable
	}

	return response.JSON(c, statusCode, healthData)
}
