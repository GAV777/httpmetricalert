package handlers

import (
	"net/http"
)

func (h *MetricsHandler) PingDB(w http.ResponseWriter, r *http.Request) {
	if err := h.Storage.Ping(); err != nil { // ✅ Вызов с ()
		http.Error(w, "Database unreachable", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("OK"))
}
