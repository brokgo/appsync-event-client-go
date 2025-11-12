package appsync_test

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/brokgo/appsync-event-client-go/appsync"
)

func runPublishAPI(port string) {
	// Start server
	server, err := newServer(port)
	if err != nil {
		panic(err)
	}
	defer server.Shutdown(context.Background())
	// Init
	clientMsg, err := server.Receive()
	if clientMsg.Type != appsync.ConnectionInitType {
		panic("expected first call to be init")
	}
	if err != nil {
		panic(err)
	}
	err = server.Send(&appsync.ReceiveMessage{
		Type:                appsync.ConnectionAckType,
		ConnectionTimeoutMs: 30000,
	})
	if err != nil {
		panic(err)
	}
	// Publish
	clientMsg, err = server.Receive()
	if err != nil {
		panic(err)
	}
	if clientMsg.Type != appsync.PublishType {
		panic("expected publish call")
	}
	err = server.Send(&appsync.ReceiveMessage{
		Type: appsync.PublishSuccessType,
		ID:   clientMsg.ID,
		Successful: []appsync.ReceiveMessageEventID{
			{Identifier: "abc-def-ghi", Index: 0},
			{Identifier: "jkl-mno-pqr", Index: 1},
		},
	})
	if err != nil {
		panic(err)
	}
}

func ExampleWebSocketClient_Publish() {
	// Local websocket server to mimic Appsync
	go runPublishAPI("8001")

	// Appsync endpoints
	httpEndpoint := "localhost:8001"     // <ID>.appsync-api.<region>.amazonaws.com
	realtimeEndpoint := "localhost:8001" // <ID>.appsync-realtime-api.<region>.amazonaws.com

	// Authentication method
	apiKey := "ab-cdefghijklmnopqrstuvwxyz"
	config := appsync.NewAPIKeyConfig(httpEndpoint, realtimeEndpoint, apiKey)

	// Use non encypted connection for local calls only
	config.HTTPProtocol = "http"
	config.WebSocketProtocol = "ws"

	ctx := context.Background()
	client, err := appsync.DialWebSocketConfig(ctx, config)
	if err != nil {
		panic(err)
	}
	eventABytes, err := json.Marshal(struct {
		A string `json:"a"`
	}{
		A: "123",
	})
	if err != nil {
		panic(err)
	}
	eventBBytes, err := json.Marshal(struct {
		B string `json:"b"`
	}{
		B: "123",
	})
	if err != nil {
		panic(err)
	}
	events := []string{string(eventABytes), string(eventBBytes)}
	channel := "/default/example"
	successIndicies, err := client.Publish(ctx, channel, events)
	if err != nil {
		panic(err)
	}
	fmt.Println(successIndicies)
	err = client.Close()
	if err != nil {
		panic(err)
	}
	// Output: [0 1]
}

func runSubscribeAPI(port string) {
	// Start server
	server, err := newServer(port)
	if err != nil {
		panic(err)
	}
	defer server.Shutdown(context.Background())
	// Init
	clientMsg, err := server.Receive()
	if clientMsg.Type != appsync.ConnectionInitType {
		panic("expected first call to be init")
	}
	if err != nil {
		panic(err)
	}
	err = server.Send(&appsync.ReceiveMessage{
		Type:                appsync.ConnectionAckType,
		ConnectionTimeoutMs: 30000,
	})
	if err != nil {
		panic(err)
	}
	// Subscribe
	clientMsg, err = server.Receive()
	if err != nil {
		panic(err)
	}
	if clientMsg.Type != appsync.SubscribeType {
		panic("expected subscribe call")
	}
	err = server.Send(&appsync.ReceiveMessage{
		Type: appsync.SubscribeSuccessType,
		ID:   clientMsg.ID,
	})
	if err != nil {
		panic(err)
	}
	// Event
	err = server.Send(&appsync.ReceiveMessage{
		Type:  appsync.SubscriptionDataType,
		ID:    clientMsg.ID,
		Event: "eventa",
	})
	if err != nil {
		panic(err)
	}
}

func ExampleWebSocketClient_Subscribe() {
	// Local websocket server to mimic Appsync
	go runSubscribeAPI("8001")

	// Appsync endpoints
	httpEndpoint := "localhost:8001"     // <ID>.appsync-api.<region>.amazonaws.com
	realtimeEndpoint := "localhost:8001" // <ID>.appsync-realtime-api.<region>.amazonaws.com

	// Authentication method
	apiKey := "ab-cdefghijklmnopqrstuvwxyz"
	config := appsync.NewAPIKeyConfig(httpEndpoint, realtimeEndpoint, apiKey)

	// Use non encypted connection for local calls only
	config.HTTPProtocol = "http"
	config.WebSocketProtocol = "ws"

	ctx := context.Background()
	client, err := appsync.DialWebSocketConfig(ctx, config)
	if err != nil {
		panic(err)
	}
	channel := "/default/example"
	msgC := make(chan *appsync.SubscriptionMessage)
	err = client.Subscribe(ctx, channel, msgC)
	if err != nil {
		panic(err)
	}
	msg := <-msgC
	if msg == nil {
		panic(client.Err)
	}
	fmt.Println(msg.Event)
	err = client.Close()
	if err != nil {
		panic(err)
	}
	// Output: eventa
}

func runUnsubscribeAPI(port string) {
	// Start server
	server, err := newServer(port)
	if err != nil {
		panic(err)
	}
	defer server.Shutdown(context.Background())
	// Init
	clientMsg, err := server.Receive()
	if clientMsg.Type != appsync.ConnectionInitType {
		panic("expected first call to be init")
	}
	if err != nil {
		panic(err)
	}
	err = server.Send(&appsync.ReceiveMessage{
		Type:                appsync.ConnectionAckType,
		ConnectionTimeoutMs: 30000,
	})
	if err != nil {
		panic(err)
	}
	// Subscribe
	clientMsg, err = server.Receive()
	if err != nil {
		panic(err)
	}
	if clientMsg.Type != appsync.SubscribeType {
		panic("expected subscribe call")
	}
	err = server.Send(&appsync.ReceiveMessage{
		Type: appsync.SubscribeSuccessType,
		ID:   clientMsg.ID,
	})
	if err != nil {
		panic(err)
	}
	// Unsubscribe
	clientMsg, err = server.Receive()
	if err != nil {
		panic(err)
	}
	if clientMsg.Type != appsync.UnsubscribeType {
		panic("expected unsubscribe call")
	}
	err = server.Send(&appsync.ReceiveMessage{
		Type: appsync.UnsubscribeSuccessType,
		ID:   clientMsg.ID,
	})
	if err != nil {
		panic(err)
	}
}

func ExampleWebSocketClient_Unsubscribe() {
	// Local websocket server to mimic Appsync
	go runUnsubscribeAPI("8001")

	// Appsync endpoints
	httpEndpoint := "localhost:8001"     // <ID>.appsync-api.<region>.amazonaws.com
	realtimeEndpoint := "localhost:8001" // <ID>.appsync-realtime-api.<region>.amazonaws.com

	// Authentication method
	apiKey := "ab-cdefghijklmnopqrstuvwxyz"
	config := appsync.NewAPIKeyConfig(httpEndpoint, realtimeEndpoint, apiKey)

	// Use non encypted connection for local calls only
	config.HTTPProtocol = "http"
	config.WebSocketProtocol = "ws"

	ctx := context.Background()
	client, err := appsync.DialWebSocketConfig(ctx, config)
	if err != nil {
		panic(err)
	}
	channel := "/default/example"
	msgC := make(chan *appsync.SubscriptionMessage)
	err = client.Subscribe(ctx, channel, msgC)
	if err != nil {
		panic(err)
	}
	err = client.Unsubscribe(ctx, channel)
	if err != nil {
		panic(err)
	}
	fmt.Println("channel is unsubscribed")
	err = client.Close()
	if err != nil {
		panic(err)
	}
	// Output: channel is unsubscribed
}
