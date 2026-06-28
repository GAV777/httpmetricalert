// Package retry предоставляет логику повторных попыток для сетевых и БД операций.
//
// Do выполняет функцию с экспоненциальной задержкой при retriable-ошибках
// (сетевые проблемы, разрывы соединения с БД). Поддерживает отмену через context.
package retry

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"net"
	"syscall"
	"time"

	"github.com/jackc/pgerrcode"
)

// Config holds retry strategy parameters
type Config struct {
	MaxRetries int             // Maximum number of additional attempts (0 = no retries)
	Delays     []time.Duration // Delays between retries (e.g., 1s, 3s, 5s)
}

// DefaultConfig returns the standard retry config: 3 retries with 1s, 3s, 5s delays
func DefaultConfig() Config {
	return Config{
		MaxRetries: 3,
		Delays:     []time.Duration{1 * time.Second, 3 * time.Second, 5 * time.Second},
	}
}

// Do executes fn with retry logic. It returns on the first success or after
// all retries are exhausted. The context is respected between retries.
func Do(ctx context.Context, cfg Config, fn func() error) error {
	var lastErr error

	// Attempt the function once first, then retry up to MaxRetries times
	for attempt := 0; attempt <= cfg.MaxRetries; attempt++ {
		if attempt > 0 {
			// Wait before this retry attempt
			delayIndex := attempt - 1
			if delayIndex >= len(cfg.Delays) {
				delayIndex = len(cfg.Delays) - 1
			}
			delay := cfg.Delays[delayIndex]

			log.Printf("⏳ Retry attempt %d/%d after %v delay", attempt, cfg.MaxRetries, delay)

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
		}

		lastErr = fn()
		if lastErr == nil {
			return nil // Success
		}

		if !isRetriable(lastErr) {
			return lastErr // Non-retriable error — give up immediately
		}
	}

	return lastErr
}

// isRetriable determines whether an error is worth retrying.
func isRetriable(err error) bool {
	if err == nil {
		return false
	}

	// Network / connection errors
	if errors.Is(err, net.ErrClosed) {
		return true
	}
	if errors.Is(err, syscall.ECONNREFUSED) {
		return true
	}
	if errors.Is(err, syscall.ECONNRESET) {
		return true
	}
	if errors.Is(err, syscall.ETIMEDOUT) {
		return true
	}

	// Generic "connection refused" / "connection reset" messages
	errMsg := err.Error()
	if containsAny(errMsg, []string{
		"connection refused",
		"connection reset",
		"no connection",
		"broken pipe",
		"i/o timeout",
		"EOF",
	}) {
		return true
	}

	// PostgreSQL transport errors (Class 08 — Connection Exception)
	var pgErr *net.OpError
	if errors.As(err, &pgErr) {
		return true
	}

	// Check for PostgreSQL error codes related to connection issues
	// The lib/pq driver exposes these via *pq.Error
	if isPostgresConnectionError(err) {
		return true
	}

	// sql.DB connection issues
	if errors.Is(err, sql.ErrConnDone) {
		return true
	}

	return false
}

// isPostgresConnectionError checks if the error is a PostgreSQL connection
// exception (Class 08) using pgerrcode.
func isPostgresConnectionError(err error) bool {
	// The pq driver wraps PostgreSQL error codes in *pq.Error
	// We use type assertion through a helper since we don't import pq directly.
	// pgerrcode provides string constants for class codes.
	// Class 08 codes include:
	//   08000 connection exception
	//   08003 connection does not exist
	//   08006 connection failure
	//   08001 SQLclient unable to establish SQLconnection
	//   08004 SQLserver rejected establishment of SQLconnection
	//   08P01 protocol violation

	// We check the error string for pgerrcode constants as a fallback
	errMsg := err.Error()
	connectionCodes := []string{
		pgerrcode.ConnectionException,
		pgerrcode.ConnectionDoesNotExist,
		pgerrcode.ConnectionFailure,
		pgerrcode.SQLClientUnableToEstablishSQLConnection,
		pgerrcode.SQLServerRejectedEstablishmentOfSQLConnection,
	}
	for _, code := range connectionCodes {
		if containsAny(errMsg, []string{code}) {
			return true
		}
	}
	return false
}

func containsAny(s string, substrings []string) bool {
	for _, sub := range substrings {
		if len(s) >= len(sub) {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
		}
	}
	return false
}
