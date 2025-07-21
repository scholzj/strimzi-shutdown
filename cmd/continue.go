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
	"context"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"log"
)

// continueCmd represents the continue command
var continueCmd = &cobra.Command{
	Use:   "continue",
	Short: "Restarts the Kafka cluster",
	Long:  "Restarts the Kafka cluster.",
	Run: func(cmd *cobra.Command, args []string) {
		name := cmd.Flag("name").Value.String()
		if name == "" {
			log.Fatal("--name option is required")
		}

		kubeConfig, kubeConfigNamespace, err := kubeConfigAndNamespace(cmd.Flag("kubeconfig").Value.String())
		if err != nil {
			log.Fatal(err)
		}

		namespace, err := determineNamespace(cmd.Flag("namespace").Value.String(), kubeConfigNamespace)
		if err != nil {
			log.Fatal(err)
		}

		strimzi := strimziClient(kubeConfig)

		kafka, err := strimzi.KafkaV1beta2().Kafkas(namespace).Get(context.TODO(), name, metav1.GetOptions{})
		if err != nil {
			log.Fatalf("Kafka cluster %v in namespace %s not found: %v", name, namespace, err)
		}

		if isReconciliationPaused(kafka) {
			log.Printf("Reconciliation of Kafka cluster %s in namespace %s will be unpaused", name, namespace)
			unpausedKafka := kafka.DeepCopy()

			if unpausedKafka.Annotations == nil {
				unpausedKafka.Annotations = map[string]string{"strimzi.io/pause-reconciliation": "false"}
			} else {
				unpausedKafka.Annotations["strimzi.io/pause-reconciliation"] = "false"
			}

			log.Printf("Unpausing reconciliation of Kafka cluster %s in namespace %s", name, namespace)
			_, err = strimzi.KafkaV1beta2().Kafkas(namespace).Update(context.TODO(), unpausedKafka, metav1.UpdateOptions{})
			if err != nil {
				log.Fatalf("failed to unpause Kafka cluster %s in namespace %s: %v", name, namespace, err)
			}

			log.Printf("Waiting for Kafka cluster %s in namespace %s to get ready.", name, namespace)
			_, err = waitUntilReady(strimzi, name, namespace)
			if err != nil {
				log.Fatal(err)
			}

			log.Printf("Kafka cluster %s in namespace %s has been restarted and should be ready", name, namespace)
		} else if isReady(kafka) {
			log.Printf("Kafka cluster %s in namespace %s is ready and does not need to be restarted", name, namespace)
		} else {
			log.Printf("Waiting for Kafka cluster %s in namespace %s does not have paused reconciliation, but is not ready.", name, namespace)
			log.Printf("Waiting for Kafka cluster %s in namespace %s to get ready.", name, namespace)
			_, err = waitUntilReady(strimzi, name, namespace)
			if err != nil {
				log.Fatal(err)
			}

			log.Printf("Kafka cluster %s in namespace %s has been restarted and should be ready", name, namespace)
		}
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
