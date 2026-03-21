package main

import (
	"github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/GAV777/httpmetricalert/internal/handlers"
	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/hlog"
)

func setupRouter(handler *handlers.MetricsHandler) http.Handler {
	r := chi.NewRouter()

	// Middleware
	r.Use(hlog.NewHandler(zerolog.New(os.Stdout)))
	r.Use(hlog.AccessHandler(accessLog))
	r.Use(hlog.RequestIDHandler("req_id", "Request-Id"))
	r.Use(noDoubleSlashes)         // Запрещает двойные слэшы
	r.Use(middleware.StripSlashes) // Удаляет слеши в конце, чтобы /value и /value/ были одинаковыми

	// Маршруты
	r.Post("/update", handler.UpdateJSONHandler)
	r.Post("/value", handler.GetValueJSONHandler)
	r.Post("/update/{type}/{name}/{value}", handler.UpdateHandler)
	r.Get("/value/{type}/{name}", handler.GetValueHandler)
	r.Get("/", handler.ListMetricsHandler)

	// Глобальный 404
	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Not Found", http.StatusNotFound)
	})

	return r
}

func accessLog(r *http.Request, status, size int, duration time.Duration) {
	hlog.FromRequest(r).Info().
		Str("method", r.Method).
		Str("uri", r.RequestURI).
		Int("status", status).
		Int("size", size).
		Dur("duration", duration).
		Msg("handled request")
}

func noDoubleSlashes(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "//") {
			http.Error(w, "Not Found", http.StatusNotFound)
			return
		}
		next.ServeHTTP(w, r)
	})
}
