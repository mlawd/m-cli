package main

import (
	"os"

	"github.com/mlawd/m-cli/cmd/m/cmd"
)

var version = "dev"

func main() {
	if err := cmd.NewRootCmd(version).Execute(); err != nil {
		os.Exit(1)
	}
}
