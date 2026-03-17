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
	"fmt"
	"runtime/debug"

	"github.com/spf13/cobra"
)

func newVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Shows a version of the Strimzi Shutdown application",
		Long:  "Shows a version of the Strimzi Shutdown application.",
		RunE:  runVersionCommand,
	}
}

func runVersionCommand(cmd *cobra.Command, args []string) error {
	buildInfo, ok := debug.ReadBuildInfo()
	if !ok {
		return fmt.Errorf("failed to get Strimzi Shutdown version information")
	}

	fmt.Println("Strimzi Shutdown version:", buildInfo.Main.Version)
	fmt.Println("Go version:", buildInfo.GoVersion)

	return nil
}

func init() {
	rootCmd.AddCommand(newVersionCommand())
}
