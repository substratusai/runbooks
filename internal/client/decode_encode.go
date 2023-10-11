package client

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/yaml"
)

func Decode(data []byte) (Object, error) {
	decoder := scheme.Codecs.UniversalDeserializer()

	runtimeObject, gvk, err := decoder.Decode(data, nil, nil)
	if gvk == nil {
		var obj unstructured.Unstructured
		jsonData, err := yaml.YAMLToJSON(data)
		if err != nil {
			return nil, fmt.Errorf("failed to convert yaml to json: %w", err)
		}
		if _, _, err := unstructured.UnstructuredJSONScheme.Decode(jsonData, nil, &obj); err != nil {
			return nil, fmt.Errorf("failed to decode to unstructured object: %w", err)
		}
		return &obj, nil
	}
	if err != nil {
		return nil, err
	}

	return runtimeObject.(Object), nil
}

func Encode(obj Object) ([]byte, error) {
	return yaml.Marshal(obj)
}
