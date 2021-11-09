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
