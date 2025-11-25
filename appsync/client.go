package appsync

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"sync"
	"time"

	"github.com/brokgo/appsync-event-client-go/appsync/message"
	"github.com/google/uuid"
)

// Possible errors returned from creation or usage of the WebSocketClient.
var (
	ErrChannelNotSubscribed = errors.New("channel is not subscribed")
	ErrContextEnded         = errors.New("context ended")
	ErrIDDoesNotExists      = errors.New("id does not exist")
	ErrIDExists             = errors.New("uuid exsists")
	ErrMarshalMsg           = errors.New("failed to marshal message")
	ErrRecieveMsg           = errors.New("failed to receive message")
	ErrServerMsg            = errors.New("server returned error")
	ErrSubscriptionCalled   = errors.New("subscription on channel already called")
	ErrTimeout              = errors.New("server timed out")
	ErrTypeAssertion        = errors.New("faild type assertion")
	ErrUnsubscriptionCalled = errors.New("unsubscription on channel already called")
	ErrUnsupportedMsgFormat = errors.New("unsupported message format")
)

// Conn is the websocket connection to the server.
type Conn interface {
	// Close closes the connection.
	Close() error
	// Read reads data from the connection.
	// Return EOF when it reaches the end of a message.
	Read(ctx context.Context, b []byte) (n int, err error)
	// Write writes data to the connection.
	Write(ctx context.Context, b []byte) (n int, err error)
}

// WebSocketClient is the client for managing a Appsync Event websocket connection.
type WebSocketClient struct {
	// Authorization is authorization details sent to the server.
	Authorization *message.Authorization
	// Conn is the websocket connection to the server.
	Conn Conn
	// Err is the first error found that prvent the client from continuing.
	// These errors range from connection errors to data processing errors.
	Err error

	done             chan struct{}
	receiverByLinkID sync.Map
	subReceiverByID  sync.Map
	linkIDByChannel  sync.Map
	wg               sync.WaitGroup
}

// NewWebSocketClient creates a websocket client.
func NewWebSocketClient(ctx context.Context, conn Conn, auth *message.Authorization) (*WebSocketClient, error) {
	// Init call
	err := write(ctx, conn, &message.SendMessage{
		Type: message.ConnectionInitType,
	})
	if err != nil {
		return nil, err
	}
	initMsg := &message.ReceiveMessage{}
	err = read(ctx, conn, initMsg)
	if err != nil {
		return nil, err
	}
	if len(initMsg.Errors) > 0 {
		return nil, errFromMsgErrors(initMsg.Errors)
	}
	// Create client
	client := &WebSocketClient{
		Authorization:    auth,
		Conn:             conn,
		done:             make(chan struct{}),
		receiverByLinkID: sync.Map{},
		subReceiverByID:  sync.Map{},
		linkIDByChannel:  sync.Map{},
		wg:               sync.WaitGroup{},
	}
	var once sync.Once
	cancel := func(err error) {
		once.Do(func() {
			client.Err = err
			go client.Close() //nolint: errcheck
		})
	}
	keepAliveC := make(chan struct{}, 1)
	client.goHandleRead(cancel, keepAliveC) //nolint:contextcheck
	client.goHandleTimeOut(cancel, time.Duration(initMsg.ConnectionTimeoutMs)*time.Millisecond, keepAliveC)

	return client, nil
}

// Close closes the connection to the server and all open subscription channels.
func (w *WebSocketClient) Close() error {
	select {
	case <-w.done:
	default:
		close(w.done) // Issue: https://github.com/brokgo/appsync-event-client-go/issues/5
	}
	err := w.Conn.Close()
	w.wg.Wait()

	return err
}

// Publish publishes an event to Appsync.
// If you want to send JSON event, marshal the object into a string.
func (w *WebSocketClient) Publish(ctx context.Context, channel string, events []string) (sucessIs []int, err error) { //nolint: nonamedreturns
	// Create link
	linkUUID, err := uuid.NewRandom()
	if err != nil {
		return nil, err
	}
	linkID := linkUUID.String()
	receiver := &msgReceiver{
		Chan: make(chan *message.ReceiveMessage, 1),
		Done: make(chan struct{}),
	}
	_, loaded := w.receiverByLinkID.LoadOrStore(linkID, receiver)
	if loaded {
		return nil, ErrIDExists
	}
	defer func() {
		w.receiverByLinkID.Delete(linkID)
		close(receiver.Done)
	}()
	// Call
	err = write(ctx, w.Conn, &message.SendMessage{
		Authorization: w.Authorization,
		Channel:       channel,
		Type:          message.PublishType,
		ID:            linkID,
		Events:        events,
	})
	if err != nil {
		return nil, err
	}
	// Handle response
	var resp *message.ReceiveMessage
	select {
	case <-ctx.Done():
		return nil, errors.Join(ErrContextEnded, ctx.Err())
	case <-w.done:
	case resp = <-receiver.Chan:
	}
	if resp == nil {
		return nil, w.Err
	}
	if len(resp.Errors) > 0 {
		return nil, errFromMsgErrors(resp.Errors)
	}
	successIndicies := []int{}
	for _, successes := range resp.Successful {
		successIndicies = append(successIndicies, successes.Index)
	}

	return successIndicies, nil
}

// Subscribe subscribes to an event channel. The chan returned, channelC, will return events for the channel subscription.
// channelC can be buffered or unbuffered. It is closed when the connection to the server is closed or channel is unsubscribed.
func (w *WebSocketClient) Subscribe(ctx context.Context, channel string, channelC chan *message.SubscriptionMessage) error {
	// Create link
	linkUUID, err := uuid.NewRandom()
	if err != nil {
		return err
	}
	linkID := linkUUID.String()
	receiver := &msgReceiver{
		Chan: make(chan *message.ReceiveMessage, 1),
		Done: make(chan struct{}),
	}
	_, loaded := w.receiverByLinkID.LoadOrStore(linkID, receiver)
	if loaded {
		return ErrIDExists
	}
	defer func() {
		w.receiverByLinkID.Delete(linkID)
		close(receiver.Done)
	}()
	// Create subscrition link
	sub := &subscriptionMsgReceiver{
		Chan: make(chan *message.SubscriptionMessage),
		Done: make(chan struct{}),
	}
	w.goHandleSubscriptionBuffer(sub, channelC)
	_, loaded = w.subReceiverByID.LoadOrStore(linkID, sub)
	if loaded {
		return ErrIDExists
	}
	_, loaded = w.linkIDByChannel.LoadOrStore(channel, linkID)
	if loaded {
		return ErrSubscriptionCalled
	}
	// Call
	err = write(ctx, w.Conn, &message.SendMessage{
		Authorization: w.Authorization,
		Type:          message.SubscribeType,
		ID:            linkID,
		Channel:       channel,
	})
	removeSub := func() {
		w.linkIDByChannel.Delete(channel)
		w.subReceiverByID.Delete(linkID)
		close(sub.Chan)
	}
	if err != nil {
		removeSub()

		return err
	}
	// Handle response
	var resp *message.ReceiveMessage
	select {
	case <-ctx.Done():
		return errors.Join(ErrContextEnded, ctx.Err())
	case <-w.done:
	case resp = <-receiver.Chan:
	}
	if resp == nil {
		removeSub()

		return w.Err
	}
	if len(resp.Errors) > 0 {
		removeSub()

		return errFromMsgErrors(resp.Errors)
	}

	return nil
}

// Unsubscribe unsubscribes to an event channel. The chan used to receive events is not closed after unsubscribing.
func (w *WebSocketClient) Unsubscribe(ctx context.Context, channel string) error {
	// Get link id for subscrition
	linkIDAny, found := w.linkIDByChannel.Load(channel)
	if !found {
		return ErrChannelNotSubscribed
	}
	linkID, ok := linkIDAny.(string)
	if !ok {
		return ErrTypeAssertion
	}
	// Create receiver
	receiver := &msgReceiver{
		Chan: make(chan *message.ReceiveMessage, 1),
		Done: make(chan struct{}),
	}
	_, loaded := w.receiverByLinkID.LoadOrStore(linkID, receiver)
	if loaded {
		return ErrIDExists
	}
	defer func() {
		w.receiverByLinkID.Delete(linkID)
		close(receiver.Done)
	}()
	// Get subscription link
	subAny, found := w.subReceiverByID.Load(linkID)
	if !found {
		return ErrIDDoesNotExists
	}
	sub, ok := subAny.(*subscriptionMsgReceiver)
	if !ok {
		return ErrTypeAssertion
	}
	// Call
	err := write(ctx, w.Conn, &message.SendMessage{
		Type: message.UnsubscribeType,
		ID:   linkID,
	})
	if err != nil {
		return err
	}
	// Handle response
	var resp *message.ReceiveMessage
	select {
	case <-ctx.Done():
		return errors.Join(ErrContextEnded, ctx.Err())
	case <-w.done:
	case resp = <-receiver.Chan:
	}
	if resp == nil {
		return w.Err
	}
	if len(resp.Errors) > 0 {
		return errFromMsgErrors(resp.Errors)
	}
	w.linkIDByChannel.Delete(channel)
	w.subReceiverByID.Delete(linkID)
	close(sub.Done)

	return nil
}

// goHandleRead handles messages received from the server.
func (w *WebSocketClient) goHandleRead(cancel context.CancelCauseFunc, keepAliveC chan struct{}) {
	w.wg.Go(func() {
		ctx := context.Background()
		for {
			select {
			case <-w.done:
				return
			default:
				msg := &message.ReceiveMessage{}
				err := read(ctx, w.Conn, msg)
				if err != nil {
					cancel(errors.Join(ErrRecieveMsg, err))

					return
				}
				switch msg.Type {
				// Send subscrription to the subscription receiver
				case message.SubscriptionDataType, message.SubscriptionBroadcastErrorType:
					subAny, found := w.subReceiverByID.Load(msg.ID)
					if !found {
						continue
					}
					sub, ok := subAny.(*subscriptionMsgReceiver)
					if !ok {
						cancel(ErrTypeAssertion)
					}
					dataMsg := &message.SubscriptionMessage{
						Errors: msg.Errors,
						Event:  msg.Event,
						Type:   msg.Type,
					}
					select {
					case <-sub.Done:
					case sub.Chan <- dataMsg:
					}
				// Keep alive connection
				case message.KeepAliveType:
					select {
					case <-w.done:
						return
					case keepAliveC <- struct{}{}:
					}
				// Handle error
				case message.ErrorType:
					cancel(errFromMsgErrors(msg.Errors))

					return
				// Send data to link receiver
				default:
					linkAny, ok := w.receiverByLinkID.Load(msg.ID)
					if !ok {
						continue
					}
					link, ok := linkAny.(*msgReceiver)
					if !ok {
						cancel(ErrTypeAssertion)

						return
					}
					select {
					case <-link.Done:
						continue
					case link.Chan <- msg:
					}
				}
			}
		}
	})
}

// goHandleSubscriptionBuffer handles a single subsctiption. It routes the data from the server to the subscription channel with a buffer in between.
func (w *WebSocketClient) goHandleSubscriptionBuffer(sub *subscriptionMsgReceiver, subOut chan *message.SubscriptionMessage) {
	w.wg.Go(func() {
		defer close(subOut)
		buff := []*message.SubscriptionMessage{}
		for {
			if len(buff) == 0 {
				select {
				case <-w.done:
					return
				case <-sub.Done:
					return
				case inMsg := <-sub.Chan:
					buff = append(buff, inMsg)
				}
			} else {
				outMsg := buff[0]
				select {
				case <-w.done:
					return
				case <-sub.Done:
					return
				case inMsg := <-sub.Chan:
					buff = append(buff, inMsg)
				case subOut <- outMsg:
					buff = buff[1:]
				}
			}
		}
	})
}

// goHandleTimeOut handles server's keep alive timeout.
func (w *WebSocketClient) goHandleTimeOut(cancel context.CancelCauseFunc, timeoutDuration time.Duration, keepAliveC chan struct{}) {
	w.wg.Go(func() {
		timer := time.NewTimer(timeoutDuration)
		for {
			select {
			case <-w.done:
				return
			case <-keepAliveC:
				timer.Reset(timeoutDuration)
			case <-timer.C:
				cancel(ErrTimeout)

				return
			}
		}
	})
}

// connReader is a wrapper for building an io.reader using the Conn.Read interface.
type connReader struct {
	conn Conn
	ctx  context.Context //nolint: containedctx
}

func (c *connReader) Read(p []byte) (int, error) {
	return c.conn.Read(c.ctx, p)
}

type msgReceiver struct {
	Chan chan *message.ReceiveMessage
	// Done is a signal to any readers or writers that the link is closed.
	Done chan struct{}
}

type subscriptionMsgReceiver struct {
	Chan chan *message.SubscriptionMessage
	// Done is a signal to any readers or writers that the subscription is closed.
	Done chan struct{}
}

// errFromMsgErrors converts the error data from the server into a go error.
func errFromMsgErrors(msgErrs []message.ErrorData) error {
	errBytes, err := json.Marshal(msgErrs)
	if err != nil {
		return errors.Join(ErrMarshalMsg, err)
	}

	return errors.Join(ErrServerMsg, errors.New(string(errBytes))) //nolint: err113
}

// read reads one message from conn.
func read(ctx context.Context, conn Conn, msg any) error {
	reader := &connReader{
		conn: conn,
		ctx:  ctx,
	}
	msgJSON, err := io.ReadAll(reader)
	if err != nil {
		return err
	}

	return json.Unmarshal(msgJSON, msg)
}

// write writes one message to conn.
func write(ctx context.Context, conn Conn, msg any) error {
	msgJSON, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	_, err = conn.Write(ctx, msgJSON)

	return err
}
