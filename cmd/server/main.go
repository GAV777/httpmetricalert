package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/GAV777/httpmetricalert/internal/handlers"
	"github.com/GAV777/httpmetricalert/internal/storage"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

var (
	serverAddress string // будет содержать только host:port, например "localhost:8080"
)

func init() {
	addr := getEnvOrDefault("ADDRESS", "localhost:8080")
	flag.StringVar(&serverAddress, "a", addr, "адрес эндпоинта HTTP-сервера")
}

// getEnvOrDefault возвращает значение переменной окружения или значение по умолчанию
func getEnvOrDefault(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

func main() {
	flag.Parse()

	if len(flag.Args()) > 0 {
		log.Fatalf("неизвестные аргументы командной строки: %v", flag.Args())
	}

	// Убедимся, что serverAddress не содержит http:// или https://
	// Очищаем от префикса, если есть
	cleanAddr := strings.TrimPrefix(serverAddress, "http://")
	cleanAddr = strings.TrimPrefix(cleanAddr, "https://")

	// Проверяем, что после очистки осталось что-то
	if cleanAddr == "" {
		log.Fatal("Invalid address: ADDRESS cannot be empty")
	}

	// Разрешаем только формат host:port или :port
	if !strings.Contains(cleanAddr, ":") {
		log.Fatal("Invalid address format: expected host:port or :port")
	}

	// Создаём хранилище
	storage := storage.NewMemStorage()

	// Создаём хендлеры с внедрением зависимости
	handler := handlers.NewMetricsHandler(storage)

	r := chi.NewRouter()

	r.Use(middleware.Recoverer)
	r.Use(noDoubleSlashes)

	// Маршруты
	r.Post("/update/{type}/{name}/{value}", handler.UpdateHandler)
	r.Get("/value/{type}/{name}", handler.GetValueHandler)
	r.Get("/", handler.ListMetricsHandler)

	// Глобальный 404
	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Not Found", http.StatusNotFound)
	})

	log.Printf("🚀 Starting server on %s", cleanAddr)
	log.Fatal(http.ListenAndServe(cleanAddr, r))
}

// noDoubleSlashes блокирует пути с двойными слешами
func noDoubleSlashes(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "//") {
			http.Error(w, "Not Found", http.StatusNotFound)
			return
		}
		next.ServeHTTP(w, r)
	})
}
