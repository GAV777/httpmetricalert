package config

import (
	"os"
	"testing"
)

func TestServerAddress(t *testing.T) {
	// Без env
	os.Unsetenv("ADDRESS")
	ParseFlags()
	addr := ServerAddress()
	if addr == "" {
		t.Error("expected non-empty server address")
	}
}

func TestStoreInterval(t *testing.T) {
	ParseFlags()
	interval := StoreInterval()
	if interval < 0 {
		t.Errorf("expected non-negative store interval, got %d", interval)
	}
}

func TestFileStoragePath(t *testing.T) {
	ParseFlags()
	path := FileStoragePath()
	if path == "" {
		t.Error("expected non-empty file storage path")
	}
}

func TestRestore(t *testing.T) {
	ParseFlags()
	restore := ShouldRestore()
	// Значение по умолчанию true
	if !restore {
		t.Error("expected restore to be true by default")
	}
}

func TestGzipEnabled(t *testing.T) {
	ParseFlags()
	gzip := GzipEnabled()
	// Значение по умолчанию false
	if gzip {
		t.Error("expected gzip to be false by default")
	}
}

func TestDatabaseDSN(t *testing.T) {
	ParseFlags()
	dsn := DatabaseDSN()
	// DSN по умолчанию пустой
	if dsn != "" {
		t.Errorf("expected empty DSN, got '%s'", dsn)
	}
}

func TestAuditFile(t *testing.T) {
	ParseFlags()
	file := AuditFile()
	if file != "" {
		t.Errorf("expected empty audit file, got '%s'", file)
	}
}

func TestAuditURL(t *testing.T) {
	ParseFlags()
	url := AuditURL()
	if url != "" {
		t.Errorf("expected empty audit URL, got '%s'", url)
	}
}
