package appsync

import (
	"context"
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

func newCoderWebSocketConn(ctx context.Context, host, url string, subprotocols []string) (*coderWebSocketConn, error) {
	dialOptions := &websocket.DialOptions{
		Host:         host,
		Subprotocols: subprotocols,
	}
	conn, _, err := websocket.Dial(ctx, url, dialOptions)
	if err != nil {
		return nil, err
	}

	return &coderWebSocketConn{Conn: conn}, nil
}
