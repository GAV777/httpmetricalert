// Package config предоставляет конфигурацию для сервера и агента.
//
// Поддерживает три источника конфигурации с приоритетом (по убыванию):
//  1. Флаги командной строки
//  2. Переменные окружения
//  3. JSON-файл конфигурации (задаётся через -config/-c или CONFIG)
//  4. Значения по умолчанию
package config

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config — структура конфигурации, соответствующая JSON-формату.
type Config struct {
	// Общие (сервер + агент)
	Address     string `json:"address"`
	StoreFile   string `json:"store_file,omitempty"`
	DatabaseDSN string `json:"database_dsn,omitempty"`
	CryptoKey   string `json:"crypto_key,omitempty"`
	Key         string `json:"key,omitempty"`

	// Сервер
	StoreInterval int    `json:"store_interval,omitempty"`
	Restore       *bool  `json:"restore,omitempty"`
	EnableGzip    *bool  `json:"enable_gzip,omitempty"`
	AuditFile     string `json:"audit_file,omitempty"`
	AuditURL      string `json:"audit_url,omitempty"`

	// Агент
	PollInterval   int `json:"poll_interval,omitempty"`
	ReportInterval int `json:"report_interval,omitempty"`
	RateLimit      int `json:"rate_limit,omitempty"`
}

// fileConfig — структура для десериализации JSON-файла.
type fileConfig struct {
	Address        string `json:"address"`
	StoreInterval  *int   `json:"store_interval"`
	StoreFile      string `json:"store_file"`
	Restore        *bool  `json:"restore"`
	EnableGzip     *bool  `json:"enable_gzip"`
	DatabaseDSN    string `json:"database_dsn"`
	Key            string `json:"key"`
	CryptoKey      string `json:"crypto_key"`
	AuditFile      string `json:"audit_file"`
	AuditURL       string `json:"audit_url"`
	PollInterval   *int   `json:"poll_interval"`
	ReportInterval *int   `json:"report_interval"`
	RateLimit      *int   `json:"rate_limit"`
}

var (
	// Флаг пути к файлу конфигурации (регистрируется первым)
	configFilePath string

	// Загруженная конфигурация
	cfg Config

	// Отслеживаем, какие флаги были установлены явно
	flagSet = make(map[string]bool)

	// Промежуточные переменные для flag.*Var — нужны для тестов и ParseFlags
	serverAddressFlag   string
	storeIntervalFlag   int
	fileStoragePathFlag string
	restoreFlag         bool
	gzipFlag            bool
	databaseDSNFlag     string
	keyFlag             string
	cryptoKeyFlag       string
	auditFileFlag       string
	auditURLFlag        string
	pollIntervalFlag    int
	reportIntervalFlag  int
	rateLimitFlag       int
)

func init() {
	flag.StringVar(&configFilePath, "config", "", "Path to JSON configuration file")
	flag.StringVar(&configFilePath, "c", "", "Path to JSON configuration file (shorthand)")

	// Серверные флаги
	flag.StringVar(&serverAddressFlag, "a", "localhost:8080", "HTTP server address")
	flag.IntVar(&storeIntervalFlag, "i", 300, "Store interval in seconds (0 for sync)")
	flag.StringVar(&fileStoragePathFlag, "f", "/tmp/metrics.json", "File path to store metrics")
	flag.BoolVar(&restoreFlag, "r", true, "Restore metrics from file on start")
	flag.BoolVar(&gzipFlag, "g", false, "Enable GZIP compression for responses")
	flag.StringVar(&databaseDSNFlag, "d", "", "Database DSN (PostgreSQL)")
	flag.StringVar(&keyFlag, "k", "", "Secret key for HMAC-SHA256")
	flag.StringVar(&cryptoKeyFlag, "crypto-key", "", "Path to RSA key file")
	flag.StringVar(&auditFileFlag, "audit-file", "", "Path to audit log file")
	flag.StringVar(&auditURLFlag, "audit-url", "", "URL to send audit logs via POST")

	// Агентские флаги
	flag.IntVar(&pollIntervalFlag, "p", 2, "Poll interval in seconds")
	flag.IntVar(&reportIntervalFlag, "report-interval", 10, "Report interval in seconds")
	flag.IntVar(&rateLimitFlag, "l", 1, "Max concurrent requests")
}

// loadFileConfig загружает конфигурацию из JSON-файла.
func loadFileConfig() (*fileConfig, error) {
	if configFilePath == "" {
		// Проверяем переменную окружения CONFIG
		configFilePath = os.Getenv("CONFIG")
	}
	if configFilePath == "" {
		return nil, nil // нет файла конфигурации
	}

	data, err := os.ReadFile(configFilePath)
	if err != nil {
		return nil, fmt.Errorf("read config file %s: %w", configFilePath, err)
	}

	var fc fileConfig
	if err := json.Unmarshal(data, &fc); err != nil {
		return nil, fmt.Errorf("parse config file %s: %w", configFilePath, err)
	}

	return &fc, nil
}

// resolveString разрешает строковое значение по приоритету:
// flag > env > file > defaultValue
func resolveString(flagName, envKey, fileVal, defaultValue string) string {
	if flagSet[flagName] {
		// Флаг установлен явно — используем его значение
		switch flagName {
		case "a":
			return serverAddressFlag
		case "d":
			return databaseDSNFlag
		case "crypto-key":
			return cryptoKeyFlag
		case "k":
			return keyFlag
		case "f":
			return fileStoragePathFlag
		case "audit-file":
			return auditFileFlag
		case "audit-url":
			return auditURLFlag
		case "config":
			return configFilePath
		}
	}
	if val := os.Getenv(envKey); val != "" {
		return val
	}
	if fileVal != "" {
		return fileVal
	}
	return defaultValue
}

// resolveInt разрешает целочисленное значение по приоритету:
// flag > env > file > defaultValue
func resolveInt(flagName, envKey string, fileVal *int, defaultValue int) int {
	if flagSet[flagName] {
		switch flagName {
		case "i":
			return storeIntervalFlag
		case "p":
			return pollIntervalFlag
		case "report-interval":
			return reportIntervalFlag
		case "l":
			return rateLimitFlag
		}
	}
	if val := os.Getenv(envKey); val != "" {
		if n, err := strconv.Atoi(val); err == nil {
			return n
		}
		// Пробуем как duration
		if n := parseDurationSeconds(val); n != 0 {
			return n
		}
	}
	if fileVal != nil {
		return *fileVal
	}
	return defaultValue
}

// resolveBool разрешает булево значение по приоритету:
// flag > env > file > defaultValue
func resolveBool(flagName, envKey string, fileVal *bool, defaultValue bool) bool {
	if flagSet[flagName] {
		switch flagName {
		case "r":
			return restoreFlag
		case "g":
			return gzipFlag
		}
	}
	if val := os.Getenv(envKey); val != "" {
		return parseBool(val)
	}
	if fileVal != nil {
		return *fileVal
	}
	return defaultValue
}

// ParseFlags разбирает флаги, загружает JSON-файл и разрешает конфигурацию.
// Должен вызываться один раз перед использованием геттеров.
func ParseFlags() {
	// Сначала регистрируем все флаги (через init в server.go и тут)
	flag.Parse()

	// Записываем явно установленные флаги
	flag.Visit(func(f *flag.Flag) {
		// Для -c/-config нормализуем имя
		name := f.Name
		if name == "c" {
			name = "config"
		}
		flagSet[name] = true
	})

	// Загружаем файл конфигурации
	fc, err := loadFileConfig()
	if err != nil {
		log.Fatalf("Failed to load config file: %v", err)
	}
	if fc == nil {
		fc = &fileConfig{}
	}

	// Разрешаем все значения
	cfg = Config{
		// Общие
		Address:     resolveString("a", "ADDRESS", fc.Address, "localhost:8080"),
		DatabaseDSN: resolveString("d", "DATABASE_DSN", fc.DatabaseDSN, ""),
		CryptoKey:   resolveString("crypto-key", "CRYPTO_KEY", fc.CryptoKey, ""),
		Key:         resolveString("k", "KEY", fc.Key, ""),

		// Сервер
		StoreInterval: resolveInt("i", "STORE_INTERVAL", fc.StoreInterval, 300),
		StoreFile:     resolveString("f", "FILE_STORAGE_PATH", fc.StoreFile, "/tmp/metrics.json"),
		Restore:       ptrBool(resolveBool("r", "RESTORE", fc.Restore, true)),
		EnableGzip:    ptrBool(resolveBool("g", "ENABLE_GZIP", fc.EnableGzip, false)),
		AuditFile:     resolveString("audit-file", "AUDIT_FILE", fc.AuditFile, ""),
		AuditURL:      resolveString("audit-url", "AUDIT_URL", fc.AuditURL, ""),

		// Агент
		PollInterval:   resolveInt("p", "POLL_INTERVAL", fc.PollInterval, 2),
		ReportInterval: resolveInt("report-interval", "REPORT_INTERVAL", fc.ReportInterval, 10),
		RateLimit:      resolveInt("l", "RATE_LIMIT", fc.RateLimit, 1),
	}

	// Специальная обработка для -d: если флаг не установлен явно,
	// используем env var даже после flag.Parse
	if !flagSet["d"] {
		if envDSN := os.Getenv("DATABASE_DSN"); envDSN != "" {
			cfg.DatabaseDSN = envDSN
		}
	}
}

// === Геттеры (общие) ===

// ServerAddress возвращает адрес сервера, очищенный от протокола.
func ServerAddress() string {
	cleanAddr := strings.TrimPrefix(cfg.Address, "http://")
	cleanAddr = strings.TrimPrefix(cleanAddr, "https://")
	return cleanAddr
}

// GetSecretKey возвращает секретный ключ для HMAC-SHA256.
func GetSecretKey() string {
	return cfg.Key
}

// CryptoKey возвращает путь к файлу RSA-ключа.
func CryptoKey() string {
	return cfg.CryptoKey
}

// === Геттеры (сервер) ===

// StoreInterval возвращает интервал сохранения в секундах.
func StoreInterval() int {
	return cfg.StoreInterval
}

// FileStoragePath возвращает путь к файлу хранилища.
func FileStoragePath() string {
	return cfg.StoreFile
}

// ShouldRestore возвращает флаг восстановления из файла.
func ShouldRestore() bool {
	return cfg.Restore != nil && *cfg.Restore
}

// GzipEnabled возвращает флаг gzip-сжатия.
func GzipEnabled() bool {
	return cfg.EnableGzip != nil && *cfg.EnableGzip
}

// DatabaseDSN возвращает DSN PostgreSQL.
func DatabaseDSN() string {
	return cfg.DatabaseDSN
}

// AuditFile возвращает путь к файлу аудита.
func AuditFile() string {
	return cfg.AuditFile
}

// AuditURL возвращает URL для аудита.
func AuditURL() string {
	return cfg.AuditURL
}

// === Геттеры (агент) ===

// PollInterval возвращает интервал опроса метрик в секундах.
func PollInterval() int {
	return cfg.PollInterval
}

// ReportInterval возвращает интервал отправки отчётов в секундах.
func ReportInterval() int {
	return cfg.ReportInterval
}

// RateLimit возвращает лимит параллельных запросов.
func RateLimit() int {
	return cfg.RateLimit
}

// === Утилиты ===

// ValidateServerAddress проверяет корректность адреса сервера.
func ValidateServerAddress() {
	cleanAddr := ServerAddress()

	if cleanAddr == "" {
		log.Fatal("Invalid address: ADDRESS cannot be empty")
	}

	if !strings.Contains(cleanAddr, ":") {
		log.Fatal("Invalid address format: expected host:port or :port")
	}

	remainingArgs := flag.Args()
	if len(remainingArgs) > 0 {
		log.Fatalf("неизвестные аргументы командной строки: %v", remainingArgs)
	}
}

func ptrBool(b bool) *bool {
	return &b
}

// parseDurationSeconds парсит строку как duration и возвращает секунды.
// Поддерживает: "300", "5s", "10m", "1h"
func parseDurationSeconds(s string) int {
	// Сначала пробуем как простое число
	if n, err := strconv.Atoi(s); err == nil {
		return n
	}
	// Пробуем как duration с суффиксом
	if d, err := time.ParseDuration(s); err == nil {
		return int(d.Seconds())
	}
	// Пробуем как число + "s"
	if d, err := time.ParseDuration(s + "s"); err == nil {
		return int(d.Seconds())
	}
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
