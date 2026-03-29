package config

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"
)

type gzipResponseWriter struct {
	http.ResponseWriter
	gz *gzip.Writer
}

func (w gzipResponseWriter) Write(b []byte) (int, error) {
	return w.gz.Write(b)
}

// GzipMiddleware обрабатывает сжатие/распаковку тел запросов и ответов
func GzipMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Проверяем, поддерживает ли клиент gzip
		supportsGzip := false
		acceptEncoding := r.Header.Get("Accept-Encoding")
		for _, v := range strings.Split(acceptEncoding, ",") {
			if strings.TrimSpace(v) == "gzip" {
				supportsGzip = true
				break
			}
		}

		// Если клиент хочет gzip — оборачиваем ResponseWriter
		if supportsGzip {
			gz := gzip.NewWriter(w)
			defer gz.Close()

			w.Header().Set("Content-Encoding", "gzip")
			w = gzipResponseWriter{ResponseWriter: w, gz: gz}
		}

		// Если прислано сжатое тело — распаковываем
		if r.Header.Get("Content-Encoding") == "gzip" {
			gr, err := gzip.NewReader(r.Body)
			if err != nil {
				http.Error(w, "Invalid gzip data", http.StatusBadRequest)
				return
			}
			defer gr.Close()
			r.Body = io.NopCloser(gr) // оборачиваем обратно в ReadCloser
		}

		next.ServeHTTP(w, r)
	})
}
