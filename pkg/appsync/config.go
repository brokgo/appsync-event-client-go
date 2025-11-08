package appsync

type Config struct {
	Authorization     *SendMessageAuthorization
	Headers           map[string]string
	HTTPEndpoint      string
	HTTPProtocol      string
	RealTimeEndpoint  string
	WebSocketProtocol string
}

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
