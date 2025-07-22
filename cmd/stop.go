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
	"log"
	"slices"
	"sync"

	kafkaapi "github.com/scholzj/strimzi-go/pkg/apis/kafka.strimzi.io/v1beta2"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// stopCmd represents the stop command
var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stops the Kafka cluster",
	Long:  "Stops the Kafka cluster.",
	Run: func(cmd *cobra.Command, args []string) {
		timeout, _ := cmd.Flags().GetUint32("timeout")

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

		kube, err := kubeClient(kubeConfig)
		if err != nil {
			log.Fatalf("Failed to create Kubernetes client: %v", err)
		}

		strimzi, err := strimziClient(kubeConfig)
		if err != nil {
			log.Fatalf("Failed to create Strimzi client: %v", err)
		}

		kafka, err := strimzi.KafkaV1beta2().Kafkas(namespace).Get(context.TODO(), name, metav1.GetOptions{})
		if err != nil {
			log.Fatalf("Kafka cluster %v in namespace %s not found: %v", name, namespace, err)
		}

		if !isReconciliationPaused(kafka) {
			pausedKafka := kafka.DeepCopy()
			if pausedKafka.Annotations == nil {
				pausedKafka.Annotations = map[string]string{"strimzi.io/pause-reconciliation": "true"}
			} else {
				pausedKafka.Annotations["strimzi.io/pause-reconciliation"] = "true"
			}

			log.Printf("Pausing reconciliation of Kafka cluster %s in namespace %s", name, namespace)
			_, err = strimzi.KafkaV1beta2().Kafkas(namespace).Update(context.TODO(), pausedKafka, metav1.UpdateOptions{})
			if err != nil {
				log.Fatalf("failed to pause Kafka cluster %s in namespace %s: %v", name, namespace, err)
			}

			log.Printf("Waiting for Kafka cluster %s in namespace %s reconciliation to be paused", name, namespace)
			_, err = waitUntilReconciliationPaused(strimzi, name, namespace, timeout)
			if err != nil {
				log.Fatal(err)
			}

			log.Printf("Reconciliation of Kafka cluster %s in namespace %s is paused", name, namespace)
		} else {
			log.Printf("Kafka cluster %s in namespace %s has already paused reconciliation", name, namespace)
		}

		var deploymentWg sync.WaitGroup

		if kafka.Spec.CruiseControl != nil {
			deploymentWg.Add(1)
			go func() {
				defer deploymentWg.Done()
				err = deleteDeployment(kube, name, "cruise-control", namespace, timeout)
				if err != nil {
					log.Fatalf("Failed to delete Cruise Control deployment: %v", err)
				}
			}()
		}

		if kafka.Spec.KafkaExporter != nil {
			deploymentWg.Add(1)
			go func() {
				defer deploymentWg.Done()
				err = deleteDeployment(kube, name, "kafka-exporter", namespace, timeout)
				if err != nil {
					log.Fatalf("Failed to delete Kafka Exporter deployment: %v", err)
				}
			}()
		}

		if kafka.Spec.EntityOperator != nil {
			deploymentWg.Add(1)
			go func() {
				defer deploymentWg.Done()
				err = deleteDeployment(kube, name, "entity-operator", namespace, timeout)
				if err != nil {
					log.Fatalf("Failed to delete Entity Operator deployment: %v", err)
				}
			}()
		}

		// Wait for deployment deletions to complete
		deploymentWg.Wait()

		nodePools, err := strimzi.KafkaV1beta2().KafkaNodePools(namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: "strimzi.io/cluster=" + name})
		if err != nil {
			log.Fatalf("failed to get KafkaNodePools belonging to Kafka cluster %s in namespace %s: %v", name, namespace, err)
		}

		log.Printf("Stopping all Kafka nodes with broker role only")
		var brokersWg sync.WaitGroup

		for _, pool := range nodePools.Items {
			if !slices.Contains(pool.Spec.Roles, kafkaapi.CONTROLLER_PROCESSROLES) {
				brokersWg.Add(1)
				go func() {
					defer brokersWg.Done()
					err = deletePodSet(kube, strimzi, name, pool.Name, namespace, timeout)
					if err != nil {
						log.Fatal(err)
					}
				}()
			}
		}

		// Wait for broker-only nodes to be removed
		brokersWg.Wait()

		log.Printf("Stopping all remaining Kafka nodes")
		var controllersWg sync.WaitGroup

		for _, pool := range nodePools.Items {
			if slices.Contains(pool.Spec.Roles, kafkaapi.CONTROLLER_PROCESSROLES) {
				controllersWg.Add(1)
				go func() {
					defer controllersWg.Done()
					err = deletePodSet(kube, strimzi, name, pool.Name, namespace, timeout)
					if err != nil {
						log.Fatal(err)
					}
				}()
			}
		}

		// Wait for controller deletion
		controllersWg.Wait()

		log.Printf("Kafka cluster %s in namespace %s has been stopped", name, namespace)
	},
}

func init() {
	rootCmd.AddCommand(stopCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// stopCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// stopCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
