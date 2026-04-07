package config

import (
	"flag"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

var (
	storeInterval   int
	fileStoragePath string
	restore         bool
	gzipEnabled     bool
	databaseDSN     string
)

func init() {
	interval := getEnvOrDefault("STORE_INTERVAL", "300")
	path := getEnvOrDefault("FILE_STORAGE_PATH", "/tmp/metrics.json")
	restoreStr := getEnvOrDefault("RESTORE", "true")
	gzipStr := getEnvOrDefault("ENABLE_GZIP", "false")

	flag.IntVar(&storeInterval, "i", parseInt(interval), "Store interval in seconds (0 for sync)")
	flag.StringVar(&fileStoragePath, "f", path, "File path to store metrics")
	flag.BoolVar(&restore, "r", parseBool(restoreStr), "Restore metrics from file on start")
	flag.BoolVar(&gzipEnabled, "g", parseBool(gzipStr), "Enable GZIP compression for responses")
	flag.StringVar(&databaseDSN, "d", "", "Database DSN (PostgreSQL)")
}

func getEnvOrDefault(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

func parseInt(s string) int {
	if n, err := time.ParseDuration(s + "s"); err == nil {
		return int(n.Seconds())
	} else if n, err := time.ParseDuration(s); err == nil {
		return int(n.Seconds())
	} else if val, err := strconv.Atoi(s); err == nil {
		return val
	}
	log.Printf("Invalid number format: %s, using default", s)
	return 0
}

func parseBool(s string) bool {
	switch strings.ToLower(s) {
	case "true", "1", "yes", "on":
		return true
	case "false", "0", "no", "off":
		return false
	default:
		return false
	}
}

// Экспорт значений
func StoreInterval() int      { return storeInterval }
func FileStoragePath() string { return fileStoragePath }
func ShouldRestore() bool     { return restore }
func GzipEnabled() bool       { return gzipEnabled }
func ParseFlags() {
	flag.Parse()
	// Если флаг -d не был установлен явно, используем переменную окружения
	if !isFlagSet("d") {
		databaseDSN = getEnvOrDefault("DATABASE_DSN", "")
	}
	// Если флаг -d установлен явно, используем его значение (даже если пустое)
}
func DatabaseDSN() string { return databaseDSN }

// isFlagSet проверяет, был ли флаг установлен в командной строке
func isFlagSet(name string) bool {
	found := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == name {
			found = true
		}
	})
	return found
}
