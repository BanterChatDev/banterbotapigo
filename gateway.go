package banter

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	opDispatch       = 0
	opHeartbeat      = 1
	opResume         = 6
	opReconnect      = 7
	opInvalidSession = 9
	opHello          = 10
	opHeartbeatAck   = 11

	closeInvalidSeq = 4007
)

var gatewayLog = NewLogger(loggerPrefix + "gateway")

type gatewayFrame struct {
	Op      int             `json:"op"`
	D       json.RawMessage `json:"d,omitempty"`
	S       *int            `json:"s,omitempty"`
	T       string          `json:"t,omitempty"`
	Type    string          `json:"type,omitempty"`
	Payload json.RawMessage `json:"payload,omitempty"`
	HasOp   bool            `json:"-"`
}

type Dispatcher func(ctx context.Context, eventType string, payload json.RawMessage)

type Gateway struct {
	token      string
	intents    int64
	wsURL      string
	dispatcher Dispatcher
	resumeSID  string
	resumeSeq  int

	mu                sync.Mutex
	conn              *websocket.Conn
	closed            bool
	heartbeatInterval int
	lastAck           bool
	missedAcks        int
	lastSeq           int
	invalidSession    bool
	closeCode         int
	closeReason       string

	heartbeatStop chan struct{}
}

const maxMissedAcks = 3

func NewGateway(token string, intents int64, wsURL string, dispatcher Dispatcher) *Gateway {
	return &Gateway{
		token:      token,
		intents:    intents,
		wsURL:      wsURL,
		dispatcher: dispatcher,
		lastSeq:    -1,
		lastAck:    true,
	}
}

func (g *Gateway) SetResume(sessionID string, seq int) {
	g.resumeSID = sessionID
	g.resumeSeq = seq
}

func (g *Gateway) LastSeq() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.lastSeq
}

func (g *Gateway) InvalidSession() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.invalidSession
}

func (g *Gateway) CloseInfo() (int, string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.closeCode, g.closeReason
}

func (g *Gateway) Connect(ctx context.Context) error {
	u := g.wsURL + fmt.Sprintf("?intents=%d", g.intents)
	if g.resumeSID != "" && g.resumeSeq >= 0 {
		u += fmt.Sprintf("&session_id=%s&seq=%d", g.resumeSID, g.resumeSeq)
	}
	hdr := http.Header{
		"Authorization": []string{"Bot " + g.token},
		"User-Agent":    []string{userAgent()},
	}
	dialer := &websocket.Dialer{
		HandshakeTimeout: 30 * time.Second,
		NetDialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
	}
	conn, resp, err := dialer.DialContext(ctx, u, hdr)
	if err != nil {
		status := 0
		if resp != nil {
			status = resp.StatusCode
		}
		if status == 401 {
			return &LoginFailure{BanterError: BanterError{Msg: "invalid bot token"}}
		}
		if status != 0 {
			return &GatewayError{BanterError: BanterError{Msg: fmt.Sprintf("connect failed: HTTP %d", status)}}
		}
		return &GatewayError{BanterError: BanterError{Msg: fmt.Sprintf("connect failed: %s", err)}}
	}
	g.mu.Lock()
	g.conn = conn
	g.lastAck = true
	g.heartbeatStop = make(chan struct{})
	g.mu.Unlock()
	return nil
}

func (g *Gateway) startHeartbeat() {
	g.mu.Lock()
	interval := g.heartbeatInterval
	stop := g.heartbeatStop
	g.mu.Unlock()
	if interval <= 0 {
		interval = 41250
	}
	ticker := time.NewTicker(time.Duration(interval) * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			g.mu.Lock()
			if !g.lastAck {
				g.missedAcks++
				if g.missedAcks >= maxMissedAcks {
					conn := g.conn
					g.mu.Unlock()
					gatewayLog.Info("gateway: %d consecutive missed heartbeat acks, closing", g.missedAcks)
					if conn != nil {
						_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
						_ = conn.WriteControl(websocket.CloseMessage,
							websocket.FormatCloseMessage(4000, "missed heartbeat ack"),
							time.Now().Add(time.Second))
						_ = conn.Close()
					}
					return
				}
				gatewayLog.Info("gateway: missed heartbeat ack %d/%d, continuing", g.missedAcks, maxMissedAcks)
			} else {
				g.missedAcks = 0
			}
			g.lastAck = false
			seq := g.lastSeq
			conn := g.conn
			g.mu.Unlock()
			var payload any
			if seq >= 0 {
				payload = seq
			}
			frame, _ := json.Marshal(map[string]any{"op": opHeartbeat, "d": payload})
			if conn != nil {
				_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
				if err := conn.WriteMessage(websocket.TextMessage, frame); err != nil {
					gatewayLog.Info("heartbeat send failed: %s", err)
					_ = conn.Close()
					return
				}
			}
		}
	}
}

func (g *Gateway) readDeadline() time.Time {
	interval := g.heartbeatInterval
	if interval <= 0 {
		interval = 41250
	}
	return time.Now().Add(time.Duration(interval)*time.Millisecond*2 + 10*time.Second)
}

func (g *Gateway) Run(ctx context.Context) error {
	g.mu.Lock()
	conn := g.conn
	g.mu.Unlock()
	if conn == nil {
		return &GatewayError{BanterError: BanterError{Msg: "not connected"}}
	}

	_ = conn.SetReadDeadline(g.readDeadline())

	go g.startHeartbeat()

	for {
		_, raw, err := conn.ReadMessage()
		if err != nil {
			g.mu.Lock()
			if ce, ok := err.(*websocket.CloseError); ok {
				g.closeCode = ce.Code
				g.closeReason = ce.Text
			}
			g.mu.Unlock()
			return nil
		}
		_ = conn.SetReadDeadline(g.readDeadline())

		var msg gatewayFrame
		if err := json.Unmarshal(raw, &msg); err != nil {
			gatewayLog.Info("gateway: dropping non-JSON frame: %s", err)
			continue
		}
		var probe map[string]json.RawMessage
		if err := json.Unmarshal(raw, &probe); err == nil {
			_, msg.HasOp = probe["op"]
		}

		if msg.HasOp && msg.Op == opHello {
			var d struct {
				HeartbeatInterval int `json:"heartbeat_interval"`
			}
			_ = json.Unmarshal(msg.D, &d)
			interval := d.HeartbeatInterval
			if interval <= 0 {
				interval = 41250
			}
			g.mu.Lock()
			g.heartbeatInterval = interval
			g.lastAck = true
			g.mu.Unlock()
			continue
		}
		if msg.HasOp && msg.Op == opHeartbeatAck {
			g.mu.Lock()
			g.lastAck = true
			g.missedAcks = 0
			g.mu.Unlock()
			continue
		}
		if msg.HasOp && msg.Op == opDispatch {
			if msg.S != nil {
				g.mu.Lock()
				if *msg.S > g.lastSeq {
					g.lastSeq = *msg.S
				}
				g.mu.Unlock()
			}
			eventType := msg.T
			payload := msg.D
			if eventType == "" && msg.Type != "" {
				eventType = msg.Type
			}
			if len(payload) == 0 {
				payload = json.RawMessage("{}")
			}
			if g.dispatcher != nil {
				g.dispatcher(ctx, eventType, payload)
			}
			continue
		}
		if msg.HasOp && msg.Op == opInvalidSession {
			g.mu.Lock()
			g.invalidSession = true
			conn := g.conn
			g.mu.Unlock()
			if conn != nil {
				_ = conn.WriteControl(websocket.CloseMessage,
					websocket.FormatCloseMessage(4000, "invalid session"),
					time.Now().Add(time.Second))
			}
			return nil
		}
		if msg.HasOp && msg.Op == opReconnect {
			g.mu.Lock()
			conn := g.conn
			g.mu.Unlock()
			if conn != nil {
				_ = conn.WriteControl(websocket.CloseMessage,
					websocket.FormatCloseMessage(4000, "server requested reconnect"),
					time.Now().Add(time.Second))
			}
			return nil
		}
		if msg.HasOp {
			gatewayLog.Debug("gateway: unknown op %d ignored", msg.Op)
			continue
		}
		eventType := msg.Type
		payload := msg.Payload
		if len(payload) == 0 {
			payload = json.RawMessage("{}")
		}
		if eventType != "" && g.dispatcher != nil {
			g.dispatcher(ctx, eventType, payload)
		}
	}
}

func (g *Gateway) Close() error {
	g.mu.Lock()
	g.closed = true
	stop := g.heartbeatStop
	conn := g.conn
	g.mu.Unlock()
	if stop != nil {
		select {
		case <-stop:
		default:
			close(stop)
		}
	}
	if conn == nil {
		return nil
	}
	return conn.Close()
}