package appsync

// Config is the configuration file for creating the WebSocketClient.
type Config struct {
	Authorization     *Authorization
	Headers           map[string]string
	HTTPEndpoint      string
	HTTPProtocol      string
	RealTimeEndpoint  string
	WebSocketProtocol string
}

// NewAPIKeyConfig creates a config for api key authentication. See https://docs.aws.amazon.com/appsync/latest/devguide/security-authz.html for authentication types.
func NewAPIKeyConfig(httpEndpoint, realTimeEndpoint, apiKey string) *Config {
	return &Config{
		Authorization: &Authorization{
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
		Authorization: &Authorization{
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
