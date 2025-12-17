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

resource "local_file" "out" {
  filename = "${path.module}/out/data.json"
  content  = jsonencode({
    api_key = aws_appsync_api_key.key.key
    appsync_event = aws_appsync_api.e2e.dns
  })
}
