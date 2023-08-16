// Package gcp provides a GCP implementation of the Substratus Cloud Interface (SCI).
package gcp

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"time"

	"cloud.google.com/go/compute/metadata"
	credentials "cloud.google.com/go/iam/credentials/apiv1"
	credentialspb "cloud.google.com/go/iam/credentials/apiv1/credentialspb"
	"cloud.google.com/go/storage"
	"github.com/sethvargo/go-envconfig"
	"github.com/substratusai/substratus/internal/sci"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/iam/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// server implements the sci.ControllerServer interface.
type Server struct {
	sci.UnimplementedControllerServer
	Clients
	SaEmail   string
	ProjectID string `env:"PROJECT_ID"`
}

type Clients struct {
	IAMCredentialsClient *credentials.IamCredentialsClient
	IAM                  *iam.Service
	Metadata             *metadata.Client
	Storage              *storage.Client
	HTTP                 *http.Client
}

func NewServer() (*Server, error) {
	s := &Server{}
	ctx := context.Background()
	if err := envconfig.Process(ctx, s); err != nil {
		return nil, fmt.Errorf("environment: %w", err)
	}
	return s, nil
}

// CreateSignedURL generates a signed URL for a specified GCS bucket and object path.
func (s *Server) CreateSignedURL(ctx context.Context, req *sci.CreateSignedURLRequest) (*sci.CreateSignedURLResponse, error) {
	log := log.FromContext(ctx)
	log.Info("creating signed URL", "bucket", req.BucketName, "object", req.ObjectName)

	bucketName, objectName, checksum := req.GetBucketName(),
		req.GetObjectName(),
		req.GetMd5Checksum()
	bucket := s.Clients.Storage.Bucket(bucketName)
	obj := bucket.Object(objectName)
	if _, err := obj.Attrs(ctx); err != nil && err != storage.ErrObjectNotExist {
		// An error occurred that was NOT ErrObjectNotExist.
		// This is an unexpected error and we should return it.
		log.Error(err, "error checking if object exists", "object", objectName)
		return nil, err
	}

	data, err := hex.DecodeString(checksum)
	if err != nil {
		log.Error(err, "error decoding MD5 checksum", "checksum", checksum)
		return nil, fmt.Errorf("failed to decode MD5 checksum: %w", err)
	}
	base64md5 := base64.StdEncoding.EncodeToString(data)

	opts := &storage.SignedURLOptions{
		Scheme: storage.SigningSchemeV4,
		Method: http.MethodPut,
		Headers: []string{
			"Content-Type:application/octet-stream",
		},
		Expires:        time.Now().Add(time.Duration(req.GetExpirationSeconds()) * time.Second),
		GoogleAccessID: s.SaEmail,
		MD5:            base64md5,
		SignBytes: func(b []byte) ([]byte, error) {
			req := &credentialspb.SignBlobRequest{
				Payload: b,
				Name:    s.SaEmail,
			}
			resp, err := s.Clients.IAMCredentialsClient.SignBlob(ctx, req)
			if err != nil {
				log.Error(err, "error signing blob")
				return nil, fmt.Errorf("failed to sign the blob: %w", err)
			}
			return resp.SignedBlob, err
		},
	}

	// Create a signed URL
	url, err := storage.SignedURL(bucketName, objectName, opts)
	if err != nil {
		log.Error(err, "error creating signed url")
		return nil, fmt.Errorf("error creating signed url: %w", err)
	}

	return &sci.CreateSignedURLResponse{Url: url}, nil
}

func (s *Server) GetObjectMd5(ctx context.Context, req *sci.GetObjectMd5Request) (*sci.GetObjectMd5Response, error) {
	bucketName, objectName := req.GetBucketName(), req.GetObjectName()
	bucket := s.Clients.Storage.Bucket(bucketName)
	obj := bucket.Object(objectName)
	attrs, err := obj.Attrs(ctx)
	if err != nil {
		return nil, err
	}
	md5str := hex.EncodeToString(attrs.MD5)
	return &sci.GetObjectMd5Response{Md5Checksum: md5str}, nil
}

func (s *Server) BindIdentity(ctx context.Context, req *sci.BindIdentityRequest) (*sci.BindIdentityResponse, error) {
	log := log.FromContext(ctx)
	log.Info("Binding K8s Service Account to GCP Service Account",
		"k8s_service_account", req.KubernetesServiceAccount, "namespace", req.KubernetesNamespace,
		"gcp_service_account", req.Principal)
	resource := fmt.Sprintf("projects/%v/serviceAccounts/%v", s.ProjectID, req.Principal)

	// There is no add iam policy binding API so have to get existing policy to
	// modify locally, then fully overwrite existing policy using set iam policy
	policy, err := s.Clients.IAM.Projects.ServiceAccounts.GetIamPolicy(resource).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to get policy of Service Account: %w", err)
	}
	policy.Bindings = append(policy.Bindings, &iam.Binding{
		Members: []string{fmt.Sprintf("serviceAccount:%s.svc.id.goog[%s/%s]",
			s.ProjectID, req.KubernetesNamespace, req.KubernetesServiceAccount)},
		Role: "roles/iam.workloadIdentityUser",
	})

	rb := &iam.SetIamPolicyRequest{Policy: policy}
	_, err = s.Clients.IAM.Projects.ServiceAccounts.SetIamPolicy(resource, rb).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("error setting IAM policy: %w", err)
	}

	return &sci.BindIdentityResponse{}, nil
}

// GetServiceAccountEmail returns the email address of the service account
// it relies on either a local metadata service or a key file.
func (s *Server) AutoConfigure(m *metadata.Client) error {
	if metadata.OnGCE() {
		email, err := m.Email("default")
		if err != nil {
			return fmt.Errorf("Error getting default creds: %w", err)
		}
		s.SaEmail = email

		if s.ProjectID == "" {
			projectID, err := m.ProjectID()
			if err != nil {
				return fmt.Errorf("Error getting project id from metadata server: %w", err)
			}
			s.ProjectID = projectID
		}
	} else {
		// Parse the service account email from the key file.
		keyFile := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")
		if keyFile == "" {
			return fmt.Errorf("GOOGLE_APPLICATION_CREDENTIALS environment variable not set")
		}

		key, err := os.ReadFile(keyFile)
		if err != nil {
			return err
		}

		cfg, err := google.JWTConfigFromJSON(key)
		if err != nil {
			return err
		}
		s.SaEmail = cfg.Email

		cred, err := google.CredentialsFromJSON(context.Background(), key)
		if err != nil {
			return err
		}
		s.ProjectID = cred.ProjectID
	}
	return nil
}

func (server *Server) Validate() error {
	// retry is needed because GKE workload identity will fail during first few seconds
	// so restarting the pod won't help
	resourceID := fmt.Sprintf("projects/%s/serviceAccounts/%s", server.ProjectID, server.SaEmail)
	var err error
	for i := 0; i < 3; i++ {
		_, err = server.Clients.IAM.Projects.ServiceAccounts.GetIamPolicy(resourceID).Context(context.Background()).Do()
		if err == nil {
			return nil
		}
		log.FromContext(context.Background()).Error(err, "error trying to get IAM policy. Retrying in for 5 seconds")
		time.Sleep(time.Second * 5)
	}
	return err
}
