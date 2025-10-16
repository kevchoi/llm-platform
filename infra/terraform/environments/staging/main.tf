terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 6.0"
    }
    kubernetes = {
      source  = "hashicorp/kubernetes"
      version = "~> 2.38"
    }
    helm = {
      source  = "hashicorp/helm"
      version = "~> 3.0"
    }
  }

  backend "s3" {
    bucket  = "your-terraform-state-bucket"
    key     = "staging/terraform.tfstate"
    region  = "us-east-1"
    encrypt = true
  }
}

module "vpc" {
  source = "../../modules/vpc"

  name = "llm-platform"
  env  = "staging"
}

module "eks" {
  source = "../../modules/eks"

  name       = "llm-platform"
  env        = "staging"
  vpc_id     = module.vpc.vpc_id
  subnet_ids = module.vpc.private_subnets
}

output "vpc_id" {
  description = "The ID of the VPC."
  value       = module.vpc.vpc_id
}

output "public_subnets" {
  description = "List of IDs of public subnets."
  value       = module.vpc.public_subnets
}

output "private_subnets" {
  description = "List of IDs of private subnets."
  value       = module.vpc.private_subnets
}