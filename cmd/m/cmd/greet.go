package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newGreetCmd() *cobra.Command {
	var caps bool

	greetCmd := &cobra.Command{
		Use:   "greet [name]",
		Short: "Print a greeting",
		Run: func(cmd *cobra.Command, args []string) {
			name := "world"
			if len(args) > 0 {
				name = strings.Join(args, " ")
			}

			message := fmt.Sprintf("Hello, %s!", name)
			if caps {
				message = strings.ToUpper(message)
			}

			fmt.Println(message)
		},
	}

	greetCmd.Flags().BoolVar(&caps, "caps", false, "Print greeting in uppercase")

	return greetCmd
}
