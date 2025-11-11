terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 6.0"
    }
  }
}

provider "aws" {
  region = "us-east-1"
}

resource "aws_appsync_api" "e2e" {
  name = "e2e"

  event_config {
    auth_provider {
      auth_type = "AWS_LAMBDA"
      lambda_authorizer_config {
        authorizer_uri = aws_lambda_function.lambda.arn
      }
    }

    connection_auth_mode {
      auth_type = "AWS_LAMBDA"
    }

    default_publish_auth_mode {
      auth_type = "AWS_LAMBDA"
    }

    default_subscribe_auth_mode {
      auth_type = "AWS_LAMBDA"
    }

    auth_provider {
      auth_type = "API_KEY"
    }

    connection_auth_mode {
      auth_type = "API_KEY"
    }

    default_publish_auth_mode {
      auth_type = "API_KEY"
    }

    default_subscribe_auth_mode {
      auth_type = "API_KEY"
    }
  }
}

resource "aws_appsync_channel_namespace" "test_pubsub" {
  name   = "test-pubsub"
  api_id = aws_appsync_api.e2e.api_id
}

# API Key

resource "aws_appsync_api_key" "key" {
  api_id  = aws_appsync_api.e2e.api_id
}

# Lambda

data "aws_iam_policy_document" "assume_lambda" {
  statement {
    actions = ["sts:AssumeRole"]
    principals {
      type        = "Service"
      identifiers = ["lambda.amazonaws.com"]
    }
  }
}

resource "aws_iam_role" "lambda" {
  name               = "AppsyncEventLambda"
  assume_role_policy = data.aws_iam_policy_document.assume_lambda.json
}

data "aws_iam_policy_document" "lambda" {
  statement {
    actions = [
      "appsync:EventConnect",
      "appsync:EventPublish",
      "appsync:EventSubscribe"
    ]
    resources = [aws_appsync_api.e2e.api_arn]
  }
}

resource "aws_iam_policy" "lambda" {
  name   = "AppsyncEventUser"
  policy = data.aws_iam_policy_document.lambda.json
}

resource "aws_iam_role_policy_attachment" "lambda" {
  role       = aws_iam_role.lambda.name
  policy_arn = aws_iam_policy.lambda.arn
}

resource "aws_lambda_function" "lambda" {
  function_name    = "AppsyncEvent"
  architectures    = ["arm64"]
  runtime          = "provided.al2023"
  filename         = "${path.module}/build/lambda.zip"
  source_code_hash = filesha256("${path.module}/build/lambda.zip")
  handler          = "bootstrap"
  timeout          = 10
  role             = aws_iam_role.lambda.arn
}

resource "local_file" "out" {
  filename = "${path.module}/out/data.json"
  content  = jsonencode({
    api_key = aws_appsync_api_key.key.key
    appsync_event = aws_appsync_api.e2e.dns
  })
}
