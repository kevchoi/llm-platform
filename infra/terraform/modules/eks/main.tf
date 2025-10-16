variable "name" {
  description = "The base name for the resources."
  type        = string
}

variable "env" {
  description = "The environment name (e.g., staging, prod)."
  type        = string
}

variable "vpc_id" {
  description = "The VPC ID where the EKS cluster will be deployed."
  type        = string
}

variable "subnet_ids" {
  description = "A list of subnet IDs for the EKS cluster."
  type        = list(string)
}

module "eks" {
  source  = "terraform-aws-modules/eks/aws"
  version = "21.4.0"

  cluster_name    = "${var.name}-${var.env}-eks"
  cluster_version = "1.34"

  cluster_endpoint_public_access           = true
  enable_cluster_creator_admin_permissions = true

  vpc_id     = var.vpc_id
  subnet_ids = var.subnet_ids

  eks_managed_node_groups = {
    one = {
      name           = "node-group-1"
      instance_types = ["t3.micro"]
      min_size       = 1
      max_size       = 1
      desired_size   = 1
    }
  }
}

output "cluster_arn" {
  description = "The Amazon Resource Name (ARN) of the cluster."
  value       = module.eks.cluster_arn
}

output "cluster_endpoint" {
  description = "The endpoint for your EKS Kubernetes API."
  value       = module.eks.cluster_endpoint
}

output "cluster_name" {
  description = "The name of the EKS cluster."
  value       = module.eks.cluster_name
}