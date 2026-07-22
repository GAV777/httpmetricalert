// Package netutil provides network utility functions.
package netutil

import "net"

// LocalIP returns the local non-loopback IPv4 address of the host.
// Returns empty string if no suitable address is found.
func LocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}
	for _, addr := range addrs {
		if ipNet, ok := addr.(*net.IPNet); ok && !ipNet.IP.IsLoopback() {
			if ipNet.IP.To4() != nil {
				return ipNet.IP.String()
			}
		}
	}
	return ""
}
