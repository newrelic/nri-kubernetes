resource aws_eks_node_group nightlies_main_node_group {
  cluster_name    = aws_eks_cluster.nightlies.name
  node_group_name = "main-node-group"
  node_role_arn   = aws_iam_role.ec2_role_assumer.arn
  subnet_ids      = flatten(aws_eks_cluster.nightlies.vpc_config.*.subnet_ids)

  scaling_config {
    desired_size = 2
    max_size     = 4
    min_size     = 2
  }

  update_config {
    max_unavailable = 1
  }

  # Ensure that IAM Role permissions are created before and deleted after EKS Node Group handling.
  # Otherwise, EKS will not be able to properly delete EC2 Instances and Elastic Network Interfaces.
  depends_on = [
    aws_iam_role_policy_attachment.policy_AmazonEKSWorkerNodePolicy,
    aws_iam_role_policy_attachment.policy_AmazonEKS_CNI_Policy,
    aws_iam_role_policy_attachment.policy_AmazonEC2ContainerRegistryReadOnly,
  ]
}

resource aws_iam_role ec2_role_assumer {
  name = "ec2-role-assumer"

  assume_role_policy = <<-EOT
          {
            "Version": "2012-10-17",
            "Statement": [
              {
                "Effect": "Allow",
                "Principal": {
                  "Service": "ec2.amazonaws.com"
                },
                "Action": "sts:AssumeRole"
              }
            ]
          }
          EOT
}

resource aws_iam_role_policy_attachment policy_AmazonEKSWorkerNodePolicy {
  policy_arn = "arn:aws:iam::aws:policy/AmazonEKSWorkerNodePolicy"
  role       = aws_iam_role.ec2_role_assumer.name
}

resource aws_iam_role_policy_attachment policy_AmazonEKS_CNI_Policy {
  policy_arn = "arn:aws:iam::aws:policy/AmazonEKS_CNI_Policy"
  role       = aws_iam_role.ec2_role_assumer.name
}

resource aws_iam_role_policy_attachment policy_AmazonEC2ContainerRegistryReadOnly {
  policy_arn = "arn:aws:iam::aws:policy/AmazonEC2ContainerRegistryReadOnly"
  role       = aws_iam_role.ec2_role_assumer.name
}
