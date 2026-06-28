package main

import (
	"github.com/GAV777/httpmetricalert/internal/audit"
	"github.com/GAV777/httpmetricalert/internal/config"
	"github.com/GAV777/httpmetricalert/internal/handlers"
	"github.com/GAV777/httpmetricalert/internal/storage"
	"log"
	"net/http"

	"github.com/rs/zerolog"
)

var (
	buildVersion = "N/A"
	buildDate    = "N/A"
	buildCommit  = "N/A"
)

func main() {
	printBuildInfo()

	// Инициализация логгера
	zerolog.TimeFieldFormat = "2006-01-02T15:04:05Z07:00"
	zerolog.SetGlobalLevel(zerolog.InfoLevel)

	config.ParseFlags()

	addr := config.ServerAddress()
	config.ValidateServerAddress()

	store := storage.NewStorage()

	// Создаём нотификатор аудита
	notifier := setupAuditNotifier()

	handler := handlers.NewMetricsHandler(store, notifier)

	router := setupRouter(handler)

	log.Printf("Starting server on %s", addr)
	log.Fatal(http.ListenAndServe(addr, router))
}

// setupAuditNotifier создаёт и настраивает нотификатор аудита
func setupAuditNotifier() *audit.Notifier {
	notifier := audit.NewNotifier()

	if file := config.AuditFile(); file != "" {
		obs, err := audit.NewFileObserver(file)
		if err != nil {
			log.Printf("Failed to create file observer for %s: %v", file, err)
		} else {
			notifier.AddObserver(obs)
			log.Printf("Audit file observer enabled: %s", file)
		}
	}

	if url := config.AuditURL(); url != "" {
		notifier.AddObserver(audit.NewHTTPObserver(url))
		log.Printf("Audit HTTP observer enabled: %s", url)
	}

	if !notifier.HasObservers() {
		log.Println("Audit disabled (no --audit-file or --audit-url configured)")
	}

	return notifier
}

func printBuildInfo() {
	log.Printf("Build version: %s\n", buildVersion)
	log.Printf("Build date: %s\n", buildDate)
	log.Printf("Build commit: %s\n", buildCommit)
}
