// Package awsmanager provides an AWS implementation of the Substratus Cloud Interface (SCI)
package awsmanager

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/substratusai/substratus/internal/sci"
)

type Server struct {
	sci.UnimplementedControllerServer
	Clients
}

type Clients struct {
	S3Client *s3.S3
}

func NewAWSServer() (*Server, error) {
	sess, err := session.NewSession()
	if err != nil {
		return nil, fmt.Errorf("failed to create AWS session: %w", err)
	}

	c := &Clients{
		S3Client: s3.New(sess),
	}

	return &Server{
		Clients: *c,
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
