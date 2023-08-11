// Package aws provides an AWS implementation of the Substratus Cloud Interface (SCI)
package awssci

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"os"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
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
	IamClient *iam.IAM
}

func NewAWSServer() (*Server, error) {
	sess, err := session.NewSession()
	if err != nil {
		return nil, fmt.Errorf("failed to create AWS session: %w", err)
	}
	ec2Svc := ec2metadata.New(sess)

	region, err := getRegion(ec2Svc)
	if err != nil {
		return nil, fmt.Errorf("failed to get region: %w", err)
	}

	clusterID, err := getClusterID(ec2Svc)
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster ID: %w", err)
	}

	accountId, err := getAccountID(ec2Svc)
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
		Bucket:      aws.String(bucketName),
		Key:         aws.String(objectName),
		ContentType: aws.String("application/octet-stream"),
		ContentMD5:  aws.String(base64md5),
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
		Bucket: aws.String(bucketName),
		Key:    aws.String(objectKey),
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
	// Construct the trust relationship

	trustPolicy := fmt.Sprintf(`{
		"Version": "2012-10-17",
		"Statement": [{
			"Effect": "Allow",
			"Principal": {
				"Federated": "%s"
			},
			"Action": "sts:AssumeRoleWithWebIdentity",
			"Condition": {
				"StringEquals": {
					"%s:sub": "system:serviceaccount:%s:%s"
				}
			}
		}]
	}`, s.OIDCProviderARN, s.OIDCProviderURL, req.KubernetesNamespace, req.KubernetesServiceAccount)

	input := &iam.UpdateAssumeRolePolicyInput{
		PolicyDocument: aws.String(trustPolicy),
		RoleName:       aws.String(req.Principal), // Assuming Principal is the AWS Role Name
	}

	_, err := s.Clients.IamClient.UpdateAssumeRolePolicy(input) // Assuming `Iam` client exists in `Clients` struct
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case iam.ErrCodeNoSuchEntityException:
				return nil, fmt.Errorf("%s: %s", iam.ErrCodeNoSuchEntityException, aerr.Error())
			case iam.ErrCodeMalformedPolicyDocumentException:
				return nil, fmt.Errorf("%s: %s", iam.ErrCodeMalformedPolicyDocumentException, aerr.Error())
			case iam.ErrCodeLimitExceededException:
				return nil, fmt.Errorf("%s: %s", iam.ErrCodeLimitExceededException, aerr.Error())
			case iam.ErrCodeUnmodifiableEntityException:
				return nil, fmt.Errorf("%s: %s", iam.ErrCodeUnmodifiableEntityException, aerr.Error())
			case iam.ErrCodeServiceFailureException:
				return nil, fmt.Errorf("%s: %s", iam.ErrCodeServiceFailureException, aerr.Error())
			default:
				return nil, fmt.Errorf(aerr.Error())
			}
		}
		return nil, fmt.Errorf(err.Error())
	}

	return &sci.BindIdentityResponse{}, nil
}

func getAccountID(ec2Svc *ec2metadata.EC2Metadata) (string, error) {
	sess := session.Must(session.NewSession())
	svc := sts.New(sess)

	result, err := svc.GetCallerIdentity(&sts.GetCallerIdentityInput{})
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

func getClusterID(ec2Svc *ec2metadata.EC2Metadata) (string, error) {
	// try EC2 metadata first
	if ec2Svc.Available() {
		userData, err := ec2Svc.GetUserData()
		if err == nil {
			return userData, nil
		}
	}

	// fall back to the environment variable
	clusterIDFromEnv := os.Getenv("EKS_CLUSTER_ID")
	if clusterIDFromEnv != "" {
		return clusterIDFromEnv, nil
	}

	return "", fmt.Errorf("could not determine the cluster ID from available sources")
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
