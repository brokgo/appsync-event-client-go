package appsync

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/google/uuid"
)

// Possible errors returned from creation or usage of the WebSocketClient.
var (
	ErrChannelNotSubscribed = errors.New("channel is not subscribed")
	ErrMarshalMsg           = errors.New("failed to marshal message")
	ErrRecieveMsg           = errors.New("failed to receive message")
	ErrServerMsg            = errors.New("server returned error")
	ErrTimeout              = errors.New("server timed out")
	ErrUnknownMessageID     = errors.New("unknown message id")
	ErrUnsupportedMsgFormat = errors.New("unsupported message format")
)

// WebSocketClient is the client for managing a Appsync Event websocket connection.
type WebSocketClient struct {
	// Err is the first error found that prvent the client from conting.
	// These errors range from connection errors to data processing errors.
	Err error

	authorization           *SendMessageAuthorization
	conn                    *websocket.Conn
	done                    chan struct{}
	linkByID                map[string]chan *ReceiveMessage
	linkMu                  sync.RWMutex
	subscriptionIDByChannel map[string]string
	subscriptionBufferByID  map[string]chan *SubscriptionMessage
	subscriptionMu          sync.RWMutex
	timeoutDuration         time.Duration
	wg                      sync.WaitGroup
}

// Close closes the connection to the server and all open subscription channels.
func (w *WebSocketClient) Close() error {
	select {
	case <-w.done:
	default:
		close(w.done) // Issue: https://github.com/brokgo/appsync-event-client-go/issues/5
	}
	err := w.conn.Close(websocket.StatusNormalClosure, "")
	w.wg.Wait()

	return err
}

// Publish publishes an event to Appsync.
// If you want to send JSON event, marshal the object into a string.
func (w *WebSocketClient) Publish(ctx context.Context, channel string, events []string) ([]int, error) {
	linkUUID, err := uuid.NewRandom()
	if err != nil {
		return nil, err
	}
	linkID := linkUUID.String()
	linkC := make(chan *ReceiveMessage, 1)
	w.linkMu.Lock()
	w.linkByID[linkID] = linkC
	w.linkMu.Unlock()
	defer deleteLink(linkID, &w.linkMu, w.linkByID)
	err = write(ctx, w.conn, &SendMessage{
		Authorization: w.authorization,
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
	case <-w.done:
	case resp = <-linkC:
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
func (w *WebSocketClient) Subscribe(ctx context.Context, channel string, channelC chan *SubscriptionMessage) error {
	linkUUID, err := uuid.NewRandom()
	if err != nil {
		return err
	}
	linkID := linkUUID.String()
	linkC := make(chan *ReceiveMessage, 1)
	w.linkMu.Lock()
	w.linkByID[linkID] = linkC
	w.linkMu.Unlock()
	defer deleteLink(linkID, &w.linkMu, w.linkByID)
	// Catch msg's that are sent after subscribing to the server and
	// before this function returns
	inBuffC := make(chan *SubscriptionMessage)
	w.goHandleSubscriptionBuffer(inBuffC, channelC)
	w.subscriptionMu.Lock()
	w.subscriptionBufferByID[linkID] = inBuffC
	w.subscriptionMu.Unlock()
	removeSub := func() {
		w.subscriptionMu.Lock()
		delete(w.subscriptionBufferByID, linkID)
		w.subscriptionMu.Unlock()
		close(inBuffC)
	}
	err = write(ctx, w.conn, &SendMessage{
		Authorization: w.authorization,
		Type:          SubscribeType,
		ID:            linkID,
		Channel:       channel,
	})
	if err != nil {
		removeSub()

		return err
	}
	var resp *ReceiveMessage
	select {
	case <-w.done:
	case resp = <-linkC:
	}
	if resp == nil {
		removeSub()

		return w.Err
	}
	if len(resp.Errors) > 0 {
		removeSub()

		return errFromMsgErrors(resp.Errors)
	}
	w.subscriptionMu.Lock()
	w.subscriptionIDByChannel[channel] = linkID
	w.subscriptionMu.Unlock()

	return nil
}

// Unsubscribe unsubscribes to an event channel. The chan used to receive events is not closed after unsubscribing.
func (w *WebSocketClient) Unsubscribe(ctx context.Context, channel string) error {
	w.subscriptionMu.RLock()
	linkID, found := w.subscriptionIDByChannel[channel]
	w.subscriptionMu.RUnlock()
	if !found {
		return ErrChannelNotSubscribed
	}
	linkC := make(chan *ReceiveMessage, 1)
	w.linkMu.Lock()
	w.linkByID[linkID] = linkC
	w.linkMu.Unlock()
	defer deleteLink(linkID, &w.linkMu, w.linkByID)
	err := write(ctx, w.conn, &SendMessage{
		Type: UnsubscribeType,
		ID:   linkID,
	})
	if err != nil {
		return err
	}
	var resp *ReceiveMessage
	select {
	case <-w.done:
	case resp = <-linkC:
	}
	if resp == nil {
		return w.Err
	}
	if len(resp.Errors) > 0 {
		return errFromMsgErrors(resp.Errors)
	}
	w.subscriptionMu.Lock()
	buffC := w.subscriptionBufferByID[linkID]
	delete(w.subscriptionIDByChannel, channel)
	delete(w.subscriptionBufferByID, linkID)
	close(buffC)
	w.subscriptionMu.Unlock()

	return nil
}

func (w *WebSocketClient) goHandleRead(ctx context.Context, cancel context.CancelCauseFunc, keepAliveC chan struct{}) {
	w.wg.Go(func() {
		for {
			select {
			case <-w.done:
				return
			default:
				msg := &ReceiveMessage{}
				err := read(ctx, w.conn, msg)
				if err != nil {
					cancel(errors.Join(ErrRecieveMsg, err))

					return
				}
				switch msg.Type {
				case SubscriptionDataType, SubscriptionBroadcastErrorType:
					w.subscriptionMu.RLock()
					subscriptionC, found := w.subscriptionBufferByID[msg.ID]
					if !found {
						w.subscriptionMu.RUnlock()
						cancel(ErrUnknownMessageID)

						return
					}
					dataMsg := &SubscriptionMessage{
						Errors: msg.Errors,
						Event:  msg.Event,
						Type:   msg.Type,
					}
					subscriptionC <- dataMsg
					w.subscriptionMu.RUnlock()
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
					w.linkMu.RLock()
					msgC, found := w.linkByID[msg.ID]
					if !found {
						w.linkMu.RUnlock()
						cancel(ErrUnknownMessageID)

						return
					}
					msgC <- msg
					w.linkMu.RUnlock()
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

func (w *WebSocketClient) goHandleTimeOut(cancel context.CancelCauseFunc, keepAliveC chan struct{}) {
	w.wg.Go(func() {
		timer := time.NewTimer(w.timeoutDuration)
		for {
			select {
			case <-w.done:
				return
			case <-keepAliveC:
				timer.Reset(w.timeoutDuration)
			case <-timer.C:
				cancel(ErrTimeout)

				return
			}
		}
	})
}

func deleteLink[KT comparable, VT any](linkID KT, locker sync.Locker, links map[KT]VT) {
	locker.Lock()
	defer locker.Unlock()
	delete(links, linkID)
}

func errFromMsgErrors(msgErrs []MessageError) error {
	errBytes, err := json.Marshal(msgErrs)
	if err != nil {
		return errors.Join(ErrMarshalMsg, err)
	}

	return errors.Join(ErrServerMsg, errors.New(string(errBytes))) //nolint: err113
}

func read(ctx context.Context, conn *websocket.Conn, msg any) error {
	msgFormat, reader, err := conn.Reader(ctx)
	if err != nil {
		return err
	}
	if msgFormat != websocket.MessageText {
		return ErrUnsupportedMsgFormat
	}
	msgJSON, err := io.ReadAll(reader)
	if err != nil {
		return err
	}

	return json.Unmarshal(msgJSON, msg)
}

func write(ctx context.Context, conn *websocket.Conn, msg any) (err error) {
	msgJSON, err := json.Marshal(msg)
	if err != nil {
		return
	}
	writer, err := conn.Writer(ctx, websocket.MessageText)
	defer func() {
		clsErr := writer.Close()
		if err == nil {
			err = clsErr
		}
	}()
	if err != nil {
		writer.Close() //nolint: errcheck

		return
	}
	_, err = writer.Write(msgJSON)

	return
}
