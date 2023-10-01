package main

import (
	"log"

	"github.com/substratusai/substratus/internal/cli"
)

var Version = "development"

func main() {
	cli.Version = Version
	if err := cli.Command().Execute(); err != nil {
		log.Fatal(err)
	}
}
