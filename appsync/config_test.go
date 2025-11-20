package appsync_test

import (
	"fmt"
	"slices"
	"testing"

	"github.com/brokgo/appsync-event-client-go/appsync"
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
			if !isSendMessageEqual(clientMsg, expectedClientMsg) {
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

func TestConfigHost(t *testing.T) {
	t.Parallel()
	testCases := map[string]*struct {
		Config *appsync.Config
		Host   string
	}{
		"apikey": {
			Config: appsync.NewAPIKeyConfig("apikey.localhost/http", "apikey.localhost/realtime", "apikeytest"),
			Host:   "https://apikey.localhost/http/event",
		},
		"lambda": {
			Config: appsync.NewLambdaConfig("lambda.localhost/http", "lambda.localhost/realtime", "tokentest"),
			Host:   "https://lambda.localhost/http/event",
		},
	}
	for testName, testParams := range testCases {
		t.Run(testName, func(t *testing.T) {
			t.Parallel()
			host, err := testParams.Config.Host()
			if err != nil {
				t.Fatal(err)
			}
			if host != testParams.Host {
				t.Fatalf("expected %v, got %v", testParams.Host, host)
			}
		})
	}
}

func TestConfigSubprotocols(t *testing.T) {
	t.Parallel()
	testCases := map[string]*struct {
		Config       *appsync.Config
		Subprotocols []string
	}{
		"apikey": {
			Config:       appsync.NewAPIKeyConfig("apikey.localhost/http", "apikey.localhost/realtime", "apikeytest"),
			Subprotocols: []string{"header-eyJob3N0IjoiYXBpa2V5LmxvY2FsaG9zdC9odHRwIiwieC1hcGkta2V5IjoiYXBpa2V5dGVzdCJ9", "aws-appsync-event-ws"},
		},
		"lambda": {
			Config:       appsync.NewLambdaConfig("lambda.localhost/http", "lambda.localhost/realtime", "tokentest"),
			Subprotocols: []string{"header-eyJhdXRob3JpemF0aW9uIjoidG9rZW50ZXN0IiwiaG9zdCI6ImxhbWJkYS5sb2NhbGhvc3QvaHR0cCJ9", "aws-appsync-event-ws"},
		},
	}
	for testName, testParams := range testCases {
		t.Run(testName, func(t *testing.T) {
			t.Parallel()
			subprotocols, err := testParams.Config.Subprotocols()
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(subprotocols, testParams.Subprotocols) {
				t.Fatalf("expected %v, got %v", testParams.Subprotocols, subprotocols)
			}
		})
	}
}

func TestConfigURL(t *testing.T) {
	t.Parallel()
	testCases := map[string]*struct {
		Config *appsync.Config
		URL    string
	}{
		"apikey": {
			Config: appsync.NewAPIKeyConfig("apikey.localhost/http", "apikey.localhost/realtime", "apikeytest"),
			URL:    "wss://apikey.localhost/realtime/event/realtime",
		},
		"lambda": {
			Config: appsync.NewLambdaConfig("lambda.localhost/http", "lambda.localhost/realtime", "tokentest"),
			URL:    "wss://lambda.localhost/realtime/event/realtime",
		},
	}
	for testName, testParams := range testCases {
		t.Run(testName, func(t *testing.T) {
			t.Parallel()
			url, err := testParams.Config.URL()
			if err != nil {
				t.Fatal(err)
			}
			if url != testParams.URL {
				t.Fatalf("expected %v, got %v", testParams.URL, url)
			}
		})
	}
}
