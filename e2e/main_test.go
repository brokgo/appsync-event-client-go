package e2e_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/brokgo/appsync-event-client-go/appsync"
	"github.com/brokgo/appsync-event-client-go/appsync/message"
)

type InfrastructureData struct {
	APIKey       string `json:"api_key"`
	AppsyncEvent struct {
		HTTP     string `json:"HTTP"`
		REALTIME string `json:"REALTIME"`
	} `json:"appsync_event"`
}

func TestPubSub(t *testing.T) {
	t.Parallel()
	infDataBytes, err := os.ReadFile(filepath.Join("infrastructure", "out", "data.json"))
	if err != nil {
		t.Fatal(err)
	}
	infrastructureData := &InfrastructureData{}
	err = json.Unmarshal(infDataBytes, infrastructureData)
	if err != nil {
		t.Fatal(err)
	}
	config := appsync.NewAPIKeyConfig(infrastructureData.AppsyncEvent.HTTP, infrastructureData.AppsyncEvent.REALTIME, infrastructureData.APIKey)
	ctx := t.Context()
	client, err := appsync.DialWebSocketConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	channel := "/test-pubsub/a"
	chanC := make(chan *message.SubscriptionMessage, 10)
	err = client.Subscribe(ctx, channel, chanC)
	if err != nil {
		t.Fatal(err)
	}
	eventABytes, err := json.Marshal(struct {
		A string `json:"a"`
	}{
		A: "123",
	})
	if err != nil {
		t.Fatal(err)
	}
	eventBBytes, err := json.Marshal(struct {
		B string `json:"b"`
	}{
		B: "123",
	})
	if err != nil {
		t.Fatal(err)
	}
	events := []string{string(eventABytes), string(eventBBytes)}
	successIndicies, err := client.Publish(ctx, channel, events)
	if err != nil {
		t.Fatal(err)
	}
	if len(successIndicies) != len(events) {
		t.Fatalf("not all events succeeded. indicies %v succeeded, expected all %v to succeeded", successIndicies, len(events))
	}
	chanEvents := []string{}
	for len(chanEvents) < 2 {
		var resp *message.SubscriptionMessage
		var ok bool
		select {
		case <-time.After(30 * time.Second):
			t.Fatal("timed out while waiting for subscription event")
		case resp, ok = <-chanC:
		}
		if !ok {
			t.Fatalf("subscription chanel is closed: %v", client.Err)
		}
		if resp.Type != message.SubscriptionDataType {
			t.Fatalf("subscription data returned %v type", resp.Type)
		}
		if len(resp.Errors) > 0 {
			t.Fatalf("errors in subscription data: %v", resp.Errors)
		}
		chanEvents = append(chanEvents, resp.Event)
	}
	slices.Sort(events)
	slices.Sort(chanEvents)
	if !slices.Equal(events, chanEvents) {
		t.Fatalf("events sent(%v) do not match events received(%v)", events, chanEvents)
	}
	err = client.Unsubscribe(ctx, channel)
	if err != nil {
		t.Fatal(err)
	}
	err = client.Close()
	if err != nil {
		t.Fatal(err)
	}
}
