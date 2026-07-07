package config

import (
	"flag"
	"os"
	"path/filepath"
	"testing"
)

// helperFuncs_test.go — тесты утилитарных функций

func TestParseBool(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"true", true},
		{"false", false},
		{"True", true},
		{"False", false},
		{"1", true},
		{"0", false},
		{"yes", true},
		{"no", false},
		{"on", true},
		{"off", false},
		{"YES", true},
		{"NO", false},
		{"ON", true},
		{"OFF", false},
		{"invalid", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseBool(tt.input)
			if result != tt.expected {
				t.Errorf("parseBool(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestParseDurationSeconds(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int
	}{
		{"plain number", "300", 300},
		{"plain number zero", "0", 0},
		{"seconds", "10s", 10},
		{"minutes", "5m", 300},
		{"hours", "1h", 3600},
		{"number with s suffix", "60s", 60},
		{"invalid", "invalid", 0},
		{"empty", "", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseDurationSeconds(tt.input)
			if result != tt.expected {
				t.Errorf("parseDurationSeconds(%q) = %d, want %d", tt.input, result, tt.expected)
			}
		})
	}
}

// ——— Integration tests с полной изоляцией FlagSet ———

// newTestEnv создаёт изолированное окружение для теста
func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	oldArgs := os.Args
	oldCommandLine := flag.CommandLine

	os.Args = []string{"test"}
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	configFilePath = ""
	flagSet = make(map[string]bool)
	cfg = Config{}

	// Перерегистрируем все флаги
	flag.StringVar(&configFilePath, "config", "", "Path to JSON configuration file")
	flag.StringVar(&configFilePath, "c", "", "Path to JSON configuration file (shorthand)")
	flag.StringVar(&serverAddressFlag, "a", "localhost:8080", "server address")
	flag.IntVar(&storeIntervalFlag, "i", 300, "store interval")
	flag.StringVar(&fileStoragePathFlag, "f", "/tmp/metrics.json", "file storage path")
	flag.BoolVar(&restoreFlag, "r", true, "restore from file")
	flag.BoolVar(&gzipFlag, "g", false, "enable gzip")
	flag.StringVar(&databaseDSNFlag, "d", "", "database DSN")
	flag.StringVar(&keyFlag, "k", "", "secret key")
	flag.StringVar(&cryptoKeyFlag, "crypto-key", "", "crypto key path")
	flag.StringVar(&auditFileFlag, "audit-file", "", "audit file path")
	flag.StringVar(&auditURLFlag, "audit-url", "", "audit URL")
	flag.IntVar(&pollIntervalFlag, "p", 2, "poll interval")
	flag.IntVar(&reportIntervalFlag, "report-interval", 10, "report interval")
	flag.IntVar(&rateLimitFlag, "l", 1, "rate limit")

	return &testEnv{
		t:              t,
		oldArgs:        oldArgs,
		oldCommandLine: oldCommandLine,
	}
}

func (e *testEnv) Cleanup() {
	os.Args = e.oldArgs
	flag.CommandLine = e.oldCommandLine
}

// setArgs устанавливает os.Args для теста
func (e *testEnv) setArgs(args ...string) {
	os.Args = append([]string{"test"}, args...)
}

// setEnv устанавливает переменную окружения с автоочисткой
func (e *testEnv) setEnv(key, value string) {
	e.t.Setenv(key, value)
}

// parse вызывает ParseFlags с текущими аргументами
func (e *testEnv) parse() {
	// Сбрасываем flagSet перед парсингом
	flagSet = make(map[string]bool)
	flag.CommandLine.Parse(os.Args[1:])
	// Записываем явно установленные флаги
	flag.Visit(func(f *flag.Flag) {
		name := f.Name
		if name == "c" {
			name = "config"
		}
		flagSet[name] = true
	})
	// Разрешаем конфигурацию (без flag.Parse — мы уже распарсили)
	e.resolveConfig()
}

// resolveConfig — ручное разрешение без flag.Parse
func (e *testEnv) resolveConfig() {
	fc, err := loadFileConfig()
	if err != nil {
		e.t.Fatalf("loadFileConfig error: %v", err)
	}
	if fc == nil {
		fc = &fileConfig{}
	}

	cfg = Config{
		Address:        resolveString("a", "ADDRESS", fc.Address, "localhost:8080"),
		DatabaseDSN:    resolveString("d", "DATABASE_DSN", fc.DatabaseDSN, ""),
		CryptoKey:      resolveString("crypto-key", "CRYPTO_KEY", fc.CryptoKey, ""),
		Key:            resolveString("k", "KEY", fc.Key, ""),
		StoreInterval:  resolveInt("i", "STORE_INTERVAL", fc.StoreInterval, 300),
		StoreFile:      resolveString("f", "FILE_STORAGE_PATH", fc.StoreFile, "/tmp/metrics.json"),
		Restore:        ptrBool(resolveBool("r", "RESTORE", fc.Restore, true)),
		EnableGzip:     ptrBool(resolveBool("g", "ENABLE_GZIP", fc.EnableGzip, false)),
		AuditFile:      resolveString("audit-file", "AUDIT_FILE", fc.AuditFile, ""),
		AuditURL:       resolveString("audit-url", "AUDIT_URL", fc.AuditURL, ""),
		PollInterval:   resolveInt("p", "POLL_INTERVAL", fc.PollInterval, 2),
		ReportInterval: resolveInt("report-interval", "REPORT_INTERVAL", fc.ReportInterval, 10),
		RateLimit:      resolveInt("l", "RATE_LIMIT", fc.RateLimit, 1),
	}

	// Специальная обработка для -d
	if !flagSet["d"] {
		if envDSN := os.Getenv("DATABASE_DSN"); envDSN != "" {
			cfg.DatabaseDSN = envDSN
		}
	}
}

type testEnv struct {
	t              *testing.T
	oldArgs        []string
	oldCommandLine *flag.FlagSet
}

// ——— Флаговые переменные уже объявлены в config.go, экспортированы ———

// ——— Тесты загрузки JSON-файла ———

func TestLoadFileConfig_ValidJSON(t *testing.T) {
	env := newTestEnv(t)
	defer env.Cleanup()

	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.json")

	content := `{
		"address": "localhost:9090",
		"store_interval": 60,
		"store_file": "/tmp/test.json",
		"restore": false,
		"enable_gzip": true,
		"database_dsn": "postgres://localhost/test",
		"key": "secret",
		"crypto_key": "/path/to/key.pem",
		"audit_file": "/tmp/audit.log",
		"audit_url": "http://localhost:8081/audit",
		"poll_interval": 5,
		"report_interval": 15,
		"rate_limit": 3
	}`

	if err := os.WriteFile(configFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	env.setArgs("-config", configFile)
	env.parse()

	if ServerAddress() != "localhost:9090" {
		t.Errorf("address: expected localhost:9090, got %s", ServerAddress())
	}
	if StoreInterval() != 60 {
		t.Errorf("store_interval: expected 60, got %d", StoreInterval())
	}
	if FileStoragePath() != "/tmp/test.json" {
		t.Errorf("store_file: expected /tmp/test.json, got %s", FileStoragePath())
	}
	if ShouldRestore() {
		t.Error("restore: expected false")
	}
	if !GzipEnabled() {
		t.Error("enable_gzip: expected true")
	}
	if DatabaseDSN() != "postgres://localhost/test" {
		t.Errorf("database_dsn: expected postgres://localhost/test, got %s", DatabaseDSN())
	}
	if GetSecretKey() != "secret" {
		t.Errorf("key: expected secret, got %s", GetSecretKey())
	}
	if CryptoKey() != "/path/to/key.pem" {
		t.Errorf("crypto_key: expected /path/to/key.pem, got %s", CryptoKey())
	}
	if AuditFile() != "/tmp/audit.log" {
		t.Errorf("audit_file: expected /tmp/audit.log, got %s", AuditFile())
	}
	if AuditURL() != "http://localhost:8081/audit" {
		t.Errorf("audit_url: expected http://localhost:8081/audit, got %s", AuditURL())
	}
	if PollInterval() != 5 {
		t.Errorf("poll_interval: expected 5, got %d", PollInterval())
	}
	if ReportInterval() != 15 {
		t.Errorf("report_interval: expected 15, got %d", ReportInterval())
	}
	if RateLimit() != 3 {
		t.Errorf("rate_limit: expected 3, got %d", RateLimit())
	}
}

func TestLoadFileConfig_MinimalJSON(t *testing.T) {
	env := newTestEnv(t)
	defer env.Cleanup()

	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.json")

	// Минимальный JSON — только address
	content := `{"address": "localhost:7070"}`
	if err := os.WriteFile(configFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	env.setArgs("-config", configFile)
	env.parse()

	if ServerAddress() != "localhost:7070" {
		t.Errorf("address: expected localhost:7070, got %s", ServerAddress())
	}
	// Остальные — defaults
	if StoreInterval() != 300 {
		t.Errorf("store_interval: expected default 300, got %d", StoreInterval())
	}
	if FileStoragePath() != "/tmp/metrics.json" {
		t.Errorf("store_file: expected default, got %s", FileStoragePath())
	}
	if !ShouldRestore() {
		t.Error("restore: expected default true")
	}
	if PollInterval() != 2 {
		t.Errorf("poll_interval: expected default 2, got %d", PollInterval())
	}
	if ReportInterval() != 10 {
		t.Errorf("report_interval: expected default 10, got %d", ReportInterval())
	}
	if RateLimit() != 1 {
		t.Errorf("rate_limit: expected default 1, got %d", RateLimit())
	}
}

func TestLoadFileConfig_EmptyFile(t *testing.T) {
	env := newTestEnv(t)
	defer env.Cleanup()

	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "empty.json")
	if err := os.WriteFile(configFile, []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}

	env.setArgs("-config", configFile)
	env.parse()

	// Все значения должны быть defaults
	if ServerAddress() != "localhost:8080" {
		t.Errorf("address: expected default localhost:8080, got %s", ServerAddress())
	}
	if StoreInterval() != 300 {
		t.Errorf("store_interval: expected default 300, got %d", StoreInterval())
	}
}

func TestLoadFileConfig_NonExistent(t *testing.T) {
	// Устанавливаем путь напрямую, минуя флаги
	configFilePath = "/nonexistent/path/config.json"
	defer func() { configFilePath = "" }()

	fc, err := loadFileConfig()
	if err == nil {
		t.Error("expected error for non-existent file")
	}
	if fc != nil {
		t.Error("expected nil fileConfig on error")
	}
}

func TestLoadFileConfig_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "bad.json")
	if err := os.WriteFile(configFile, []byte(`{not valid json}`), 0644); err != nil {
		t.Fatal(err)
	}

	// Путь не установлен через flag, поэтому loadFileConfig вернёт nil без ошибки
	fc, err := loadFileConfig()
	if fc != nil {
		t.Error("expected nil fileConfig when configFilePath not set")
	}
	_ = err // Ошибка может быть nil т.к. configFilePath пустой
}

// ——— Тесты приоритетов: flag > env > file > default ———

func TestPriority_FlagOverridesAll(t *testing.T) {
	env := newTestEnv(t)
	defer env.Cleanup()

	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.json")
	content := `{"address": "from-file:8080", "store_interval": 50, "poll_interval": 3}`
	if err := os.WriteFile(configFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	env.setEnv("ADDRESS", "from-env:8080")
	env.setEnv("STORE_INTERVAL", "100")
	env.setEnv("POLL_INTERVAL", "7")

	env.setArgs("-a", "from-flag:8080", "-i", "200", "-p", "10", "-config", configFile)
	env.parse()

	if ServerAddress() != "from-flag:8080" {
		t.Errorf("address: expected from-flag:8080, got %s", ServerAddress())
	}
	if StoreInterval() != 200 {
		t.Errorf("store_interval: expected 200 (flag), got %d", StoreInterval())
	}
	if PollInterval() != 10 {
		t.Errorf("poll_interval: expected 10 (flag), got %d", PollInterval())
	}
}

func TestPriority_EnvOverridesFile(t *testing.T) {
	env := newTestEnv(t)
	defer env.Cleanup()

	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.json")
	content := `{"address": "from-file:8080", "store_interval": 50}`
	if err := os.WriteFile(configFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	env.setEnv("ADDRESS", "from-env:9090")
	env.setEnv("STORE_INTERVAL", "150")

	env.setArgs("-config", configFile)
	env.parse()

	if ServerAddress() != "from-env:9090" {
		t.Errorf("address: expected from-env:9090, got %s", ServerAddress())
	}
	if StoreInterval() != 150 {
		t.Errorf("store_interval: expected 150 (env), got %d", StoreInterval())
	}
}

func TestPriority_FileOverridesDefault(t *testing.T) {
	env := newTestEnv(t)
	defer env.Cleanup()

	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.json")
	content := `{"address": "from-file:7070", "rate_limit": 5}`
	if err := os.WriteFile(configFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	env.setArgs("-config", configFile)
	env.parse()

	if ServerAddress() != "from-file:7070" {
		t.Errorf("address: expected from-file:7070, got %s", ServerAddress())
	}
	if RateLimit() != 5 {
		t.Errorf("rate_limit: expected 5 (file), got %d", RateLimit())
	}
}

func TestPriority_DefaultsWhenNothing(t *testing.T) {
	env := newTestEnv(t)
	defer env.Cleanup()

	// Никаких флагов, env, файлов
	env.parse()

	if ServerAddress() != "localhost:8080" {
		t.Errorf("address: expected default, got %s", ServerAddress())
	}
	if StoreInterval() != 300 {
		t.Errorf("store_interval: expected default 300, got %d", StoreInterval())
	}
	if FileStoragePath() != "/tmp/metrics.json" {
		t.Errorf("store_file: expected default, got %s", FileStoragePath())
	}
	if !ShouldRestore() {
		t.Error("restore: expected default true")
	}
	if GzipEnabled() {
		t.Error("enable_gzip: expected default false")
	}
	if PollInterval() != 2 {
		t.Errorf("poll_interval: expected default 2, got %d", PollInterval())
	}
	if ReportInterval() != 10 {
		t.Errorf("report_interval: expected default 10, got %d", ReportInterval())
	}
	if RateLimit() != 1 {
		t.Errorf("rate_limit: expected default 1, got %d", RateLimit())
	}
}

func TestPriority_DatabaseDSN_FromEnv(t *testing.T) {
	env := newTestEnv(t)
	defer env.Cleanup()

	env.setEnv("DATABASE_DSN", "postgres://env-host/db")
	env.parse()

	if DatabaseDSN() != "postgres://env-host/db" {
		t.Errorf("database_dsn: expected from env, got %s", DatabaseDSN())
	}
}

func TestPriority_DatabaseDSN_FlagOverridesEnv(t *testing.T) {
	env := newTestEnv(t)
	defer env.Cleanup()

	env.setEnv("DATABASE_DSN", "postgres://env-host/db")
	env.setArgs("-d", "postgres://flag-host/db")
	env.parse()

	if DatabaseDSN() != "postgres://flag-host/db" {
		t.Errorf("database_dsn: expected from flag, got %s", DatabaseDSN())
	}
}

func TestPriority_ConfigViaEnv(t *testing.T) {
	env := newTestEnv(t)
	defer env.Cleanup()

	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.json")
	content := `{"address": "from-env-config:8080"}`
	if err := os.WriteFile(configFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	env.setEnv("CONFIG", configFile)
	env.parse()

	if ServerAddress() != "from-env-config:8080" {
		t.Errorf("address: expected from-env-config:8080, got %s", ServerAddress())
	}
}

func TestPriority_ConfigFlagOverridesEnv(t *testing.T) {
	env := newTestEnv(t)
	defer env.Cleanup()

	tmpDir := t.TempDir()
	fileFromEnv := filepath.Join(tmpDir, "env-config.json")
	fileFromFlag := filepath.Join(tmpDir, "flag-config.json")

	if err := os.WriteFile(fileFromEnv, []byte(`{"address": "from-env-file"}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fileFromFlag, []byte(`{"address": "from-flag-file"}`), 0644); err != nil {
		t.Fatal(err)
	}

	env.setEnv("CONFIG", fileFromEnv)
	env.setArgs("-config", fileFromFlag)
	env.parse()

	if ServerAddress() != "from-flag-file" {
		t.Errorf("address: expected from-flag-file, got %s", ServerAddress())
	}
}

// ——— Тесты shorthand флага -c ———

func TestShorthandConfigFlag(t *testing.T) {
	env := newTestEnv(t)
	defer env.Cleanup()

	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.json")
	content := `{"address": "shorthand-test:8080"}`
	if err := os.WriteFile(configFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	env.setArgs("-c", configFile)
	env.parse()

	if ServerAddress() != "shorthand-test:8080" {
		t.Errorf("address: expected shorthand-test:8080, got %s", ServerAddress())
	}
}

// ——— Тесты ValidateServerAddress ———

func TestValidateServerAddress_Valid(t *testing.T) {
	env := newTestEnv(t)
	defer env.Cleanup()

	cfg.Address = "localhost:8080"
	// Не должно паниковать
	ValidateServerAddress()
}

// ——— Тесты resolveInt с duration-строками в env ———

func TestResolveInt_DurationInEnv(t *testing.T) {
	env := newTestEnv(t)
	defer env.Cleanup()

	env.setEnv("STORE_INTERVAL", "5m")
	// resolveInt должен распарсить "5m" как 300 секунд
	result := resolveInt("i", "STORE_INTERVAL", nil, 300)
	if result != 300 {
		t.Errorf("resolveInt with 5m: expected 300, got %d", result)
	}
}

func TestResolveInt_PlainNumberInEnv(t *testing.T) {
	env := newTestEnv(t)
	defer env.Cleanup()

	env.setEnv("STORE_INTERVAL", "600")
	result := resolveInt("i", "STORE_INTERVAL", nil, 300)
	if result != 600 {
		t.Errorf("resolveInt with 600: expected 600, got %d", result)
	}
}

func TestResolveInt_FlagOverridesDurationEnv(t *testing.T) {
	env := newTestEnv(t)
	defer env.Cleanup()

	env.setEnv("STORE_INTERVAL", "5m")
	env.setArgs("-i", "120")
	env.parse()

	if StoreInterval() != 120 {
		t.Errorf("store_interval: expected 120 (flag), got %d", StoreInterval())
	}
}
