// Package gcp provides an AWS implementation of the Substratus Cloud Interface (SCI)
package awsmanager

// Presigned URL example: https://docs.aws.amazon.com/AmazonS3/latest/userguide/example_s3_Scenario_PresignedUrl_section.html
// Checking object integrity: https://docs.aws.amazon.com/AmazonS3/latest/userguide/checking-object-integrity.html

// Update policy: https://docs.aws.amazon.com/sdk-for-go/api/service/iam/#IAM.UpdateAssumeRolePolicy
// REST API: https://docs.aws.amazon.com/IAM/latest/APIReference/API_UpdateAssumeRolePolicy.html

// requires:
// 1. a policy document which is an aws.String having a string encoded JSON blob of the trust policy
// 2. a rolename that we will refer to as the principal here
