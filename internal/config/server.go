package config

import (
	"flag"
	"log"
	"strings"
)

var serverAddress string

func init() {
	addr := getEnvOrDefault("ADDRESS", "localhost:8080")
	flag.StringVar(&serverAddress, "a", addr, "адрес эндпоинта HTTP-сервера")
}

func GetServerAddress() string {
	cleanAddr := strings.TrimPrefix(serverAddress, "http://")
	cleanAddr = strings.TrimPrefix(cleanAddr, "https://")

	if cleanAddr == "" {
		log.Fatal("Invalid address: ADDRESS cannot be empty")
	}

	if !strings.Contains(cleanAddr, ":") {
		log.Fatal("Invalid address format: expected host:port or :port")
	}

	// Проверяем неизвестные аргументы после парсинга всех флагов
	remainingArgs := flag.Args()
	if len(remainingArgs) > 0 {
		log.Fatalf("неизвестные аргументы командной строки: %v", remainingArgs)
	}

	return cleanAddr
}
