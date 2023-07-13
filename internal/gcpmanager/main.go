// Package gcp provides a GCP implementation of the Substratus Cloud Interface (SCI).
package gcpmanager

import (
	"context"
	"time"

	sci "github.com/substratusai/substratus/internal/sci"

	storage "cloud.google.com/go/storage"
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

	opts := &storage.SignedURLOptions{
		// this is optional storage.SigningSchemeV2 is the default. storage.SigningSchemeV4 is also available.
		Scheme:  storage.SigningSchemeV2,
		Method:  "POST",
		Expires: time.Now().Add(time.Duration(req.GetExpirationSeconds()) * time.Second),
	}

	// Create a signed URL
	// TODO(bjb): Need to test this on a live pod with WI with the SA having roles/iam.serviceAccountTokenCreator.
	// I've only used this API with a static credential previously: https://cloud.google.com/storage/docs/access-control/signing-urls-with-helpers#upload-object
	// a few techniques here: https://stackoverflow.com/questions/62439257/how-to-generate-signed-urls-for-google-cloud-storage-objects-in-gke-go
	url, err := storage.SignedURL(bucketName, objectName, opts)
	if err != nil {
		return nil, err
	}

	return &sci.CreateSignedURLResponse{Url: url}, nil
}
