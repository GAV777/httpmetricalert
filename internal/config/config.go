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

// Loader загружает конфигурацию из флагов, env и JSON-файла.
// Потокобезопасен, может использоваться в тестах с изолированным FlagSet.
type Loader struct {
	fs *flag.FlagSet
}

// NewLoader создаёт новый Loader. Если fs == nil, используется flag.CommandLine.
func NewLoader(fs *flag.FlagSet) *Loader {
	if fs == nil {
		fs = flag.CommandLine
	}
	return &Loader{fs: fs}
}

// Load разбирает флаги, загружает JSON-файл и возвращает Config.
func (l *Loader) Load() (*Config, error) {
	flagSet := make(map[string]bool)

	// Записываем явно установленные флаги
	l.fs.Visit(func(f *flag.Flag) {
		name := f.Name
		if name == "c" {
			name = "config"
		}
		flagSet[name] = true
	})

	// Определяем путь к файлу: флаг > env CONFIG
	// configFilePath уже заполнен через flag.*Var в init()
	path := configFilePath
	if path == "" {
		path = os.Getenv("CONFIG")
	}

	// Загружаем файл конфигурации
	fc, err := loadConfigFile(path)
	if err != nil {
		return nil, fmt.Errorf("load config file: %w", err)
	}
	if fc == nil {
		fc = &fileConfig{}
	}

	// Разрешаем все значения
	cfg := &Config{
		// Общие
		Address:     resolveStringWithMap("a", "ADDRESS", fc.Address, "localhost:8080", flagSet),
		DatabaseDSN: resolveStringWithMap("d", "DATABASE_DSN", fc.DatabaseDSN, "", flagSet),
		CryptoKey:   resolveStringWithMap("crypto-key", "CRYPTO_KEY", fc.CryptoKey, "", flagSet),
		Key:         resolveStringWithMap("k", "KEY", fc.Key, "", flagSet),

		// Сервер
		StoreInterval: resolveDurationWithMap("i", "STORE_INTERVAL", fc.StoreInterval, 300, flagSet),
		StoreFile:     resolveStringWithMap("f", "FILE_STORAGE_PATH", fc.StoreFile, "/tmp/metrics.json", flagSet),
		Restore:       ptrBool(resolveBoolWithMap("restore", "RESTORE", fc.Restore, true, flagSet)),
		EnableGzip:    ptrBool(resolveBoolWithMap("g", "ENABLE_GZIP", fc.EnableGzip, false, flagSet)),
		AuditFile:     resolveStringWithMap("audit-file", "AUDIT_FILE", fc.AuditFile, "", flagSet),
		AuditURL:      resolveStringWithMap("audit-url", "AUDIT_URL", fc.AuditURL, "", flagSet),
		TrustedSubnet: resolveStringWithMap("t", "TRUSTED_SUBNET", fc.TrustedSubnet, "", flagSet),
		GRPCAddress:   resolveStringWithMap("grpc", "GRPC_ADDRESS", fc.GRPCAddress, "", flagSet),

		// Агент
		PollInterval:   resolveDurationWithMap("p", "POLL_INTERVAL", fc.PollInterval, 2, flagSet),
		ReportInterval: resolveDurationWithMap("report-interval", "REPORT_INTERVAL", fc.ReportInterval, 10, flagSet),
		RateLimit:      resolveIntWithMap("l", "RATE_LIMIT", fc.RateLimit, 1, flagSet),
		UseGRPC:        resolveBoolWithMap("use-grpc", "USE_GRPC", fc.UseGRPC, false, flagSet),
	}

	return cfg, nil
}

// === Resolve-функции с явной передачей flagSet ===

func resolveStringWithMap(flagName, envKey, fileVal, defaultValue string, flagSet map[string]bool) string {
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

func resolveIntWithMap(flagName, envKey string, fileVal *int, defaultValue int, flagSet map[string]bool) int {
	if flagSet[flagName] {
		if getter, ok := flagIntGetters[flagName]; ok {
			return getter()
		}
	}
	if val := os.Getenv(envKey); val != "" {
		if n, err := strconv.Atoi(val); err == nil {
			return n
		}
		if n := parseDurationSeconds(val); n != 0 {
			return n
		}
	}
	if fileVal != nil {
		return *fileVal
	}
	return defaultValue
}

func resolveDurationWithMap(flagName, envKey string, fileVal string, defaultValue int, flagSet map[string]bool) int {
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

func resolveBoolWithMap(flagName, envKey string, fileVal *bool, defaultValue bool, flagSet map[string]bool) bool {
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

// === Методы на Config (удобные геттеры) ===

// ServerAddress возвращает адрес сервера, очищенный от протокола.
func (c *Config) ServerAddress() string {
	cleanAddr := strings.TrimPrefix(c.Address, "http://")
	cleanAddr = strings.TrimPrefix(cleanAddr, "https://")
	return cleanAddr
}

// ShouldRestore возвращает флаг восстановления из файла.
func (c *Config) ShouldRestore() bool {
	return c.Restore != nil && *c.Restore
}

// GzipEnabled возвращает флаг gzip-сжатия.
func (c *Config) GzipEnabled() bool {
	return c.EnableGzip != nil && *c.EnableGzip
}

// Validate проверяет корректность адреса сервера.
func (c *Config) Validate() error {
	cleanAddr := c.ServerAddress()
	if cleanAddr == "" {
		return fmt.Errorf("invalid address: ADDRESS cannot be empty")
	}
	if !strings.Contains(cleanAddr, ":") {
		return fmt.Errorf("invalid address format: expected host:port or :port")
	}
	return nil
}

func ptrBool(b bool) *bool {
	return &b
}

// === Геттеры обратной совместимости (используют глобальный cfg) ===

// Deprecated: use cfg.GetSecretKey() on the Config returned by Loader.Load().
func GetSecretKey() string { return cfg.Key }

// Deprecated: use cfg.CryptoKey() on the Config returned by Loader.Load().
func CryptoKey() string { return cfg.CryptoKey }

// Deprecated: use cfg.StoreInterval() on the Config returned by Loader.Load().
func StoreInterval() int { return cfg.StoreInterval }

// Deprecated: use cfg.StoreFile on the Config returned by Loader.Load().
func FileStoragePath() string { return cfg.StoreFile }

// Deprecated: use cfg.ShouldRestore() on the Config returned by Loader.Load().
func ShouldRestore() bool { return cfg.ShouldRestore() }

// Deprecated: use cfg.GzipEnabled() on the Config returned by Loader.Load().
func GzipEnabled() bool { return cfg.GzipEnabled() }

// Deprecated: use cfg.DatabaseDSN on the Config returned by Loader.Load().
func DatabaseDSN() string { return cfg.DatabaseDSN }

// Deprecated: use cfg.AuditFile on the Config returned by Loader.Load().
func AuditFile() string { return cfg.AuditFile }

// Deprecated: use cfg.AuditURL on the Config returned by Loader.Load().
func AuditURL() string { return cfg.AuditURL }

// Deprecated: use cfg.TrustedSubnet on the Config returned by Loader.Load().
func TrustedSubnet() string { return cfg.TrustedSubnet }

// Deprecated: use cfg.GRPCAddress on the Config returned by Loader.Load().
func GRPCAddress() string { return cfg.GRPCAddress }

// Deprecated: use cfg.PollInterval() on the Config returned by Loader.Load().
func PollInterval() int { return cfg.PollInterval }

// Deprecated: use cfg.ReportInterval() on the Config returned by Loader.Load().
func ReportInterval() int { return cfg.ReportInterval }

// Deprecated: use cfg.RateLimit on the Config returned by Loader.Load().
func RateLimit() int { return cfg.RateLimit }

// Deprecated: use cfg.ShouldUseGRPC() on the Config returned by Loader.Load().
func ShouldUseGRPC() bool { return cfg.UseGRPC }

// Deprecated: use cfg.ServerAddress() on the Config returned by Loader.Load().
func ServerAddress() string { return cfg.ServerAddress() }

// Deprecated: use cfg.Validate() on the Config returned by Loader.Load().
func ValidateServerAddress() {
	if err := cfg.Validate(); err != nil {
		remainingArgs := flag.Args()
		if len(remainingArgs) > 0 {
			log.Fatalf("неизвестные аргументы командной строки: %v", remainingArgs)
		}
		log.Fatal(err)
	}
	remainingArgs := flag.Args()
	if len(remainingArgs) > 0 {
		log.Fatalf("неизвестные аргументы командной строки: %v", remainingArgs)
	}
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
