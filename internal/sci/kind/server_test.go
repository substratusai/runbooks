package kind_test

import (
	"context"
	"fmt"
	"net"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	sci "github.com/substratusai/substratus/internal/sci"
	scikind "github.com/substratusai/substratus/internal/sci/kind"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func TestServer(t *testing.T) {
	s := &scikind.Server{
		BucketDir: os.TempDir(),
	}

	signedURLServer := httptest.NewServer(s)
	defer signedURLServer.Close()
	s.SignedURLAddress = signedURLServer.URL

	gs := grpc.NewServer()
	sci.RegisterControllerServer(gs, s)

	grpcAddr := fmt.Sprintf(":%v", 2222)
	go func() {
		lis, err := net.Listen("tcp", grpcAddr)
		require.NoError(t, err)
		require.NoError(t, gs.Serve(lis))
	}()

	conn, err := grpc.Dial(grpcAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer conn.Close()
	c := sci.NewControllerClient(conn)

	ctx := context.Background()
	resp, err := c.CreateSignedURL(ctx, &sci.CreateSignedURLRequest{
		Md5Checksum: "1234",
		ObjectName:  "my-obj/name/here",
	})
	require.NoError(t, err)
	require.Equal(t, fmt.Sprintf("%v/my-obj/name/here", signedURLServer.URL), resp.Url)
}
