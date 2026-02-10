package main

import (
	"flag"
	"fmt"
	"os"
)

var version = "dev"

func usage() {
	fmt.Fprintf(flag.CommandLine.Output(), "m - a tiny starter CLI\n\n")
	fmt.Fprintf(flag.CommandLine.Output(), "Usage:\n")
	fmt.Fprintf(flag.CommandLine.Output(), "  m [command]\n\n")
	fmt.Fprintf(flag.CommandLine.Output(), "Commands:\n")
	fmt.Fprintf(flag.CommandLine.Output(), "  version   Print the current version\n")
	fmt.Fprintf(flag.CommandLine.Output(), "  help      Show this help message\n")
}

func main() {
	flag.Usage = usage
	flag.Parse()

	if flag.NArg() == 0 {
		fmt.Println("Hello from m. Try: m help")
		return
	}

	switch flag.Arg(0) {
	case "version":
		fmt.Println(version)
	case "help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", flag.Arg(0))
		usage()
		os.Exit(1)
	}
}
