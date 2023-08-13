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
	if err != nil {
		t.Errorf("error instantiating GCP IAM client: %v", err)
	}
	server.Clients.IAM = iamClient

	err = server.AutoConfigure(mc)
	if err != nil {
		t.Errorf("Error running AutoConfigure %v", err)
	}

	expectedMember := fmt.Sprintf("serviceAccount:%s.svc.id.goog[integration-test/integration-test]", server.ProjectID)
	resourceID := fmt.Sprintf("projects/%s/serviceAccounts/%s", server.ProjectID, server.SaEmail)
	// Get current policy and remove bindings left behind from previous tests
	policy, err := server.Clients.IAM.Projects.ServiceAccounts.GetIamPolicy(resourceID).Context(ctx).Do()
	if err != nil {
		t.Errorf("error getting IAM policy of service account: %v", err)
	}
	logIAMPolicyBindings(t, policy.Bindings, "policy bindings before BindIdentity call")
	for _, binding := range policy.Bindings {
		if index := slices.Index(binding.Members, expectedMember); index != -1 {
			t.Logf("Cleaning up from previous test. Removing member %v", expectedMember)
			binding.Members = slices.Delete(binding.Members, index, index+1)
		}
	}

	rb := &iam.SetIamPolicyRequest{Policy: policy}
	policy, err = server.Clients.IAM.Projects.ServiceAccounts.SetIamPolicy(resourceID, rb).Context(ctx).Do()
	if err != nil {
		t.Errorf("error setting IAM policy: %v", err)
	}
	logIAMPolicyBindings(t, policy.Bindings, "policy bindings after cleaning up from previous tests")

	_, err = server.BindIdentity(ctx, &sci.BindIdentityRequest{
		Principal:                server.SaEmail,
		KubernetesServiceAccount: "integration-test",
		KubernetesNamespace:      "integration-test",
	})
	if err != nil {
		t.Errorf("error binding identity: %v", err)
	}

	policy, err = server.Clients.IAM.Projects.ServiceAccounts.GetIamPolicy(resourceID).Context(ctx).Do()
	if err != nil {
		t.Errorf("error getting IAM policy of service account: %v", err)
	}
	logIAMPolicyBindings(t, policy.Bindings, "policy bindings after BindIdentity")
	bindingWasSet := false
	for _, binding := range policy.Bindings {
		if slices.Contains(binding.Members, expectedMember) {
			bindingWasSet = true
		}
	}

	require.Equal(t, bindingWasSet, true)

}

func logIAMPolicyBindings(t *testing.T, bindings []*iam.Binding, message string) {
	t.Log(message)
	for _, binding := range bindings {
		t.Logf("role: %v, members: %v", binding.Role, binding.Members)
	}
}
