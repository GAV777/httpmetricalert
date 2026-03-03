// cmd/server/main.go
package main

import (
	"flag"
	"github.com/GAV777/httpmetricalert/internal/handlers"
	"github.com/GAV777/httpmetricalert/internal/storage"
	"log"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// === Middleware для блокировки путей с двойными слешами ===
func noDoubleSlashes(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "//") {
			http.Error(w, "Not Found", http.StatusNotFound)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// === Основная функция — ТОЧКА ВХОДА ===
func main() {
	addr := flag.String("a", "localhost:8080", "адрес эндпоинта HTTP-сервера")
	flag.Parse()

	if len(flag.Args()) > 0 {
		log.Fatalf("неизвестные аргументы командной строки: %v", flag.Args())
	}

	// Создаём хранилище
	storage := storage.NewMemStorage()

	// Создаём хендлеры с внедрением зависимости
	handler := handlers.NewMetricsHandler(storage)

	r := chi.NewRouter()

	r.Use(middleware.Recoverer)
	r.Use(noDoubleSlashes)

	// Маршруты
	r.Post("/update/{type}/{name}/{value}", handler.UpdateHandler)
	r.Get("/value/{type}/{name}", handler.GetValueHandler)
	r.Get("/", handler.ListMetricsHandler)

	// Глобальный 404
	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Not Found", http.StatusNotFound)
	})

	log.Printf("🚀 Starting server on %s", *addr)
	log.Fatal(http.ListenAndServe(*addr, r))
}
