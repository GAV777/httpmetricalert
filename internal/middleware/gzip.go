package middleware

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

// GzipMiddleware handles request/response compression.
// It compresses responses if the client supports gzip (via Accept-Encoding header)
// and decompresses request bodies if they are gzip-encoded (via Content-Encoding header).
func GzipMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if client supports gzip
		supportsGzip := false
		acceptEncoding := r.Header.Get("Accept-Encoding")
		for _, v := range strings.Split(acceptEncoding, ",") {
			if strings.TrimSpace(v) == "gzip" {
				supportsGzip = true
				break
			}
		}

		// If client wants gzip — wrap ResponseWriter
		if supportsGzip {
			gz := gzip.NewWriter(w)
			defer gz.Close()

			w.Header().Set("Content-Encoding", "gzip")
			w = gzipResponseWriter{ResponseWriter: w, gz: gz}
		}

		// If request body is gzip-encoded — decompress it
		if r.Header.Get("Content-Encoding") == "gzip" {
			gr, err := gzip.NewReader(r.Body)
			if err != nil {
				http.Error(w, "Invalid gzip data", http.StatusBadRequest)
				return
			}
			defer gr.Close()
			r.Body = io.NopCloser(gr)
		}

		next.ServeHTTP(w, r)
	})
}
