package cloud

import (
	"crypto/md5"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Common struct {
	ClusterName       string     `env:"CLUSTER_NAME" validate:"required"`
	ArtifactBucketURL *BucketURL `env:"ARTIFACT_BUCKET_URL" validate:"required"`
	RegistryURL       string     `env:"REGISTRY_URL" validate:"required"`
}

func (c *Common) ObjectBuiltImageURL(obj client.Object) string {
	kind := obj.GetObjectKind().GroupVersionKind().Kind
	if kind == "" {
		// This can be empty if the Go object was not instantiated with the kind field set.
		// Better to panic than hash the wrong thing silently.
		panic("kind is empty")
	}
	return fmt.Sprintf("%s/%s-%s-%s-%s", c.RegistryURL,
		c.ClusterName, strings.ToLower(kind), obj.GetNamespace(), obj.GetName())
}

func (c *Common) ObjectArtifactURL(obj Object) *BucketURL {
	u := *c.ArtifactBucketURL
	u.Path = filepath.Join(u.Path, objectHash(c.ClusterName, obj))
	return &u
}

func objectHash(cluster string, obj Object) string {
	h := md5.New()
	io.WriteString(h, objectHashInput(cluster, obj))
	return fmt.Sprintf("%x", h.Sum(nil))
}

func objectHashInput(cluster string, obj Object) string {
	kind := obj.GetObjectKind().GroupVersionKind().Kind
	if kind == "" {
		// This can be empty if the Go object was not instantiated with the kind field set.
		// Better to panic than hash the wrong thing silently.
		panic("kind is empty")
	}
	return fmt.Sprintf("clusters/%s/namespaces/%s/%s/%s", cluster,
		obj.GetNamespace(), strings.ToLower(kind)+"s", obj.GetName())
}
