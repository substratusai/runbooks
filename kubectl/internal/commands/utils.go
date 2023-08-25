package commands

import (
	"io"
	"os"

	"github.com/google/uuid"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/klog/v2"

	"github.com/substratusai/substratus/kubectl/internal/client"
)

var Version = "development"

// NewClient is a dirty hack to allow the client to be mocked out in tests.
var NewClient = client.NewClient

// NotebookStdout is a dirty hack to allow stdout to be inspected in tests.
var NotebookStdout io.Writer = os.Stdout

var NewUUID = func() string {
	return uuid.New().String()
}

func buildConfigFromFlags(masterUrl, kubeconfigPath string) (string, *restclient.Config, error) {
	if kubeconfigPath == "" && masterUrl == "" {
		klog.Warning("Neither --kubeconfig nor --master was specified.  Using the inClusterConfig.  This might not work.")
		kubeconfig, err := restclient.InClusterConfig()
		if err == nil {
			return "", kubeconfig, nil
		}
		klog.Warning("error creating inClusterConfig, falling back to default config: ", err)
	}
	cc := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfigPath},
		&clientcmd.ConfigOverrides{ClusterInfo: clientcmdapi.Cluster{Server: masterUrl}})

	ns, _, err := cc.Namespace()
	if err != nil {
		return "", nil, err
	}
	rst, err := cc.ClientConfig()

	return ns, rst, err
}
