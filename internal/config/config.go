package config

import (
	"flag"
	"log"
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
	dsn := getEnvOrDefault("DATABASE_DSN", "")

	flag.IntVar(&storeInterval, "i", parseInt(interval), "Store interval in seconds (0 for sync)")
	flag.StringVar(&fileStoragePath, "f", path, "File path to store metrics")
	flag.BoolVar(&restore, "r", parseBool(restoreStr), "Restore metrics from file on start")
	flag.BoolVar(&gzipEnabled, "g", parseBool(gzipStr), "Enable GZIP compression for responses")
	flag.StringVar(&databaseDSN, "d", dsn, "Database DSN (PostgreSQL)")
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
func ParseFlags()             { flag.Parse() }
func DatabaseDSN() string     { return databaseDSN }
