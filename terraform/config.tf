provider "aws" {
  region = "ap-northeast-1"
}

terraform {
  required_version = "= 1.0.8"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "= 3.64.2"
    }
  }
  backend "s3" {
    key    = "modern-access-counter/terraform.tfstate"
    region = "ap-northeast-1"
  }
}

