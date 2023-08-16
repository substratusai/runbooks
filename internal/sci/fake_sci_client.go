package sci

import (
	context "context"

	grpc "google.golang.org/grpc"
)

type FakeSCIControllerClient struct{}

func (c *FakeSCIControllerClient) CreateSignedURL(ctx context.Context, in *CreateSignedURLRequest, opts ...grpc.CallOption) (*CreateSignedURLResponse, error) {
	return &CreateSignedURLResponse{}, nil
}

func (c *FakeSCIControllerClient) GetObjectMd5(ctx context.Context, in *GetObjectMd5Request, opts ...grpc.CallOption) (*GetObjectMd5Response, error) {
	return &GetObjectMd5Response{}, nil
}

func (c *FakeSCIControllerClient) BindIdentity(ctx context.Context, in *BindIdentityRequest, opts ...grpc.CallOption) (*BindIdentityResponse, error) {
	return &BindIdentityResponse{}, nil
}
