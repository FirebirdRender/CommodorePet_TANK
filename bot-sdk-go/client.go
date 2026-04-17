package botsdk

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/coder/websocket"
)

// ErrSendBufferFull is returned by SendInput / SendPlayAgain when the outgoing
// websocket send buffer is full. This surfaces writer-side backpressure to the
// caller instead of blocking the goroutine invoking the send, which — when
// called from inside a read-loop callback (OnTick / OnTickDelta) — would
// otherwise deadlock the client by preventing the read loop from draining new
// server messages.
//
// Callers (typically bots) should treat this as "input dropped; try again
// next tick or back off" rather than as a fatal error.
var ErrSendBufferFull = errors.New("bot-sdk: send buffer full")

// Client connects to a NetTank server and handles the bot protocol.
type Client struct {
	// configuration
	ServerURL  string
	RoomCode   string
	PlayerID   int
	Token      string
	PlayerName string

	// callbacks (user sets these)
	OnGameStart    func(msg *GameStartPayload)
	OnTick         func(msg *TickPayload)
	OnTickDelta    func(msg *TickDeltaPayload)
	OnRoundOver    func(msg *RoundOverPayload)
	OnGameOver     func(msg *GameOverPayload)
	OnError        func(code string, message string)
	OnOpponentLeft func(reason string)
	OnJoined       func(msg *JoinedPayload)

	// reconnect policy
	MaxRetries  int
	BaseBackoff time.Duration
	MaxBackoff  time.Duration

	// internal
	conn      *websocket.Conn
	mu        sync.Mutex
	done      chan struct{}
	closeOnce sync.Once
	sendCh    chan []byte

	// state
	CurrentTick uint64
	connected   bool
}

// NewClient creates a new bot client.
func NewClient(serverURL, roomCode string, playerID int, token, playerName string) *Client {
	return &Client{
		ServerURL:   serverURL,
		RoomCode:    roomCode,
		PlayerID:    playerID,
		Token:       token,
		PlayerName:  playerName,
		MaxRetries:  5,
		BaseBackoff: time.Second,
		MaxBackoff:  30 * time.Second,
		done:        make(chan struct{}),
		sendCh:      make(chan []byte, 16),
	}
}

// Connect dials the WebSocket connection and sends rejoin message.
func (c *Client) Connect(ctx context.Context) error {
	c.mu.Lock()
	if c.connected {
		c.mu.Unlock()
		return fmt.Errorf("already connected")
	}
	c.mu.Unlock()

	conn, _, err := websocket.Dial(ctx, c.ServerURL, nil)
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}

	c.mu.Lock()
	c.conn = conn
	c.connected = true
	c.mu.Unlock()

	// Send rejoin message
	rejoin := RejoinPayload{
		RoomCode:              c.RoomCode,
		PlayerID:              c.PlayerID,
		Token:                 c.Token,
		PlayerName:            c.PlayerName,
		ClientProtocolVersion: ProtocolVersion,
	}

	if err := c.sendJSONRaw(ctx, "rejoin", rejoin); err != nil {
		c.Close()
		return fmt.Errorf("send rejoin: %w", err)
	}

	return nil
}

// Run runs the main read loop, blocking until disconnect or error.
func (c *Client) Run(ctx context.Context) error {
	// Start write pump
	go c.writePump()

	// Read loop
	for {
		select {
		case <-ctx.Done():
			c.Close()
			return ctx.Err()
		case <-c.done:
			return nil
		default:
		}

		msgType, payload, err := c.readEnvelope(ctx)
		if err != nil {
			c.handleReadError(ctx, err)
			return err
		}

		if err := c.dispatchMessage(msgType, payload); err != nil {
			c.handleReadError(ctx, err)
			return err
		}
	}
}

// SendInput sends an input message for a specific tick.
func (c *Client) SendInput(tick uint64, key, action string) error {
	input := InputPayload{
		Tick:   tick,
		Key:    key,
		Action: action,
	}
	return c.sendJSON("input", input)
}

// SendPlayAgain sends a play_again message.
func (c *Client) SendPlayAgain() error {
	return c.sendJSON("play_again", struct{}{})
}

// Close performs a clean shutdown.
func (c *Client) Close() {
	c.closeOnce.Do(func() {
		c.mu.Lock()
		defer c.mu.Unlock()

		if c.connected && c.conn != nil {
			c.conn.Close(websocket.StatusNormalClosure, "client closing")
			c.conn = nil
		}
		c.connected = false
		close(c.done)
	})
}

// writePump handles outgoing messages from sendCh by writing them directly
// to the websocket. It must NOT call sendRaw, because sendRaw enqueues into
// sendCh — doing so would create an infinite re-enqueue loop where bytes
// never actually reach conn.Write.
func (c *Client) writePump() {
	for {
		select {
		case msg := <-c.sendCh:
			if msg == nil {
				return
			}
			c.mu.Lock()
			conn := c.conn
			connected := c.connected
			c.mu.Unlock()
			if !connected || conn == nil {
				return
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			err := conn.Write(ctx, websocket.MessageText, msg)
			cancel()
			if err != nil {
				return
			}
		case <-c.done:
			return
		}
	}
}

// sendJSON marshals and queues a message.
func (c *Client) sendJSON(msgType string, payload any) error {
	data, err := WrapMessage(msgType, payload)
	if err != nil {
		return err
	}
	return c.sendRaw(data)
}

// sendJSONRaw marshals and sends a message directly (for reconnect flow).
func (c *Client) sendJSONRaw(ctx context.Context, msgType string, payload any) error {
	data, err := WrapMessage(msgType, payload)
	if err != nil {
		return err
	}
	return c.conn.Write(ctx, websocket.MessageText, data)
}

// sendRaw sends raw bytes.
func (c *Client) sendRaw(data []byte) error {
	c.mu.Lock()
	connected := c.connected
	conn := c.conn
	c.mu.Unlock()

	if !connected || conn == nil {
		return fmt.Errorf("not connected")
	}

	select {
	case c.sendCh <- data:
		return nil
	case <-c.done:
		return fmt.Errorf("client closed")
	default:
		return ErrSendBufferFull
	}
}

// readEnvelope reads a JSON envelope from the connection.
func (c *Client) readEnvelope(ctx context.Context) (string, []byte, error) {
	c.mu.Lock()
	conn := c.conn
	c.mu.Unlock()

	if conn == nil {
		return "", nil, fmt.Errorf("no connection")
	}

	_, msg, err := conn.Read(ctx)
	if err != nil {
		return "", nil, err
	}

	var env Envelope
	if err := json.Unmarshal(msg, &env); err != nil {
		return "", nil, err
	}

	return env.Type, env.Payload, nil
}

// dispatchMessage routes incoming messages to the appropriate callback.
func (c *Client) dispatchMessage(msgType string, payload []byte) error {
	switch msgType {
	case MsgTypeGameStart:
		var p GameStartPayload
		if err := json.Unmarshal(payload, &p); err != nil {
			return fmt.Errorf("unmarshal game_start: %w", err)
		}
		if c.OnGameStart != nil {
			c.OnGameStart(&p)
		}
	case MsgTypeTick:
		var p TickPayload
		if err := json.Unmarshal(payload, &p); err != nil {
			return fmt.Errorf("unmarshal tick: %w", err)
		}
		c.CurrentTick = p.Tick
		if c.OnTick != nil {
			c.OnTick(&p)
		}
	case MsgTypeTickDelta:
		var p TickDeltaPayload
		if err := json.Unmarshal(payload, &p); err != nil {
			return fmt.Errorf("unmarshal tick_delta: %w", err)
		}
		c.CurrentTick = p.Tick
		if c.OnTickDelta != nil {
			c.OnTickDelta(&p)
		}
	case MsgTypeRoundOver:
		var p RoundOverPayload
		if err := json.Unmarshal(payload, &p); err != nil {
			return fmt.Errorf("unmarshal round_over: %w", err)
		}
		if c.OnRoundOver != nil {
			c.OnRoundOver(&p)
		}
	case MsgTypeGameOver:
		var p GameOverPayload
		if err := json.Unmarshal(payload, &p); err != nil {
			return fmt.Errorf("unmarshal game_over: %w", err)
		}
		if c.OnGameOver != nil {
			c.OnGameOver(&p)
		}
	case MsgTypeError:
		var p ErrorPayload
		if err := json.Unmarshal(payload, &p); err != nil {
			return fmt.Errorf("unmarshal error: %w", err)
		}
		if c.OnError != nil {
			c.OnError(p.Code, p.Message)
		}
	case MsgTypeOpponentLeft:
		var p OpponentLeftPayload
		if err := json.Unmarshal(payload, &p); err != nil {
			return fmt.Errorf("unmarshal opponent_left: %w", err)
		}
		if c.OnOpponentLeft != nil {
			c.OnOpponentLeft(p.Reason)
		}
	case MsgTypeJoined:
		var p JoinedPayload
		if err := json.Unmarshal(payload, &p); err != nil {
			return fmt.Errorf("unmarshal joined: %w", err)
		}
		if c.OnJoined != nil {
			c.OnJoined(&p)
		}
	case MsgTypePlayAgainAck:
		// Server-emitted post-game acknowledgment. The bot does not
		// participate in rematch flow (room is torn down at game end), so
		// this is a no-op. Without this case the default branch returns
		// an error, killing the WS read loop with noisy EOF logs.
	default:
		return fmt.Errorf("unknown message type: %s", msgType)
	}
	return nil
}

// handleReadError handles read errors and triggers reconnect if needed.
func (c *Client) handleReadError(ctx context.Context, err error) {
	// Check if it's a close error or connection issue
	if websocket.CloseStatus(err) != -1 {
		// Normal close, no reconnect
		c.Close()
		return
	}

	// For other errors, trigger reconnection logic
	// The caller decides whether to reconnect
}

// WrapMessage wraps a message payload into an envelope.
func WrapMessage(msgType string, payload any) ([]byte, error) {
	msg, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	env := Envelope{
		Type:    msgType,
		Payload: msg,
	}
	return json.Marshal(env)
}

// UnwrapMessage unwraps a byte slice into message type and payload.
func UnwrapMessage(data []byte) (string, []byte, error) {
	var env Envelope
	if err := json.Unmarshal(data, &env); err != nil {
		return "", nil, err
	}
	return env.Type, env.Payload, nil
}

// ReadEnvelope wraps the connection Read call.
func ReadEnvelope(ctx context.Context, conn *websocket.Conn) (string, []byte, error) {
	_, msg, err := conn.Read(ctx)
	if err != nil {
		return "", nil, err
	}

	var env Envelope
	if err := json.Unmarshal(msg, &env); err != nil {
		return "", nil, err
	}

	return env.Type, env.Payload, nil
}

// ReadAll wraps reading until EOF or error using io.ReadAll.
func ReadAll(ctx context.Context, conn *websocket.Conn) ([]byte, error) {
	_, r, err := conn.Reader(ctx)
	if err != nil {
		return nil, err
	}
	return io.ReadAll(r)
}
