# Appsync Event Client [![Build Status][github-actions-image]][github-actions-url] [![GoDoc][godoc-image]][documentation]
Appsync event client is a go websocket client library for connecting with the [AWS Appsync Event API].

## Install

`go get github.com/brokgo/appsync-event-client-go/appsync`

## Quick Start

### Publish

The following examples shows how to publish an event to the Appsync Event using an api key. Replace `httpEndpoint`, `realtimeEndpoint`, and `apiKey` with your own values.

```go
import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/brokgo/appsync-event-client-go/appsync"
)

// Appsync endpoints
httpEndpoint := "<ID>.appsync-api.<region>.amazonaws.com"
realtimeEndpoint := "<ID>.appsync-realtime-api.<region>.amazonaws.com"

// Authentication method
apiKey := "ab-cdefghijklmnopqrstuvwxyz"
config := appsync.NewAPIKeyConfig(httpEndpoint, realtimeEndpoint, apiKey)

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
```

### Subscription

The following examples shows how to subscribe to an Appsync Event channel using an api key. Replace `httpEndpoint`, `realtimeEndpoint`, and `apiKey` with your own values.

```go
import (
	"context"
	"fmt"

	"github.com/brokgo/appsync-event-client-go/appsync"
    "github.com/brokgo/appsync-event-client-go/appsync/message"
)

// Appsync endpoints
httpEndpoint := "<ID>.appsync-api.<region>.amazonaws.com"
realtimeEndpoint := "<ID>.appsync-realtime-api.<region>.amazonaws.com"

// Authentication method
apiKey := "ab-cdefghijklmnopqrstuvwxyz"
config := appsync.NewAPIKeyConfig(httpEndpoint, realtimeEndpoint, apiKey)

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
```

## Documentation
See [documentation] to see additional examples and the API reference for the library.

## Websocket Appsync Event API
![Websocket Appsync Event API Flow](https://docs.aws.amazon.com/images/appsync/latest/eventapi/images/WebSocket-protocol.png)
Image from https://docs.aws.amazon.com/appsync/latest/eventapi/event-api-websocket-protocol.html.


[AWS Appsync Event API]: https://docs.aws.amazon.com/appsync/latest/eventapi/event-api-welcome.html
[documentation]: https://pkg.go.dev/github.com/brokgo/appsync-event-client-go/appsync
[github-actions-image]: https://github.com/brokgo/appsync-event-client-go/actions/workflows/ci.yml/badge.svg?branch=main
[github-actions-url]: https://github.com/brokgo/appsync-event-client-go/actions
[godoc-image]: https://pkg.go.dev/badge/github.com/brokgo/appsync-event-client-go/appsync
