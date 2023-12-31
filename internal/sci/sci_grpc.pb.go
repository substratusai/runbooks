// Code generated by protoc-gen-go-grpc. DO NOT EDIT.

package sci

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.32.0 or later.
const _ = grpc.SupportPackageIsVersion7

// ControllerClient is the client API for Controller service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type ControllerClient interface {
	CreateSignedURL(ctx context.Context, in *CreateSignedURLRequest, opts ...grpc.CallOption) (*CreateSignedURLResponse, error)
	GetObjectMd5(ctx context.Context, in *GetObjectMd5Request, opts ...grpc.CallOption) (*GetObjectMd5Response, error)
	BindIdentity(ctx context.Context, in *BindIdentityRequest, opts ...grpc.CallOption) (*BindIdentityResponse, error)
}

type controllerClient struct {
	cc grpc.ClientConnInterface
}

func NewControllerClient(cc grpc.ClientConnInterface) ControllerClient {
	return &controllerClient{cc}
}

func (c *controllerClient) CreateSignedURL(ctx context.Context, in *CreateSignedURLRequest, opts ...grpc.CallOption) (*CreateSignedURLResponse, error) {
	out := new(CreateSignedURLResponse)
	err := c.cc.Invoke(ctx, "/sci.v1.Controller/CreateSignedURL", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *controllerClient) GetObjectMd5(ctx context.Context, in *GetObjectMd5Request, opts ...grpc.CallOption) (*GetObjectMd5Response, error) {
	out := new(GetObjectMd5Response)
	err := c.cc.Invoke(ctx, "/sci.v1.Controller/GetObjectMd5", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *controllerClient) BindIdentity(ctx context.Context, in *BindIdentityRequest, opts ...grpc.CallOption) (*BindIdentityResponse, error) {
	out := new(BindIdentityResponse)
	err := c.cc.Invoke(ctx, "/sci.v1.Controller/BindIdentity", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ControllerServer is the server API for Controller service.
// All implementations must embed UnimplementedControllerServer
// for forward compatibility
type ControllerServer interface {
	CreateSignedURL(context.Context, *CreateSignedURLRequest) (*CreateSignedURLResponse, error)
	GetObjectMd5(context.Context, *GetObjectMd5Request) (*GetObjectMd5Response, error)
	BindIdentity(context.Context, *BindIdentityRequest) (*BindIdentityResponse, error)
	mustEmbedUnimplementedControllerServer()
}

// UnimplementedControllerServer must be embedded to have forward compatible implementations.
type UnimplementedControllerServer struct {
}

func (UnimplementedControllerServer) CreateSignedURL(context.Context, *CreateSignedURLRequest) (*CreateSignedURLResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method CreateSignedURL not implemented")
}
func (UnimplementedControllerServer) GetObjectMd5(context.Context, *GetObjectMd5Request) (*GetObjectMd5Response, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetObjectMd5 not implemented")
}
func (UnimplementedControllerServer) BindIdentity(context.Context, *BindIdentityRequest) (*BindIdentityResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method BindIdentity not implemented")
}
func (UnimplementedControllerServer) mustEmbedUnimplementedControllerServer() {}

// UnsafeControllerServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to ControllerServer will
// result in compilation errors.
type UnsafeControllerServer interface {
	mustEmbedUnimplementedControllerServer()
}

func RegisterControllerServer(s grpc.ServiceRegistrar, srv ControllerServer) {
	s.RegisterService(&Controller_ServiceDesc, srv)
}

func _Controller_CreateSignedURL_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(CreateSignedURLRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ControllerServer).CreateSignedURL(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/sci.v1.Controller/CreateSignedURL",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ControllerServer).CreateSignedURL(ctx, req.(*CreateSignedURLRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Controller_GetObjectMd5_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetObjectMd5Request)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ControllerServer).GetObjectMd5(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/sci.v1.Controller/GetObjectMd5",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ControllerServer).GetObjectMd5(ctx, req.(*GetObjectMd5Request))
	}
	return interceptor(ctx, in, info, handler)
}

func _Controller_BindIdentity_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(BindIdentityRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ControllerServer).BindIdentity(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/sci.v1.Controller/BindIdentity",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ControllerServer).BindIdentity(ctx, req.(*BindIdentityRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// Controller_ServiceDesc is the grpc.ServiceDesc for Controller service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var Controller_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "sci.v1.Controller",
	HandlerType: (*ControllerServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "CreateSignedURL",
			Handler:    _Controller_CreateSignedURL_Handler,
		},
		{
			MethodName: "GetObjectMd5",
			Handler:    _Controller_GetObjectMd5_Handler,
		},
		{
			MethodName: "BindIdentity",
			Handler:    _Controller_BindIdentity_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "sci.proto",
}
