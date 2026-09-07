package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"ecommerce/backend/internal/database"
)

// HealthHandler answers the two infrastructure probes.
//
// It takes *gorm.DB and *redis.Client directly rather than going through a
// service, because there is no business logic to apply — it is reporting on the
// process's own plumbing, not on domain state.
type HealthHandler struct {
	db  *gorm.DB
	rdb *redis.Client
}

// NewHealthHandler wires the health probes to the live connections.
func NewHealthHandler(db *gorm.DB, rdb *redis.Client) *HealthHandler {
	return &HealthHandler{db: db, rdb: rdb}
}

type dependencyStatus struct {
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

type readinessResponse struct {
	Status       string                      `json:"status"`
	Dependencies map[string]dependencyStatus `json:"dependencies"`
}

// Live answers GET /health.
//
// It checks nothing. Liveness means "the process is running and can serve HTTP";
// if it ever fails, the right response is to restart the container. Deliberately
// NOT checking the database here is the whole point: a liveness probe that pings
// Postgres will restart-loop every instance during a brief database blip, turning
// a short outage into a total one.
func (h *HealthHandler) Live(c *gin.Context) {
	OK(c, gin.H{"status": "ok"})
}

// Ready answers GET /health/ready.
//
// It reports whether this instance can actually serve traffic right now. A 503
// tells a load balancer to route elsewhere — not to restart the process.
//
// Postgres failing means not ready: it is the source of truth. Redis failing
// means degraded but still serving, since it is only a cache; it is reported so
// the state is visible without taking the instance out of rotation.
func (h *HealthHandler) Ready(c *gin.Context) {
	// Bound the probe independently of the request: a hung database must not
	// make the health check itself hang.
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()

	deps := make(map[string]dependencyStatus, 2)
	ready := true

	if err := h.pingPostgres(ctx); err != nil {
		deps["postgres"] = dependencyStatus{Status: "down", Error: err.Error()}
		ready = false
	} else {
		deps["postgres"] = dependencyStatus{Status: "up"}
	}

	if err := database.PingRedis(ctx, h.rdb); err != nil {
		deps["redis"] = dependencyStatus{Status: "down", Error: err.Error()}
	} else {
		deps["redis"] = dependencyStatus{Status: "up"}
	}

	body := readinessResponse{Status: "ready", Dependencies: deps}
	status := http.StatusOK

	if !ready {
		body.Status = "not_ready"
		status = http.StatusServiceUnavailable
	} else if deps["redis"].Status == "down" {
		body.Status = "degraded"
	}

	Respond(c, status, body)
}

func (h *HealthHandler) pingPostgres(ctx context.Context) error {
	sqlDB, err := h.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.PingContext(ctx)
}
