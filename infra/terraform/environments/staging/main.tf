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
    key     = "staging/terraform.tfstate"
    region  = "us-east-1"
    encrypt = true
  }
}

locals {
  name         = "llm"
  env          = "staging"
  cluster_name = "${local.name}-${local.env}"
}

# VPC configuration
module "vpc" {
  source  = "terraform-aws-modules/vpc/aws"
  version = "6.4.0"

  name = "${local.name}-${local.env}-vpc"
  cidr = "10.0.0.0/16"

  azs             = ["us-east-1a", "us-east-1b"]
  private_subnets = ["10.0.1.0/24", "10.0.2.0/24"]
  public_subnets  = ["10.0.101.0/24", "10.0.102.0/24"]

  enable_nat_gateway   = true
  single_nat_gateway   = true
  enable_dns_hostnames = true

  public_subnet_tags = {
    "kubernetes.io/role/elb" = 1
  }

  private_subnet_tags = {
    "kubernetes.io/role/internal-elb" = 1
  }
}

data "aws_iam_policy_document" "assume_role" {
  statement {
    effect = "Allow"

    principals {
      type        = "Service"
      identifiers = ["pods.eks.amazonaws.com"]
    }

    actions = [
      "sts:AssumeRole",
      "sts:TagSession"
    ]
  }
}

resource "aws_iam_role" "ebs_csi_driver_role" {
  name               = "${local.cluster_name}-ebs-csi-driver-role"
  assume_role_policy = data.aws_iam_policy_document.assume_role.json
}

resource "aws_iam_role_policy_attachment" "ebs_csi_driver_policy" {
  role       = aws_iam_role.ebs_csi_driver_role.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AmazonEBSCSIDriverPolicy"
}

# EKS cluster configuration
module "eks" {
  source  = "terraform-aws-modules/eks/aws"
  version = "21.4.0"

  name               = local.cluster_name
  kubernetes_version = "1.34"

  addons = {
    coredns = {}
    eks-pod-identity-agent = {
      before_compute = true
    }
    kube-proxy = {}
    vpc-cni = {
      before_compute = true
    }
    aws-ebs-csi-driver = {
      pod_identity_association = [{
        role_arn        = aws_iam_role.ebs_csi_driver_role.arn
        service_account = "ebs-csi-controller-sa"
      }]
    }
  }

  endpoint_public_access                   = true
  enable_cluster_creator_admin_permissions = true

  vpc_id     = module.vpc.vpc_id
  subnet_ids = module.vpc.private_subnets

  eks_managed_node_groups = {
    # General purpose node group for system components (ArgoCD) and monitoring
    default = {
      name           = "${local.cluster_name}-default"
      instance_types = ["t3.large"]
      min_size       = 1
      max_size       = 2
      desired_size   = 1

      labels = {
        "purpose" = "default"
      }

      capacity_type = "SPOT"
    }

    gpu = {
      name           = "${local.cluster_name}-gpu"
      instance_types = ["g5.xlarge"] # Use 8xlarge for EFA
      ami_type       = "AL2023_x86_64_NVIDIA"
      min_size       = 1
      max_size       = 2
      desired_size   = 1

      # Restrict GPU nodes to a single AZ for EFA support?
      # subnet_ids     = [module.vpc.private_subnets[0]]
      # enable_efa_support = true # Note, not supported for xlarge instances (but 8xlarge is supported)

      block_device_mappings = {
        xvda = {
          device_name = "/dev/xvda"
          ebs = {
            volume_size = 64
            volume_type = "gp3"
          }
        }
      }

      labels = {
        "vpc.amazonaws.com/efa.present" = "true"
        "nvidia.com/gpu" = "true"
        "purpose"        = "gpu"
      }

      taints = {
        "nvidia.com/gpu" = {
          key    = "nvidia.com/gpu"
          value  = "true"
          effect = "NO_SCHEDULE"
        }
      }

      capacity_type = "SPOT"
    }
  }
}

data "aws_eks_cluster" "cluster" {
  name = module.eks.cluster_name
}

data "aws_eks_cluster_auth" "cluster" {
  name = module.eks.cluster_name
}

# Kubernetes Provider Configuration
provider "kubernetes" {
  host                   = data.aws_eks_cluster.cluster.endpoint
  cluster_ca_certificate = base64decode(data.aws_eks_cluster.cluster.certificate_authority[0].data)
  token                  = data.aws_eks_cluster_auth.cluster.token
}

# Helm Provider Configuration
provider "helm" {
  kubernetes = {
    host                   = data.aws_eks_cluster.cluster.endpoint
    cluster_ca_certificate = base64decode(data.aws_eks_cluster.cluster.certificate_authority[0].data)
    token                  = data.aws_eks_cluster_auth.cluster.token
  }
}

# ArgoCD Namespace
resource "kubernetes_namespace" "argocd" {
  metadata {
    name = "argocd"
  }

  depends_on = [module.eks]
}

# ArgoCD Installation
resource "helm_release" "argocd" {
  name       = "argocd"
  repository = "https://argoproj.github.io/argo-helm"
  chart      = "argo-cd"
  version    = "9.0.3"
  namespace  = kubernetes_namespace.argocd.metadata[0].name

  depends_on = [module.eks, kubernetes_namespace.argocd]
}

# ArgoCD Application - Needs to be commented out during initial `terraform apply`
resource "kubernetes_manifest" "argocd_app" {
  manifest = {
    apiVersion = "argoproj.io/v1alpha1"
    kind       = "Application"
    metadata = {
      name      = local.cluster_name
      namespace = kubernetes_namespace.argocd.metadata[0].name
    }
    spec = {
      project = "default"
      source = {
        repoURL        = "https://github.com/kevchoi/llm-platform.git"
        targetRevision = "HEAD"
        path           = "infra/k8s/${local.env}/apps"
      }
      destination = {
        server    = "https://kubernetes.default.svc"
        namespace = kubernetes_namespace.argocd.metadata[0].name
      }
      syncPolicy = {
        automated = {
          prune    = true
          selfHeal = true
        }
        syncOptions = ["CreateNamespace=true"]
      }
    }
  }

  depends_on = [helm_release.argocd]
}