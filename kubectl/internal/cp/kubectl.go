// Package cp uses kubectl to copy the files.
// Its probably OK to rely on the kubectl binary being present since
// this is currently implemented as a plugin. In the case where this binary gets
// implemented as a standalone cli tool, this should be switched to Go code.
package cp

import (
	"context"
	"os"
	"os/exec"

	"k8s.io/apimachinery/pkg/types"
)

func ToPod(ctx context.Context, src, dst string, pod types.NamespacedName, container string) error {
	cmd := exec.CommandContext(ctx, "kubectl", "cp", "-n", pod.Namespace, "-c", container, src, pod.Name+":"+dst)
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func FromPod(ctx context.Context, src, dst string, pod types.NamespacedName, container string) error {
	cmd := exec.CommandContext(ctx, "kubectl", "cp", "-n", pod.Namespace, "-c", container, pod.Name+":"+src, dst)
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
