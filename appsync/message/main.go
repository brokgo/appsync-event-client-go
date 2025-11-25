// Package message contains all the objects that are sent between the server and client.
package message

// SendType is the message types that are sent to the Appsync Event server.
type SendType string

const (
	ConnectionInitType SendType = "connection_init"
	PublishType        SendType = "publish"
	SubscribeType      SendType = "subscribe"
	UnsubscribeType    SendType = "unsubscribe"
)

// Authorization contains the client authentication details. See https://docs.aws.amazon.com/appsync/latest/eventapi/event-api-websocket-protocol.html#authorization-formatting-by-mode
// for more information on authorization formatting.
type Authorization struct {
	Authorization     string `json:"authorization,omitempty"`
	Host              string `json:"host,omitempty"`
	XAmzDate          string `json:"x-amz-date,omitempty"`
	XAmzSecurityToken string `json:"x-amz-security-token,omitempty"`
	XAPIKey           string `json:"x-api-key,omitempty"`
}

// SendMessage are messages that are sent to the Appsync Event server.
type SendMessage struct {
	Authorization *Authorization `json:"authorization,omitempty"`
	Channel       string         `json:"channel,omitempty"`
	Events        []string       `json:"events,omitempty"`
	ID            string         `json:"id,omitempty"`
	Type          SendType       `json:"type"`
}

// ReceiveType is the message types that can be received from the Appsync Event server.
type ReceiveType string

const (
	ConnectionAckType              ReceiveType = "connection_ack"
	ConnectionErrType              ReceiveType = "connection_error"
	ErrorType                      ReceiveType = "error"
	KeepAliveType                  ReceiveType = "ka"
	PublishErrType                 ReceiveType = "publish_error"
	PublishSuccessType             ReceiveType = "publish_success"
	SubscribeErrType               ReceiveType = "subscribe_error"
	SubscribeSuccessType           ReceiveType = "subscribe_success"
	SubscriptionBroadcastErrorType ReceiveType = "broadcast_error"
	SubscriptionDataType           ReceiveType = "data"
	UnsubscribeErrType             ReceiveType = "unsubscribe_error"
	UnsubscribeSuccessType         ReceiveType = "unsubscribe_success"
)

// ErrorData are errors received from the Appsync Event server.
type ErrorData struct {
	ErrorType string `json:"errorType"`
	Message   string `json:"message"`
}

// SubscriptionMessage are the subscription event messages received from the Appsync Event server.
type SubscriptionMessage struct {
	Errors []ErrorData `json:"errors,omitempty"`
	Event  string      `json:"event,omitempty"`
	Type   ReceiveType `json:"type"`
}

// ReceiveEvent are the event identifiers that are used for reporting success or failed of individual events.
type ReceiveEvent struct {
	Identifier string `json:"identifier"`
	Index      int    `json:"index"`
}

// ReceiveMessage are messages that are received from the Appsync Event server.
type ReceiveMessage struct {
	ConnectionTimeoutMs int            `json:"connectionTimeoutMs,omitempty"`
	Errors              []ErrorData    `json:"errors,omitempty"`
	Event               string         `json:"event,omitempty"`
	Failed              []ReceiveEvent `json:"failed,omitempty"`
	Successful          []ReceiveEvent `json:"successful,omitempty"`
	ID                  string         `json:"id,omitempty"`
	Type                ReceiveType    `json:"type"`
}
