// Durable Streams protocol implementation https://github.com/durable-streams/durable-streams
package ds

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/anycable/anycable-go/common"
	"github.com/anycable/anycable-go/node"
	"github.com/anycable/anycable-go/server"
	"github.com/anycable/anycable-go/sse"
	"github.com/anycable/anycable-go/utils"
)

// Cursor epoch for CDN collapsing (October 9, 2024 00:00:00 UTC)
var cursorEpoch = time.Date(2024, 10, 9, 0, 0, 0, 0, time.UTC)

// Cursor interval in seconds (20 seconds per DS spec)
const cursorInterval = 20

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

	// Modes
	LongPollMode = "long-poll"
	SSEMode      = "sse"

	// Metrics

	metricsRequestsTotal = "ds_requests_total"
	metricsPollTotal     = "ds_poll_requests_total"
	metricsSSETotal      = "ds_sse_requests_total"
	metricsPollNum       = "ds_poll_clients_num"
	metricsSSENum        = "ds_sse_clients_num"
)

type StreamParams struct {
	// Name is the name of the stream in AnyCable (could differ from Path in the future)
	Name      string
	Cursor    string
	Offset    uint64
	Epoch     string
	LiveMode  string
	Path      string
	RawOffset string
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

	if liveMode != LongPollMode && liveMode != SSEMode && liveMode != "" {
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

	name := streamPath

	return &StreamParams{
		Path:      streamPath,
		Name:      name,
		Cursor:    cursor,
		Offset:    offset,
		Epoch:     epoch,
		RawOffset: offsetStr,
		LiveMode:  liveMode,
	}, nil
}

// subscribeIdentifier represents an Action Cable subscription identifier
// for durable stream subscriptions (pub/sub for now)
type subscribeIdentifier struct {
	Channel          string `json:"channel"`
	StreamName       string `json:"stream_name"`
	SignedStreamName string `json:"signed_stream_name,omitempty"`
}

const pubsubChannel = "$pubsub"

func (sp *StreamParams) ToSubscribeCommand() *common.Message {
	return &common.Message{
		Command: "subscribe",
		Identifier: string(utils.ToJSON(subscribeIdentifier{
			Channel:    pubsubChannel,
			StreamName: sp.Name,
		})),
	}
}

// Stream represents a durable stream connection be wrapping the corresponding node.Session that also carries
// the original stream request information.
type Stream struct {
	Session *node.Session
	Params  *StreamParams
	Conn    node.Connection

	nextCursor string
	cMu        sync.Mutex
}

// NextCursor generates a cursor value for CDN collapsing based on time intervals.
// Per DS spec §8.1, cursors are interval-based (20-second intervals from epoch).
// If a client provides a cursor >= current interval, we add jitter to ensure monotonic progression.
func (s *Stream) NextCursor() string {
	s.cMu.Lock()
	defer s.cMu.Unlock()

	if s.nextCursor != "" {
		return s.nextCursor
	}

	now := time.Now().UTC()
	currentInterval := int64(now.Sub(cursorEpoch).Seconds() / cursorInterval)

	// If client provided a cursor, ensure we return a strictly greater value
	if s.Params.Cursor != "" {
		clientInterval, err := strconv.ParseInt(s.Params.Cursor, 10, 64)
		if err == nil && clientInterval >= currentInterval {
			// Add jitter (1-60 intervals forward) to ensure progression
			currentInterval = clientInterval + 1 + (now.UnixNano() % 60)
		}
	}

	s.nextCursor = strconv.FormatInt(currentInterval, 10)
	return s.nextCursor
}

// NewDSSession creates a node.Session struct to represents a durable stream connection and register it within
// the AnyCable node. TODO: Integrate JWT authentication.
func NewDSSession(n *node.Node, w http.ResponseWriter, info *server.RequestInfo, sp *StreamParams) (*Stream, error) {
	sopts := []node.SessionOption{}
	var conn node.Connection

	stream := &Stream{Params: sp}

	conn = NewPollConnection(w)

	switch sp.LiveMode {
	case "":
		// Create a one-off session that doesn't need to be registered in the pub/sub system.
		// We need it to provide a common interface for all DS modes.
		sopts = append(sopts, node.WithEncoder(NoopEncoder{}), node.InactiveSession())
	case LongPollMode:
		sopts = append(sopts, node.WithEncoder(&PollEncoder{}))
	case SSEMode:
		conn = sse.NewConnection(w)
		sseEncoder := &SSEEncoder{Cursor: stream.NextCursor()}
		sopts = append(sopts, node.WithEncoder(sseEncoder))
	default:
		return nil, fmt.Errorf("unsupported live mode: %s", sp.LiveMode)
	}

	session := node.NewSession(n, conn, info.URL, info.Headers, info.UID, sopts...)
	session.Log = session.Log.With("ds", sp.Path)

	// TODO: add authentication (JWT?)
	n.Authenticated(session, fmt.Sprintf("ds::%s", session.GetID()))

	stream.Session = session
	stream.Conn = conn

	return stream, nil
}
