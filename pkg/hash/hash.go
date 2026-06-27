package hash

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
)

// Sign вычисляет HMAC-SHA256 подпись от данных value с использованием ключа key.
// Возвращает hex-строку с подписью.
func Sign(value, key string) string {
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write([]byte(value))
	return hex.EncodeToString(mac.Sum(nil))
}

// Verify проверяет соответствие подписи signature данным value с ключом key.
func Verify(value, key, signature string) bool {
	expected := Sign(value, key)
	return hmac.Equal([]byte(expected), []byte(signature))
}
