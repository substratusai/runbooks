// Package gcp provides a GCP implementation of the Substratus Cloud Interface (SCI).
package gcpmanager

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	credentials "cloud.google.com/go/iam/credentials/apiv1"
	storage "cloud.google.com/go/storage"
	sci "github.com/substratusai/substratus/internal/sci"
	credentialspb "google.golang.org/genproto/googleapis/iam/credentials/v1"
)

// server implements the sci.ControllerServer interface.
type Server struct {
	sci.UnimplementedControllerServer
	StorageClient *storage.Client
}

// CreateSignedURL generates a signed URL for a specified GCS bucket and object path.
func (s *Server) CreateSignedURL(ctx context.Context, req *sci.CreateSignedURLRequest) (*sci.CreateSignedURLResponse, error) {
	bucketName, objectName := req.GetBucketName(), req.GetObjectName()

	bucket := s.StorageClient.Bucket(bucketName)
	obj := bucket.Object(objectName)
	if _, err := obj.Attrs(ctx); err != nil {
		if err != storage.ErrObjectNotExist {
			// An error occurred that was NOT ErrObjectNotExist.
			// This is an unexpected error and we should return it.
			return nil, err
		}
		// the object doesn't exist and we want to continue and create the signed URL.
	}

	saEmail, err := getServiceAccountEmail()
	if err != nil {
		return nil, err
	}

	c, err := credentials.NewIamCredentialsClient(ctx)
	if err != nil {
		return nil, err
	}

	opts := &storage.SignedURLOptions{
		// this is optional storage.SigningSchemeV2 is the default. storage.SigningSchemeV4 is also available.
		Scheme: storage.SigningSchemeV2,
		Method: http.MethodPost,
		Headers: []string{
			"Content-Type:multipart/form-data",
		},
		Expires:        time.Now().Add(time.Duration(req.GetExpirationSeconds()) * time.Second),
		GoogleAccessID: saEmail,
		SignBytes: func(b []byte) ([]byte, error) {
			req := &credentialspb.SignBlobRequest{
				Payload: b,
				Name:    saEmail,
			}
			resp, err := c.SignBlob(ctx, req)
			if err != nil {
				panic(err)
			}
			return resp.SignedBlob, err
		},
	}

	// Create a signed URL
	url, err := storage.SignedURL(bucketName, objectName, opts)
	if err != nil {
		return nil, err
	}

	return &sci.CreateSignedURLResponse{Url: url}, nil
}

func getServiceAccountEmail() (string, error) {
	url := "http://metadata.google.internal/computeMetadata/v1/instance/service-accounts/default/email"

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		fmt.Printf("Error creating request: %v\n", err)
		return "", err
	}
	req.Header.Add("Metadata-Flavor", "Google")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("Error making request: %v\n", err)
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("Request failed with status code: %v\n", resp.StatusCode)
		return "", err
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("Error reading response body: %v\n", err)
		return "", err
	}

	return string(body), nil
}
