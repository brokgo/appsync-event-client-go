// Package appsync is a libraty to connect to AWS's Appsync Event API. See https://docs.aws.amazon.com/appsync/.
package appsync

import (
	"context"
	"time"
)

const initTimeOut = 30 * time.Second

// DialWebSocketConfig creates a Appsync websocket client. For more information on the Appsync websocket API, see https://docs.aws.amazon.com/appsync/latest/eventapi/event-api-websocket-protocol.html.
func DialWebSocketConfig(ctx context.Context, config *Config) (*WebSocketClient, error) {
	// Dial conn
	host, err := config.Host()
	if err != nil {
		return nil, err
	}
	url, err := config.URL()
	if err != nil {
		return nil, err
	}
	subprotocols, err := config.Subprotocols()
	if err != nil {
		return nil, err
	}
	conn, err := dialCoderWebSocket(ctx, host, url, subprotocols)
	if err != nil {
		return nil, err
	}
	// Create timeout when initializing connection.
	defer timeoutCanel()

	return NewWebSocketClient(timeoutCtx, conn, config.Authorization)
}
