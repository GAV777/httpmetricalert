package main

import (
	"context"
	"crypto/rsa"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/GAV777/httpmetricalert/internal/audit"
	"github.com/GAV777/httpmetricalert/internal/config"
	"github.com/GAV777/httpmetricalert/internal/handlers"
	"github.com/GAV777/httpmetricalert/internal/storage"
	"github.com/GAV777/httpmetricalert/pkg/crypto"
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

	// Загружаем приватный ключ, если указан
	var privKey *rsa.PrivateKey
	if cryptoKeyPath := config.CryptoKey(); cryptoKeyPath != "" {
		var err error
		privKey, err = crypto.LoadPrivateKey(cryptoKeyPath)
		if err != nil {
			log.Fatalf("Failed to load private key: %v", err)
		}
		log.Println("RSA decryption enabled")
	}

	// Создаём нотификатор аудита
	notifier := setupAuditNotifier()

	handler := handlers.NewMetricsHandler(store, notifier)

	router := setupRouter(handler, privKey)

	srv := &http.Server{
		Addr:    addr,
		Handler: router,
	}

	// Канал для сигналов завершения
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT)

	// Запускаем сервер в горутине
	go func() {
		log.Printf("Starting server on %s", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	// Ждём сигнал завершения
	sig := <-sigCh
	log.Printf("Received signal: %v", sig)

	// Graceful shutdown: даём 30с на завершение in-flight запросов
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	log.Println("Shutting down server...")
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("Server forced to shutdown: %v", err)
	}

	// Финальное сохранение и очистка ресурсов
	if err := store.Close(); err != nil {
		log.Printf("Failed to close storage: %v", err)
	}
	log.Println("Storage closed")

	notifier.Close()
	log.Println("Audit notifier closed")

	log.Println("Server stopped gracefully")
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
