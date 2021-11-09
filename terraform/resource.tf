variable "data_bucket" {}

resource "aws_s3_bucket" "data" {
  bucket = var.data_bucket
  acl    = "private"
}

resource "aws_iam_role" "function" {
  name = "modern-access-counter"
  path = "/"

  assume_role_policy = jsonencode({
    "Version" : "2012-10-17",
    "Statement" : [
      {
        "Effect" : "Allow",
        "Principal" : {
          "Service" : "lambda.amazonaws.com"
        },
        "Action" : "sts:AssumeRole"
      }
    ]
  })
}

resource "aws_iam_role_policy" "function" {
  name = "modern-access-counter"
  role = aws_iam_role.function.id

  policy = jsonencode(
    {
      Version = "2012-10-17"
      Statement = [
        {
          Sid = "LambdaCloudWatchLog"
          "Action" = [
            "logs:CreateLogGroup",
            "logs:CreateLogStream",
            "logs:PutLogEvents",
          ]
          "Effect"   = "Allow",
          "Resource" = "*",
        },
        {
          Sid = "setddblock"
          "Action" = [
            "dynamodb:CreateTable",
            "dynamodb:UpdateTimeToLive",
            "dynamodb:PutItem",
            "dynamodb:DescribeTable",
            "dynamodb:GetItem",
            "dynamodb:UpdateItem",
          ]
          "Effect"   = "Allow",
          "Resource" = "*",
        },
        {
          Sid = "s3Aaccess"
          "Action" = [
            "s3:GetObject",
            "s3:PutObject",
          ]
          "Effect" = "Allow",
          "Resource" = [
            aws_s3_bucket.data.arn,
            "${aws_s3_bucket.data.arn}/*"
          ]
        },
      ]
    }
  )
}

data "aws_lambda_function" "main" {
  function_name = "modern-access-counter"
}

resource "aws_apigatewayv2_api" "main" {
  name          = "modern-access-counter"
  protocol_type = "HTTP"
}

resource "aws_apigatewayv2_stage" "default" {
  api_id      = aws_apigatewayv2_api.main.id
  name        = "$default"
  auto_deploy = true
}
resource "aws_apigatewayv2_integration" "main" {
  api_id = aws_apigatewayv2_api.main.id

  integration_uri        = data.aws_lambda_function.main.arn
  integration_type       = "AWS_PROXY"
  integration_method     = "POST"
  payload_format_version = "2.0"
}

resource "aws_lambda_permission" "api_gw" {
  statement_id  = "AllowExecutionFromAPIGateway"
  action        = "lambda:InvokeFunction"
  function_name = data.aws_lambda_function.main.function_name
  principal     = "apigateway.amazonaws.com"

  source_arn = "${aws_apigatewayv2_api.main.execution_arn}/*/*"
}

resource "aws_apigatewayv2_route" "root" {
  api_id = aws_apigatewayv2_api.main.id

  route_key = "ANY /"
  target    = "integrations/${aws_apigatewayv2_integration.main.id}"
}

resource "aws_apigatewayv2_route" "gif" {
  api_id = aws_apigatewayv2_api.main.id

  route_key = "ANY /counter.gif"
  target    = "integrations/${aws_apigatewayv2_integration.main.id}"
}
