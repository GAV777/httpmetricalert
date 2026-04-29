package hash

import (
	"testing"
)

func TestSign(t *testing.T) {
	tests := []struct {
		name  string
		value string
		key   string
	}{
		{"simple", "hello", "secret"},
		{"empty value", "", "secret"},
		{"empty key", "hello", ""},
		{"special chars", `{"id":"test","type":"gauge"}`, "my-key_123"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash := Sign(tt.value, tt.key)
			if hash == "" {
				t.Error("Sign() returned empty hash")
			}
			// SHA256 = 64 hex chars
			if len(hash) != 64 {
				t.Errorf("Sign() hash length = %d, want 64", len(hash))
			}
		})
	}
}

func TestSignDeterministic(t *testing.T) {
	h1 := Sign("test", "key")
	h2 := Sign("test", "key")
	if h1 != h2 {
		t.Errorf("Sign() not deterministic: %s != %s", h1, h2)
	}
}

func TestSignDifferentInputs(t *testing.T) {
	h1 := Sign("test1", "key")
	h2 := Sign("test2", "key")
	if h1 == h2 {
		t.Error("Sign() returned same hash for different values")
	}

	h3 := Sign("test", "key1")
	h4 := Sign("test", "key2")
	if h3 == h4 {
		t.Error("Sign() returned same hash for different keys")
	}
}

func TestVerify(t *testing.T) {
	value := "test data"
	key := "mykey"
	signature := Sign(value, key)

	if !Verify(value, key, signature) {
		t.Error("Verify() failed for valid signature")
	}

	if Verify(value, "wrongkey", signature) {
		t.Error("Verify() succeeded with wrong key")
	}

	if Verify("wrong data", key, signature) {
		t.Error("Verify() succeeded with wrong data")
	}

	if Verify(value, key, "wronghash") {
		t.Error("Verify() succeeded with wrong signature")
	}
}
