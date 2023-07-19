package main_test

// TODO(any): This test requires some infrastructure that we should build and
// tear down via dedicated terraform templates run through a test harness.
// e.g., service account, a key, a bucket, permissions, etc.
// [terratest](https://terratest.gruntwork.io/examples/) could help orchestrate
// the infra set up and tear down.

// The curl command used to test the signed URL was:
// URL=$(kubectl get Notebook falcon-7b-instruct -o json | jq '.Status.ImageStatus.uploadURL' -r)
// curl -v -X PUT \
//		-H "Content-Type: application/octet-stream" \
//		-H "Content-MD5: $(openssl dgst -md5 -binary the-file.tar.gz | openssl base64)" \
//		--upload-file the-file.tar.gz \
//		$URL

// the following function was successfully used to exercise gcpmanager.Server.CreateSignedURL()
// func invokeManually(storageClient *storage.Client) {
// 	payload := sci.CreateSignedURLRequest{
// 		BucketName:        "substratus-ai-001-substratus-notebooks",
// 		ObjectName:        "notebook.tar.gz",
// 		ExpirationSeconds: 300,
// 	}
// 	serv := gcpmanager.Server{
// 		StorageClient: storageClient,
// 	}
// 	fmt.Println("calling CreateSignedURL with payload:")

//		resp, err := serv.CreateSignedURL(context.Background(), &payload)
//		if err != nil {
//			log.Fatalf("failed to create signed URL: %v", err)
//		}
//		fmt.Printf("signed URL: %v\n", resp.Url)
//	}
