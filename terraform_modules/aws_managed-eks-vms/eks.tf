resource aws_eks_cluster nightlies {
  name     = "nightlies"
  role_arn = aws_iam_role.eks_role_assumer.arn

  vpc_config {
    endpoint_private_access = false
    endpoint_public_access  = true

    subnet_ids = [
      # If I use a random provider random numbers could collide so https://xkcd.com/221/
      data.terraform_remote_state.base_framework.outputs.aws_network_private_subnets[0].id,
      data.terraform_remote_state.base_framework.outputs.aws_network_private_subnets[1].id,
      data.terraform_remote_state.base_framework.outputs.aws_network_private_subnets[5].id,
    ]
  }

  kubernetes_network_config {
    service_ipv4_cidr = "10.78.82.0/23"
  }

  depends_on = [
    aws_iam_role_policy_attachment.policy_AmazonEKSClusterPolicy,
    aws_iam_role_policy_attachment.policy_AmazonEKSVPCResourceController,
  ]
}

resource aws_iam_role eks_role_assumer {
  name = "eks-role-assumer"

  assume_role_policy = <<-EOT
          {
            "Version": "2012-10-17",
            "Statement": [
              {
                "Effect": "Allow",
                "Principal": {
                  "Service": "eks.amazonaws.com"
                },
                "Action": "sts:AssumeRole"
              }
            ]
          }
          EOT
}

resource aws_iam_role_policy_attachment policy_AmazonEKSClusterPolicy {
  policy_arn = "arn:aws:iam::aws:policy/AmazonEKSClusterPolicy"
  role       = aws_iam_role.eks_role_assumer.name
}

resource aws_iam_role_policy_attachment policy_AmazonEKSVPCResourceController {
  policy_arn = "arn:aws:iam::aws:policy/AmazonEKSVPCResourceController"
  role       = aws_iam_role.eks_role_assumer.name
}
