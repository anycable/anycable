package lp

import (
	"context"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/anycable/anycable-go/common"
	"github.com/anycable/anycable-go/metrics"
	"github.com/anycable/anycable-go/node"
	"github.com/anycable/anycable-go/server"
	"github.com/anycable/anycable-go/utils"
	"github.com/anycable/anycable-go/ws"

	nanoid "github.com/matoous/go-nanoid"
)

const (
	metricsClientsNum         = "longpoll_clients_num"
	metricsStaleRequestsTotal = "longpoll_stale_requests_total"
)

const (
	// Respond with this message when request is made with an expired session ID
	expiredMessage = "{\"type\":\"disconnect\",\"reason\":\"session_expired\",\"reconnect\":true}"
)

// Registration encapsulates session and it's last observed timestamp
// to be used with a priority queue
type Registration struct {
	session *node.Session
	id      string
}

// Hub keeps track of all long-polling connections
type Hub struct {
	node    *node.Node
	metrics metrics.Instrumenter
	config  *Config

	sessions map[string]*utils.PriorityQueueItem[*Registration, int64]
	// min-max heap used to track stale sessions
	heap  *utils.PriorityQueue[*Registration, int64]
	sesMu sync.RWMutex

	mu         sync.Mutex
	closed     bool
	shutdownCh chan struct{}

	log *slog.Logger
}

func NewHub(n *node.Node, m metrics.Instrumenter, c *Config, l *slog.Logger) *Hub {
	h := Hub{
		node:       n,
		metrics:    m,
		config:     c,
		sessions:   make(map[string]*utils.PriorityQueueItem[*Registration, int64]),
		heap:       utils.NewPriorityQueue[*Registration, int64](),
		shutdownCh: make(chan struct{}, 1),
		log:        l.With("context", "lp"),
	}

	if m != nil {
		h.registerMetrics()
	}

	return &h
}

func (h *Hub) NewSession(w http.ResponseWriter, info *server.RequestInfo) (string, *node.Session, error) {
	conn := NewConnection(h.config.FlushInterval)
	conn.ResetWriter(w)

	session := node.NewSession(h.node, conn, info.URL, info.Headers, info.UID, node.WithPingInterval(0))
	res, err := h.node.Authenticate(session)

	if err != nil {
		return "", nil, err
	}

	if res.Status == common.SUCCESS {
		newID := h.addSession(session)
		return newID, session, nil
	} else {
		return "", session, nil
	}
}

func (h *Hub) FindSession(w http.ResponseWriter, id string) (*node.Session, error) {
	session := h.lookupSession(id)

	if session == nil {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(expiredMessage)) // nolint: errcheck
		return nil, nil
	}

	// Existing session found, reset its writer
	session.UnderlyingConn().(*Connection).ResetWriter(w)

	return session, nil
}

// Called when HTTP request was closed
func (h *Hub) Disconnected(id string) {
	deadline := time.Now().UnixMilli() + int64(h.config.KeepaliveTimeout*1000)

	h.resetSessionDeadline(id, deadline)
}

func (h *Hub) Run() {
	reapInterval := (h.config.KeepaliveTimeout * 1000) / 2

	ticker := time.NewTicker(time.Duration(reapInterval) * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			h.reapStaleSessions()
		case <-h.shutdownCh:
			return
		}
	}
}

func (h *Hub) Shutdown(ctx context.Context) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.closed {
		return nil
	}

	h.closed = true
	close(h.shutdownCh)

	return nil
}

func (h *Hub) Size() int {
	h.sesMu.RLock()
	defer h.sesMu.RUnlock()

	return len(h.sessions)
}

func (h *Hub) lookupSession(id string) *node.Session {
	h.sesMu.RLock()
	if item, ok := h.sessions[id]; ok {
		ses := item.Value().session
		h.sesMu.RUnlock()

		// Session might have been disconnected by server in the middle of the poll cycle,
		// we must remove it from the hub in this case
		if ses.IsConnected() {
			h.log.Debug("session found", "id", id)
			deadline := time.Now().UnixMilli() + int64((h.config.PollInterval+h.config.KeepaliveTimeout)*1000)
			h.resetSessionDeadline(id, deadline)
			return ses
		} else {
			h.log.Debug("session found, but already disconnected", "id", id)
			h.removeSession(id)
		}
	} else {
		h.log.Debug("session not found", "id", id)
		h.sesMu.RUnlock()

		if h.metrics != nil {
			h.metrics.CounterIncrement(metricsStaleRequestsTotal)
		}
	}

	return nil
}

func (h *Hub) addSession(ses *node.Session) string {
	id, _ := nanoid.Nanoid(8)

	h.sesMu.Lock()

	reg := &Registration{
		session: ses,
		id:      id,
	}

	deadline := time.Now().Unix() + int64((h.config.PollInterval+h.config.KeepaliveTimeout)*1000)
	item := h.heap.PushItem(reg, deadline)

	// fmt.Printf("[%s] new session: id=%s, priority=%d\n", time.Now().String(), item.Value().id, item.Priority())

	h.sessions[id] = item

	h.sesMu.Unlock()

	return id
}

func (h *Hub) resetSessionDeadline(id string, deadline int64) {
	h.sesMu.Lock()
	defer h.sesMu.Unlock()

	if item, ok := h.sessions[id]; ok {
		// fmt.Printf("[%s] reset deadline: id=%s, old priority=%d, new priority=%d\n", time.Now().String(), item.Value().id, item.Priority(), deadline)

		h.heap.Update(item, deadline)
	}
}

func (h *Hub) removeSession(id string) {
	h.sesMu.Lock()
	if item, ok := h.sessions[id]; ok {
		delete(h.sessions, id)
		h.heap.Remove(item)
	}
	h.sesMu.Unlock()
}

func (h *Hub) reapStaleSessions() {
	for {
		h.sesMu.Lock()

		item := h.heap.Peek()

		if item == nil {
			h.sesMu.Unlock()
			break
		}

		now := time.Now().UnixMilli()
		priority := item.Priority()

		if priority > now {
			h.sesMu.Unlock()
			break
		}

		// fmt.Printf("[%s] reap: id=%s, priority=%d, now=%d\n", time.Now().String(), item.Value().id, priority, now)

		item = h.heap.PopItem()

		reg := item.Value()
		delete(h.sessions, reg.id)
		h.sesMu.Unlock()

		if reg.session.IsConnected() {
			h.log.Debug("disconnecting stale session", "id", reg.id)
			reg.session.Disconnect("Timeout", ws.CloseNormalClosure)
		}
	}

	if h.metrics != nil {
		h.metrics.GaugeSet(metricsClientsNum, uint64((h.Size())))
	}
}

func (h *Hub) registerMetrics() {
	h.metrics.RegisterGauge(metricsClientsNum, "The number of active long-polling clients")
	h.metrics.RegisterCounter(metricsStaleRequestsTotal, "The total number of long-polling requests with stale credentials")
}
