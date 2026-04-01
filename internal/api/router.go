package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// NewRouter creates the REST API router with all routes.
func NewRouter(h *Handlers) http.Handler {
	r := chi.NewRouter()

	// Built-in middleware
	r.Use(middleware.Recoverer)
	r.Use(middleware.RealIP)

	// Rate limiting: 10 requests/second per IP, burst of 20 (RNF03.4)
	loginLimiter := NewRateLimiter(5, 10) // stricter for login
	apiLimiter := NewRateLimiter(30, 60)  // general API

	// Health check — no auth, no rate limit (RNF06.3)
	r.Get("/health", h.HealthCheck)

	// Public auth endpoint with strict rate limiting
	r.With(loginLimiter.Middleware).Post("/api/auth/login", h.Login)

	// Protected API routes (RF10.2, RF10.3)
	r.Route("/api", func(r chi.Router) {
		r.Use(apiLimiter.Middleware)
		r.Use(h.AuthMiddleware)
		// Refresh token (RNF03.3)
		r.Post("/auth/refresh", h.RefreshToken)

		// Dashboard stats (RF08.1)
		r.Get("/dashboard", h.GetDashboardStats)

		// Metrics (RF10.6)
		r.Get("/metrics", h.GetMetrics)

		// Logs — viewer and admin (RF10.1)
		r.Get("/logs", h.GetLogs)

		// Cache management (RF10.5)
		r.Delete("/cache/{domain}", h.InvalidateCache)

		// Admin-only routes
		r.Group(func(r chi.Router) {
			r.Use(h.AdminOnly)

			// Users CRUD (RF08.4, RF10.1)
			r.Get("/users", h.ListUsers)
			r.Post("/users", h.CreateUser)
			r.Put("/users/{id}", h.UpdateUser)
			r.Delete("/users/{id}", h.DeleteUser)

			// Groups CRUD (RF08.5, RF10.1)
			r.Get("/groups", h.ListGroups)
			r.Post("/groups", h.CreateGroup)
			r.Put("/groups/{id}", h.UpdateGroup)
			r.Delete("/groups/{id}", h.DeleteGroup)
			// Categories & Policies (RF03.3, RF03.4)
			r.Get("/categories", h.ListCategories)
			r.Get("/groups/{id}/policy", h.GetGroupPolicy)
			r.Put("/groups/{id}/policy", h.SetGroupPolicy)

			// Blocklists CRUD (RF08.6, RF10.1)
			r.Get("/lists", h.ListBlocklists)
			r.Post("/lists", h.CreateBlocklist)
			r.Put("/lists/{id}", h.UpdateBlocklist)
			r.Delete("/lists/{id}", h.DeleteBlocklist)
			r.Post("/lists/{id}/entries", h.AddEntries)
			r.Post("/lists/reload", h.ReloadLists)
			r.Post("/lists/download", h.DownloadLists)

			r.Get("/lists/{id}/categories", h.GetBlocklistCategories)
			r.Put("/lists/{id}/categories", h.SetBlocklistCategories)

			// IP Ranges CRUD (RF08.7, RF10.1)
			r.Get("/ranges", h.ListRanges)
			r.Post("/ranges", h.CreateRange)
			r.Put("/ranges/{id}", h.UpdateRange)
			r.Delete("/ranges/{id}", h.DeleteRange)
		})
	})

	// Serve React dashboard from embedded frontend (RNF07.1)
	r.NotFound(FrontendHandler().ServeHTTP)

	return r
}
