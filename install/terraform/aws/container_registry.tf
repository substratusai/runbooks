resource "aws_ecr_repository" "main" {
  count                = var.existing_ecr_repository_arn == "" ? 1 : 0
  name                 = var.name_prefix
  image_tag_mutability = "MUTABLE"
  image_scanning_configuration {
    scan_on_push = var.image_scan_on_push
  }
}
