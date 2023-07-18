package cloud

import (
	"crypto/md5"
	"fmt"
	"io"
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Common struct {
	ClusterName       string `env:"CLUSTER_NAME" validate:"required"`
	ArtifactBucketURL string `env:"ARTIFACT_BUCKET_URL" validate:"required"`
	RegistryURL       string `env:"REGISTRY_URL" validate:"required"`
}

func (c *Common) ObjectBuiltImageURL(obj client.Object) string {
	kind := obj.GetObjectKind().GroupVersionKind().Kind
	if kind == "" {
		// This can be empty if the Go object was not instantiated with the kind field set.
		// Better to panic than hash the wrong thing silently.
		panic("kind is empty")
	}
	return fmt.Sprintf("%s/%s-%s-%s", c.RegistryURL, strings.ToLower(kind), obj.GetNamespace(), obj.GetName())
}

func (c *Common) ObjectArtifactURL(obj ArtifactObject) string {
	if u := obj.GetStatusURL(); u != "" {
		return u
	}
	hash := artifactHash(c.ClusterName, obj)
	return fmt.Sprintf("%s/%s", c.ArtifactBucketURL, hash)
}

func artifactHash(cluster string, obj ArtifactObject) string {
	h := md5.New()
	io.WriteString(h, artifactHashInput(cluster, obj))
	return fmt.Sprintf("%x", h.Sum(nil))
}

func artifactHashInput(cluster string, obj ArtifactObject) string {
	kind := obj.GetObjectKind().GroupVersionKind().Kind
	if kind == "" {
		// This can be empty if the Go object was not instantiated with the kind field set.
		// Better to panic than hash the wrong thing silently.
		panic("kind is empty")
	}
	return fmt.Sprintf("clusters/%s/namespaces/%s/%s/%s", cluster,
		obj.GetNamespace(), strings.ToLower(kind)+"s", obj.GetName())
}
