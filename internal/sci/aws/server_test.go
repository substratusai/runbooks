package aws_test

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	awsSdk "github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/stretchr/testify/assert"
	"github.com/substratusai/substratus/internal/sci"
	sciAws "github.com/substratusai/substratus/internal/sci/aws"
)

const charset = "abcdefghijklmnopqrstuvwxyz0123456789"

func init() {
	rand.Seed(time.Now().UnixNano())
}

func randomString(length int, charset string) string {
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}

func awsCredentialsPresent() bool {
	sess, err := session.NewSession()
	if err != nil {
		fmt.Printf("Failed to create session: %v\n", err)
		return false
	}

	creds := sess.Config.Credentials
	_, err = creds.Get()
	if err != nil {
		if err == credentials.ErrNoValidProvidersFoundInChain {
			fmt.Println("No AWS credentials found, skipping test")
			return false
		} else {
			fmt.Printf("Failed to retrieve AWS credentials: %v\n", err)
			return false
		}
	}

	// Check if the credentials are expired by making an actual call
	stsSvc := sts.New(sess)
	_, err = stsSvc.GetCallerIdentity(&sts.GetCallerIdentityInput{})
	if err != nil {
		if awsErr, ok := err.(awserr.Error); ok {
			if awsErr.Code() == "ExpiredToken" {
				fmt.Println("AWS credentials have expired, skipping test")
				return false
			}
		}
	}

	return true
}

func TestGetObjectMd5(t *testing.T) {
	if !awsCredentialsPresent() {
		t.Skip("AWS credentials not found")
	}
	envAccountID := os.Getenv("AWS_ACCOUNT_ID")
	if envAccountID == "" {
		t.Skip("Skipping TestGetObjectMd5 because AWS_ACCOUNT_ID is not set")
	}
	envClusterName := os.Getenv("CLUSTER_NAME")
	if envClusterName == "" {
		t.Skip("Skipping TestGetObjectMd5 because CLUSTER_NAME is not set")
	}
	sess, err := session.NewSession()
	assert.NoError(t, err)

	s3Client := s3.New(sess)
	server := &sciAws.Server{
		Clients: sciAws.Clients{
			S3Client: s3Client,
		},
	}

	bucket := "substratus-test-bucket-" + randomString(8, charset)
	object := "test-object"

	_, err = server.Clients.S3Client.CreateBucket(&s3.CreateBucketInput{
		Bucket: &bucket,
	})
	assert.NoError(t, err)

	defer func() {
		listOutput, listErr := server.Clients.S3Client.ListObjectsV2(&s3.ListObjectsV2Input{
			Bucket: &bucket,
		})
		if listErr != nil {
			log.Printf("Error listing objects in bucket %s: %v", bucket, listErr)
			return
		}

		// Delete each object prior to bucket deletion
		for _, object := range listOutput.Contents {
			_, delErr := server.Clients.S3Client.DeleteObject(&s3.DeleteObjectInput{
				Bucket: &bucket,
				Key:    object.Key,
			})
			if delErr != nil {
				log.Printf("Error deleting object %s in bucket %s: %v", *object.Key, bucket, delErr)
			}
		}

		// finally, delete the bucket
		_, err := server.Clients.S3Client.DeleteBucket(&s3.DeleteBucketInput{Bucket: &bucket})
		if err != nil {
			log.Printf("Error deleting bucket %s: %v", bucket, err)
		}
	}()

	_, err = server.Clients.S3Client.PutObject(&s3.PutObjectInput{
		Bucket: &bucket,
		Key:    &object,
		Body:   strings.NewReader("test-data"),
	})
	assert.NoError(t, err)

	resp, err := server.GetObjectMd5(context.TODO(), &sci.GetObjectMd5Request{
		BucketName: bucket,
		ObjectName: object,
	})
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	if resp != nil {
		assert.NotEmpty(t, resp.Md5Checksum)
	}
}

func TestBindIdentity(t *testing.T) {
	if !awsCredentialsPresent() {
		t.Skip("AWS credentials not found")
	}
	// TODO(bjb): see setup techique here: https://pkg.go.dev/testing#hdr-Main
	envAccountID := os.Getenv("AWS_ACCOUNT_ID")
	if envAccountID == "" {
		t.Skip("Skipping TestBindIdentity because AWS_ACCOUNT_ID is not set")
	}
	envClusterName := os.Getenv("CLUSTER_NAME")
	if envClusterName == "" {
		t.Skip("Skipping TestBindIdentity because CLUSTER_NAME is not set")
	}

	sess, err := session.NewSession()
	assert.NoError(t, err)
	iamClient := iam.New(sess)

	oidcProviderURL := "oidc.eks.us-west-2.amazonaws.com/id/C2A3CBF5FF8C55D72C8843756CD44444"
	server := &sciAws.Server{
		Clients: sciAws.Clients{
			IAMClient: iamClient,
		},
		OIDCProviderURL: oidcProviderURL,
		OIDCProviderARN: "arn:aws:iam::243019462222:oidc-provider/" + oidcProviderURL,
	}

	roleName := "test-role" + randomString(8, charset)
	rolePolicy := `{
		"Version": "2012-10-17",
		"Statement": [
		  {
			"Effect": "Allow",
			"Principal": {
			  "Service": "lambda.amazonaws.com"
			},
			"Action": "sts:AssumeRole"
		  }
		]
	  }`

	_, err = server.Clients.IAMClient.CreateRole(&iam.CreateRoleInput{
		RoleName:                 &roleName,
		AssumeRolePolicyDocument: awsSdk.String(rolePolicy),
	})
	assert.NoError(t, err)

	defer func() {
		if _, err := server.Clients.IAMClient.DeleteRole(&iam.DeleteRoleInput{RoleName: &roleName}); err != nil {
			t.Logf("Failed to delete IAM role: %v", err)
		}
	}()

	// Debug: Fetch and print the current trust policy before making the BindIdentity call
	getRoleInput := &iam.GetRoleInput{
		RoleName: awsSdk.String(roleName),
	}
	getRoleOutput, err := server.Clients.IAMClient.GetRole(getRoleInput)
	if err != nil {
		t.Fatalf("Debug: failed to get the role: %v", err)
	}
	t.Logf("Debug: Current Trust Policy: %s", *getRoleOutput.Role.AssumeRolePolicyDocument)

	resp, err := server.BindIdentity(context.TODO(), &sci.BindIdentityRequest{
		Principal:                roleName,
		KubernetesNamespace:      "test-namespace",
		KubernetesServiceAccount: "test-serviceaccount",
	})
	if err != nil {
		t.Fatalf("Error in BindIdentity: %v", err)
	}

	getRoleOutput, err = server.Clients.IAMClient.GetRole(getRoleInput)
	if err != nil {
		t.Fatalf("Debug: failed to get the role: %v", err)
	}

	encodedPolicy := *getRoleOutput.Role.AssumeRolePolicyDocument
	decodedPolicy, err := url.QueryUnescape(encodedPolicy)
	if err != nil {
		t.Fatalf("Error decoding policy document: %v", err)
	}
	assert.Contains(t, decodedPolicy, "system:serviceaccount:test-namespace:test-serviceaccount")
	assert.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestCreateSignedURL(t *testing.T) {
	if !awsCredentialsPresent() {
		t.Skip("AWS credentials not found")
	} else {
		fmt.Println("somehow credentials found?")
	}
	sess, err := session.NewSession(&awsSdk.Config{
		Region: awsSdk.String("us-west-2"),
	})
	assert.NoError(t, err)

	s3Client := s3.New(sess)
	s := &sciAws.Server{
		Clients: sciAws.Clients{
			S3Client: s3Client,
		},
	}
	bucketName := "substratus-test-bucket-" + randomString(8, charset)
	objectName := "test-object.txt"
	content := "test content"
	checksum := "9473fdd0d880a43c21b7778d34872157"
	h := md5.New()
	io.WriteString(h, content)
	calculatedChecksum := fmt.Sprintf("%x", h.Sum(nil))
	if calculatedChecksum != checksum {
		t.Fatalf("MD5 mismatch. Expected %s but got %s", checksum, calculatedChecksum)
	}

	_, err = s.Clients.S3Client.HeadBucket(&s3.HeadBucketInput{
		Bucket: awsSdk.String(bucketName),
	})
	if err == nil {
		_, delErr := s.Clients.S3Client.DeleteBucket(&s3.DeleteBucketInput{
			Bucket: awsSdk.String(bucketName),
		})
		if delErr != nil {
			t.Fatalf("Failed to delete existing bucket: %v", delErr)
		}
	}

	_, err = s.Clients.S3Client.CreateBucket(&s3.CreateBucketInput{
		Bucket: awsSdk.String(bucketName),
	})
	assert.NoError(t, err)

	// Cleanup resources after tests
	defer func() {
		// Delete all objects
		objects, _ := s.Clients.S3Client.ListObjectsV2(&s3.ListObjectsV2Input{
			Bucket: awsSdk.String(bucketName),
		})
		for _, object := range objects.Contents {
			s.Clients.S3Client.DeleteObject(&s3.DeleteObjectInput{
				Bucket: awsSdk.String(bucketName),
				Key:    object.Key,
			})
		}

		_, err := s.Clients.S3Client.DeleteBucket(&s3.DeleteBucketInput{
			Bucket: awsSdk.String(bucketName),
		})
		if err != nil {
			t.Log("Failed to delete bucket:", err)
		}
	}()

	req := &sci.CreateSignedURLRequest{
		BucketName:        bucketName,
		ObjectName:        objectName,
		Md5Checksum:       checksum,
		ExpirationSeconds: 3600,
	}
	resp, err := s.CreateSignedURL(context.TODO(), req)
	assert.NoError(t, err)
	assert.NotEmpty(t, resp.Url)

	// Use the signed URL to PUT the object
	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	// Convert hex MD5 to base64
	data, err := hex.DecodeString(checksum)
	if err != nil {
		t.Fatalf("failed to decode MD5 checksum: %v", err)
	}
	base64md5 := base64.StdEncoding.EncodeToString(data)

	putReq, err := http.NewRequest(http.MethodPut, resp.Url, strings.NewReader(content))
	if err != nil {
		t.Fatalf("failed to create new PUT request: %v", err)
	}

	putReq.Header.Set("Content-MD5", base64md5)
	putReq.Header.Set("Content-Type", "application/octet-stream")

	putRes, err := client.Do(putReq)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, putRes.StatusCode)
	putRes.Body.Close()

	getObjectOutput, err := s.Clients.S3Client.GetObject(&s3.GetObjectInput{
		Bucket: awsSdk.String(bucketName),
		Key:    awsSdk.String(objectName),
	})
	assert.NoError(t, err)

	if getObjectOutput == nil || getObjectOutput.Body == nil {
		t.Fatalf("GetObjectOutput or its Body is nil")
	}
	buf := new(bytes.Buffer)
	_, err = buf.ReadFrom(getObjectOutput.Body)
	assert.NoError(t, err)
	getObjectOutput.Body.Close()

	newContent := buf.String()
	assert.Equal(t, content, newContent)
}
