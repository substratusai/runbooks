package main_test

// TODO(any): This test requires some infrastructure that we should build and
// tear down via dedicated terraform templates run through a test harness.
// e.g., service account, a key, a bucket, permissions, etc.
// [terratest](https://terratest.gruntwork.io/examples/) could help orchestrate
// the infra set up and tear down.
// We'll also need to adapt CreateSignedURL to work with a static credential.
// instead of blob signing via IAM.

// Potentially a bug: the generated URL is escaped in a way that means it doesn't
// work with curl. backslashes needed to be deleted throughout.

// The curl command used to test the signed URL was:
// curl -v -X PUT -H "Content-Type: application/octet-stream" --upload-file README.md $url

// the following function was successfully used to exercise gcpmanager.Server.CreateSignedURL()
// func invokeManually(s *gcpmanager.Server) {
// 	payload := sci.CreateSignedURLRequest{
// 		BucketName:        "substratus-ai-001-substratus-notebooks",
// 		ObjectName:        "README.md",
// 		ExpirationSeconds: 600,
// 	}
// 	fmt.Println("calling CreateSignedURL with payload:")

// 	resp, err := s.CreateSignedURL(context.Background(), &payload)
// 	if err != nil {
// 		log.Fatalf("failed to create signed URL: %v", err)
// 	}
// 	fmt.Printf("signed URL: %v\n", resp.Url)
// }
