package appsync

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/brokgo/appsync-event-client-go/appsync/message"
)

// Config is the configuration file for creating the WebSocketClient.
type Config struct {
	Authorization     *message.Authorization
	Headers           map[string]string
	HTTPEndpoint      string
	HTTPProtocol      string
	RealTimeEndpoint  string
	WebSocketProtocol string
}

// NewAPIKeyConfig creates a config for api key authentication. See https://docs.aws.amazon.com/appsync/latest/devguide/security-authz.html for authentication types.
func NewAPIKeyConfig(httpEndpoint, realTimeEndpoint, apiKey string) *Config {
	return &Config{
		Authorization: &message.Authorization{
			Host:    httpEndpoint,
			XAPIKey: apiKey,
		},
		Headers: map[string]string{
			"host":      httpEndpoint,
			"x-api-key": apiKey,
		},
		HTTPEndpoint:      httpEndpoint,
		HTTPProtocol:      "https",
		RealTimeEndpoint:  realTimeEndpoint,
		WebSocketProtocol: "wss",
	}
}

// NewLambdaConfig creates a config for lambda authentication. See https://docs.aws.amazon.com/appsync/latest/devguide/security-authz.html for authentication types.
func NewLambdaConfig(httpEndpoint, realTimeEndpoint, authorizationToken string) *Config {
	return &Config{
		Authorization: &message.Authorization{
			Authorization: authorizationToken,
			Host:          httpEndpoint,
		},
		Headers: map[string]string{
			"host":          httpEndpoint,
			"authorization": authorizationToken,
		},
		HTTPEndpoint:      httpEndpoint,
		HTTPProtocol:      "https",
		RealTimeEndpoint:  realTimeEndpoint,
		WebSocketProtocol: "wss",
	}
}

// Host returns the url of the host.
func (c *Config) Host() (string, error) {
	return url.JoinPath(fmt.Sprintf("%v://", c.HTTPProtocol), c.HTTPEndpoint, "/event")
}

// Subprotocols Returns the websocket subprotocols.
func (c *Config) Subprotocols() ([]string, error) {
	jsonHeaders, err := json.Marshal(c.Headers)
	if err != nil {
		return nil, err
	}

	return []string{"header-" + base64.RawURLEncoding.EncodeToString(jsonHeaders), "aws-appsync-event-ws"}, nil
}

// URL returns the url of the webscoket.
func (c *Config) URL() (string, error) {
	return url.JoinPath(fmt.Sprintf("%v://", c.WebSocketProtocol), c.RealTimeEndpoint, "/event/realtime")
}
