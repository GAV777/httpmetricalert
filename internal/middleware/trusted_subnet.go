package middleware

import (
	"net"
	"net/http"
)

// TrustedSubnetMiddleware создаёт middleware для проверки IP-адреса клиента
// против доверенной подсети (CIDR). Если подсеть не задана, проверка пропускается.
// Если IP не входит в подсеть, возвращается 403 Forbidden.
func TrustedSubnetMiddleware(trustedSubnet string) func(http.Handler) http.Handler {
	// Парсим подсеть один раз при создании middleware
	var ipNet *net.IPNet
	if trustedSubnet != "" {
		_, ipNet, _ = net.ParseCIDR(trustedSubnet)
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Если подсеть не задана — пропускаем без проверки
			if ipNet == nil {
				next.ServeHTTP(w, r)
				return
			}

			clientIP := r.Header.Get("X-Real-IP")
			if clientIP == "" {
				http.Error(w, "X-Real-IP header is required", http.StatusForbidden)
				return
			}

			ip := net.ParseIP(clientIP)
			if ip == nil {
				http.Error(w, "Invalid X-Real-IP header", http.StatusForbidden)
				return
			}

			if !ipNet.Contains(ip) {
				http.Error(w, "Forbidden: IP not in trusted subnet", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
