package server

import "net/http"

// https://www.youtube.com/watch?v=I_izvAbhExY
var healthMsg = []byte("Ah, ha, ha, ha, stayin' alive, stayin' alive.")

// HealthHandler always reponds with 200 status
func (s *HTTPServer) HealthHandler(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write(healthMsg)
}
