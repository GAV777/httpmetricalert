package middleware

import (
	"bytes"
	"crypto/rsa"
	"io"
	"net/http"

	pkgcrypto "github.com/GAV777/httpmetricalert/pkg/crypto"
)

// CryptoMiddleware расшифровывает запросы, зашифрованные публичным RSA-ключом.
// Если приватный ключ не задан, middleware пропускает запрос без изменений.
// Определяет зашифрованные запросы по заголовку "X-Crypto: rsa".
func CryptoMiddleware(privKey *rsa.PrivateKey) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Если ключ не задан — пропускаем
			if privKey == nil {
				next.ServeHTTP(w, r)
				return
			}

			// Проверяем, зашифрован ли запрос
			if r.Header.Get("X-Crypto") != "rsa" {
				next.ServeHTTP(w, r)
				return
			}

			// Читаем зашифрованное тело
			encryptedData, err := io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, "Failed to read request body", http.StatusBadRequest)
				return
			}
			r.Body.Close()

			// Расшифровываем
			decryptedData, err := pkgcrypto.Decrypt(privKey, encryptedData)
			if err != nil {
				http.Error(w, "Failed to decrypt request", http.StatusBadRequest)
				return
			}

			// Заменяем тело на расшифрованное
			r.Body = io.NopCloser(bytes.NewReader(decryptedData))
			r.ContentLength = int64(len(decryptedData))
			r.Header.Del("X-Crypto")

			next.ServeHTTP(w, r)
		})
	}
}
