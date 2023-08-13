// Package aws provides an AWS implementation of the Substratus Cloud Interface (SCI)
package aws

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"time"

	awsSdk "github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/substratusai/substratus/internal/sci"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type Server struct {
	sci.UnimplementedControllerServer
	OIDCProviderURL string
	OIDCProviderARN string
	Clients
}

type Clients struct {
	S3Client  *s3.S3
	IamClient *iam.IAM
}

func NewAWSServer() (*Server, error) {
	sess, err := session.NewSession()
	if err != nil {
		return nil, fmt.Errorf("failed to create AWS session: %w", err)
	}

	clusterID, err := getClusterID()
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster ID: %w", err)
	}

	ec2Svc := ec2metadata.New(sess)
	region, err := getRegion(ec2Svc)
	if err != nil {
		return nil, fmt.Errorf("failed to get region: %w", err)
	}

	stsSvc := sts.New(sess)
	accountId, err := getAccountID(stsSvc)
	if err != nil {
		return nil, fmt.Errorf("failed to get account ID: %w", err)
	}

	OIDCProviderURL := fmt.Sprintf("oidc.eks.%s.amazonaws.com/id/%s", region, clusterID)
	OIDCProviderARN := fmt.Sprintf("arn:aws:iam::%s:oidc-provider/%s", accountId, OIDCProviderURL)

	c := &Clients{
		S3Client:  s3.New(sess),
		IamClient: iam.New(sess),
	}

	return &Server{
		Clients:         *c,
		OIDCProviderURL: OIDCProviderURL,
		OIDCProviderARN: OIDCProviderARN,
	}, nil
}

func (s *Server) GetObjectMd5(ctx context.Context, req *sci.GetObjectMd5Request) (*sci.GetObjectMd5Response, error) {
	headResult, err := s.getObjectMetadata(s.Clients.S3Client, req.BucketName, req.ObjectName)
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

	// verify that the object doesn't exist
	if _, err := s.getObjectMetadata(s.Clients.S3Client, bucketName, objectName); err != nil {
		return nil, err
	}

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

func (s *Server) getObjectMetadata(sc *s3.S3, bucketName, objectKey string) (*s3.HeadObjectOutput, error) {
	input := &s3.HeadObjectInput{
		Bucket: awsSdk.String(bucketName),
		Key:    awsSdk.String(objectKey),
	}
	result, err := sc.HeadObject(input)

	if err != nil {
		awsErr, ok := err.(awserr.Error)
		if ok {
			if awsErr.Code() == s3.ErrCodeNoSuchKey {
				return &s3.HeadObjectOutput{}, nil
			}
		}
		// An error occurred that was NOT s3.ErrCodeNoSuchKey.
		// This is an unexpected error and we should return it.
		return &s3.HeadObjectOutput{}, err
	}
	return result, nil
}
func (s *Server) BindIdentity(ctx context.Context, req *sci.BindIdentityRequest) (*sci.BindIdentityResponse, error) {
	// Fetch the current trust policy
	getRoleInput := &iam.GetRoleInput{
		RoleName: awsSdk.String(req.Principal),
	}
	getRoleOutput, err := s.Clients.IamClient.GetRole(getRoleInput)
	if err != nil {
		return nil, fmt.Errorf("failed to get the role: %v", err)
	}

	// Decode the current trust policy
	var existingTrustPolicy map[string]interface{}
	if err := json.Unmarshal([]byte(*getRoleOutput.Role.AssumeRolePolicyDocument), &existingTrustPolicy); err != nil {
		return nil, fmt.Errorf("failed to unmarshal trust policy: %v", err)
	}

	subValue := fmt.Sprintf("system:serviceaccount:%s:%s", req.KubernetesNamespace, req.KubernetesServiceAccount)

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

	_, err = s.Clients.IamClient.UpdateAssumeRolePolicy(input)
	if err != nil {
		// handle AWS-specific errors
		// ... (as per your existing error handling code) ...
		return nil, fmt.Errorf("failed to update trust policy: %v", err)
	}

	return &sci.BindIdentityResponse{}, nil
}

func getAccountID(stsSvc *sts.STS) (string, error) {
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

func getClusterID() (string, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return "", fmt.Errorf("failed to set up in-cluster config: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return "", fmt.Errorf("failed to create clientset: %w", err)
	}

	configMapName := "substratus-global"
	configMapNamespace := "substratus"
	configMapKey := "cluster_id"

	configMap, err := clientset.CoreV1().ConfigMaps(configMapNamespace).Get(context.TODO(), configMapName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to fetch ConfigMap: %w", err)
	}

	// Extract cluster ID from the ConfigMap
	clusterID, exists := configMap.Data[configMapKey]
	if !exists {
		// Fall back to the environment variable
		clusterIDFromEnv := os.Getenv("EKS_CLUSTER_ID")
		if clusterIDFromEnv != "" {
			return clusterIDFromEnv, nil
		}
		return "", fmt.Errorf("cluster ID key not found in ConfigMap and environment variable not set")
	}

	return clusterID, nil
}

func getRegion(ec2Svc *ec2metadata.EC2Metadata) (string, error) {
	// Try to get the region from the EC2 metadata service
	if ec2Svc.Available() {
		region, err := ec2Svc.Region()
		if err == nil {
			return region, nil
		}
	}

	// Fall back to the environment variable if the metadata service fails
	envRegion := os.Getenv("AWS_REGION")
	if envRegion != "" {
		return envRegion, nil
	}

	return "", fmt.Errorf("failed to determine AWS region from both EC2 metadata and environment variable")
}
