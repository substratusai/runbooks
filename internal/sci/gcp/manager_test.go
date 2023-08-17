package gcp_test

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"testing"

	"cloud.google.com/go/compute/metadata"
	"github.com/stretchr/testify/require"
	"github.com/substratusai/substratus/internal/sci"
	"github.com/substratusai/substratus/internal/sci/gcp"
	"google.golang.org/api/iam/v1"

	// Can be changed to slices once we go to 1.21
	"golang.org/x/exp/slices"
)

func TestServer(t *testing.T) {
	// This tests requires a GCP Service Account key file and pointing GOOGLE_APPLICATION_CREDENTIALS
	// to the location of the key file. For example run:
	// `export GOOGLE_APPLICATION_CREDENTIALS=$PWD/secrets/substratus-sa.json`
	// this test won't run if GOOGLE_APPLICATION_CREDENTIALS is not set
	if os.Getenv("GOOGLE_APPLICATION_CREDENTIALS") == "" {
		t.Skip("Skipping test because GOOGLE_APPLICATION_CREDENTIALS is not set")
	}
	server, err := gcp.NewServer()
	if err != nil {
		t.Errorf("Error creating server")
	}

	hc := &http.Client{}
	mc := metadata.NewClient(hc)
	ctx := context.Background()
	iamClient, err := iam.NewService(ctx)
	require.NoError(t, err, "error instantiating GCP IAM client")

	server.Clients.IAM = iamClient

	err = server.AutoConfigure(mc)
	require.NoError(t, err, "error running AutoConfigure")

	expectedMember := fmt.Sprintf("serviceAccount:%s.svc.id.goog[integration-test/integration-test]", server.ProjectID)
	resourceID := fmt.Sprintf("projects/%s/serviceAccounts/%s", server.ProjectID, server.SaEmail)
	// Get current policy and remove bindings left behind from previous tests
	policy, err := server.Clients.IAM.Projects.ServiceAccounts.GetIamPolicy(resourceID).Context(ctx).Do()
	require.NoErrorf(t, err, "error getting IAM Policy of SA: %v", resourceID)
	logIAMPolicyBindings(t, policy.Bindings, "policy bindings before BindIdentity call")

	policy.Bindings = removeBindingMember(t, policy.Bindings, expectedMember)
	rb := &iam.SetIamPolicyRequest{Policy: policy}
	policy, err = server.Clients.IAM.Projects.ServiceAccounts.SetIamPolicy(resourceID, rb).Context(ctx).Do()

	require.NoErrorf(t, err, "error setting IAM Policy of SA: %v", resourceID)
	logIAMPolicyBindings(t, policy.Bindings, "policy bindings after cleaning up from previous tests")

	_, err = server.BindIdentity(ctx, &sci.BindIdentityRequest{
		Principal:                server.SaEmail,
		KubernetesServiceAccount: "integration-test",
		KubernetesNamespace:      "integration-test",
	})
	require.NoErrorf(t, err, "error calling BindIdentity: %v", resourceID)

	policy, err = server.Clients.IAM.Projects.ServiceAccounts.GetIamPolicy(resourceID).Context(ctx).Do()
	require.NoErrorf(t, err, "error calling GetIAMPolicy on SA: %v", resourceID)
	logIAMPolicyBindings(t, policy.Bindings, "policy bindings after BindIdentity")

	bindingWasSet := false
	for _, binding := range policy.Bindings {
		if slices.Contains(binding.Members, expectedMember) {
			bindingWasSet = true
		}
	}
	require.Equal(t, bindingWasSet, true)
	policy.Bindings = removeBindingMember(t, policy.Bindings, expectedMember)
	rb = &iam.SetIamPolicyRequest{Policy: policy}
	_, err = server.Clients.IAM.Projects.ServiceAccounts.SetIamPolicy(resourceID, rb).Context(ctx).Do()
	require.NoErrorf(t, err, "error removing IAM policy on SA: %v", resourceID)
}

func logIAMPolicyBindings(t *testing.T, bindings []*iam.Binding, message string) {
	t.Log(message)
	for _, binding := range bindings {
		t.Logf("role: %v, members: %v", binding.Role, binding.Members)
	}
}

func removeBindingMember(t *testing.T, bindings []*iam.Binding, member string) []*iam.Binding {
	for _, binding := range bindings {
		if index := slices.Index(binding.Members, member); index != -1 {
			t.Logf("Removing member %v from role %v", member, binding.Role)
			binding.Members = slices.Delete(binding.Members, index, index+1)
		}
	}
	return bindings
}
