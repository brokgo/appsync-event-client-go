package appsync

// SendMsgType is the message types that are sent to the Appsync Event server.
type SendMsgType string

const (
	ConnectionInitType SendMsgType = "connection_init"
	PublishType        SendMsgType = "publish"
	SubscribeType      SendMsgType = "subscribe"
	UnsubscribeType    SendMsgType = "unsubscribe"
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
	Type          SendMsgType    `json:"type"`
}

// ReceiveMsgType is the message types that can be received from the Appsync Event server.
type ReceiveMsgType string

const (
	ConnectionAckType              ReceiveMsgType = "connection_ack"
	ConnectionErrType              ReceiveMsgType = "connection_error"
	ErrorType                      ReceiveMsgType = "error"
	KeepAliveType                  ReceiveMsgType = "ka"
	PublishErrType                 ReceiveMsgType = "publish_error"
	PublishSuccessType             ReceiveMsgType = "publish_success"
	SubscribeErrType               ReceiveMsgType = "subscribe_error"
	SubscribeSuccessType           ReceiveMsgType = "subscribe_success"
	SubscriptionBroadcastErrorType ReceiveMsgType = "broadcast_error"
	SubscriptionDataType           ReceiveMsgType = "data"
	UnsubscribeErrType             ReceiveMsgType = "unsubscribe_error"
	UnsubscribeSuccessType         ReceiveMsgType = "unsubscribe_success"
)

// MessageError are errors received from the Appsync Event server.
type MessageError struct {
	ErrorType string `json:"errorType"`
	Message   string `json:"message"`
}

// SubscriptionMessage are the subscription event messages received from the Appsync Event server.
type SubscriptionMessage struct {
	Errors []MessageError `json:"errors,omitempty"`
	Event  string         `json:"event,omitempty"`
	Type   ReceiveMsgType `json:"type"`
}

// ReceiveMessageEventID are the event identifiers that are used for reporting success or failed of individual events.
type ReceiveMessageEventID struct {
	Identifier string `json:"identifier"`
	Index      int    `json:"index"`
}

// ReceiveMessage are messages that are received from the Appsync Event server.
type ReceiveMessage struct {
	ConnectionTimeoutMs int                     `json:"connectionTimeoutMs,omitempty"`
	Errors              []MessageError          `json:"errors,omitempty"`
	Event               string                  `json:"event,omitempty"`
	Failed              []ReceiveMessageEventID `json:"failed,omitempty"`
	Successful          []ReceiveMessageEventID `json:"successful,omitempty"`
	ID                  string                  `json:"id,omitempty"`
	Type                ReceiveMsgType          `json:"type"`
}
