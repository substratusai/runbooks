package client

import (
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/yaml"
)

func Decode(data []byte) (Object, error) {
	decoder := scheme.Codecs.UniversalDeserializer()

	runtimeObject, gvk, err := decoder.Decode(data, nil, nil)
	if gvk == nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return runtimeObject.(Object), nil
}

func Encode(obj Object) ([]byte, error) {
	return yaml.Marshal(obj)
}
