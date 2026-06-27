package config

import (
	"flag"
	"log"
	"strings"
)

var serverAddress string
var secretKey string

func init() {
	addr := getEnvOrDefault("ADDRESS", "localhost:8080")
	key := getEnvOrDefault("KEY", "")
	flag.StringVar(&serverAddress, "a", addr, "адрес эндпоинта HTTP-сервера")
	flag.StringVar(&secretKey, "k", key, "секретный ключ для SHA256 хеширования")
}

// ServerAddress возвращает адрес сервера, очищенный от протокола.
// Не имеет side effects — безопасен для вызова в любом контексте.
func ServerAddress() string {
	cleanAddr := strings.TrimPrefix(serverAddress, "http://")
	cleanAddr = strings.TrimPrefix(cleanAddr, "https://")
	return cleanAddr
}

// ValidateServerAddress проверяет корректность адреса и наличие неизвестных аргументов.
// Должен вызываться после flag.Parse(). При ошибках вызывает log.Fatal.
func ValidateServerAddress() {
	cleanAddr := ServerAddress()

	if cleanAddr == "" {
		log.Fatal("Invalid address: ADDRESS cannot be empty")
	}

	if !strings.Contains(cleanAddr, ":") {
		log.Fatal("Invalid address format: expected host:port or :port")
	}

	remainingArgs := flag.Args()
	if len(remainingArgs) > 0 {
		log.Fatalf("неизвестные аргументы командной строки: %v", remainingArgs)
	}
}

// GetSecretKey возвращает секретный ключ для хеширования
func GetSecretKey() string {
	return secretKey
}
