resource kubernetes_config_map aws_auth {
  metadata {
    name = "aws-auth"
    namespace = "kube-system"
  }

  data = {
    mapUsers = yamlencode([
      {
        userarn  = data.terraform_remote_state.base_framework.outputs.iam.aws_iam_user.base_framework.arn,
        username = "kubernetes-admin"
        groups   = [
          "system:masters",
          "system:authenticated",
        ]
      },
      {
        userarn  = "arn:aws:iam::801306408012:role/AWSReservedSSO_NRAdmin_0ca613258382a79b",
        username = "NRAdmin:{{SessionName}}"
        groups   = [
          "system:masters",
          "system:authenticated",
        ]
      },
      {
        userarn  = aws_iam_role.ec2_role_assumer.arn
        username = "system:node:{{EC2PrivateDNSName}}"
        groups   = [
          "system:bootstrappers",
          "system:nodes",
        ]
      }
    ])
  }
}
