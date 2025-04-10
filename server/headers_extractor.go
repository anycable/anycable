package server

import (
	"net"
	"net/http"
	"strings"
)

type HeadersExtractor interface {
	FromRequest(r *http.Request) map[string]string
}

type DefaultHeadersExtractor struct {
	AuthHeader string
	Headers    []string
	Cookies    []string
}

func (h *DefaultHeadersExtractor) FromRequest(r *http.Request) map[string]string {
	res := make(map[string]string)

	for _, header := range h.Headers {
		value := r.Header.Get(header)
		if strings.ToLower(header) == "cookie" {
			value = parseCookies(value, h.Cookies)
		}

		if value != "" {
			res[header] = value
		}
	}

	if h.AuthHeader != "" {
		if wsProtocols := r.Header.Get("sec-websocket-protocol"); wsProtocols != "" {
			if strings.Contains(wsProtocols, "anycable-token.") {
				var token string
				protocols := strings.Split(wsProtocols, ",")

				for _, rawProtocol := range protocols {
					protocol := strings.TrimSpace(rawProtocol)
					if strings.HasPrefix(protocol, "anycable-token.") {
						token = strings.TrimPrefix(protocol, "anycable-token.")
						break
					}
				}

				if token != "" {
					res[h.AuthHeader] = token
				}
			}
		}

		value := r.Header.Get(h.AuthHeader)
		if value != "" {
			res[h.AuthHeader] = value
		}
	}

	res[remoteAddrHeader], _, _ = net.SplitHostPort(r.RemoteAddr)

	return res
}
