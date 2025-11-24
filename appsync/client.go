package appsync

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"sync"
	"time"

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
	Authorization *Authorization
	// Conn is the websocket connection to the server.
	Conn Conn
	// Err is the first error found that prvent the client from continuing.
	// These errors range from connection errors to data processing errors.
	Err error

	done                    chan struct{}
	linkByID                sync.Map
	subscriptionByID        sync.Map
	subscriptionIDByChannel sync.Map
	wg                      sync.WaitGroup
}

// NewWebSocketClient creates a websocket client.
func NewWebSocketClient(ctx context.Context, conn Conn, auth *Authorization) (*WebSocketClient, error) {
	// Init call
	err := write(ctx, conn, &SendMessage{
		Type: ConnectionInitType,
	})
	if err != nil {
		return nil, err
	}
	initMsg := &ReceiveMessage{}
	err = read(ctx, conn, initMsg)
	if err != nil {
		return nil, err
	}
	if len(initMsg.Errors) > 0 {
		return nil, errFromMsgErrors(initMsg.Errors)
	}
	// Create client
	client := &WebSocketClient{
		Authorization:           auth,
		Conn:                    conn,
		done:                    make(chan struct{}),
		linkByID:                sync.Map{},
		subscriptionByID:        sync.Map{},
		subscriptionIDByChannel: sync.Map{},
		wg:                      sync.WaitGroup{},
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
	linkChan, err := w.registerLink(ctx, linkID)
	if err != nil {
		return nil, err
	}
	defer func() {
		deregErr := w.deregisterLink(linkID)
		if deregErr != nil {
			err = deregErr
		}
	}()
	// Call
	err = write(ctx, w.Conn, &SendMessage{
		Authorization: w.Authorization,
		Channel:       channel,
		Type:          PublishType,
		ID:            linkID,
		Events:        events,
	})
	if err != nil {
		return nil, err
	}
	// Handle response
	var resp *ReceiveMessage
	select {
	case <-ctx.Done():
		return nil, errors.Join(ErrContextEnded, ctx.Err())
	case <-w.done:
	case resp = <-linkChan:
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
// channelC can be buffered or unbuffered. It is closed when the connection to the server is closed.
func (w *WebSocketClient) Subscribe(ctx context.Context, channel string, channelC chan *SubscriptionMessage) (err error) {
	// Create link
	linkUUID, err := uuid.NewRandom()
	if err != nil {
		return err
	}
	linkID := linkUUID.String()
	linkChan, err := w.registerLink(ctx, linkID)
	if err != nil {
		return err
	}
	defer func() {
		deregErr := w.deregisterLink(linkID)
		if deregErr != nil {
			err = deregErr
		}
	}()
	// Create subscrition
	sub := &subscription{
		Chan: make(chan *SubscriptionMessage),
		Mu:   sync.Mutex{},
	}
	w.goHandleSubscriptionBuffer(sub.Chan, channelC)
	sub.Mu.Lock()
	defer sub.Mu.Unlock()
	_, loaded := w.subscriptionByID.LoadOrStore(linkID, sub)
	if loaded {
		return ErrIDExists
	}
	_, loaded = w.subscriptionIDByChannel.LoadOrStore(channel, linkID)
	if loaded {
		return ErrSubscriptionCalled
	}
	// Call
	err = write(ctx, w.Conn, &SendMessage{
		Authorization: w.Authorization,
		Type:          SubscribeType,
		ID:            linkID,
		Channel:       channel,
	})
	removeSub := func() {
		w.subscriptionIDByChannel.Delete(channel)
		w.subscriptionByID.Delete(linkID)
		close(sub.Chan)
	}
	if err != nil {
		removeSub()

		return err
	}
	// Handle response
	var resp *ReceiveMessage
	select {
	case <-ctx.Done():
		return errors.Join(ErrContextEnded, ctx.Err())
	case <-w.done:
	case resp = <-linkChan:
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
func (w *WebSocketClient) Unsubscribe(ctx context.Context, channel string) (err error) {
	// Get link id for subscrition
	linkIDAny, found := w.subscriptionIDByChannel.Load(channel)
	if !found {
		return ErrChannelNotSubscribed
	}
	linkID, ok := linkIDAny.(string)
	if !ok {
		return ErrTypeAssertion
	}
	// Create link
	linkChan, err := w.registerLink(ctx, linkID)
	if err != nil {
		return err
	}
	defer func() {
		deregErr := w.deregisterLink(linkID)
		if deregErr != nil {
			err = deregErr
		}
	}()
	// Get subscription
	subAny, found := w.subscriptionByID.Load(linkID)
	if !found {
		return ErrIDDoesNotExists
	}
	sub, ok := subAny.(*subscription)
	if !ok {
		return ErrTypeAssertion
	}
	sub.Mu.Lock()
	defer sub.Mu.Unlock()
	// Call
	err = write(ctx, w.Conn, &SendMessage{
		Type: UnsubscribeType,
		ID:   linkID,
	})
	if err != nil {
		return err
	}
	// Handle response
	var resp *ReceiveMessage
	select {
	case <-ctx.Done():
		return errors.Join(ErrContextEnded, ctx.Err())
	case <-w.done:
	case resp = <-linkChan:
	}
	if resp == nil {
		return w.Err
	}
	if len(resp.Errors) > 0 {
		return errFromMsgErrors(resp.Errors)
	}
	w.subscriptionIDByChannel.Delete(channel)
	w.subscriptionByID.Delete(linkID)
	close(sub.Chan)

	return nil
}

// deregisterLink deregisters the linkID specified.
func (w *WebSocketClient) deregisterLink(linkID string) error {
	linkAny, ok := w.linkByID.Load(linkID)
	if !ok {
		return ErrIDDoesNotExists
	}
	link, ok := linkAny.(*clientLink)
	if !ok {
		return ErrTypeAssertion
	}
	w.linkByID.Delete(linkID)
	close(link.Done)

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
				msg := &ReceiveMessage{}
				err := read(ctx, w.Conn, msg)
				if err != nil {
					cancel(errors.Join(ErrRecieveMsg, err))

					return
				}
				switch msg.Type {
				// Send subscrription msg data to the subscription channel
				case SubscriptionDataType, SubscriptionBroadcastErrorType:
					subAny, found := w.subscriptionByID.Load(msg.ID)
					if !found {
						continue
					}
					sub, ok := subAny.(*subscription)
					if !ok {
						cancel(ErrTypeAssertion)
					}
					sub.Mu.Lock()
					_, found = w.subscriptionByID.Load(msg.ID)
					if !found {
						sub.Mu.Unlock()

						continue
					}
					dataMsg := &SubscriptionMessage{
						Errors: msg.Errors,
						Event:  msg.Event,
						Type:   msg.Type,
					}
					sub.Chan <- dataMsg
					sub.Mu.Unlock()
				// Keep alive connection
				case KeepAliveType:
					select {
					case <-w.done:
						return
					case keepAliveC <- struct{}{}:
					}
				// Handle error
				case ErrorType:
					cancel(errFromMsgErrors(msg.Errors))

					return
				// Send data to link
				default:
					linkAny, ok := w.linkByID.Load(msg.ID)
					if !ok {
						continue
					}
					link, ok := linkAny.(*clientLink)
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
func (w *WebSocketClient) goHandleSubscriptionBuffer(subIn chan *SubscriptionMessage, subOut chan *SubscriptionMessage) {
	w.wg.Go(func() {
		buff := []*SubscriptionMessage{}
		for {
			if len(buff) == 0 {
				select {
				case <-w.done:
					close(subOut)

					return
				case inMsg, ok := <-subIn:
					if !ok {
						return
					}
					buff = append(buff, inMsg)
				}
			} else {
				outMsg := buff[0]
				select {
				case <-w.done:
					close(subOut)

					return
				case inMsg, ok := <-subIn:
					if !ok {
						return
					}
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

// registerLink registers the linkID specified. Only one link per ID can be registered, otherwise it will wait for the link to be released.
func (w *WebSocketClient) registerLink(ctx context.Context, linkID string) (chan *ReceiveMessage, error) {
	link := &clientLink{
		Chan: make(chan *ReceiveMessage, 1),
		Done: make(chan struct{}),
	}
	for {
		otherLinkAny, loaded := w.linkByID.LoadOrStore(linkID, link)
		if !loaded {
			break
		}
		// Wait for link to be released
		otherLink, ok := otherLinkAny.(*clientLink)
		if !ok {
			return nil, ErrTypeAssertion
		}
		select {
		case <-ctx.Done():
			return nil, ErrContextEnded
		case <-otherLink.Done:
		}
	}

	return link.Chan, nil
}

type clientLink struct {
	Chan chan *ReceiveMessage
	// Done is a signal to any writers that the link is closed.
	Done chan struct{}
}

// connReader is a wrapper for building an io.reader using the Conn.Read interface.
type connReader struct {
	conn Conn
	ctx  context.Context //nolint: containedctx
}

func (c *connReader) Read(p []byte) (int, error) {
	return c.conn.Read(c.ctx, p)
}

type subscription struct {
	Chan chan *SubscriptionMessage
	Mu   sync.Mutex
}

// errFromMsgErrors converts the error data from the server into a go error.
func errFromMsgErrors(msgErrs []MessageError) error {
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
