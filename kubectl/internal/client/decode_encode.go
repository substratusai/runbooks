package client

import (
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

func Decode(data []byte) (client.Object, error) {
	decoder := scheme.Codecs.UniversalDeserializer()

	runtimeObject, gvk, err := decoder.Decode(data, nil, nil)
	if err != nil {
		return nil, err
	}

	_ = gvk

	return runtimeObject.(client.Object), nil
}

func Encode(obj client.Object) ([]byte, error) {
	return yaml.Marshal(obj)
}
