// Package appsync is a libraty to connect to AWS's Appsync (https://docs.aws.amazon.com/appsync/)
package appsync

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"sync"
	"time"

	"github.com/coder/websocket"
)

const initTimeOut = 30 * time.Second

// DialWebSocketConfig is used for creating a Appsync web socet. For more information, see https://docs.aws.amazon.com/appsync/latest/eventapi/event-api-websocket-protocol.html
func DialWebSocketConfig(ctx context.Context, config *Config) (*WebSocketClient, error) {
	jsonHeaders, err := json.Marshal(config.Headers)
	if err != nil {
		return nil, err
	}
	dialOptions := &websocket.DialOptions{
		Host:         config.HTTPURL,
		Subprotocols: []string{"header-" + base64.RawURLEncoding.EncodeToString(jsonHeaders), "aws-appsync-event-ws"},
	}
	conn, _, err := websocket.Dial(ctx, config.RealTimeURL, dialOptions)
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
		authorization:           config.Authorization,
		conn:                    conn,
		done:                    make(chan struct{}),
		linkByID:                map[string]chan *ReceiveMessage{},
		subscriptionIDByChannel: map[string]string{},
		subscriptionLinkByID:    map[string]chan *SubscriptionMessage{},
		timeoutDuration:         time.Duration(initMsg.ConnectionTimeoutMs) * time.Millisecond,
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
	client.goHandleRead(ctx, cancel, keepAliveC)
	client.goHandleTimeOut(cancel, keepAliveC)

	return client, nil
}
