// Package sciaws provides an AWS implementation of the Substratus Cloud Interface (SCI)
package aws

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	awsSdk "github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/eks"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/substratusai/substratus/internal/sci"
)

type Server struct {
	sci.UnimplementedControllerServer
	OIDCProviderURL string
	OIDCProviderARN string
	Clients
}

type Clients struct {
	S3Client  *s3.S3
	IAMClient *iam.IAM
}

func (s *Server) GetObjectMd5(ctx context.Context, req *sci.GetObjectMd5Request) (*sci.GetObjectMd5Response, error) {
	// ensure the object is accessible
	bucketName, objectName := req.GetBucketName(), req.GetObjectName()
	input := &s3.HeadObjectInput{
		Bucket: awsSdk.String(bucketName),
		Key:    awsSdk.String(objectName),
	}
	headResult, err := s.Clients.S3Client.HeadObject(input)
	if err != nil {
		return nil, err
	}

	// NOTE: AWS returns an MD5 checksum as an ETag except for multi-part uploads where it's an MD5 with a dash suffix.
	if headResult.ETag == nil {
		return nil, fmt.Errorf("object does not exist: %s", s3.ErrCodeNoSuchKey)
	}

	md5 := *headResult.ETag

	return &sci.GetObjectMd5Response{
		Md5Checksum: md5,
	}, nil
}

func (s *Server) CreateSignedURL(ctx context.Context, req *sci.CreateSignedURLRequest) (*sci.CreateSignedURLResponse, error) {
	bucketName, objectName, checksum := req.GetBucketName(),
		req.GetObjectName(),
		req.GetMd5Checksum()

	// Convert hex MD5 to base64
	data, err := hex.DecodeString(checksum)
	if err != nil {
		return nil, fmt.Errorf("failed to decode MD5 checksum: %w", err)
	}
	base64md5 := base64.StdEncoding.EncodeToString(data)

	reqInput := &s3.PutObjectInput{
		Bucket:      awsSdk.String(bucketName),
		Key:         awsSdk.String(objectName),
		ContentType: awsSdk.String("application/octet-stream"),
		ContentMD5:  awsSdk.String(base64md5),
	}

	expiration := time.Duration(req.GetExpirationSeconds()) * time.Second
	putReq, _ := s.Clients.S3Client.PutObjectRequest(reqInput)
	url, err := putReq.Presign(expiration)
	if err != nil {
		return nil, fmt.Errorf("failed to presign request: %w", err)
	}
	return &sci.CreateSignedURLResponse{Url: url}, nil
}

func (s *Server) BindIdentity(ctx context.Context, req *sci.BindIdentityRequest) (*sci.BindIdentityResponse, error) {
	// Fetch the current trust policy
	getRoleInput := &iam.GetRoleInput{
		RoleName: awsSdk.String(req.Principal),
	}
	getRoleOutput, err := s.Clients.IAMClient.GetRole(getRoleInput)
	if err != nil {
		return nil, fmt.Errorf("failed to get the role: %v", err)
	}

	// URL decode the trust policy before decoding
	decodedPolicy, err := url.QueryUnescape(*getRoleOutput.Role.AssumeRolePolicyDocument)
	if err != nil {
		return nil, fmt.Errorf("failed to decode trust policy: %v", err)
	}

	// Decode the current trust policy
	var existingTrustPolicy map[string]interface{}
	if err := json.Unmarshal([]byte(decodedPolicy), &existingTrustPolicy); err != nil {
		return nil, fmt.Errorf("failed to unmarshal trust policy: %v", err)
	}

	subValue := fmt.Sprintf("system:serviceaccount:%s:%s", req.KubernetesNamespace, req.KubernetesServiceAccount)

	// Check if the OIDC provider's trust relationship already exists
	statements := existingTrustPolicy["Statement"].([]interface{})
	alreadyExists := false
	for _, stmt := range statements {
		stmtMap := stmt.(map[string]interface{})
		if principal, ok := stmtMap["Principal"].(map[string]interface{}); ok {
			if federated, ok := principal["Federated"].(string); ok && federated == s.OIDCProviderARN {
				condition := stmtMap["Condition"].(map[string]interface{})["StringEquals"].(map[string]interface{})
				condition[fmt.Sprintf("%s:sub", s.OIDCProviderURL)] = subValue
				alreadyExists = true
				break
			}
		}
	}

	// Construct the new trust relationship
	newTrustRelationship := map[string]interface{}{
		"Effect": "Allow",
		"Principal": map[string]interface{}{
			"Federated": s.OIDCProviderARN,
		},
		"Action": "sts:AssumeRoleWithWebIdentity",
		"Condition": map[string]interface{}{
			"StringEquals": map[string]string{
				fmt.Sprintf("%s:sub", s.OIDCProviderURL): subValue,
			},
		},
	}
	if !alreadyExists {
		// Append the new trust relationship to the existing policy
		existingTrustPolicy["Statement"] = append(statements, newTrustRelationship)
	}

	updatedTrustPolicy, err := json.Marshal(existingTrustPolicy)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal updated trust policy: %v", err)
	}

	// Apply the updated policy
	input := &iam.UpdateAssumeRolePolicyInput{
		PolicyDocument: awsSdk.String(string(updatedTrustPolicy)),
		RoleName:       awsSdk.String(req.Principal),
	}

	_, err = s.Clients.IAMClient.UpdateAssumeRolePolicy(input)
	if err != nil {
		return nil, fmt.Errorf("failed to update trust policy: %v", err)
	}

	return &sci.BindIdentityResponse{}, nil
}

func GetAccountID(stsSvc *sts.STS) (string, error) {
	result, err := stsSvc.GetCallerIdentity(&sts.GetCallerIdentityInput{})
	if err == nil {
		return *result.Account, nil
	}

	// Fall back to the environment variable if the sts:GetCallerIdentity call fails
	envAccountID := os.Getenv("AWS_ACCOUNT_ID")
	if envAccountID != "" {
		return envAccountID, nil
	}

	return "", fmt.Errorf("failed to determine AWS account ID from both STS and environment variable")
}

func GetClusterID() (string, error) {
	clusterID := os.Getenv("CLUSTER_NAME")
	if clusterID == "" {
		return "", fmt.Errorf("CLUSTER_NAME env var not found")
	}
	return clusterID, nil
}

func GetOidcProviderUrl(sess *session.Session, clusterName string) (string, error) {
	svc := eks.New(sess)
	input := &eks.DescribeClusterInput{
		Name: aws.String(clusterName),
	}

	result, err := svc.DescribeCluster(input)
	if err != nil {
		return "", err
	}

	return *result.Cluster.Identity.Oidc.Issuer, nil
}
