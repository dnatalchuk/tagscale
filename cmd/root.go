package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "tagscale",
	Short: "TagScale - Cloud cost insights without perfect tagging",
	Long: `TagScale analyzes cloud cost data, infers missing tags,
and provides dashboards and reports for cost attribution.`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
	}
}
