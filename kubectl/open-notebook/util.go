package main

import (
	"fmt"
	"io/ioutil"
	"os"

	"k8s.io/apimachinery/pkg/util/yaml"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return os.Getenv("USERPROFILE") // windows
}

func readManifest(path string) (*unstructured.Unstructured, error) {
	var u unstructured.Unstructured

	yml, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading input manifest file: %w", err)
	}

	if err := yaml.Unmarshal(yml, &u); err != nil {
		fmt.Println(string(yml))
		return nil, fmt.Errorf("unmarshalling input: %w", err)
	}

	return &u, nil
}

func hasCondition(u *unstructured.Unstructured, typ, status string) bool {
	conditions, found, err := unstructured.NestedSlice(u.Object, "status", "conditions")
	if !found || err != nil {
		return false
	}
	for _, c := range conditions {
		c := c.(map[string]interface{})
		if c["type"] == typ && c["status"] == status {
			return true
		}
	}
	return false
}

func isSuspended(u *unstructured.Unstructured) bool {
	spec := u.Object["spec"].(map[string]interface{})
	suspended, _ := spec["suspend"].(bool)
	return suspended
}

func unsuspend(u *unstructured.Unstructured) {
	spec := u.Object["spec"].(map[string]interface{})
	spec["suspend"] = false
}
