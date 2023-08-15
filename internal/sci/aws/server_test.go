package aws

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"testing"
	"time"

	awssdk "github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/stretchr/testify/assert"
	"github.com/substratusai/substratus/internal/sci"
	"google.golang.org/api/sts/v1"
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

func NewServer() (*Server, error) {
	sess, err := session.NewSession()
	if err != nil {
		return nil, fmt.Errorf("failed to create AWS session: %w", err)
	}

	clusterID, err := GetClusterID()
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster ID: %w", err)
	}

	ec2Svc := ec2metadata.New(sess)
	region, err := GetRegion(ec2Svc)
	if err != nil {
		return nil, fmt.Errorf("failed to get region: %w", err)
	}

	stsSvc := sts.New(sess)
	accountId, err := GetAccountID(stsSvc)
	if err != nil {
		return nil, fmt.Errorf("failed to get account ID: %w", err)
	}

	// TODO(bjb): I think we need another cluster identifier (oidc provider id)
	// oidcProviderURL := "oidc.eks.us-west-2.amazonaws.com/id/C2A3CBF5FF8C55D72C8843756CD44444"
	// oidcProviderARN := "arn:aws:iam::243019462222:oidc-provider/" + oidcProviderURL
	oidcProviderURL := fmt.Sprintf("oidc.eks.%s.amazonaws.com/id/%s", region, clusterID)
	oidcProviderARN := fmt.Sprintf("arn:aws:iam::%s:oidc-provider/%s", accountId, oidcProviderURL)

	c := &Clients{
		S3Client:  s3.New(sess),
		IamClient: iam.New(sess),
	}

	return &Server{
		Clients:         *c,
		OIDCProviderURL: oidcProviderURL,
		OIDCProviderARN: oidcProviderARN,
	}, nil
}

func TestGetObjectMd5(t *testing.T) {
	server, err := getTestServer()
	if err != nil {
		t.Fatal(err)
	}
	bucket := "substratus-test-bucket-" + randomString(8, charset)
	object := "test-object"

	_, err = server.Clients.S3Client.CreateBucket(&s3.CreateBucketInput{
		Bucket: &bucket,
	})
	assert.NoError(t, err)

	defer server.Clients.S3Client.DeleteBucket(&s3.DeleteBucketInput{Bucket: &bucket})

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
	server, err := getTestServer()
	if err != nil {
		t.Fatal(err)
	}

	server := &Server{
		Clients: Clients{
			IamClient: iamClient,
		},
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

	_, err = iamClient.CreateRole(&iam.CreateRoleInput{
		RoleName:                 &roleName,
		AssumeRolePolicyDocument: awssdk.String(rolePolicy),
	})
	assert.NoError(t, err)

	defer func() {
		if _, err := iamClient.DeleteRole(&iam.DeleteRoleInput{RoleName: &roleName}); err != nil {
			t.Logf("Failed to delete IAM role: %v", err)
		}
	}()

	// Debug: Fetch and print the current trust policy before making the BindIdentity call
	getRoleInput := &iam.GetRoleInput{
		RoleName: awssdk.String(roleName),
	}
	getRoleOutput, err := iamClient.GetRole(getRoleInput)
	if err != nil {
		t.Fatalf("Debug: failed to get the role: %v", err)
	}
	t.Logf("Debug: Current Trust Policy: %s", *getRoleOutput.Role.AssumeRolePolicyDocument)

	resp, err := server.BindIdentity(context.TODO(), &sci.BindIdentityRequest{
		Principal:                roleName,
		KubernetesNamespace:      "test-namespace",
		KubernetesServiceAccount: "test-serviceaccount",
	})
	assert.NoError(t, err)
	assert.NotNil(t, resp)
}
