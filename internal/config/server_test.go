package config

import (
	"flag"
	"os"
	"testing"
)

// resetFlagSet создаёт новый FlagSet для изоляции тестов
func resetFlagSet(t *testing.T) {
	t.Helper()
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	configFilePath = ""
	flagSet = make(map[string]bool)
	cfg = Config{}
	// Перерегистрируем флаги
	flag.StringVar(&configFilePath, "config", "", "Path to JSON configuration file")
	flag.StringVar(&configFilePath, "c", "", "Path to JSON configuration file (shorthand)")
}

func TestServerAddressDefault(t *testing.T) {
	resetFlagSet(t)
	cfg = Config{Address: "localhost:8080"}
	addr := ServerAddress()
	if addr != "localhost:8080" {
		t.Errorf("expected localhost:8080, got %s", addr)
	}
}

func TestStoreIntervalDefault(t *testing.T) {
	resetFlagSet(t)
	cfg = Config{StoreInterval: 300}
	if StoreInterval() != 300 {
		t.Errorf("expected 300, got %d", StoreInterval())
	}
}

func TestFileStoragePathDefault(t *testing.T) {
	resetFlagSet(t)
	cfg = Config{StoreFile: "/tmp/metrics.json"}
	if FileStoragePath() != "/tmp/metrics.json" {
		t.Errorf("expected /tmp/metrics.json, got %s", FileStoragePath())
	}
}

func TestShouldRestore(t *testing.T) {
	resetFlagSet(t)
	b := true
	cfg = Config{Restore: &b}
	if !ShouldRestore() {
		t.Error("expected restore to be true")
	}

	b = false
	cfg = Config{Restore: &b}
	if ShouldRestore() {
		t.Error("expected restore to be false")
	}
}

func TestGzipEnabled(t *testing.T) {
	resetFlagSet(t)
	cfg = Config{EnableGzip: nil}
	if GzipEnabled() {
		t.Error("expected gzip to be false when nil")
	}

	b := true
	cfg = Config{EnableGzip: &b}
	if !GzipEnabled() {
		t.Error("expected gzip to be true")
	}
}

func TestDatabaseDSN(t *testing.T) {
	resetFlagSet(t)
	cfg = Config{DatabaseDSN: ""}
	if DatabaseDSN() != "" {
		t.Errorf("expected empty DSN, got '%s'", DatabaseDSN())
	}
}

func TestAuditFile(t *testing.T) {
	resetFlagSet(t)
	cfg = Config{AuditFile: ""}
	if AuditFile() != "" {
		t.Errorf("expected empty audit file, got '%s'", AuditFile())
	}
}

func TestAuditURL(t *testing.T) {
	resetFlagSet(t)
	cfg = Config{AuditURL: ""}
	if AuditURL() != "" {
		t.Errorf("expected empty audit URL, got '%s'", AuditURL())
	}
}

func TestGetSecretKey(t *testing.T) {
	resetFlagSet(t)
	cfg = Config{Key: "my-secret"}
	if GetSecretKey() != "my-secret" {
		t.Errorf("expected 'my-secret', got '%s'", GetSecretKey())
	}
}

func TestPollInterval(t *testing.T) {
	resetFlagSet(t)
	cfg = Config{PollInterval: 2}
	if PollInterval() != 2 {
		t.Errorf("expected 2, got %d", PollInterval())
	}
}

func TestReportInterval(t *testing.T) {
	resetFlagSet(t)
	cfg = Config{ReportInterval: 10}
	if ReportInterval() != 10 {
		t.Errorf("expected 10, got %d", ReportInterval())
	}
}

func TestRateLimit(t *testing.T) {
	resetFlagSet(t)
	cfg = Config{RateLimit: 1}
	if RateLimit() != 1 {
		t.Errorf("expected 1, got %d", RateLimit())
	}
}
