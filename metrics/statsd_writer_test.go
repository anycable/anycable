package metrics

import (
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestStatsdWriter(t *testing.T) {
	m := NewMetrics(nil, 0)

	m.RegisterCounter("test_count", "")
	m.RegisterGauge("test_gauge", "")

	for i := 0; i < 10; i++ {
		m.Counter("test_count").Inc()
	}

	m.Gauge("test_gauge").Set(123)

	socket, received := startServer(t)
	defer socket.Close()

	t.Run("Write send UDP with metrics", func(t *testing.T) {
		c := NewStatsdConfig()
		c.Host = socket.LocalAddr().String()
		w := NewStatsdWriter(c, nil)
		_ = w.Run(0)
		defer w.Stop()

		err := w.Write(m)
		assert.NoError(t, err)

		var buf []byte

		select {
		case buf = <-received:
		case <-time.After(time.Second):
			t.Error("timeout waiting for UDP payload")
			return
		}

		payload := string(buf)

		assert.Contains(t, payload, "anycable_go.test_count:10|c")
		assert.Contains(t, payload, "anycable_go.test_gauge:123|g")
	})

	t.Run("Write uses custom prefix", func(t *testing.T) {
		c := NewStatsdConfig()
		c.Host = socket.LocalAddr().String()
		c.Prefix = "ws."
		w := NewStatsdWriter(c, nil)
		_ = w.Run(0)
		defer w.Stop()

		err := w.Write(m)
		assert.NoError(t, err)

		var buf []byte

		select {
		case buf = <-received:
		case <-time.After(time.Second):
			t.Error("timeout waiting for UDP payload")
			return
		}

		payload := string(buf)

		assert.Contains(t, payload, "ws.test_count:10|c")
		assert.Contains(t, payload, "ws.test_gauge:123|g")
	})

	t.Run("Adds tags in DataDog format", func(t *testing.T) {
		c := NewStatsdConfig()
		c.TagFormat = "datadog"
		c.Host = socket.LocalAddr().String()
		w := NewStatsdWriter(c, map[string]string{"env": "dev"})
		_ = w.Run(0)
		defer w.Stop()

		err := w.Write(m)
		assert.NoError(t, err)

		var buf []byte

		select {
		case buf = <-received:
		case <-time.After(time.Second):
			t.Error("timeout waiting for UDP payload")
			return
		}

		payload := string(buf)

		assert.Contains(t, payload, "anycable_go.test_count:10|c|#env:dev")
		assert.Contains(t, payload, "anycable_go.test_gauge:123|g|#env:dev")
	})

	t.Run("Adds multiple tags in DataDog format", func(t *testing.T) {
		c := NewStatsdConfig()
		c.TagFormat = "datadog"
		c.Host = socket.LocalAddr().String()
		w := NewStatsdWriter(c, map[string]string{"env": "dev", "rev": "1.1"})
		_ = w.Run(0)
		defer w.Stop()

		err := w.Write(m)
		assert.NoError(t, err)

		var buf []byte

		select {
		case buf = <-received:
		case <-time.After(time.Second):
			t.Error("timeout waiting for UDP payload")
			return
		}

		payload := string(buf)

		assert.Contains(t, payload, "anycable_go.test_count:10|c|#")
		assert.Contains(t, payload, "anycable_go.test_gauge:123|g|#")
		assert.Contains(t, payload, "env:dev")
		assert.Contains(t, payload, "rev:1.1")
	})

	t.Run("Adds tags in InfluxDB format", func(t *testing.T) {
		c := NewStatsdConfig()
		c.TagFormat = "influxdb"
		c.Host = socket.LocalAddr().String()
		w := NewStatsdWriter(c, map[string]string{"env": "dev"})
		_ = w.Run(0)
		defer w.Stop()

		err := w.Write(m)
		assert.NoError(t, err)

		var buf []byte

		select {
		case buf = <-received:
		case <-time.After(time.Second):
			t.Error("timeout waiting for UDP payload")
			return
		}

		payload := string(buf)

		assert.Contains(t, payload, "anycable_go.test_count,env=dev:10|c")
		assert.Contains(t, payload, "anycable_go.test_gauge,env=dev:123|g")
	})

	t.Run("Adds tags in Graphite format", func(t *testing.T) {
		c := NewStatsdConfig()
		c.TagFormat = "graphite"
		c.Host = socket.LocalAddr().String()
		w := NewStatsdWriter(c, map[string]string{"env": "dev"})
		_ = w.Run(0)
		defer w.Stop()

		err := w.Write(m)
		assert.NoError(t, err)

		var buf []byte

		select {
		case buf = <-received:
		case <-time.After(time.Second):
			t.Error("timeout waiting for UDP payload")
			return
		}

		payload := string(buf)

		assert.Contains(t, payload, "anycable_go.test_count;env=dev:10|c")
		assert.Contains(t, payload, "anycable_go.test_gauge;env=dev:123|g")
	})
}

func startServer(t *testing.T) (*net.UDPConn, chan []byte) {
	inSocket, err := net.ListenUDP("udp4", &net.UDPAddr{
		IP: net.IPv4(127, 0, 0, 1),
	})
	if err != nil {
		t.Error(err)
	}

	received := make(chan []byte, 1024)

	go func() {
		for {
			buf := make([]byte, 1500)

			n, err := inSocket.Read(buf)
			if err != nil {
				return
			}

			received <- buf[0:n]
		}

	}()

	return inSocket, received
}
