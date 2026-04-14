package client

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/coder/websocket"
)

// Network manages a WebSocket connection to the TANK! server.
// It uses goroutines for reading/writing and channels to bridge
// to the Ebitengine game loop.
type Network struct {
	conn       *websocket.Conn
	Incoming   chan []byte // Raw JSON messages from server (buffered 256)
	Outgoing   chan []byte // Raw JSON messages to server (buffered 256)
	Connected  bool
	cancel     context.CancelFunc
	done       chan struct{}
	mu         sync.Mutex // Protects conn.Write
	RetryCount int
	MaxRetries int
}

// NewNetwork connects to the server at serverURL.
// Returns a Network with read/write goroutines already running.
func NewNetwork(ctx context.Context, serverURL string) (*Network, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, serverURL, nil)
	if err != nil {
		return nil, err
	}
	// Increase read limit for game state messages (grid is ~840 cells)
	conn.SetReadLimit(1 << 20) // 1MB

	n := &Network{
		conn:       conn,
		Incoming:   make(chan []byte, 256),
		Outgoing:   make(chan []byte, 256),
		Connected:  true,
		done:       make(chan struct{}),
		MaxRetries: 3,
	}

	go n.readLoop()
	go n.writeLoop()

	return n, nil
}

// ConnectWithRetry attempts to establish a WebSocket connection with exponential backoff.
// It will retry up to MaxRetries times with backoff intervals of 1s, 2s, 4s, etc.
// Returns nil on success, or an error if all attempts fail.
func (n *Network) ConnectWithRetry(serverURL string) error {
	n.MaxRetries = 3
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	for attempt := 0; attempt <= n.MaxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff: 1s, 2s, 4s
			backoff := time.Duration(1<<(attempt-1)) * time.Second
			time.Sleep(backoff)
		}

		n.mu.Lock()
		n.RetryCount = attempt
		n.mu.Unlock()

		conn, _, err := websocket.Dial(ctx, serverURL, nil)
		if err == nil {
			// Success - update internal state
			n.mu.Lock()
			n.conn = conn
			n.conn.SetReadLimit(1 << 20)
			n.Connected = true
			n.RetryCount = 0
			n.mu.Unlock()

			go n.readLoop()
			go n.writeLoop()
			return nil
		}

		if attempt == n.MaxRetries {
			return fmt.Errorf("failed to connect after %d attempts: %w", n.MaxRetries, err)
		}
	}

	return fmt.Errorf("failed to connect after %d attempts", n.MaxRetries)
}

func (n *Network) Send(msgType string, payload any) bool {
	data, err := WrapMessage(msgType, payload)
	if err != nil {
		return false
	}
	select {
	case n.Outgoing <- data:
		return true
	default:
		return false
	}
}

// Close gracefully shuts down the connection.
func (n *Network) Close() {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.Connected {
		n.Connected = false
		if n.cancel != nil {
			n.cancel()
		}
		if n.conn != nil {
			_ = n.conn.Close(websocket.StatusNormalClosure, "client disconnecting")
		}
	}
}

// DrainIncoming drains all pending messages from the incoming channel.
// Call this from the game Update() loop — non-blocking.
func (n *Network) DrainIncoming() [][]byte {
	var msgs [][]byte
	for {
		select {
		case msg := <-n.Incoming:
			msgs = append(msgs, msg)
		default:
			return msgs
		}
	}
}

func (n *Network) readLoop() {
	defer close(n.done)
	defer func() {
		n.mu.Lock()
		n.Connected = false
		n.mu.Unlock()
	}()

	for {
		_, data, err := n.conn.Read(context.Background())
		if err != nil {
			log.Printf("network read error: %v", err)
			return
		}

		// Wrap message processing in panic recovery
		func() {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("panic in readLoop: %v", r)
				}
			}()

			_, _, err := UnwrapMessage(data)
			if err != nil {
				log.Printf("invalid message format: %v", err)
				return
			}
			// Send full envelope to Incoming so waitForMsg can properly match types
			n.Incoming <- data

		}()
	}
}

func (n *Network) writeLoop() {
	for data := range n.Outgoing {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		n.mu.Lock()
		err := n.conn.Write(ctx, websocket.MessageText, data)
		n.mu.Unlock()
		cancel()
		if err != nil {
			log.Printf("network write error: %v", err)
			return
		}
	}
}
