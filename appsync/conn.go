package appsync

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"

	"github.com/coder/websocket"
)

type coderWebSocketConn struct {
	Conn         *websocket.Conn
	danglingRead []byte
}

func (c *coderWebSocketConn) Close() error {
	return c.Conn.Close(websocket.StatusNormalClosure, "")
}

func (c *coderWebSocketConn) Read(ctx context.Context, b []byte) (int, error) {
	var currentB []byte
	if len(c.danglingRead) != 0 {
		currentB = c.danglingRead
		c.danglingRead = []byte{}
	} else {
		msgFormat, readB, err := c.Conn.Read(ctx)
		if err != nil {
			return 0, err
		}
		if msgFormat != websocket.MessageText {
			return 0, ErrUnsupportedMsgFormat
		}
		currentB = readB
	}
	copyN := min(len(b), len(currentB))
	copy(b, currentB[:copyN])
	if len(currentB) == copyN {
		return copyN, io.EOF
	}
	c.danglingRead = currentB[copyN:]

	return copyN, nil
}

func (c *coderWebSocketConn) Write(ctx context.Context, b []byte) (int, error) {
	err := c.Conn.Write(ctx, websocket.MessageText, b)
	if err != nil {
		return 0, err
	}

	return len(b), nil
}

func newCoderWebSocketConn(ctx context.Context, httpURL, realTimeURL string, headers map[string]string) (*coderWebSocketConn, error) {
	jsonHeaders, err := json.Marshal(headers)
	if err != nil {
		return nil, err
	}
	dialOptions := &websocket.DialOptions{
		Host:         httpURL,
		Subprotocols: []string{"header-" + base64.RawURLEncoding.EncodeToString(jsonHeaders), "aws-appsync-event-ws"},
	}
	conn, _, err := websocket.Dial(ctx, realTimeURL, dialOptions)
	if err != nil {
		return nil, err
	}

	return &coderWebSocketConn{Conn: conn}, nil
}
