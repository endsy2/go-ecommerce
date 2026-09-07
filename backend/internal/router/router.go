// Package router owns the route table. It receives already-constructed handlers
// and returns a configured *gin.Engine; it constructs nothing itself, so the
// wiring stays visible in one place (cmd/api/main.go).
package router

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"ecommerce/backend/internal/config"
	"ecommerce/backend/internal/handler"
	"ecommerce/backend/internal/middleware"
)

// Handlers is the set of handler structs the router needs. Adding a feature
// means adding a field here and a route below — the compiler then forces main.go
// to construct it, so a half-wired feature cannot reach runtime.
type Handlers struct {
	Health *handler.HealthHandler
	Auth   *handler.AuthHandler
}

// New builds the engine: middleware chain first, then routes.
//
// tokens is what RequireAuth validates with. It is a parameter rather than
// something constructed here, for the same reason the handlers are: this
// package assembles nothing, so cmd/api/main.go remains the single readable
// account of how the application is put together.
func New(cfg *config.Config, h Handlers, tokens middleware.TokenValidator) *gin.Engine {
	if !cfg.IsDevelopment() {
		gin.SetMode(gin.ReleaseMode)
	}

	// gin.New() rather than gin.Default(): Default installs gin's own logger and
	// recovery, and we want our structured slog versions instead.
	engine := gin.New()

	// Order is deliberate:
	//   RequestID first, so every later log line and error carries the id.
	//   Recovery next, so it wraps everything downstream of it.
	//   Logger after Recovery, so a recovered panic is still logged as a 500.
	//   CORS last, so preflight requests are cheap and still correlated.
	//
	// These are engine-wide. Auth is NOT here: it belongs to the groups below,
	// because most of this API is deliberately public.
	engine.Use(
		middleware.RequestID(),
		middleware.Recovery(),
		middleware.Logger(),
		middleware.CORS(cfg.HTTP.CORSAllowedOrigins),
	)

	// Health probes sit OUTSIDE /api/v1 on purpose: they are infrastructure
	// endpoints for orchestrators and load balancers, not part of the public API
	// contract. Versioning them would mean a v2 cutover silently breaks every
	// probe configuration.
	engine.GET("/health", h.Health.Live)
	engine.GET("/health/ready", h.Health.Ready)

	v1 := api(engine)

	// --- Public -----------------------------------------------------------
	// No auth middleware. Login is here because requiring a token in order to
	// get a token would be circular; the catalogue is here because the
	// storefront has to render before anyone has signed in.
	v1.POST("/auth/login", h.Auth.Login)
	// v1.GET("/products", h.Product.List)
	// v1.GET("/products/:slug", h.Product.GetBySlug)
	// v1.GET("/categories", h.Category.List)
	// v1.GET("/categories/:slug", h.Category.GetBySlug)

	// --- Authenticated ----------------------------------------------------
	// RequireAuth is attached to the GROUP, not repeated on each route. A route
	// registered on this group cannot skip the middleware, so protection is
	// structural rather than something the next handler has to remember.
	//
	// Group("") adds no path segment: the group exists purely to carry
	// middleware, so these routes keep their natural /api/v1/... paths.
	protected := v1.Group("", middleware.RequireAuth(tokens))
	protected.GET("/auth/me", h.Auth.Me)

	// --- Admin ------------------------------------------------------------
	// Nested inside protected, so the chain is RequireAuth -> RequireRole and
	// the ordering cannot be got wrong. Hanging RequireRole off v1 instead
	// would allow an admin route that never authenticates, where RequireRole
	// finds no role on the context and has to reject blind.
	//
	// Uncomment together with the first admin handler.
	//
	// Writes address a product by :id while the public reads use :slug. A slug
	// is derived from the name, so a PUT that renames a product changes the
	// slug and invalidates the very URL it was sent to. The id never moves.
	//
	// The two names can coexist only because gin keeps a SEPARATE radix tree
	// per HTTP method: GET /products/:slug and PUT /products/:id live in
	// different trees and never meet. Within ONE method, two different
	// wildcard names at the same position panic at startup — so adding a
	// second GET on /products/:id would refuse to boot. Verified, not assumed.
	//
	// admin := protected.Group("", middleware.RequireRole(model.RoleAdmin))
	// admin.POST("/products", h.Product.Create)
	// admin.PUT("/products/:id", h.Product.Update)
	// admin.DELETE("/products/:id", h.Product.Delete)
	// admin.POST("/categories", h.Category.Create)
	// admin.PUT("/categories/:id", h.Category.Update)
	// admin.DELETE("/categories/:id", h.Category.Delete)

	engine.NoRoute(func(c *gin.Context) {
		handler.RespondError(c, http.StatusNotFound, "not_found", "no such endpoint")
	})
	engine.NoMethod(func(c *gin.Context) {
		handler.RespondError(c, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed for this endpoint")
	})

	return engine
}

// api creates the versioned route group required by CLAUDE.md (/api/v1/...).
func api(engine *gin.Engine) *gin.RouterGroup {
	return engine.Group("/api/v1")
}
