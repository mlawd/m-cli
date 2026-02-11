package main

import (
	"fmt"
	"os"

	"github.com/mlawd/m-cli/cmd/m/cmd"
)

var version = "dev"

func main() {
	if err := cmd.NewRootCmd(version).Execute(); err != nil {
		fmt.Fprintln(os.Stderr, cmd.FormatCLIError(os.Stderr, err))
		os.Exit(1)
	}
}
