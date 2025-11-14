package appsync

// Config is the configuration file for creating the client.
type Config struct {
	Authorization     *SendMessageAuthorization
	Headers           map[string]string
	HTTPEndpoint      string
	HTTPProtocol      string
	RealTimeEndpoint  string
	WebSocketProtocol string
}

// NewAPIKeyConfig creates a config for api key authentication. See https://docs.aws.amazon.com/appsync/latest/devguide/security-authz.html.
func NewAPIKeyConfig(httpEndpoint, realTimeEndpoint, apiKey string) *Config {
	return &Config{
		Authorization: &SendMessageAuthorization{
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

// NewLambdaConfig creates a config for lambda authentication. See https://docs.aws.amazon.com/appsync/latest/devguide/security-authz.html.
func NewLambdaConfig(httpEndpoint, realTimeEndpoint, authorizationToken string) *Config {
	return &Config{
		Authorization: &SendMessageAuthorization{
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
