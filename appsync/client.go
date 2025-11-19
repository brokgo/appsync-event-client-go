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
	Authorization *SendMessageAuthorization
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
	linkUUID, err := uuid.NewRandom()
	if err != nil {
		return err
	}
	linkID := linkUUID.String()
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
	linkIDAny, found := w.subscriptionIDByChannel.Load(channel)
	if !found {
		return ErrChannelNotSubscribed
	}
	linkID, ok := linkIDAny.(string)
	if !ok {
		return ErrTypeAssertion
	}
	subAny, found := w.subscriptionByID.Load(linkID)
	if !found {
		return ErrIDDoesNotExists
	}
	sub, ok := subAny.(*subscription)
	if !ok {
		return ErrTypeAssertion
	}
	sub.Mu.Lock()
	_, found = w.subscriptionByID.Load(linkID)
	if !found {
		return ErrUnsubscriptionCalled
	}
	defer sub.Mu.Unlock()
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
	err = write(ctx, w.Conn, &SendMessage{
		Type: UnsubscribeType,
		ID:   linkID,
	})
	if err != nil {
		return err
	}
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
				case KeepAliveType:
					select {
					case <-w.done:
						return
					case keepAliveC <- struct{}{}:
					}
				case ErrorType:
					cancel(errFromMsgErrors(msg.Errors))

					return
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
	Done chan struct{}
}

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

func errFromMsgErrors(msgErrs []MessageError) error {
	errBytes, err := json.Marshal(msgErrs)
	if err != nil {
		return errors.Join(ErrMarshalMsg, err)
	}

	return errors.Join(ErrServerMsg, errors.New(string(errBytes))) //nolint: err113
}

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

func write(ctx context.Context, conn Conn, msg any) error {
	msgJSON, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	_, err = conn.Write(ctx, msgJSON)

	return err
}
