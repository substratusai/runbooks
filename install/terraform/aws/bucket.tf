data "aws_caller_identity" "current" {}

resource "aws_s3_bucket" "artifacts" {
  count  = var.existing_artifacts_bucket == null ? 1 : 0
  bucket = "${data.aws_caller_identity.current.account_id}-${var.name_prefix}-artifacts"
}
