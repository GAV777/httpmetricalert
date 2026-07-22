package main

import (
	"crypto/rsa"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/GAV777/httpmetricalert/internal/config"
	"github.com/GAV777/httpmetricalert/internal/handlers"
	"github.com/GAV777/httpmetricalert/internal/middleware"
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/hlog"
)

func setupRouter(handler *handlers.MetricsHandler, privKey *rsa.PrivateKey, cfg *config.Config) http.Handler {
	r := chi.NewRouter()

	// Логирование
	r.Use(hlog.NewHandler(zerolog.New(os.Stdout)))
	r.Use(hlog.AccessHandler(accessLog))
	r.Use(hlog.RequestIDHandler("req_id", "Request-Id"))

	// Защита от //
	r.Use(noDoubleSlashes)
	r.Use(chimiddleware.StripSlashes)

	// CryptoMiddleware — расшифровка запросов (до gzip)
	r.Use(middleware.CryptoMiddleware(privKey))

	// Подключаем gzip middleware
	r.Use(middleware.GzipMiddleware)

	// Подключаем middleware для проверки хеша (после gzip — хеш от распакованного тела)
	secretKey := cfg.Key
	r.Use(middleware.HashMiddleware(secretKey))

	// Подключаем middleware для проверки доверенной подсети
	trustedSubnet := cfg.TrustedSubnet
	r.Use(middleware.TrustedSubnetMiddleware(trustedSubnet))

	// Маршруты
	r.Post("/update", handler.UpdateJSONHandler)
	r.Post("/value", handler.GetValueJSONHandler)
	r.Post("/update/{type}/{name}/{value}", handler.UpdateHandler)
	r.Post("/updates", handler.UpdateBatchHandler)
	r.Get("/value/{type}/{name}", handler.GetValueHandler)
	r.Get("/", handler.ListMetricsHandler)
	r.Get("/ping", handler.PingDB)
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
