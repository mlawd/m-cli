package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

func newTimeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "time [zone]",
		Short: "Print current time",
		Long:  "Print current time in local timezone or UTC.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			now := time.Now()
			if len(args) == 0 {
				fmt.Printf("local: %s\n", now.Format(time.RFC1123))
				fmt.Printf("utc:   %s\n", now.UTC().Format(time.RFC1123))
				return nil
			}

			switch args[0] {
			case "local":
				fmt.Println(now.Format(time.RFC1123))
			case "utc":
				fmt.Println(now.UTC().Format(time.RFC1123))
			default:
				return fmt.Errorf("unknown time zone: %s (use local or utc)", args[0])
			}

			return nil
		},
	}
}
