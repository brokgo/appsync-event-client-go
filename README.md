# Appsync Event Client

## Installation

`go get github.com/brokgo/appsync-event-client-go`

## Usage

### Authentication

#### API Key

```go
httpEndpoint := "aabbccddee.appsync-api.ab-cdef-1.amazonaws.com"
realtimeEndpoint := "aabbccddee.appsync-realtime-api.ab-cdef-1.amazonaws.com"

// Config with API Authentication
apiKey := "ab-cdefghijklmnopqrstuvwxyz"
config, err := appsync.NewAPIKeyConfig(httpEndpoint, realtimeEndpoint, apiKey)
```

#### Lambda

```go
httpEndpoint := "aabbccddee.appsync-api.ab-cdef-1.amazonaws.com"
realtimeEndpoint := "aabbccddee.appsync-realtime-api.ab-cdef-1.amazonaws.com"

// Config with Lambda Authentication
authorizationToken := "ab-cdefghijklmnopqrstuvwxyz"
config, err := appsync.NewLambdaConfig(httpEndpoint, realtimeEndpoint, authorizationToken)
```

### Publish & Subscribe

#### Publish

```go
import (
	"encoding/json"

	"github.com/brokgo/appsync-event-client-go/appsync"
)

func main() {
	// Appsync endpoints
	httpEndpoint := "aabbccddee.appsync-api.ab-cdef-1.amazonaws.com"
	realtimeEndpoint := "aabbccddee.appsync-realtime-api.ab-cdef-1.amazonaws.com"

	apiKey := "ab-cdefghijklmnopqrstuvwxyz"
	channel := "/default/example"

	config, err := appsync.NewAPIKeyConfig(httpEndpoint, realtimeEndpoint, apiKey)
	if err != nil {
		panic(err)
	}
	client, err := appsync.DialWebScoketConfig(context.Background(), config)
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
	successIndicies, err := client.Publish(channel, events)
	if err != nil {
		panic(err)
	}
}

```

#### Subscribe

```go
import (
	"fmt"

	"github.com/brokgo/appsync-event-client-go/pkg/appsync"
)

func main() {
	// Appsync endpoints
	httpEndpoint := "aabbccddee.appsync-api.ab-cdef-1.amazonaws.com"
	realtimeEndpoint := "aabbccddee.appsync-realtime-api.ab-cdef-1.amazonaws.com"

	apiKey := "ab-cdefghijklmnopqrstuvwxyz"
	channel := "/default/example"

	config, err := appsync.NewAPIKeyConfig(httpEndpoint, realtimeEndpoint, apiKey)
	if err != nil {
		panic(err)
	}
	client, err := appsync.DialWebScoketConfig(context.Background(), config)
	if err != nil {
		panic(err)
	}
	chanC := make(chan *appsync.SubscriptionMessage, 10)
	err = client.Subscribe(channel, chanC)
	if err != nil {
		panic(err)
	}
	for {
		msg, ok := <-chanC
		if !ok {
			panic("channel closed")
		}
		switch msg.Type {
		case appsync.SubscriptionBroadcastErrorType:
			fmt.Println(msg.Errors)
		case appsync.SubscriptionDataType:
			fmt.Println(msg.Event)
		}
	}
}
```
