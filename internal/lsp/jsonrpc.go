package lsp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
)

// JSON-RPC 2.0 over LSP base protocol (Content-Length framing).

// Request is a JSON-RPC 2.0 request.
type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      *int64          `json:"id,omitempty"` // nil for notifications
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// Response is a JSON-RPC 2.0 response.
type Response struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      *int64           `json:"id"`
	Result  json.RawMessage  `json:"result,omitempty"`
	Error   *ResponseError   `json:"error,omitempty"`
}

// ResponseError is a JSON-RPC 2.0 error.
type ResponseError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

func (e *ResponseError) Error() string {
	return fmt.Sprintf("LSP error %d: %s", e.Code, e.Message)
}

// Transport handles LSP base protocol framing over a reader/writer pair.
type Transport struct {
	reader *bufio.Reader
	writer io.Writer
	mu     sync.Mutex // protects writer
}

// NewTransport creates a new Transport.
func NewTransport(r io.Reader, w io.Writer) *Transport {
	return &Transport{
		reader: bufio.NewReaderSize(r, 64*1024),
		writer: w,
	}
}

// WriteMessage sends a JSON-RPC message with Content-Length framing.
func (t *Transport) WriteMessage(data []byte) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(data))
	if _, err := io.WriteString(t.writer, header); err != nil {
		return fmt.Errorf("write header: %w", err)
	}
	if _, err := t.writer.Write(data); err != nil {
		return fmt.Errorf("write body: %w", err)
	}
	return nil
}

// ReadMessage reads a JSON-RPC message with Content-Length framing.
func (t *Transport) ReadMessage() ([]byte, error) {
	contentLength := -1

	// Read headers
	for {
		line, err := t.reader.ReadString('\n')
		if err != nil {
			return nil, fmt.Errorf("read header: %w", err)
		}
		line = strings.TrimRight(line, "\r\n")

		if line == "" {
			// End of headers
			break
		}

		if strings.HasPrefix(line, "Content-Length: ") {
			val := strings.TrimPrefix(line, "Content-Length: ")
			contentLength, err = strconv.Atoi(val)
			if err != nil {
				return nil, fmt.Errorf("parse Content-Length %q: %w", val, err)
			}
		}
		// Ignore other headers (Content-Type, etc.)
	}

	if contentLength < 0 {
		return nil, fmt.Errorf("missing Content-Length header")
	}

	body := make([]byte, contentLength)
	if _, err := io.ReadFull(t.reader, body); err != nil {
		return nil, fmt.Errorf("read body (%d bytes): %w", contentLength, err)
	}

	return body, nil
}

// Conn manages JSON-RPC communication with pending request tracking.
type Conn struct {
	transport *Transport
	nextID    atomic.Int64
	pending   map[int64]chan *Response
	mu        sync.Mutex

	// NotificationHandler is called for server-initiated notifications.
	NotificationHandler func(method string, params json.RawMessage)

	done chan struct{}
}

// NewConn creates a new JSON-RPC connection.
func NewConn(t *Transport) *Conn {
	c := &Conn{
		transport: t,
		pending:   make(map[int64]chan *Response),
		done:      make(chan struct{}),
	}
	go c.readLoop()
	return c
}

// Call sends a request and waits for the response.
func (c *Conn) Call(method string, params interface{}) (json.RawMessage, error) {
	id := c.nextID.Add(1)

	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("marshal params: %w", err)
	}

	req := Request{
		JSONRPC: "2.0",
		ID:      &id,
		Method:  method,
		Params:  paramsJSON,
	}

	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	// Register pending request before sending
	ch := make(chan *Response, 1)
	c.mu.Lock()
	c.pending[id] = ch
	c.mu.Unlock()

	if err := c.transport.WriteMessage(data); err != nil {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return nil, fmt.Errorf("write request: %w", err)
	}

	// Wait for response
	select {
	case resp := <-ch:
		if resp.Error != nil {
			return nil, resp.Error
		}
		return resp.Result, nil
	case <-c.done:
		return nil, fmt.Errorf("connection closed")
	}
}

// Notify sends a notification (no response expected).
func (c *Conn) Notify(method string, params interface{}) error {
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return fmt.Errorf("marshal params: %w", err)
	}

	req := Request{
		JSONRPC: "2.0",
		Method:  method,
		Params:  paramsJSON,
	}

	data, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal notification: %w", err)
	}

	return c.transport.WriteMessage(data)
}

// Close signals the read loop to stop.
func (c *Conn) Close() {
	select {
	case <-c.done:
	default:
		close(c.done)
	}
}

func (c *Conn) readLoop() {
	for {
		select {
		case <-c.done:
			return
		default:
		}

		data, err := c.transport.ReadMessage()
		if err != nil {
			// Connection closed or error — signal all pending requests.
			c.Close()
			return
		}

		// Try to parse as a response (has "id" field)
		var msg struct {
			ID     *int64          `json:"id"`
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		if err := json.Unmarshal(data, &msg); err != nil {
			continue
		}

		if msg.Method != "" && msg.ID == nil {
			// Server notification (e.g., textDocument/publishDiagnostics)
			if c.NotificationHandler != nil {
				c.NotificationHandler(msg.Method, msg.Params)
			}
			continue
		}

		if msg.ID != nil {
			var resp Response
			if err := json.Unmarshal(data, &resp); err != nil {
				continue
			}

			c.mu.Lock()
			ch, ok := c.pending[*msg.ID]
			if ok {
				delete(c.pending, *msg.ID)
			}
			c.mu.Unlock()

			if ok {
				ch <- &resp
			}
		}
	}
}
