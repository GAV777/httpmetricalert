//go:build ignore

// Генерация тестовых RSA-ключей для интеграционных тестов.
// Запуск: go run testdata/genkeys.go
package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"log"
	"os"
)

func main() {
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		log.Fatalf("GenerateKey: %v", err)
	}

	// Приватный ключ в PKCS#8
	pkcs8Bytes, err := x509.MarshalPKCS8PrivateKey(privKey)
	if err != nil {
		log.Fatalf("MarshalPKCS8PrivateKey: %v", err)
	}

	privPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: pkcs8Bytes,
	})

	if err := os.WriteFile("testdata/private.pem", privPEM, 0600); err != nil {
		log.Fatalf("Write private key: %v", err)
	}

	// Публичный ключ в PKIX
	pkixBytes, err := x509.MarshalPKIXPublicKey(&privKey.PublicKey)
	if err != nil {
		log.Fatalf("MarshalPKIXPublicKey: %v", err)
	}

	pubPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pkixBytes,
	})

	if err := os.WriteFile("testdata/public.pem", pubPEM, 0644); err != nil {
		log.Fatalf("Write public key: %v", err)
	}

	log.Println("Generated testdata/private.pem and testdata/public.pem")
}
