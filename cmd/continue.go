/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// continueCmd represents the continue command
var continueCmd = &cobra.Command{
	Use:   "continue",
	Short: "Restarts the Kafka cluster",
	Long:  "Restarts the Kafka cluster.",
	Run: func(cmd *cobra.Command, args []string) {
		//cmd.Fl
		fmt.Println("continue called")
	},
}

func init() {
	rootCmd.AddCommand(continueCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// continueCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// continueCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
