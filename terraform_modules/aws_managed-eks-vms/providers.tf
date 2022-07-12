terraform {
  required_providers {
    aws        = {
      source  = "hashicorp/aws"
      version = ">= 3.20.0"
    }
    kubernetes = {
      source  = "hashicorp/kubernetes"
      version = ">= 2.9.0"
    }
  }

  required_version = ">= 0.14"

  backend s3 {
    bucket         = "nr-coreint-terraform-tfstates"
    dynamodb_table = "nr-coreint-terraform-locking"
    key            = "tests/eks.tfstate"
    region         = "us-east-1"
    profile        = "base-framework"
  }
}

# ########################################### #
#  AWS                                        #
# ########################################### #
provider aws {
  region  = var.aws_region
  profile = var.aws_profile

  default_tags {
    tags = {
      "owning_team" = "COREINT"
      "purpose"     = "eks-testing"
    }
  }
}

# Variables so we can change them using Environment variables.
variable aws_region {
  type    = string
  default = "us-east-1"
}
variable aws_profile {
  type    = string
  default = "coreint"
}

# ########################################### #
#  Kubernetes                                 #
# ########################################### #
provider "kubernetes" {
  host                   = aws_eks_cluster.nightlies.endpoint
  cluster_ca_certificate = base64decode(aws_eks_cluster.nightlies.certificate_authority[0].data)  # HARDCODED
  exec {
    api_version = "client.authentication.k8s.io/v1alpha1"
    command     = "aws"
    args        = ["eks", "get-token", "--cluster-name", aws_eks_cluster.nightlies.name]
    env         = {
      AWS_PROFILE : "coreint"
    }
  }
}
