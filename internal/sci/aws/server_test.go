package aws

import (
	"context"
	"log"
	"math/rand"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	awssdk "github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/stretchr/testify/assert"
	"github.com/substratusai/substratus/internal/sci"
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

func TestGetObjectMd5(t *testing.T) {
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
	server := &Server{
		Clients: Clients{
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
		_, err := server.Clients.S3Client.DeleteBucket(&s3.DeleteBucketInput{Bucket: &bucket})
		if err != nil {
			log.Printf("Error deleting bucket %s: %v", bucket, err)
		}
	}()

	// Upload an object
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
	server := &Server{
		Clients: Clients{
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
		AssumeRolePolicyDocument: awssdk.String(rolePolicy),
	})
	assert.NoError(t, err)

	defer func() {
		if _, err := server.Clients.IAMClient.DeleteRole(&iam.DeleteRoleInput{RoleName: &roleName}); err != nil {
			t.Logf("Failed to delete IAM role: %v", err)
		}
	}()

	// Debug: Fetch and print the current trust policy before making the BindIdentity call
	getRoleInput := &iam.GetRoleInput{
		RoleName: awssdk.String(roleName),
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
