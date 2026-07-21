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
	TrustedSubnet string `json:"trusted_subnet,omitempty"`
	GRPCAddress   string `json:"grpc_address,omitempty"`

	// Агент
	PollInterval   int  `json:"poll_interval,omitempty"`
	ReportInterval int  `json:"report_interval,omitempty"`
	RateLimit      int  `json:"rate_limit,omitempty"`
	UseGRPC        bool `json:"use_grpc,omitempty"`
}

// fileConfig — структура для десериализации JSON-файла.
type fileConfig struct {
	Address        string `json:"address"`
	StoreInterval  string `json:"store_interval"`
	StoreFile      string `json:"store_file"`
	Restore        *bool  `json:"restore"`
	EnableGzip     *bool  `json:"enable_gzip"`
	DatabaseDSN    string `json:"database_dsn"`
	Key            string `json:"key"`
	CryptoKey      string `json:"crypto_key"`
	AuditFile      string `json:"audit_file"`
	AuditURL       string `json:"audit_url"`
	TrustedSubnet  string `json:"trusted_subnet"`
	GRPCAddress    string `json:"grpc_address"`
	PollInterval   string `json:"poll_interval"`
	ReportInterval string `json:"report_interval"`
	RateLimit      *int   `json:"rate_limit"`
	UseGRPC        *bool  `json:"use_grpc"`
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
	trustedSubnetFlag   string
	grpcAddressFlag     string
	useGRPCFlag         bool
	pollIntervalFlag    int
	reportIntervalFlag  int
	rateLimitFlag       int

	// flagStringGetters мапит имя флага в функцию-геттер строкового значения.
	flagStringGetters = map[string]func() string{
		"a":          func() string { return serverAddressFlag },
		"d":          func() string { return databaseDSNFlag },
		"crypto-key": func() string { return cryptoKeyFlag },
		"k":          func() string { return keyFlag },
		"f":          func() string { return fileStoragePathFlag },
		"audit-file": func() string { return auditFileFlag },
		"audit-url":  func() string { return auditURLFlag },
		"config":     func() string { return configFilePath },
		"t":          func() string { return trustedSubnetFlag },
		"grpc":       func() string { return grpcAddressFlag },
	}

	// flagIntGetters мапит имя флага в функцию-геттер int значения.
	flagIntGetters = map[string]func() int{
		"i":               func() int { return storeIntervalFlag },
		"p":               func() int { return pollIntervalFlag },
		"r":               func() int { return reportIntervalFlag },
		"report-interval": func() int { return reportIntervalFlag },
		"l":               func() int { return rateLimitFlag },
	}

	// flagBoolGetters мапит имя флага в функцию-геттер bool значения.
	flagBoolGetters = map[string]func() bool{
		"restore":  func() bool { return restoreFlag },
		"g":        func() bool { return gzipFlag },
		"use-grpc": func() bool { return useGRPCFlag },
	}
)

func init() {
	flag.StringVar(&configFilePath, "config", "", "Path to JSON configuration file")
	flag.StringVar(&configFilePath, "c", "", "Path to JSON configuration file (shorthand)")

	// Серверные флаги
	flag.StringVar(&serverAddressFlag, "a", "localhost:8080", "HTTP server address")
	flag.IntVar(&storeIntervalFlag, "i", 300, "Store interval in seconds (0 for sync)")
	flag.StringVar(&fileStoragePathFlag, "f", "/tmp/metrics.json", "File path to store metrics")
	flag.BoolVar(&restoreFlag, "restore", true, "Restore metrics from file on start")
	flag.BoolVar(&gzipFlag, "g", false, "Enable GZIP compression for responses")
	flag.StringVar(&databaseDSNFlag, "d", "", "Database DSN (PostgreSQL)")
	flag.StringVar(&keyFlag, "k", "", "Secret key for HMAC-SHA256")
	flag.StringVar(&cryptoKeyFlag, "crypto-key", "", "Path to RSA key file")
	flag.StringVar(&auditFileFlag, "audit-file", "", "Path to audit log file")
	flag.StringVar(&auditURLFlag, "audit-url", "", "URL to send audit logs via POST")
	flag.StringVar(&trustedSubnetFlag, "t", "", "Trusted subnet (CIDR) for agent IP verification")
	flag.StringVar(&grpcAddressFlag, "grpc", "", "gRPC server address")

	// Агентские флаги
	flag.IntVar(&pollIntervalFlag, "p", 2, "Poll interval in seconds")
	flag.IntVar(&reportIntervalFlag, "r", 10, "Report interval in seconds")
	flag.IntVar(&reportIntervalFlag, "report-interval", 10, "Report interval in seconds (long form)")
	flag.IntVar(&rateLimitFlag, "l", 1, "Max concurrent requests")
	flag.BoolVar(&useGRPCFlag, "use-grpc", false, "Use gRPC instead of HTTP for metric submission")
}

// loadConfigFile загружает конфигурацию из JSON-файла по указанному пути.
// Возвращает nil, если path пустой.
func loadConfigFile(path string) (*fileConfig, error) {
	if path == "" {
		return nil, nil // нет файла конфигурации
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file %s: %w", path, err)
	}

	var fc fileConfig
	if err := json.Unmarshal(data, &fc); err != nil {
		return nil, fmt.Errorf("parse config file %s: %w", path, err)
	}

	return &fc, nil
}

// resolveString разрешает строковое значение по приоритету:
// flag > env > file > defaultValue
func resolveString(flagName, envKey, fileVal, defaultValue string) string {
	if flagSet[flagName] {
		if getter, ok := flagStringGetters[flagName]; ok {
			return getter()
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
		if getter, ok := flagIntGetters[flagName]; ok {
			return getter()
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

// resolveDuration разрешает значение-длительность по приоритету:
// flag > env > file > defaultValue.
// Файловые значения парсятся как duration-строки ("300s", "5m").
func resolveDuration(flagName, envKey string, fileVal string, defaultValue int) int {
	if flagSet[flagName] {
		if getter, ok := flagIntGetters[flagName]; ok {
			return getter()
		}
	}
	if val := os.Getenv(envKey); val != "" {
		if n := parseDurationSeconds(val); n != 0 {
			return n
		}
	}
	if fileVal != "" {
		if n := parseDurationSeconds(fileVal); n != 0 {
			return n
		}
	}
	return defaultValue
}

// resolveBool разрешает булево значение по приоритету:
// flag > env > file > defaultValue
func resolveBool(flagName, envKey string, fileVal *bool, defaultValue bool) bool {
	if flagSet[flagName] {
		if getter, ok := flagBoolGetters[flagName]; ok {
			return getter()
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

	// Определяем путь к файлу: флаг > env CONFIG
	path := configFilePath
	if path == "" {
		path = os.Getenv("CONFIG")
	}

	// Загружаем файл конфигурации
	fc, err := loadConfigFile(path)
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
		StoreInterval: resolveDuration("i", "STORE_INTERVAL", fc.StoreInterval, 300),
		StoreFile:     resolveString("f", "FILE_STORAGE_PATH", fc.StoreFile, "/tmp/metrics.json"),
		Restore:       ptrBool(resolveBool("restore", "RESTORE", fc.Restore, true)),
		EnableGzip:    ptrBool(resolveBool("g", "ENABLE_GZIP", fc.EnableGzip, false)),
		AuditFile:     resolveString("audit-file", "AUDIT_FILE", fc.AuditFile, ""),
		AuditURL:      resolveString("audit-url", "AUDIT_URL", fc.AuditURL, ""),
		TrustedSubnet: resolveString("t", "TRUSTED_SUBNET", fc.TrustedSubnet, ""),
		GRPCAddress:   resolveString("grpc", "GRPC_ADDRESS", fc.GRPCAddress, ""),

		// Агент
		PollInterval:   resolveDuration("p", "POLL_INTERVAL", fc.PollInterval, 2),
		ReportInterval: resolveDuration("report-interval", "REPORT_INTERVAL", fc.ReportInterval, 10),
		RateLimit:      resolveInt("l", "RATE_LIMIT", fc.RateLimit, 1),
		UseGRPC:        resolveBool("use-grpc", "USE_GRPC", fc.UseGRPC, false),
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

// TrustedSubnet возвращает доверенную подсеть (CIDR).
func TrustedSubnet() string {
	return cfg.TrustedSubnet
}

// GRPCAddress возвращает адрес gRPC-сервера.
func GRPCAddress() string {
	return cfg.GRPCAddress
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

// ShouldUseGRPC возвращает флаг использования gRPC.
func ShouldUseGRPC() bool {
	return cfg.UseGRPC
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
