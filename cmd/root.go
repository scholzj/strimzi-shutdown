/*
Copyright © 2025 Jakub Scholz

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "strimzi-shutdown",
	Short: "Utility for stopping Kafka cluster",
	Long:  "Utility for stopping or continuing a Strimzi-based Apache Kafka clusters.",
	// Uncomment the following line if your bare application
	// has an action associated with it:
	// Run: func(cmd *cobra.Command, args []string) { },
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.
	rootCmd.PersistentFlags().String("kubeconfig", "", "Path to the kubeconfig file to use for Kubernetes API requests. If not specified, strimzi-shutdown will try to auto-detect the Kubernetes configuration.")
	rootCmd.PersistentFlags().String("namespace", "", "Namespace of the Kafka cluster. If not specified, defaults to the namespace from your Kubernetes configuration.")
	rootCmd.PersistentFlags().String("name", "", "Name of the Kafka cluster")
	rootCmd.PersistentFlags().Uint32P("timeout", "t", 300000, "Timeout for how long to wait when stopping or continuing the Kafka cluster. In milliseconds.")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	//rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
