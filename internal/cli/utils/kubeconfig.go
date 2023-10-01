package utils

import (
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/klog/v2"
)

// BuildConfigFromFlags is a modified version of clientcmd.BuildConfigFromFlags
// that returns the namespace set in the kubeconfig to make sure we play nicely
// with tools like kubens.
func BuildConfigFromFlags(masterUrl, kubeconfigPath string) (string, *restclient.Config, error) {
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
