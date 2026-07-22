// Package crypto предоставляет функции для асимметричного шифрования RSA.
//
// Загрузка ключей из PEM-файлов, шифрование (публичный ключ)
// и расшифровка (приватный ключ) с использованием PKCS#1 v1.5.
package crypto

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
)

// ErrInvalidKey возвращается при неверном формате ключа.
var ErrInvalidKey = errors.New("invalid key format")

// LoadPublicKey загружает публичный RSA-ключ из PEM-файла.
func LoadPublicKey(path string) (*rsa.PublicKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read public key file: %w", err)
	}

	block, _ := pem.Decode(data)
	if block == nil {
		return nil, ErrInvalidKey
	}

	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse public key: %w", err)
	}

	rsaPub, ok := pub.(*rsa.PublicKey)
	if !ok {
		return nil, ErrInvalidKey
	}

	return rsaPub, nil
}

// LoadPrivateKey загружает приватный RSA-ключ из PEM-файла.
func LoadPrivateKey(path string) (*rsa.PrivateKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read private key file: %w", err)
	}

	block, _ := pem.Decode(data)
	if block == nil {
		return nil, ErrInvalidKey
	}

	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		// Пробуем формат PKCS#1
		key, err = x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parse private key: %w", err)
		}
		return key.(*rsa.PrivateKey), nil
	}

	rsaKey, ok := key.(*rsa.PrivateKey)
	if !ok {
		return nil, ErrInvalidKey
	}

	return rsaKey, nil
}

// Encrypt шифрует данные публичным RSA-ключом (PKCS#1 v1.5).
// Максимальный размер данных: key.Size() - 11 байт.
func Encrypt(pubKey *rsa.PublicKey, data []byte) ([]byte, error) {
	if pubKey == nil {
		return nil, errors.New("public key is nil")
	}

	encrypted, err := rsa.EncryptPKCS1v15(rand.Reader, pubKey, data)
	if err != nil {
		return nil, fmt.Errorf("rsa encrypt: %w", err)
	}

	return encrypted, nil
}

// Decrypt расшифровывает данные приватным RSA-ключом (PKCS#1 v1.5).
func Decrypt(privKey *rsa.PrivateKey, data []byte) ([]byte, error) {
	if privKey == nil {
		return nil, errors.New("private key is nil")
	}

	decrypted, err := rsa.DecryptPKCS1v15(rand.Reader, privKey, data)
	if err != nil {
		return nil, fmt.Errorf("rsa decrypt: %w", err)
	}

	return decrypted, nil
}

// GenerateKeyPair генерирует пару RSA-ключей заданной длины.
// Используется для тестов и утилит.
func GenerateKeyPair(bits int) (*rsa.PrivateKey, *rsa.PublicKey, error) {
	privKey, err := rsa.GenerateKey(rand.Reader, bits)
	if err != nil {
		return nil, nil, fmt.Errorf("generate rsa key: %w", err)
	}

	return privKey, &privKey.PublicKey, nil
}
