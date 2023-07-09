package controller

import (
	"math"

	apiv1 "github.com/substratusai/substratus/api/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func boolPtr(b bool) *bool {
	return &b
}
func int32Ptr(i int32) *int32 {
	return &i
}
func int64Ptr(i int64) *int64 {
	return &i
}
func strPtr(s string) *string {
	return &s
}

func nextPowOf2(n int64) int64 {
	k := int64(1)
	for k < n {
		k = k << 1
	}
	return k
}

const (
	thousand = 1000
	million  = 1000 * 1000
	billion  = 1000 * 1000 * 1000

	gigabyte = int64(1024 * 1024 * 1024)
)

func roundUpGB(bytes int64) int64 {
	return int64(math.Ceil(float64(bytes)/float64(gigabyte))) * gigabyte
}

type Object interface {
	client.Object
	GetConditions() *[]metav1.Condition
}

func isReady(obj Object) bool {
	condition := meta.FindStatusCondition(*obj.GetConditions(), apiv1.ConditionReady)
	return condition != nil && condition.Status == metav1.ConditionTrue
}
