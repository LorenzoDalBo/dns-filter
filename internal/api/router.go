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

	// Health check — no auth required (RNF06.3)
	r.Get("/health", h.HealthCheck)

	// Public auth endpoint
	r.Post("/api/auth/login", h.Login)

	// Protected API routes (RF10.2, RF10.3)
	r.Route("/api", func(r chi.Router) {
		r.Use(h.AuthMiddleware)

		// Metrics (RF10.6)
		r.Get("/metrics", h.GetMetrics)

		// Logs — viewer and admin (RF10.1)
		r.Get("/logs", h.GetLogs)

		// Cache management (RF10.5)
		r.Delete("/cache/{domain}", h.InvalidateCache)

		// Admin-only routes
		r.Group(func(r chi.Router) {
			r.Use(h.AdminOnly)

			// Users CRUD (RF08.4)
			r.Get("/users", h.ListUsers)
			r.Post("/users", h.CreateUser)

			// Groups CRUD (RF08.5)
			r.Get("/groups", h.ListGroups)
			r.Post("/groups", h.CreateGroup)

			// Blocklists CRUD (RF08.6)
			r.Get("/lists", h.ListBlocklists)
			r.Post("/lists", h.CreateBlocklist)
			r.Post("/lists/{id}/entries", h.AddEntries)
			r.Post("/lists/reload", h.ReloadLists)

			// IP Ranges CRUD (RF08.7)
			r.Get("/ranges", h.ListRanges)
			r.Post("/ranges", h.CreateRange)
		})
	})

	return r
}