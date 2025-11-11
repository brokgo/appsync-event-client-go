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

func (m *SendMessageAuthorization) Equal(otherM *SendMessageAuthorization) bool {
	if otherM == nil {
		return false
	}

	return m.Authorization == otherM.Authorization &&
		m.Host == otherM.Host &&
		m.XAmzDate == otherM.XAmzDate &&
		m.XAmzSecurityToken == otherM.XAmzSecurityToken &&
		m.XAPIKey == otherM.XAPIKey
}

type SendMessage struct {
	Authorization *SendMessageAuthorization `json:"authorization,omitempty"`
	Channel       string                    `json:"channel,omitempty"`
	Events        []string                  `json:"events,omitempty"`
	ID            string                    `json:"id,omitempty"`
	Type          SendMsgType               `json:"type"`
}

func (m *SendMessage) Equal(otherM *SendMessage) bool {
	if otherM == nil {
		return false
	}
	if len(m.Events) != len(otherM.Events) {
		return false
	}
	for i := range m.Events {
		if m.Events[i] != otherM.Events[i] {
			return false
		}
	}
	if m.Authorization == nil && otherM.Authorization != nil {
		return false
	}
	if m.Authorization != nil && !m.Authorization.Equal(otherM.Authorization) {
		return false
	}

	return m.Channel == otherM.Channel &&
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

func (m *SubscriptionMessage) Equal(otherM *SubscriptionMessage) bool {
	if otherM == nil {
		return false
	}
	if len(m.Errors) != len(otherM.Errors) {
		return false
	}
	for i := range m.Errors {
		if m.Errors[i] != otherM.Errors[i] {
			return false
		}
	}

	return m.Event == otherM.Event &&
		m.Type == otherM.Type
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
