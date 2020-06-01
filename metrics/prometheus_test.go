package metrics

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPrometheus(t *testing.T) {
	m := NewMetrics(nil, 10)

	m.RegisterCounter("test_total", "Total number of smth")
	m.RegisterCounter("any_total", "Total number of anything")
	m.RegisterGauge("tests", "Number of active smth")
	m.RegisterGauge("any_tests", "Number of active anything")

	m.Gauge("tests").Set(123)
	m.Counter("test_total").Add(3)

	actual := m.Prometheus()

	assert.Contains(t, actual,
		`
# HELP anycable_go_test_total Total number of smth
# TYPE anycable_go_test_total counter
anycable_go_test_total 3
`,
	)

	assert.Contains(t, actual,
		`
# HELP anycable_go_any_total Total number of anything
# TYPE anycable_go_any_total counter
anycable_go_any_total 0
`,
	)

	assert.Contains(t, actual,
		`
# HELP anycable_go_tests Number of active smth
# TYPE anycable_go_tests gauge
anycable_go_tests 123
`,
	)

	assert.Contains(t, actual,
		`
# HELP anycable_go_any_tests Number of active anything
# TYPE anycable_go_any_tests gauge
anycable_go_any_tests 0
`,
	)
}

func TestPrometheusHandler(t *testing.T) {
	m := NewMetrics(nil, 10)

	m.RegisterCounter("test_total", "Total number of smth")
	m.RegisterCounter("any_total", "Total number of anything")

	m.Counter("test_total").Add(3)

	req, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(m.PrometheusHandler)

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	body := rr.Body.String()

	assert.Contains(t, body, "anycable_go_test_total 3")
	assert.Contains(t, body, "anycable_go_any_total 0")
}
