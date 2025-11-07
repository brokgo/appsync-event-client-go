package appsync

type SendMsgType string

const (
	ConnectionInitType SendMsgType = "connection_init"
	PublishType        SendMsgType = "publish"
	SubscribeType      SendMsgType = "subscribe"
	UnsubscribeType    SendMsgType = "unsubscribe"
)

type SendMessageAuthorization struct {
	Authorization     string `json:"authorization,omitempty"`
	Host              string `json:"host,omitempty"`
	XAmzDate          string `json:"x-amz-date,omitempty"`
	XAmzSecurityToken string `json:"x-amz-security-token,omitempty"`
	XAPIKey           string `json:"x-api-key,omitempty"`
}

type SendMessage struct {
	Authorization *SendMessageAuthorization `json:"authorization,omitempty"`
	Channel       string                    `json:"channel,omitempty"`
	Events        []string                  `json:"events,omitempty"`
	ID            string                    `json:"id,omitempty"`
	Type          SendMsgType               `json:"type"`
}

func (m *SendMessage) Equal(otherM *SendMessage) bool {
	if len(m.Events) != len(otherM.Events) {
		return false
	}
	for i := range m.Events {
		if m.Events[i] != otherM.Events[i] {
			return false
		}
	}

	return m.Authorization == otherM.Authorization &&
		m.Channel == otherM.Channel &&
		m.ID == otherM.ID &&
		m.Type == otherM.Type
}

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

type MessageError struct {
	ErrorType string `json:"errorType"`
	Message   string `json:"message"`
}

type SubscriptionMessage struct {
	Errors []MessageError `json:"errors,omitempty"`
	Event  string         `json:"event,omitempty"`
	Type   ReceiveMsgType `json:"type"`
}

type ReceiveMessageEventID struct {
	Identifier string `json:"identifier"`
	Index      int    `json:"index"`
}

type ReceiveMessage struct {
	ConnectionTimeoutMs int                     `json:"connectionTimeoutMs,omitempty"`
	Errors              []MessageError          `json:"errors,omitempty"`
	Event               string                  `json:"event,omitempty"`
	Failed              []ReceiveMessageEventID `json:"failed,omitempty"`
	Successful          []ReceiveMessageEventID `json:"successful,omitempty"`
	ID                  string                  `json:"id,omitempty"`
	Type                ReceiveMsgType          `json:"type"`
}
