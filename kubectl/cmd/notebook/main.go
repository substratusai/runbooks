package main

import (
	"github.com/substratusai/substratus/kubectl/internal/commands"
	"k8s.io/klog/v2"
)

var Version = "development"

func main() {
	commands.Version = Version
	if err := commands.Notebook().Execute(); err != nil {
		klog.Fatal(err)
	}
}
