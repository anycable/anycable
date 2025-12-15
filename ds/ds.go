// Durable Streams protocol implementation https://github.com/durable-streams/durable-streams
package ds

import (
	"errors"
	"net/http"
	"strings"

	"github.com/anycable/anycable-go/common"
	"github.com/anycable/anycable-go/node"
	"github.com/anycable/anycable-go/server"
	"github.com/joomcode/errorx"
)

const (
	// Response header containing the next offset to read from.
	// Offsets are opaque tokens - clients MUST NOT interpret the format.
	StreamOffsetHeader = "Stream-Next-Offset"

	// Response header for cursor (used for CDN collapsing).
	// Echo this value in subsequent long-poll requests.
	StreamCursorHeader = "Stream-Cursor"

	// Presence header indicating response ends at current end of stream.
	// When present (any value), indicates up-to-date.
	StreamUpToDateHeader = "Stream-Up-To-Date"

	// ============================================================================
	// Request Headers
	// ============================================================================

	// Request header for writer coordination sequence.
	// Monotonic, lexicographic. If lower than last appended seq -> 409 Conflict.
	StreamSeqHeader = "Stream-Seq"

	// Request header for stream TTL in seconds (on create).
	StreamTTLHeader = "Stream-TTL"

	// Request header for absolute stream expiry time (RFC3339, on create).
	StreamExpiresAtHeader = "Stream-Expires-At"

	// ============================================================================
	// Query Parameters
	// ============================================================================

	// Query parameter for starting offset.
	OffsetQueryParam = "offset"

	// Query parameter for live mode.
	// Values: "long-poll", "sse"
	LiveQueryParam = "live"

	// Query parameter for echoing cursor (CDN collapsing).
	CursorQueryParam = "cursor"

	// Metrics

	metricsRequestsTotal = "ds_requests_total"
	metricsPollTotal     = "ds_poll_clients_total"
	metricsSSETotal      = "ds_sse_clients_total"
	metricsPollNum       = "ds_poll_clients_num"
	metricsSSENum        = "ds_sse_clients_num"
)

type StreamParams struct {
	Path      string
	Cursor    string
	Offset    uint64
	Epoch     string
	RawOffset string
	LiveMode  string
}

func StreamParamsFromReq(r *http.Request, c *Config) (*StreamParams, error) {
	// Extract stream name from URL path
	streamPath := strings.TrimPrefix(r.URL.Path, c.Path)
	streamPath = strings.TrimPrefix(streamPath, "/")

	if streamPath == "" {
		return nil, errors.New("stream is missing in the URL")
	}

	offsetStr := r.URL.Query().Get(OffsetQueryParam)
	liveMode := r.URL.Query().Get(LiveQueryParam)
	cursor := r.URL.Query().Get(CursorQueryParam)

	if liveMode != "long-poll" && liveMode != "sse" && liveMode != "" {
		return nil, errors.New("invalid live mode")
	}

	if offsetStr == "" && liveMode != "" {
		return nil, errors.New("offset is missing for live mode")
	}

	// Default offset to start for catch-up mode
	if offsetStr == "" {
		offsetStr = StartOffset
	}

	// Ensure offset has correct format
	offset, epoch, err := DecodeOffset(offsetStr)
	if err != nil {
		return nil, err
	}

	return &StreamParams{
		Path:      streamPath,
		Cursor:    cursor,
		Offset:    offset,
		Epoch:     epoch,
		RawOffset: offsetStr,
		LiveMode:  liveMode,
	}, nil
}

func NewDSSession(n *node.Node, w http.ResponseWriter, info *server.RequestInfo, sp *StreamParams) (*node.Session, error) {
	sopts := []node.SessionOption{
		node.WithEncoder(&Encoder{Cursor: sp.Cursor}),
	}

	if sp.LiveMode == "" {
		// Create a temporary session for authentication
		// Use a dummy connection since we don't need it for non-live modes
		sopts = append(sopts, node.AsIdleSession())
	}

	conn := NewConnection(w)
	session := node.NewSession(n, conn, info.URL, info.Headers, info.UID, sopts...)

	res, err := n.Authenticate(session)

	// canceled
	if res == nil && err == nil {
		return nil, nil
	}

	if err != nil || res.Status == common.ERROR {
		return nil, errorx.Decorate(err, "failed to authenticate")
	}

	return session, nil
}
