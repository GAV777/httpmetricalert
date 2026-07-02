package middleware

import (
	"bytes"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	pkgcrypto "github.com/GAV777/httpmetricalert/pkg/crypto"
)

func TestCryptoMiddleware_WithValidKey(t *testing.T) {
	t.Parallel()

	privKey, pubKey, err := pkgcrypto.GenerateKeyPair(2048)
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}

	plaintext := []byte(`{"id":"test","type":"gauge","value":42}`)
	encrypted, err := pkgcrypto.Encrypt(pubKey, plaintext)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	req := httptest.NewRequest("POST", "/update", bytes.NewReader(encrypted))
	req.Header.Set("X-Crypto", "rsa")
	req.Header.Set("Content-Type", "application/octet-stream")

	rec := httptest.NewRecorder()

	var gotBody []byte
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	})

	middleware := CryptoMiddleware(privKey)
	middleware(next).ServeHTTP(rec, req)

	if !bytes.Equal(gotBody, plaintext) {
		t.Errorf("decrypted body = %q, want %q", gotBody, plaintext)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	// Заголовок X-Crypto должен быть удалён
	if req.Header.Get("X-Crypto") != "" {
		t.Error("X-Crypto header should be removed after decryption")
	}
}

func TestCryptoMiddleware_WithoutKey(t *testing.T) {
	t.Parallel()

	plaintext := []byte(`{"id":"test","type":"gauge","value":42}`)
	req := httptest.NewRequest("POST", "/update", bytes.NewReader(plaintext))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()

	var gotBody []byte
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	})

	// nil ключ — middleware пропускает
	middleware := CryptoMiddleware(nil)
	middleware(next).ServeHTTP(rec, req)

	if !bytes.Equal(gotBody, plaintext) {
		t.Errorf("body = %q, want %q", gotBody, plaintext)
	}
}

func TestCryptoMiddleware_WithoutCryptoHeader(t *testing.T) {
	t.Parallel()

	privKey, _, err := pkgcrypto.GenerateKeyPair(2048)
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}

	plaintext := []byte(`{"id":"test","type":"gauge","value":42}`)
	req := httptest.NewRequest("POST", "/update", bytes.NewReader(plaintext))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()

	var gotBody []byte
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	})

	middleware := CryptoMiddleware(privKey)
	middleware(next).ServeHTTP(rec, req)

	if !bytes.Equal(gotBody, plaintext) {
		t.Errorf("body = %q, want %q", gotBody, plaintext)
	}
}

func TestCryptoMiddleware_InvalidEncryptedData(t *testing.T) {
	t.Parallel()

	privKey, _, err := pkgcrypto.GenerateKeyPair(2048)
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}

	req := httptest.NewRequest("POST", "/update", bytes.NewReader([]byte("not encrypted data")))
	req.Header.Set("X-Crypto", "rsa")

	rec := httptest.NewRecorder()

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("next handler should not be called")
	})

	middleware := CryptoMiddleware(privKey)
	middleware(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

// helperSaveKeys saves RSA keys to temporary files and returns the paths
func helperSaveKeys(t *testing.T, privKey *rsa.PrivateKey, pubKey *rsa.PublicKey) (privPath, pubPath string) {
	t.Helper()

	pkcs8Bytes, err := x509.MarshalPKCS8PrivateKey(privKey)
	if err != nil {
		t.Fatalf("MarshalPKCS8PrivateKey: %v", err)
	}

	privPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: pkcs8Bytes,
	})

	privFile := t.TempDir() + "/private.pem"
	if err := os.WriteFile(privFile, privPEM, 0600); err != nil {
		t.Fatalf("Write private key: %v", err)
	}

	pkixBytes, err := x509.MarshalPKIXPublicKey(pubKey)
	if err != nil {
		t.Fatalf("MarshalPKIXPublicKey: %v", err)
	}

	pubPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pkixBytes,
	})

	pubFile := t.TempDir() + "/public.pem"
	if err := os.WriteFile(pubFile, pubPEM, 0644); err != nil {
		t.Fatalf("Write public key: %v", err)
	}

	return privFile, pubFile
}
