package middleware

import (
	"bytes"
	"io"
	"net/http"

	"github.com/GAV777/httpmetricalert/pkg/hash"
)

// HashSHA256Header имя заголовка для хеша
const HashSHA256Header = "HashSHA256"

// HashMiddleware создаёт middleware для проверки и установки SHA256 хеша.
// Если ключ пустой, middleware пропускает проверку.
func HashMiddleware(secretKey string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Если ключ не задан, пропускаем проверку
			if secretKey == "" {
				next.ServeHTTP(w, r)
				return
			}

			// Для POST/PUT запросов проверяем входящий хеш
			if r.Method == http.MethodPost || r.Method == http.MethodPut {
				// Читаем тело запроса
				bodyBytes, err := io.ReadAll(r.Body)
				if err != nil {
					http.Error(w, "Failed to read request body", http.StatusBadRequest)
					return
				}
				// Закрываем оригинальный Body и заменяем его
				r.Body.Close()
				r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

				// Проверяем хеш — всегда требуем заголовок при наличии ключа
				receivedHash := r.Header.Get(HashSHA256Header)
				expectedHash := hash.Sign(string(bodyBytes), secretKey)
				if receivedHash != expectedHash {
					http.Error(w, "Hash mismatch", http.StatusBadRequest)
					return
				}
			}

			// Обёртка для захвата тела ответа — полностью буферизуем
			bw := &bufferedResponseWriter{
				body:   bytes.NewBuffer(nil),
				code:   http.StatusOK,
				header: w.Header(),
			}

			// Вызываем следующий обработчик с нашей обёрткой
			next.ServeHTTP(bw, r)

			// Вычисляем хеш тела ответа и добавляем в заголовок
			responseBody := bw.body.Bytes()
			if len(responseBody) > 0 {
				computedHash := hash.Sign(string(responseBody), secretKey)
				bw.header.Set(HashSHA256Header, computedHash)
			}

			// Теперь отправляем ответ
			w.WriteHeader(bw.code)
			if len(responseBody) > 0 {
				n, _ := w.Write(responseBody)
				_ = n
			}
		})
	}
}

// bufferedResponseWriter полностью буферизует ответ
type bufferedResponseWriter struct {
	body   *bytes.Buffer
	code   int
	header http.Header
}

func (bw *bufferedResponseWriter) Header() http.Header {
	return bw.header
}

func (bw *bufferedResponseWriter) Write(b []byte) (int, error) {
	return bw.body.Write(b)
}

func (bw *bufferedResponseWriter) WriteHeader(code int) {
	bw.code = code
}
