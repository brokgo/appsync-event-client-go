package appsync_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/brokgo/appsync-event-client-go/pkg/appsync"
	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

func TestPublish(t *testing.T) {
	t.Parallel()
	errC, clientMsgC, serverMsgC := buildServer(t, context.Background(), ":8080")
	client := dial(t, errC, clientMsgC, serverMsgC)
	channel := "test/publish"
	events := []string{"eventa", "eventb"}
	doneC := make(chan struct{})
	go func() {
		successfullIs, err := client.Publish(context.Background(), channel, events)
		if err != nil {
			errC <- err
		}
		if len(successfullIs) != len(events) {
			errC <- fmt.Errorf("%v events succeeded out of %v", len(successfullIs), len(events))
		}
		close(doneC)
	}()
	var clientMsg *appsync.SendMessage
	select {
	case err := <-errC:
		t.Fatal(err)
	case clientMsg = <-clientMsgC:
	}
	publishID := clientMsg.ID
	expectedClientMsg := &appsync.SendMessage{
		Type:    appsync.PublishType,
		Channel: channel,
		ID:      publishID,
		Events:  events,
	}
	if !clientMsg.Equal(expectedClientMsg) {
		t.Fatalf("unexpected publish data: %v", clientMsg)
	}
	serverMsg := &appsync.ReceiveMessage{
		Type:       appsync.SubscribeSuccessType,
		ID:         publishID,
		Successful: []appsync.ReceiveMessageEventID{{Identifier: "abc", Index: 0}, {Identifier: "def", Index: 1}},
	}
	select {
	case err := <-errC:
		t.Fatal(err)
	case serverMsgC <- serverMsg:
	}
	select {
	case err := <-errC:
		t.Fatal(err)
	case <-doneC:
	}
}

func buildServer(t *testing.T, ctx context.Context, host string) (chan error, chan *appsync.SendMessage, chan *appsync.ReceiveMessage) {
	t.Helper()
	errC := make(chan error)
	clientMsgC := make(chan *appsync.SendMessage)
	serverMsgC := make(chan *appsync.ReceiveMessage)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			t.Fatal(err)
		}
		go func() {
			defer conn.CloseNow() //nolint: errcheck
			for {
				clientMsg := &appsync.SendMessage{}
				err = wsjson.Read(ctx, conn, clientMsg)
				if err != nil {
					errC <- err

					return
				}
				clientMsgC <- clientMsg
			}
		}()
		go func() {
			defer conn.CloseNow() //nolint: errcheck
			for {
				serverMsg := <-serverMsgC
				err = wsjson.Write(ctx, conn, serverMsg)
				if err != nil {
					errC <- err

					return
				}
			}
		}()
	})
	go func() {
		serv := http.Server{ // #nosec G112
			Addr:    host,
			Handler: handler,
		}
		err := serv.ListenAndServe()
		if err != nil {
			errC <- err
		}
	}()
	select {
	case err := <-errC:
		t.Fatal(errors.Join(errors.New("failed to create server"), err))
	default:
	}

	return errC, clientMsgC, serverMsgC
}

func dial(t *testing.T, errC chan error, clientMsgC chan *appsync.SendMessage, serverMsgC chan *appsync.ReceiveMessage) *appsync.WebSocketClient {
	t.Helper()
	clientC := make(chan *appsync.WebSocketClient)
	go func() {
		client, err := appsync.DialWebSocketConfig(context.Background(), &appsync.Config{
			HTTPURL:     "http://localhost:8080/event",
			RealTimeURL: "ws://localhost:8080/event/realtime",
		})
		if err != nil {
			errC <- err
		}
		clientC <- client
	}()
	var clientMsg *appsync.SendMessage
	select {
	case err := <-errC:
		t.Fatal(errors.Join(errors.New("failed getting client msg during init"), err))
	case clientMsg = <-clientMsgC:
	}
	expectedClientMsg := &appsync.SendMessage{
		Type: appsync.ConnectionInitType,
	}
	if !clientMsg.Equal(expectedClientMsg) {
		t.Fatalf("unexpected init data: %v", clientMsg)
	}
	serverMsg := &appsync.ReceiveMessage{
		Type:                appsync.ConnectionAckType,
		ConnectionTimeoutMs: 30000, // 30 Seconds
	}
	select {
	case err := <-errC:
		t.Fatal(errors.Join(errors.New("failed sending client msg during init"), err))
	case serverMsgC <- serverMsg:
	}
	var client *appsync.WebSocketClient
	select {
	case err := <-errC:
		t.Fatal(errors.Join(errors.New("failed to create client"), err))
	case client = <-clientC:
	}

	return client
}
