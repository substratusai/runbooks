package client

import (
	"context"
	"path/filepath"

	apiv1 "github.com/substratusai/substratus/api/v1"
	"github.com/substratusai/substratus/kubectl/internal/cp"
)

func CopySrcToNotebook(ctx context.Context, baseDir string, nb *apiv1.Notebook) error {
	return cp.ToPod(ctx, filepath.Join(baseDir, "src"), "/content/", podForNotebook(nb))
}

func CopySrcFromNotebook(ctx context.Context, baseDir string, nb *apiv1.Notebook) error {
	return cp.FromPod(ctx, "/content/src", filepath.Join(baseDir, "src"), podForNotebook(nb))
}
