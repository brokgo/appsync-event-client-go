package appsync_test

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/brokgo/appsync-event-client-go/appsync"
	"github.com/brokgo/appsync-event-client-go/appsync/message"
	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

type Pool[T any] struct {
	items []T
	mu    sync.Mutex
}

func (p *Pool[T]) Get() T {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.items) == 0 {
		panic("no items left in the pool")
	}
	port := p.items[0]
	p.items = p.items[1:]

	return port
}

func (p *Pool[T]) Put(item T) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.items = append(p.items, item)
}

type Server struct {
	clientC chan *message.SendMessage
	ErrC    chan error
	server  *http.Server
	serverC chan *message.ReceiveMessage
}

func (s *Server) DialConfig(config *appsync.Config, connectionTimeout time.Duration) (*appsync.WebSocketClient, error) {
	clientC := make(chan *appsync.WebSocketClient)
	go func() {
		client, err := appsync.DialWebSocketConfig(context.Background(), config)
		if err != nil {
			s.ErrC <- err
		}
		clientC <- client
	}()
	clientMsg, err := s.Receive()
	if err != nil {
		return nil, errors.Join(errors.New("failed getting client msg during init"), err)
	}
	expectedClientMsg := &message.SendMessage{
		Type: message.ConnectionInitType,
	}
	if !isSendMessageEqual(clientMsg, expectedClientMsg) {
		return nil, fmt.Errorf("unexpected init data: %v", clientMsg)
	}
	err = s.Send(&message.ReceiveMessage{
		Type:                message.ConnectionAckType,
		ConnectionTimeoutMs: int(connectionTimeout.Milliseconds()),
	})
	if err != nil {
		return nil, errors.Join(errors.New("failed sending client msg during init"), err)
	}
	select {
	case err := <-s.ErrC:
		return nil, errors.Join(errors.New("failed to create client"), err)
	case client := <-clientC:
		return client, nil
	}
}

func (s *Server) Receive() (*message.SendMessage, error) {
	select {
	case err := <-s.ErrC:
		return nil, err
	case msg := <-s.clientC:
		return msg, nil
	}
}

func (s *Server) Send(msg *message.ReceiveMessage) error {
	select {
	case err := <-s.ErrC:
		return err
	case s.serverC <- msg:
	}

	return nil
}

func (s *Server) Shutdown(ctx context.Context) {
	s.server.Shutdown(ctx) //nolint: errcheck
}

func TestKeepAlive(t *testing.T) {
	t.Parallel()
	serverPort := portPool.Get()
	server, err := newServer(serverPort)
	if err != nil {
		t.Fatal(err)
	}
	defer server.Shutdown(t.Context())
	address := fmt.Sprintf("localhost:%v", serverPort)
	config := appsync.NewAPIKeyConfig(address, address, "apikeytest")
	config.HTTPProtocol = "http"
	config.WebSocketProtocol = "ws"
	connectionTimeout := 10 * time.Millisecond
	client, err := server.DialConfig(config, connectionTimeout)
	if err != nil {
		t.Fatal(err)
	}
	for range 4 {
		err := server.Send(&message.ReceiveMessage{
			Type: message.KeepAliveType,
		})
		if err != nil {
			t.Fatal(err)
		}
		if client.Err != nil {
			t.Fatal(err)
		}
		<-time.After(connectionTimeout / 2)
	}
	if client.Err != nil {
		t.Fatal(err)
	}
	err = client.Close()
	if err != nil {
		t.Fatal(err)
	}
}

func TestPublish(t *testing.T) {
	t.Parallel()
	responseEvents := []message.ReceiveEvent{
		{Identifier: "abc", Index: 0},
		{Identifier: "def", Index: 1},
		{Identifier: "ghi", Index: 2},
		{Identifier: "jkl", Index: 3},
	}
	testCases := map[string]struct {
		ExpectedErr           error
		ExpectedSuccessfullIs []int
		ServerResponse        *message.ReceiveMessage
	}{
		"success": {
			ExpectedErr:           nil,
			ExpectedSuccessfullIs: []int{0, 1, 2, 3},
			ServerResponse: &message.ReceiveMessage{
				Type:       message.PublishSuccessType,
				Successful: responseEvents,
			},
		},
		"fail": {
			ExpectedErr:           appsync.ErrServerMsg,
			ExpectedSuccessfullIs: []int{},
			ServerResponse: &message.ReceiveMessage{
				Type:   message.PublishErrType,
				Errors: []message.ErrorData{{ErrorType: "errtest", Message: "errtestmsg"}},
			},
		},
		"half_success": {
			ExpectedErr:           nil,
			ExpectedSuccessfullIs: []int{0, 1},
			ServerResponse: &message.ReceiveMessage{
				Type:       message.PublishSuccessType,
				Successful: responseEvents[:2],
			},
		},
	}
	for testName, testParams := range testCases {
		t.Run(testName, func(t *testing.T) {
			t.Parallel()
			serverPort := portPool.Get()
			server, err := newServer(serverPort)
			if err != nil {
				t.Fatal(err)
			}
			defer server.Shutdown(t.Context())
			address := fmt.Sprintf("localhost:%v", serverPort)
			config := appsync.NewAPIKeyConfig(address, address, "apikeytest")
			config.HTTPProtocol = "http"
			config.WebSocketProtocol = "ws"
			client, err := server.DialConfig(config, defaultTimeout)
			if err != nil {
				t.Fatal(err)
			}
			channel := "test/publish"
			events := []string{"eventa", "eventb", "eventc", "eventd"}
			successfullIC := make(chan []int)
			errC := make(chan error)
			go func() {
				defer close(successfullIC)
				successfullIs, err := client.Publish(t.Context(), channel, events)
				if err != nil {
					errC <- err

					return
				}
				close(errC)
				successfullIC <- successfullIs
			}()
			clientMsg, err := server.Receive()
			if err != nil {
				t.Fatal()
			}
			publishID := clientMsg.ID
			expectedClientMsg := &message.SendMessage{
				Authorization: config.Authorization,
				Type:          message.PublishType,
				Channel:       channel,
				ID:            publishID,
				Events:        events,
			}
			if !isSendMessageEqual(clientMsg, expectedClientMsg) {
				t.Fatalf("unexpected publish data: %v", clientMsg)
			}
			testParams.ServerResponse.ID = publishID
			err = server.Send(testParams.ServerResponse)
			if err != nil {
				t.Fatal(err)
			}
			err = <-errC
			if !errors.Is(err, testParams.ExpectedErr) {
				t.Fatal(err)
			}
			successfullIs := <-successfullIC
			slices.Sort(successfullIs)
			slices.Sort(testParams.ExpectedSuccessfullIs)
			if !slices.Equal(successfullIs, testParams.ExpectedSuccessfullIs) {
				t.Fatalf("expected %v, got %v", testParams.ExpectedSuccessfullIs, successfullIs)
			}
			err = client.Close()
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestSubscribe(t *testing.T) {
	t.Parallel()
	testCases := map[string]struct {
		ExpectedErr    error
		ServerData     *message.ReceiveMessage
		ServerResponse *message.ReceiveMessage
	}{
		"success": {
			ExpectedErr: nil,
			ServerData: &message.ReceiveMessage{
				Type:  message.SubscriptionDataType,
				Event: "abc123",
			},
			ServerResponse: &message.ReceiveMessage{
				Type: message.SubscribeSuccessType,
			},
		},
		"fail": {
			ExpectedErr: appsync.ErrServerMsg,
			ServerResponse: &message.ReceiveMessage{
				Type:   message.SubscribeErrType,
				Errors: []message.ErrorData{{ErrorType: "errtest", Message: "errtestmsg"}},
			},
		},
		"data_error": {
			ExpectedErr: nil,
			ServerData: &message.ReceiveMessage{
				Type:  message.SubscriptionBroadcastErrorType,
				Event: "abc123",
			},
			ServerResponse: &message.ReceiveMessage{
				Type: message.SubscribeSuccessType,
			},
		},
	}
	for testName, testParams := range testCases {
		t.Run(testName, func(t *testing.T) {
			t.Parallel()
			serverPort := portPool.Get()
			server, err := newServer(serverPort)
			if err != nil {
				t.Fatal(err)
			}
			defer server.Shutdown(t.Context())
			address := fmt.Sprintf("localhost:%v", serverPort)
			config := appsync.NewAPIKeyConfig(address, address, "apikeytest")
			config.HTTPProtocol = "http"
			config.WebSocketProtocol = "ws"
			client, err := server.DialConfig(config, defaultTimeout)
			if err != nil {
				t.Fatal(err)
			}
			channel := "test/subscribe"
			channelC := make(chan *message.SubscriptionMessage)
			errC := make(chan error)
			go func() {
				defer close(errC)
				err := client.Subscribe(t.Context(), channel, channelC)
				if err != nil {
					errC <- err
				}
			}()
			clientMsg, err := server.Receive()
			if err != nil {
				t.Fatal(err)
			}
			publishID := clientMsg.ID
			expectedClientMsg := &message.SendMessage{
				Authorization: config.Authorization,
				Type:          message.SubscribeType,
				Channel:       channel,
				ID:            publishID,
			}
			if !isSendMessageEqual(clientMsg, expectedClientMsg) {
				t.Fatalf("unexpected subscribe data: %v", clientMsg)
			}
			testParams.ServerResponse.ID = publishID
			err = server.Send(testParams.ServerResponse)
			if err != nil {
				t.Fatal(err)
			}
			err = <-errC
			if !errors.Is(err, testParams.ExpectedErr) {
				t.Error(err)
			}
			if err != nil {
				client.Close() //nolint: errcheck

				return
			}
			testParams.ServerData.ID = publishID
			err = server.Send(testParams.ServerData)
			if err != nil {
				t.Fatal(err)
			}
			expectedMsg := &message.SubscriptionMessage{
				Type:  testParams.ServerData.Type,
				Event: testParams.ServerData.Event,
			}
			select {
			case err = <-server.ErrC:
				t.Fatal(err)
			case subscriptionMsg := <-channelC:
				if !isSubscriptionMessageEqual(subscriptionMsg, expectedMsg) {
					t.Fatalf("unexpected subscription data: %v", clientMsg)
				}
			}
			err = client.Close()
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestTimeout(t *testing.T) {
	t.Parallel()
	serverPort := portPool.Get()
	server, err := newServer(serverPort)
	if err != nil {
		t.Fatal(err)
	}
	defer server.Shutdown(t.Context())
	address := fmt.Sprintf("localhost:%v", serverPort)
	config := appsync.NewAPIKeyConfig(address, address, "apikeytest")
	config.HTTPProtocol = "http"
	config.WebSocketProtocol = "ws"
	connectionTimeout := 10 * time.Millisecond
	client, err := server.DialConfig(config, connectionTimeout)
	if err != nil {
		t.Fatal(err)
	}
	if client.Err != nil {
		t.Fatal(err)
	}
	<-time.After(connectionTimeout * 2)
	if !errors.Is(client.Err, appsync.ErrTimeout) {
		t.Fatalf("Expected timeout error, got %v", err)
	}
}

func TestUnsubscribe(t *testing.T) {
	t.Parallel()
	testCases := map[string]struct {
		ExpectedSubErr      error
		ExpectedUnsubErr    error
		ServerData          *message.ReceiveMessage
		ServerSubResponse   *message.ReceiveMessage
		ServerUnsubResponse *message.ReceiveMessage
	}{
		"success": {
			ExpectedSubErr:   nil,
			ExpectedUnsubErr: nil,
			ServerData: &message.ReceiveMessage{
				Type:  message.SubscriptionDataType,
				Event: "abc123",
			},
			ServerSubResponse: &message.ReceiveMessage{
				Type: message.SubscribeSuccessType,
			},
			ServerUnsubResponse: &message.ReceiveMessage{
				Type: message.UnsubscribeSuccessType,
			},
		},
		"fail": {
			ExpectedSubErr:   nil,
			ExpectedUnsubErr: appsync.ErrServerMsg,
			ServerData: &message.ReceiveMessage{
				Type:  message.SubscriptionDataType,
				Event: "abc123",
			},
			ServerSubResponse: &message.ReceiveMessage{
				Type: message.SubscribeSuccessType,
			},
			ServerUnsubResponse: &message.ReceiveMessage{
				Type:   message.UnsubscribeErrType,
				Errors: []message.ErrorData{{ErrorType: "errtest", Message: "errtestmsg"}},
			},
		},
	}
	for testName, testParams := range testCases {
		t.Run(testName, func(t *testing.T) {
			t.Parallel()
			serverPort := portPool.Get()
			server, err := newServer(serverPort)
			if err != nil {
				t.Fatal(err)
			}
			defer server.Shutdown(t.Context())
			address := fmt.Sprintf("localhost:%v", serverPort)
			config := appsync.NewAPIKeyConfig(address, address, "apikeytest")
			config.HTTPProtocol = "http"
			config.WebSocketProtocol = "ws"
			client, err := server.DialConfig(config, defaultTimeout)
			if err != nil {
				t.Fatal(err)
			}
			channel := "test/unsubscribe"
			subErrC := make(chan error)
			go func() {
				defer close(subErrC)
				channelC := make(chan *message.SubscriptionMessage)
				err := client.Subscribe(t.Context(), channel, channelC)
				if err != nil {
					subErrC <- err

					return
				}
			}()
			clientMsg, err := server.Receive()
			if err != nil {
				t.Fatal(err)
			}
			publishID := clientMsg.ID
			expectedClientMsg := &message.SendMessage{
				Authorization: config.Authorization,
				Type:          message.SubscribeType,
				Channel:       channel,
				ID:            publishID,
			}
			if !isSendMessageEqual(clientMsg, expectedClientMsg) {
				t.Fatalf("unexpected subscribe data: %v", clientMsg)
			}
			testParams.ServerSubResponse.ID = publishID
			err = server.Send(testParams.ServerSubResponse)
			if err != nil {
				t.Fatal(err)
			}
			err = <-subErrC
			if !errors.Is(err, testParams.ExpectedSubErr) {
				t.Fatal(err)
			}
			if err != nil {
				client.Close() //nolint: errcheck

				return
			}
			unsubErrC := make(chan error)
			go func() {
				err := client.Unsubscribe(t.Context(), channel)
				if err != nil {
					unsubErrC <- err

					return
				}
				close(unsubErrC)
			}()
			clientMsg, err = server.Receive()
			if err != nil {
				t.Fatal(err)
			}
			expectedClientMsg = &message.SendMessage{
				Type: message.UnsubscribeType,
				ID:   publishID,
			}
			if !isSendMessageEqual(clientMsg, expectedClientMsg) {
				t.Fatalf("unexpected unsubscribe data: %v", clientMsg)
			}
			testParams.ServerUnsubResponse.ID = publishID
			err = server.Send(testParams.ServerUnsubResponse)
			if err != nil {
				t.Fatal(err)
			}
			err = <-unsubErrC
			if !errors.Is(err, testParams.ExpectedUnsubErr) {
				t.Fatal(err)
			}
			if err != nil {
				client.Close() //nolint: errcheck

				return
			}
			err = client.Close()
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

var portPool = Pool[string]{}         //nolint: gochecknoglobals
var defaultTimeout = 30 * time.Second //nolint: gochecknoglobals

func isSendMessageAuthorizationEqual(msg1, msg2 *message.Authorization) bool {
	if msg1 == nil && msg2 == nil {
		return true
	}
	if msg1 == nil {
		return false
	}
	if msg2 == nil {
		return false
	}

	return msg1.Authorization == msg2.Authorization &&
		msg1.Host == msg2.Host &&
		msg1.XAmzDate == msg2.XAmzDate &&
		msg1.XAmzSecurityToken == msg2.XAmzSecurityToken &&
		msg1.XAPIKey == msg2.XAPIKey
}

func isSendMessageEqual(msg1, msg2 *message.SendMessage) bool {
	if msg1 == nil && msg2 == nil {
		return true
	}
	if msg1 == nil {
		return false
	}
	if msg2 == nil {
		return false
	}
	if len(msg1.Events) != len(msg2.Events) {
		return false
	}
	for i := range msg1.Events {
		if msg1.Events[i] != msg2.Events[i] {
			return false
		}
	}
	if msg1.Authorization == nil && msg2.Authorization != nil {
		return false
	}
	if msg1.Authorization != nil && !isSendMessageAuthorizationEqual(msg1.Authorization, msg2.Authorization) {
		return false
	}

	return msg1.Channel == msg2.Channel &&
		msg1.ID == msg2.ID &&
		msg1.Type == msg2.Type
}

func isSubscriptionMessageEqual(msg1, msg2 *message.SubscriptionMessage) bool {
	if msg1 == nil && msg2 == nil {
		return true
	}
	if msg1 == nil {
		return false
	}
	if msg2 == nil {
		return false
	}
	if len(msg1.Errors) != len(msg2.Errors) {
		return false
	}
	for i := range msg1.Errors {
		if msg1.Errors[i] != msg2.Errors[i] {
			return false
		}
	}

	return msg1.Event == msg2.Event &&
		msg1.Type == msg2.Type
}

func newServer(port string) (*Server, error) {
	errC := make(chan error)
	clientC := make(chan *message.SendMessage)
	serverC := make(chan *message.ReceiveMessage)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqCtx := r.Context()
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			errC <- err
		}
		wsWaitGroup := sync.WaitGroup{}
		wsWaitGroup.Go(func() {
			defer conn.CloseNow() //nolint: errcheck
			for {
				clientMsg := &message.SendMessage{}
				err = wsjson.Read(reqCtx, conn, clientMsg)
				if err != nil {
					errC <- err

					return
				}
				select {
				case <-reqCtx.Done():
					return
				case clientC <- clientMsg:
				}
			}
		})
		wsWaitGroup.Go(func() {
			defer conn.CloseNow() //nolint: errcheck
			for {
				var serverMsg *message.ReceiveMessage
				select {
				case <-reqCtx.Done():
					return
				case serverMsg = <-serverC:
				}
				err = wsjson.Write(reqCtx, conn, serverMsg)
				if err != nil {
					errC <- err

					return
				}
			}
		})
		wsWaitGroup.Wait()
	})
	address := fmt.Sprintf(":%v", port)
	server := &http.Server{ // #nosec G112
		Addr:    address,
		Handler: handler,
	}
	go func() {
		err := server.ListenAndServe()
		if err != nil {
			errC <- err
		}
	}()
	serverRunning := false
	dialer := &net.Dialer{
		Timeout: 2 * time.Second,
	}
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(10*time.Second))
	defer cancel()
	for !serverRunning {
		select {
		case <-ctx.Done():
			return nil, errors.New("server not starting")
		default:
			conn, err := dialer.DialContext(ctx, "tcp", address)
			if err != nil {
				continue
			}
			serverRunning = true
			err = conn.Close()
			if err != nil {
				return nil, errors.Join(errors.New("failed to close test connection"), err)
			}
		}
	}
	servre := &Server{
		clientC: clientC,
		ErrC:    errC,
		server:  server,
		serverC: serverC,
	}

	return servre, nil
}

func TestMain(m *testing.M) {
	// Issue: https://github.com/brokgo/appsync-event-client-go/issues/7
	nServers := 20
	for i := range nServers {
		portPool.Put(fmt.Sprintf("9%03d", i))
	}
	os.Exit(m.Run())
}
