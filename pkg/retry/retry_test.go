package retry

import (
	"context"
	"database/sql"
	"errors"
	"net"
	"syscall"
	"testing"
	"time"
)

func TestDo_SuccessOnFirstAttempt(t *testing.T) {
	t.Parallel()

	called := 0
	fn := func() error {
		called++
		return nil
	}

	err := Do(context.Background(), DefaultConfig(), fn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if called != 1 {
		t.Errorf("called %d times, want 1", called)
	}
}

func TestDo_RetryOnFailure(t *testing.T) {
	t.Parallel()

	attempts := 0
	fn := func() error {
		attempts++
		if attempts < 3 {
			return syscall.ECONNREFUSED // retriable
		}
		return nil
	}

	cfg := Config{
		MaxRetries: 3,
		Delays:     []time.Duration{1 * time.Millisecond, 1 * time.Millisecond},
	}

	err := Do(context.Background(), cfg, fn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if attempts != 3 {
		t.Errorf("attempts = %d, want 3", attempts)
	}
}

func TestDo_NoRetryForNonRetriableError(t *testing.T) {
	t.Parallel()

	attempts := 0
	nonRetriable := errors.New("non-retriable error")
	fn := func() error {
		attempts++
		return nonRetriable
	}

	cfg := Config{MaxRetries: 3, Delays: []time.Duration{1 * time.Millisecond}}
	err := Do(context.Background(), cfg, fn)

	if err != nonRetriable {
		t.Errorf("error = %v, want %v", err, nonRetriable)
	}
	if attempts != 1 {
		t.Errorf("attempts = %d, want 1", attempts)
	}
}

func TestDo_ContextCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // отменяем сразу

	fn := func() error {
		return syscall.ECONNREFUSED // retriable
	}

	cfg := Config{
		MaxRetries: 3,
		Delays:     []time.Duration{10 * time.Millisecond},
	}

	err := Do(ctx, cfg, fn)
	if err != context.Canceled {
		t.Errorf("error = %v, want context.Canceled", err)
	}
}

func TestDo_ExhaustsAllRetries(t *testing.T) {
	t.Parallel()

	attempts := 0
	fn := func() error {
		attempts++
		return syscall.ECONNRESET // всегда retriable
	}

	cfg := Config{
		MaxRetries: 2,
		Delays:     []time.Duration{1 * time.Millisecond, 1 * time.Millisecond},
	}

	err := Do(context.Background(), cfg, fn)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if attempts != 3 { // initial + 2 retries
		t.Errorf("attempts = %d, want 3", attempts)
	}
}

func TestIsRetriable_NilError(t *testing.T) {
	t.Parallel()

	if isRetriable(nil) {
		t.Error("isRetriable(nil) should return false")
	}
}

func TestIsRetriable_NetworkErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"ECONNREFUSED", syscall.ECONNREFUSED, true},
		{"ECONNRESET", syscall.ECONNRESET, true},
		{"ETIMEDOUT", syscall.ETIMEDOUT, true},
		{"sql.ErrConnDone", sql.ErrConnDone, true},
		{"net.ErrClosed", net.ErrClosed, true},
		{"connection refused message", errors.New("connection refused"), true},
		{"connection reset message", errors.New("connection reset by peer"), true},
		{"EOF message", errors.New("EOF"), true},
		{"i/o timeout message", errors.New("i/o timeout"), true},
		{"broken pipe message", errors.New("broken pipe"), true},
		{"no connection message", errors.New("no connection"), true},
		{"random error", errors.New("something went wrong"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isRetriable(tt.err); got != tt.want {
				t.Errorf("isRetriable(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestDefaultConfig(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	if cfg.MaxRetries != 3 {
		t.Errorf("MaxRetries = %d, want 3", cfg.MaxRetries)
	}
	if len(cfg.Delays) != 3 {
		t.Errorf("len(Delays) = %d, want 3", len(cfg.Delays))
	}
}

func TestDo_ZeroRetries(t *testing.T) {
	t.Parallel()

	attempts := 0
	fn := func() error {
		attempts++
		return errors.New("fail")
	}

	cfg := Config{MaxRetries: 0, Delays: []time.Duration{}}
	err := Do(context.Background(), cfg, fn)

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if attempts != 1 {
		t.Errorf("attempts = %d, want 1", attempts)
	}
}

func TestDo_SuccessAfterRetry(t *testing.T) {
	t.Parallel()

	attempts := 0
	fn := func() error {
		attempts++
		if attempts <= 2 {
			return errors.New("connection refused") // retriable via message
		}
		return nil
	}

	cfg := Config{
		MaxRetries: 3,
		Delays:     []time.Duration{1 * time.Millisecond, 1 * time.Millisecond},
	}

	err := Do(context.Background(), cfg, fn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if attempts != 3 {
		t.Errorf("attempts = %d, want 3", attempts)
	}
}
