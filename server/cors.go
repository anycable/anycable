package server

import (
	"net/http"
	"net/url"
	"strings"
)

// OriginAllowed checks whether the Origin header value matches one of the allowed hosts.
// A host may start with a wildcard: both "*.example.com" and "*example.com" match subdomains
// (the latter also matches the apex), but never a different domain sharing the suffix.
func OriginAllowed(origin string, hosts []string) bool {
	u, err := url.Parse(strings.ToLower(origin))
	if err != nil || u.Host == "" {
		return false
	}

	for _, host := range hosts {
		host = strings.ToLower(strings.TrimSpace(host))

		if host == "" {
			continue
		}

		if u.Host == host {
			return true
		}

		suffix, wildcard := strings.CutPrefix(host, "*")
		if !wildcard {
			continue
		}

		if suffix == "" {
			return true
		}

		if strings.HasPrefix(suffix, ".") {
			if strings.HasSuffix(u.Host, suffix) {
				return true
			}
		} else if u.Host == suffix || strings.HasSuffix(u.Host, "."+suffix) {
			return true
		}
	}

	return false
}

func WriteCORSHeaders(w http.ResponseWriter, r *http.Request, origins []string) {
	if len(origins) == 0 {
		w.Header().Set("Access-Control-Allow-Origin", "*")
	} else {
		w.Header().Add("Vary", "Origin")

		if origin := r.Header.Get("Origin"); OriginAllowed(origin, origins) {
			w.Header().Set("Access-Control-Allow-Origin", strings.ToLower(origin))
		}
	}

	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Credentials", "true")
	w.Header().Set("Access-Control-Allow-Headers", "Origin, X-Requested-With, X-Request-ID, Content-Type, Accept, X-CSRF-Token, Authorization")
}
