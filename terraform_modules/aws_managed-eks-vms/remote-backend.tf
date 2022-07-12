data terraform_remote_state base_framework {
  backend = "s3"

  config = {
    bucket         = "nr-coreint-terraform-tfstates"
    dynamodb_table = "nr-coreint-terraform-locking"
    key            = "base-framework/global-state-store.tfstate"
    region         = "us-east-1"
    profile        = "base-framework"
  }
}
