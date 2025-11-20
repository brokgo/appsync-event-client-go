// Package appsync is a libraty to connect to AWS's Appsync Event API. See https://docs.aws.amazon.com/appsync/.
package appsync

import (
	"context"
	"time"
)

const initTimeOut = 30 * time.Second

// DialWebSocketConfig creates a Appsync websocket client. For more information on the Appsync websocket API, see https://docs.aws.amazon.com/appsync/latest/eventapi/event-api-websocket-protocol.html.
func DialWebSocketConfig(ctx context.Context, config *Config) (*WebSocketClient, error) {
	httpURL, err := config.HTTPURL()
	if err != nil {
		return nil, err
	}
	realTimeURL, err := config.RealTimeURL()
	if err != nil {
		return nil, err
	}
	conn, err := newCoderWebSocketConn(ctx, httpURL, realTimeURL, config.Headers)
	if err != nil {
		return nil, err
	}
	timeoutCtx, timeoutCanel := context.WithTimeout(ctx, initTimeOut)
	defer timeoutCanel()

	return NewWebSocketClient(timeoutCtx, conn, config.Authorization)
}
