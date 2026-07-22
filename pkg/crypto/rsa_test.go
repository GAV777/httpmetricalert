package crypto

import (
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
)

func TestEncryptDecrypt_RoundTrip(t *testing.T) {
	t.Parallel()

	privKey, pubKey, err := GenerateKeyPair(2048)
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}

	plaintext := []byte("hello, metrics!")

	encrypted, err := Encrypt(pubKey, plaintext)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	if string(encrypted) == string(plaintext) {
		t.Fatal("encrypted data should differ from plaintext")
	}

	decrypted, err := Decrypt(privKey, encrypted)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}

	if string(decrypted) != string(plaintext) {
		t.Errorf("decrypted = %q, want %q", decrypted, plaintext)
	}
}

func TestEncrypt_NilPublicKey(t *testing.T) {
	t.Parallel()

	_, err := Encrypt(nil, []byte("test"))
	if err == nil {
		t.Fatal("expected error for nil public key")
	}
}

func TestDecrypt_NilPrivateKey(t *testing.T) {
	t.Parallel()

	_, err := Decrypt(nil, []byte("test"))
	if err == nil {
		t.Fatal("expected error for nil private key")
	}
}

func TestLoadPublicKey(t *testing.T) {
	t.Parallel()

	_, pubKey, err := GenerateKeyPair(2048)
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}

	// Сохраняем публичный ключ в PEM-файл
	pkixBytes, err := x509.MarshalPKIXPublicKey(pubKey)
	if err != nil {
		t.Fatalf("MarshalPKIXPublicKey: %v", err)
	}

	dir := t.TempDir()
	pubPath := filepath.Join(dir, "public.pem")

	pemData := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pkixBytes,
	})

	if err := os.WriteFile(pubPath, pemData, 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	loadedPubKey, err := LoadPublicKey(pubPath)
	if err != nil {
		t.Fatalf("LoadPublicKey: %v", err)
	}

	if loadedPubKey.N.Cmp(pubKey.N) != 0 {
		t.Error("loaded public key does not match original")
	}
}

func TestLoadPrivateKey(t *testing.T) {
	t.Parallel()

	privKey, _, err := GenerateKeyPair(2048)
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}

	dir := t.TempDir()
	privPath := filepath.Join(dir, "private.pem")

	// PKCS#8 формат
	pkcs8Bytes, err := x509.MarshalPKCS8PrivateKey(privKey)
	if err != nil {
		t.Fatalf("MarshalPKCS8PrivateKey: %v", err)
	}

	pemData := pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: pkcs8Bytes,
	})

	if err := os.WriteFile(privPath, pemData, 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	loadedPrivKey, err := LoadPrivateKey(privPath)
	if err != nil {
		t.Fatalf("LoadPrivateKey: %v", err)
	}

	if loadedPrivKey.N.Cmp(privKey.N) != 0 {
		t.Error("loaded private key does not match original")
	}
}

func TestLoadKey_NonExistentFile(t *testing.T) {
	t.Parallel()

	_, err := LoadPublicKey("/nonexistent/public.pem")
	if err == nil {
		t.Fatal("expected error for non-existent file")
	}

	_, err = LoadPrivateKey("/nonexistent/private.pem")
	if err == nil {
		t.Fatal("expected error for non-existent file")
	}
}

func TestLoadKey_InvalidPEM(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	invalidPath := filepath.Join(dir, "invalid.pem")

	if err := os.WriteFile(invalidPath, []byte("not a pem"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, err := LoadPublicKey(invalidPath)
	if err == nil {
		t.Fatal("expected error for invalid PEM")
	}

	_, err = LoadPrivateKey(invalidPath)
	if err == nil {
		t.Fatal("expected error for invalid PEM")
	}
}

func TestGenerateKeyPair(t *testing.T) {
	t.Parallel()

	privKey, pubKey, err := GenerateKeyPair(2048)
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}

	if privKey.N.BitLen() != 2048 {
		t.Errorf("key size = %d, want 2048", privKey.N.BitLen())
	}

	if pubKey.N.Cmp(privKey.N) != 0 {
		t.Error("public key N does not match private key N")
	}
}

func TestEncryptDecrypt_LargeData(t *testing.T) {
	t.Parallel()

	privKey, pubKey, err := GenerateKeyPair(2048)
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}

	// RSA-2048 PKCS#1 v1.5 max plaintext: 256 - 11 = 245 bytes
	plaintext := make([]byte, 200)
	for i := range plaintext {
		plaintext[i] = byte(i)
	}

	encrypted, err := Encrypt(pubKey, plaintext)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	decrypted, err := Decrypt(privKey, encrypted)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}

	for i := range plaintext {
		if decrypted[i] != plaintext[i] {
			t.Errorf("byte %d: got %d, want %d", i, decrypted[i], plaintext[i])
			break
		}
	}
}
