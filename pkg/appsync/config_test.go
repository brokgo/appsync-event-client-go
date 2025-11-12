package appsync_test

import (
	"fmt"
	"testing"

	"github.com/brokgo/appsync-event-client-go/pkg/appsync"
)

func TestConfig(t *testing.T) {
	t.Parallel()
	testCases := map[string]*struct {
		Address string
		Port    string
	}{"apikey": {}, "lambda": {}}
	for _, testParams := range testCases {
		testParams.Port = portPool.Get()
		testParams.Address = fmt.Sprintf("localhost:%v", testParams.Port)
	}
	testConfigs := map[string]*appsync.Config{
		"apikey": appsync.NewAPIKeyConfig(testCases["apikey"].Address, testCases["apikey"].Address, "apikeytest"),
		"lambda": appsync.NewLambdaConfig(testCases["lambda"].Address, testCases["lambda"].Address, "tokentest"),
	}
	for testName, testParams := range testCases {
		t.Run(testName, func(t *testing.T) {
			t.Parallel()
			server, err := newServer(testParams.Port)
			if err != nil {
				t.Fatal(err)
			}
			defer server.Shutdown(t.Context())
			config := testConfigs[testName]
			config.HTTPProtocol = "http"
			config.WebSocketProtocol = "ws"
			client, err := server.DialConfig(config, defaultTimeout)
			if err != nil {
				t.Fatal(err)
			}
			channel := "test/publish"
			events := []string{"eventa", "eventb"}
			errC := make(chan error)
			go func() {
				successfullIs, err := client.Publish(t.Context(), channel, events)
				if err != nil {
					errC <- err

					return
				}
				if len(successfullIs) != len(events) {
					errC <- fmt.Errorf("%v events succeeded out of %v", len(successfullIs), len(events))

					return
				}
				close(errC)
			}()
			clientMsg, err := server.Receive()
			if err != nil {
				t.Fatal()
			}
			publishID := clientMsg.ID
			expectedClientMsg := &appsync.SendMessage{
				Authorization: config.Authorization,
				Type:          appsync.PublishType,
				Channel:       channel,
				ID:            publishID,
				Events:        events,
			}
			if !clientMsg.Equal(expectedClientMsg) {
				t.Fatalf("unexpected publish data: %v", clientMsg)
			}
			err = server.Send(&appsync.ReceiveMessage{
				Type:       appsync.PublishSuccessType,
				ID:         publishID,
				Successful: []appsync.ReceiveMessageEventID{{Identifier: "abc", Index: 0}, {Identifier: "def", Index: 1}},
			})
			if err != nil {
				t.Fatal(err)
			}
			err, ok := <-errC
			if ok {
				t.Fatal(err)
			}
			err = client.Close()
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}
