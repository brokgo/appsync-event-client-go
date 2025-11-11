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

type Conn interface {
	Close(code websocket.StatusCode, reason string) error
	Reader(ctx context.Context) (websocket.MessageType, io.Reader, error)
	Writer(ctx context.Context, typ websocket.MessageType) (io.WriteCloser, error)
}

var ErrChannelNotSubscribed = errors.New("channel is not subscribed")
var ErrMarshalMsg = errors.New("failed to marshal message")
var ErrRecieveMsg = errors.New("failed to receive message")
var ErrServerMsg = errors.New("server returned error")
var ErrTimeout = errors.New("server timed out")
var ErrUnknownMessageID = errors.New("unknown message id")
var ErrUnsupportedMsgFormat = errors.New("unsupported message format")

type WebSocketClient struct {
	authorization           *SendMessageAuthorization
	conn                    Conn
	done                    chan struct{}
	Err                     error
	linkByID                map[string]chan *ReceiveMessage
	linkMu                  sync.RWMutex
	subscriptionIDByChannel map[string]string
	subscriptionLinkByID    map[string]chan *SubscriptionMessage
	subscriptionMu          sync.RWMutex
	timeoutDuration         time.Duration
	wg                      sync.WaitGroup
}

func (w *WebSocketClient) Close() error {
	select {
	case <-w.done:
	default:
		close(w.done) // Issue: https://github.com/brokgo/appsync-event-client-go/issues/5
	}
	err := w.conn.Close(websocket.StatusNormalClosure, "")
	w.wg.Wait()
	w.subscriptionMu.Lock()
	defer w.subscriptionMu.Unlock()
	for _, subscriptionC := range w.subscriptionLinkByID {
		select {
		case _, ok := <-subscriptionC:
			if !ok {
				continue
			}
			close(subscriptionC)
		default:
			close(subscriptionC)
		}
	}

	return err
}

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
	var ok bool
	select {
	case <-w.done:
		return nil, w.Err
	case resp, ok = <-linkC:
	}
	if !ok {
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
	err = write(ctx, w.conn, &SendMessage{
		Authorization: w.authorization,
		Type:          SubscribeType,
		ID:            linkID,
		Channel:       channel,
	})
	if err != nil {
		return err
	}
	var resp *ReceiveMessage
	var ok bool
	select {
	case <-w.done:
		return w.Err
	case resp, ok = <-linkC:
	}
	if !ok {
		return w.Err
	}
	if len(resp.Errors) > 0 {
		return errFromMsgErrors(resp.Errors)
	}
	w.subscriptionMu.Lock()
	w.subscriptionLinkByID[linkID] = channelC
	w.subscriptionIDByChannel[channel] = linkID
	w.subscriptionMu.Unlock()

	return nil
}

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
	var ok bool
	select {
	case <-w.done:
		return w.Err
	case resp, ok = <-linkC:
	}
	if !ok {
		return w.Err
	}
	if len(resp.Errors) > 0 {
		return errFromMsgErrors(resp.Errors)
	}
	w.subscriptionMu.Lock()
	delete(w.subscriptionIDByChannel, channel)
	delete(w.subscriptionLinkByID, linkID)
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
					subscriptionC, found := w.subscriptionLinkByID[msg.ID]
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

func read(ctx context.Context, conn Conn, msg any) error {
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

func write(ctx context.Context, conn Conn, msg any) error {
	msgJSON, err := json.Marshal(msg)
	if err != nil {
		return err
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

		return err
	}
	_, err = writer.Write(msgJSON)

	return err
}
