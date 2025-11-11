package appsync

import (
	"net/url"
)

type Config struct {
	Authorization *SendMessageAuthorization
	Headers       map[string]string
	HTTPURL       string
	RealTimeURL   string
}

func NewAPIKeyConfig(httpEndpoint, realTimeEndpoint, apiKey string) (*Config, error) {
	httpURL, err := url.JoinPath("https://", httpEndpoint, "/event")
	if err != nil {
		return nil, err
	}
	realTimeURL, err := url.JoinPath("wss://", realTimeEndpoint, "/event/realtime")
	if err != nil {
		return nil, err
	}
	config := &Config{
		Authorization: &SendMessageAuthorization{
			Host:    httpEndpoint,
			XAPIKey: apiKey,
		},
		Headers: map[string]string{
			"host":      httpEndpoint,
			"x-api-key": apiKey,
		},
		HTTPURL:     httpURL,
		RealTimeURL: realTimeURL,
	}

	return config, nil
}

func NewLambdaConfig(httpEndpoint, realTimeEndpoint, authorizationToken string) (*Config, error) {
	httpURL, err := url.JoinPath("https://", httpEndpoint, "/event")
	if err != nil {
		return nil, err
	}
	realTimeURL, err := url.JoinPath("wss://", realTimeEndpoint, "/event/realtime")
	if err != nil {
		return nil, err
	}
	config := &Config{
		Authorization: &SendMessageAuthorization{
			Authorization: authorizationToken,
			Host:          httpEndpoint,
		},
		Headers: map[string]string{
			"host":          httpEndpoint,
			"authorization": authorizationToken,
		},
		HTTPURL:     httpURL,
		RealTimeURL: realTimeURL,
	}

	return config, nil
}
