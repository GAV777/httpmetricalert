package main

import (
	"github.com/GAV777/httpmetricalert/internal/config"
	"github.com/GAV777/httpmetricalert/internal/handlers"
	"github.com/GAV777/httpmetricalert/internal/storage"
	"log"
	"net/http"

	"github.com/rs/zerolog"
)

func main() {
	// Инициализация логгера
	zerolog.TimeFieldFormat = "2006-01-02T15:04:05Z07:00"
	zerolog.SetGlobalLevel(zerolog.InfoLevel)

	config.ParseFlags()

	addr := config.GetServerAddress()
	store := storage.NewStorage(config.MigrationsDir())
	handler := handlers.NewMetricsHandler(store)

	router := setupRouter(handler)

	log.Printf("Starting server on %s", addr)
	log.Fatal(http.ListenAndServe(addr, router))
}
