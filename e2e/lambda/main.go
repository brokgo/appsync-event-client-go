package main

import (
	"github.com/aws/aws-lambda-go/lambda"
)

func handleRequest() error {
	// Issue: https://github.com/brokgo/appsync-event-client-go/issues/12
	return nil
}

func main() {
	lambda.Start(handleRequest)
}
