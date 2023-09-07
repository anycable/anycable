package server

import (
	"net/http"
	"net/url"
	"strings"
)

func WriteCORSHeaders(w http.ResponseWriter, r *http.Request, origins []string) {
	if len(origins) == 0 {
		w.Header().Set("Access-Control-Allow-Origin", "*")
	} else {
		origin := strings.ToLower(r.Header.Get("Origin"))
		u, err := url.Parse(origin)
		if err == nil {
			for _, host := range origins {
				if host[0] == '*' && strings.HasSuffix(u.Host, host[1:]) {
					w.Header().Set("Access-Control-Allow-Origin", origin)
				}
				if u.Host == host {
					w.Header().Set("Access-Control-Allow-Origin", origin)
				}
			}
		}
	}

	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Credentials", "true")
	w.Header().Set("Access-Control-Allow-Headers", "Origin, X-Requested-With, X-Request-ID, Content-Type, Accept, X-CSRF-Token, Authorization")
}
