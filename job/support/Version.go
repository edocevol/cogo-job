package support

import (
	"fmt"

	"github.com/spf13/cobra"
)

var Version = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of gobake",

	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("cogo version: v1.0.0")
	},
}
