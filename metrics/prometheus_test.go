package metrics

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrometheus(t *testing.T) {
	m := NewMetrics(nil, 10, slog.Default())

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

func TestPrometheusWithTags(t *testing.T) {
	m := NewMetrics(nil, 10, slog.Default())
	m.DefaultTags(map[string]string{"env": "dev", "instance": "R2D2"})

	m.RegisterCounter("test_total", "Total number of smth")
	m.RegisterGauge("tests", "Number of active smth")

	m.Gauge("tests").Set(123)
	m.Counter("test_total").Add(3)

	actual := m.Prometheus()

	r := regexp.MustCompile(`anycable_go_test_total{(.+)}\s+(\d+)`)
	matches := r.FindStringSubmatch(actual)

	require.NotNil(t, matches)

	tagsStr := matches[1]
	valStr := matches[2]

	tags := strings.Split(tagsStr, ", ")

	assert.Contains(t, tags, `env="dev"`)
	assert.Contains(t, tags, `instance="R2D2"`)

	assert.Equal(t, "3", valStr)
}

func TestPrometheusHandler(t *testing.T) {
	m := NewMetrics(nil, 10, slog.Default())

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
