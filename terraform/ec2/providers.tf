terraform {
  backend "s3" {
    bucket = "temporal-poc-state-bucket"
    key    = "ec2/terraform.tfstate"
    region = "us-east-1"
  }

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 6"
    }
  }
}

provider "aws" {
  region = var.region
}
