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
	"fmt"
	"log"

	strimzi "github.com/scholzj/strimzi-go/pkg/client/clientset/versioned"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type continueOptions struct {
	name       string
	namespace  string
	kubeconfig string
	timeout    uint32
}

// Used for testing
var waitUntilReadyFn = waitUntilReady

func newContinueCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "continue",
		Short: "Restarts the Kafka cluster",
		Long:  "Restarts the Kafka cluster.",
		RunE:  runContinueCommand,
	}
}

func runContinueCommand(cmd *cobra.Command, args []string) error {
	opts, err := continueOptionsFromCmd(cmd)
	if err != nil {
		return err
	}

	return runContinue(opts)
}

func continueOptionsFromCmd(cmd *cobra.Command) (continueOptions, error) {
	timeout, _ := cmd.Flags().GetUint32("timeout")

	name := cmd.Flag("name").Value.String()
	if name == "" {
		return continueOptions{}, fmt.Errorf("--name option is required")
	}

	return continueOptions{
		name:       name,
		namespace:  cmd.Flag("namespace").Value.String(),
		kubeconfig: cmd.Flag("kubeconfig").Value.String(),
		timeout:    timeout,
	}, nil
}

func runContinue(opts continueOptions) error {
	kubeConfig, kubeConfigNamespace, err := kubeConfigAndNamespace(opts.kubeconfig)
	if err != nil {
		return err
	}

	namespace, err := determineNamespace(opts.namespace, kubeConfigNamespace)
	if err != nil {
		return err
	}

	strimziClient, err := strimziClient(kubeConfig)
	if err != nil {
		return fmt.Errorf("failed to create Strimzi client: %w", err)
	}

	return runContinueWithClients(opts.name, namespace, opts.timeout, strimziClient)
}

func runContinueWithClients(name string, namespace string, timeout uint32, strimziClient strimzi.Interface) error {
	kafka, err := strimziClient.KafkaV1().Kafkas(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("Kafka cluster %v in namespace %s not found: %w", name, namespace, err)
	}

	if isReady(kafka) {
		log.Printf("Kafka cluster %s in namespace %s is ready and does not need to be restarted", name, namespace)
		return nil
	} else {
		if isReconciliationPaused(kafka) {
			log.Printf("Reconciliation of Kafka cluster %s in namespace %s will be unpaused", name, namespace)
			unpausedKafka := kafka.DeepCopy()

			if unpausedKafka.Annotations == nil {
				unpausedKafka.Annotations = map[string]string{"strimzi.io/pause-reconciliation": "false"}
			} else {
				unpausedKafka.Annotations["strimzi.io/pause-reconciliation"] = "false"
			}

			log.Printf("Unpausing reconciliation of Kafka cluster %s in namespace %s", name, namespace)
			_, err = strimziClient.KafkaV1().Kafkas(namespace).Update(context.TODO(), unpausedKafka, metav1.UpdateOptions{})
			if err != nil {
				return fmt.Errorf("failed to unpause Kafka cluster %s in namespace %s: %w", name, namespace, err)
			}
		} else {
			log.Printf("Kafka cluster %s in namespace %s does not have paused reconciliation, but is not ready.", name, namespace)
		}

		log.Printf("Waiting for Kafka cluster %s in namespace %s to get ready.", name, namespace)
		_, err = waitUntilReadyFn(strimziClient, name, namespace, timeout)
		if err != nil {
			return err
		}

		log.Printf("Kafka cluster %s in namespace %s is ready", name, namespace)
		return nil
	}
}

func init() {
	rootCmd.AddCommand(newContinueCommand())
}
