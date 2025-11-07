package main

import (
	"context"

	"github.com/aws/aws-lambda-go/lambda"
)

func handleRequest(ctx context.Context) error {
	return nil
}

func main() {
	lambda.Start(handleRequest)
}
