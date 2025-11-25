package appsync_test

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/brokgo/appsync-event-client-go/appsync"
	"github.com/brokgo/appsync-event-client-go/appsync/message"
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
	if clientMsg.Type != message.ConnectionInitType {
		panic("expected first call to be init")
	}
	if err != nil {
		panic(err)
	}
	err = server.Send(&message.ReceiveMessage{
		Type:                message.ConnectionAckType,
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
	if clientMsg.Type != message.PublishType {
		panic("expected publish call")
	}
	err = server.Send(&message.ReceiveMessage{
		Type: message.PublishSuccessType,
		ID:   clientMsg.ID,
		Successful: []message.ReceiveMessageEventID{
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

	// Client
	ctx := context.Background()
	client, err := appsync.DialWebSocketConfig(ctx, config)
	if err != nil {
		panic(err)
	}

	// JSON event(s) to publish
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

	// Publish
	channel := "/default/example"
	successIndicies, err := client.Publish(ctx, channel, events)
	if err != nil {
		panic(err)
	}

	// Results
	fmt.Println(successIndicies)

	// Close
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
	if clientMsg.Type != message.ConnectionInitType {
		panic("expected first call to be init")
	}
	if err != nil {
		panic(err)
	}
	err = server.Send(&message.ReceiveMessage{
		Type:                message.ConnectionAckType,
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
	if clientMsg.Type != message.SubscribeType {
		panic("expected subscribe call")
	}
	err = server.Send(&message.ReceiveMessage{
		Type: message.SubscribeSuccessType,
		ID:   clientMsg.ID,
	})
	if err != nil {
		panic(err)
	}
	// Event
	err = server.Send(&message.ReceiveMessage{
		Type:  message.SubscriptionDataType,
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

	// Client
	ctx := context.Background()
	client, err := appsync.DialWebSocketConfig(ctx, config)
	if err != nil {
		panic(err)
	}

	// Subscribe
	channel := "/default/example"
	msgC := make(chan *message.SubscriptionMessage)
	err = client.Subscribe(ctx, channel, msgC)
	if err != nil {
		panic(err)
	}

	// Result
	msg, ok := <-msgC
	if !ok {
		panic(client.Err)
	}
	fmt.Println(msg.Event)

	// Close
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
	if clientMsg.Type != message.ConnectionInitType {
		panic("expected first call to be init")
	}
	if err != nil {
		panic(err)
	}
	err = server.Send(&message.ReceiveMessage{
		Type:                message.ConnectionAckType,
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
	if clientMsg.Type != message.SubscribeType {
		panic("expected subscribe call")
	}
	err = server.Send(&message.ReceiveMessage{
		Type: message.SubscribeSuccessType,
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
	if clientMsg.Type != message.UnsubscribeType {
		panic("expected unsubscribe call")
	}
	err = server.Send(&message.ReceiveMessage{
		Type: message.UnsubscribeSuccessType,
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

	// Client
	ctx := context.Background()
	client, err := appsync.DialWebSocketConfig(ctx, config)
	if err != nil {
		panic(err)
	}

	// Subscribe
	channel := "/default/example"
	msgC := make(chan *message.SubscriptionMessage)
	err = client.Subscribe(ctx, channel, msgC)
	if err != nil {
		panic(err)
	}

	// Unsubscribe
	err = client.Unsubscribe(ctx, channel)
	if err != nil {
		panic(err)
	}

	fmt.Println("channel is unsubscribed")

	// Close
	err = client.Close()
	if err != nil {
		panic(err)
	}

	// Output: channel is unsubscribed
}
