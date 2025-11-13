package appsync_test

import (
	"testing"

	"github.com/brokgo/appsync-event-client-go/appsync"
)

func TestSendMessageAuthorization(t *testing.T) {
	t.Parallel()
	testCases := map[string]struct {
		msgA           *appsync.SendMessageAuthorization
		msgB           *appsync.SendMessageAuthorization
		expectedResult bool
	}{
		"equal": {
			msgA: &appsync.SendMessageAuthorization{
				Authorization:     "authtest",
				Host:              "hosttest",
				XAmzDate:          "xamzdatetes",
				XAmzSecurityToken: "xamzsectest",
				XAPIKey:           "apigeytest",
			},
			msgB: &appsync.SendMessageAuthorization{
				Authorization:     "authtest",
				Host:              "hosttest",
				XAmzDate:          "xamzdatetes",
				XAmzSecurityToken: "xamzsectest",
				XAPIKey:           "apigeytest",
			},
			expectedResult: true,
		},
		"not_equal": {
			msgA: &appsync.SendMessageAuthorization{
				Authorization:     "authtest",
				Host:              "hosttest",
				XAmzDate:          "xamzdatetes",
				XAmzSecurityToken: "xamzsectest",
				XAPIKey:           "apigeytest",
			},
			msgB: &appsync.SendMessageAuthorization{
				Authorization:     "authtest",
				Host:              "hosttest",
				XAmzDate:          "xamzdatetes",
				XAmzSecurityToken: "notequal",
				XAPIKey:           "apigeytest",
			},
			expectedResult: false,
		},
		"nil": {
			msgA: &appsync.SendMessageAuthorization{
				Authorization:     "authtest",
				Host:              "hosttest",
				XAmzDate:          "xamzdatetes",
				XAmzSecurityToken: "xamzsectest",
				XAPIKey:           "apigeytest",
			},
			msgB:           nil,
			expectedResult: false,
		},
	}
	for testName, testParams := range testCases {
		t.Run(testName, func(t *testing.T) {
			t.Parallel()
			if testParams.msgA.Equal(testParams.msgB) != testParams.expectedResult {
				t.Fail()
			}
		})
	}
}

func TestSendMessage(t *testing.T) {
	t.Parallel()
	testCases := map[string]struct {
		msgA           *appsync.SendMessage
		msgB           *appsync.SendMessage
		expectedResult bool
	}{
		"equal": {
			msgA: &appsync.SendMessage{
				Authorization: &appsync.SendMessageAuthorization{
					Host: "abc123",
				},
				Channel: "chantest",
				Events:  []string{"testa", "testb"},
				ID:      "testid",
				Type:    "abc",
			},
			msgB: &appsync.SendMessage{
				Authorization: &appsync.SendMessageAuthorization{
					Host: "abc123",
				},
				Channel: "chantest",
				Events:  []string{"testa", "testb"},
				ID:      "testid",
				Type:    "abc",
			},
			expectedResult: true,
		},
		"not_equal": {
			msgA: &appsync.SendMessage{
				Authorization: &appsync.SendMessageAuthorization{
					Host: "abc123",
				},
				Channel: "chantest",
				Events:  []string{"testa", "testb"},
				ID:      "testid",
				Type:    "abc",
			},
			msgB: &appsync.SendMessage{
				Authorization: &appsync.SendMessageAuthorization{
					Host: "abc123",
				},
				Channel: "chantest",
				Events:  []string{"testa", "testc"},
				ID:      "testid",
				Type:    "abc",
			},
			expectedResult: false,
		},
		"nil": {
			msgA: &appsync.SendMessage{
				Authorization: &appsync.SendMessageAuthorization{
					Host: "abc123",
				},
				Channel: "chantest",
				Events:  []string{"testa", "testb"},
				ID:      "testid",
				Type:    "abc",
			},
			msgB:           nil,
			expectedResult: false,
		},
		"event_len": {
			msgA: &appsync.SendMessage{
				Authorization: &appsync.SendMessageAuthorization{
					Host: "abc123",
				},
				Channel: "chantest",
				Events:  []string{"testa", "testb"},
				ID:      "testid",
				Type:    "abc",
			},
			msgB: &appsync.SendMessage{
				Authorization: &appsync.SendMessageAuthorization{
					Host: "abc123",
				},
				Channel: "chantest",
				Events:  []string{"testa"},
				ID:      "testid",
				Type:    "abc",
			},
			expectedResult: false,
		},
		"event_order": {
			msgA: &appsync.SendMessage{
				Authorization: &appsync.SendMessageAuthorization{
					Host: "abc123",
				},
				Channel: "chantest",
				Events:  []string{"testa", "testb"},
				ID:      "testid",
				Type:    "abc",
			},
			msgB: &appsync.SendMessage{
				Authorization: &appsync.SendMessageAuthorization{
					Host: "abc123",
				},
				Channel: "chantest",
				Events:  []string{"testb", "testa"},
				ID:      "testid",
				Type:    "abc",
			},
			expectedResult: false,
		},
		"eventa_auth_nil": {
			msgA: &appsync.SendMessage{
				Authorization: nil,
				Channel:       "chantest",
				Events:        []string{"testa", "testb"},
				ID:            "testid",
				Type:          "abc",
			},
			msgB: &appsync.SendMessage{
				Authorization: &appsync.SendMessageAuthorization{
					Host: "abc123",
				},
				Channel: "chantest",
				Events:  []string{"testa", "testb"},
				ID:      "testid",
				Type:    "abc",
			},
			expectedResult: false,
		},
		"eventb_auth_nil": {
			msgA: &appsync.SendMessage{
				Authorization: &appsync.SendMessageAuthorization{
					Host: "abc123",
				},
				Channel: "chantest",
				Events:  []string{"testa", "testb"},
				ID:      "testid",
				Type:    "abc",
			},
			msgB: &appsync.SendMessage{
				Authorization: nil,
				Channel:       "chantest",
				Events:        []string{"testa", "testb"},
				ID:            "testid",
				Type:          "abc",
			},
			expectedResult: false,
		},
	}
	for testName, testParams := range testCases {
		t.Run(testName, func(t *testing.T) {
			t.Parallel()
			if testParams.msgA.Equal(testParams.msgB) != testParams.expectedResult {
				t.Fail()
			}
		})
	}
}

func TestSubscriptionMessage(t *testing.T) {
	t.Parallel()
	testCases := map[string]struct {
		msgA           *appsync.SubscriptionMessage
		msgB           *appsync.SubscriptionMessage
		expectedResult bool
	}{
		"equal": {
			msgA: &appsync.SubscriptionMessage{
				Errors: []appsync.MessageError{
					{ErrorType: "errtest1", Message: "msgtest1"},
					{ErrorType: "errtest2", Message: "msgtest2"},
				},
				Event: "subevent",
				Type:  "abc",
			},
			msgB: &appsync.SubscriptionMessage{
				Errors: []appsync.MessageError{
					{ErrorType: "errtest1", Message: "msgtest1"},
					{ErrorType: "errtest2", Message: "msgtest2"},
				},
				Event: "subevent",
				Type:  "abc",
			},
			expectedResult: true,
		},
		"not_equal": {
			msgA: &appsync.SubscriptionMessage{
				Errors: []appsync.MessageError{
					{ErrorType: "errtest1", Message: "msgtest1"},
					{ErrorType: "errtest2", Message: "msgtest2"},
				},
				Event: "subevent",
				Type:  "abc",
			},
			msgB: &appsync.SubscriptionMessage{
				Errors: []appsync.MessageError{
					{ErrorType: "errtest1", Message: "msgtest1"},
					{ErrorType: "errtest2", Message: "notequal"},
				},
				Event: "subevent",
				Type:  "abc",
			},
			expectedResult: false,
		},
		"nil": {
			msgA: &appsync.SubscriptionMessage{
				Errors: []appsync.MessageError{
					{ErrorType: "errtest1", Message: "msgtest1"},
					{ErrorType: "errtest2", Message: "msgtest2"},
				},
				Event: "subevent",
				Type:  "abc",
			},
			msgB:           nil,
			expectedResult: false,
		},
		"error_len": {
			msgA: &appsync.SubscriptionMessage{
				Errors: []appsync.MessageError{},
				Event:  "subevent",
				Type:   "abc",
			},
			msgB: &appsync.SubscriptionMessage{
				Errors: []appsync.MessageError{
					{ErrorType: "errtest1", Message: "msgtest1"},
					{ErrorType: "errtest2", Message: "notequal"},
				},
				Event: "subevent",
				Type:  "abc",
			},
			expectedResult: false,
		},
		"error_order": {
			msgA: &appsync.SubscriptionMessage{
				Errors: []appsync.MessageError{
					{ErrorType: "errtest1", Message: "msgtest1"},
					{ErrorType: "errtest2", Message: "msgtest2"},
				},
				Event: "subevent",
				Type:  "abc",
			},
			msgB: &appsync.SubscriptionMessage{
				Errors: []appsync.MessageError{
					{ErrorType: "errtest2", Message: "msgtest2"},
					{ErrorType: "errtest1", Message: "msgtest1"},
				},
				Event: "subevent",
				Type:  "abc",
			},
			expectedResult: false,
		},
	}
	for testName, testParams := range testCases {
		t.Run(testName, func(t *testing.T) {
			t.Parallel()
			if testParams.msgA.Equal(testParams.msgB) != testParams.expectedResult {
				t.Fail()
			}
		})
	}
}
