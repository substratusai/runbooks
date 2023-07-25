package main

import (
	"github.com/substratusai/substratus/kubectl/internal/commands"
	"k8s.io/klog/v2"
)

func main() {
	if err := commands.ApplyBuild().Execute(); err != nil {
		klog.Fatal(err)
	}
}