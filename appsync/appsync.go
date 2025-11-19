// Package appsync is a libraty to connect to AWS's Appsync Event API. See https://docs.aws.amazon.com/appsync/.
package appsync

import (
	"context"
	"fmt"
	"net/url"
	"sync"
	"time"
)

const initTimeOut = 30 * time.Second

// DialWebSocketConfig creates a Appsync websocket client. For more information on the Appsync websocket API, see https://docs.aws.amazon.com/appsync/latest/eventapi/event-api-websocket-protocol.html.
func DialWebSocketConfig(ctx context.Context, config *Config) (*WebSocketClient, error) {
	httpURL, err := url.JoinPath(fmt.Sprintf("%v://", config.HTTPProtocol), config.HTTPEndpoint, "/event")
	if err != nil {
		return nil, err
	}
	realTimeURL, err := url.JoinPath(fmt.Sprintf("%v://", config.WebSocketProtocol), config.RealTimeEndpoint, "/event/realtime")
	if err != nil {
		return nil, err
	}
	conn, err := newCoderWebSocketConn(ctx, httpURL, realTimeURL, config.Headers)
	if err != nil {
		return nil, err
	}
	err = write(ctx, conn, &SendMessage{
		Type: ConnectionInitType,
	})
	if err != nil {
		return nil, err
	}
	timeoutCtx, timeoutCanel := context.WithTimeout(ctx, initTimeOut)
	defer timeoutCanel()
	initMsg := &ReceiveMessage{}
	err = read(timeoutCtx, conn, initMsg)
	if err != nil {
		return nil, err
	}
	if len(initMsg.Errors) > 0 {
		return nil, errFromMsgErrors(initMsg.Errors)
	}
	client := &WebSocketClient{
		Authorization:           config.Authorization,
		Conn:                    conn,
		done:                    make(chan struct{}),
		linkByID:                sync.Map{},
		subscriptionByID:        sync.Map{},
		subscriptionIDByChannel: sync.Map{},
		wg:                      sync.WaitGroup{},
	}
	var once sync.Once
	cancel := func(err error) {
		once.Do(func() {
			client.Err = err
			go client.Close() //nolint: errcheck
		})
	}
	keepAliveC := make(chan struct{}, 1)
	client.goHandleRead(cancel, keepAliveC) //nolint:contextcheck
	client.goHandleTimeOut(cancel, time.Duration(initMsg.ConnectionTimeoutMs)*time.Millisecond, keepAliveC)

	return client, nil
}
