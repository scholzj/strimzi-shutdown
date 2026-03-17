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
	"slices"
	"sync"

	kafkaapi "github.com/scholzj/strimzi-go/pkg/apis/kafka.strimzi.io/v1"
	strimzi "github.com/scholzj/strimzi-go/pkg/client/clientset/versioned"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	appsv1client "k8s.io/client-go/kubernetes/typed/apps/v1"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
)

type stopOptions struct {
	name       string
	namespace  string
	kubeconfig string
	timeout    uint32
}

type kubeClients interface {
	AppsV1() appsv1client.AppsV1Interface
	CoreV1() corev1client.CoreV1Interface
}

var waitUntilReconciliationPausedFn = waitUntilReconciliationPaused
var deleteDeploymentFn = deleteDeployment
var deletePodSetFn = deletePodSet

func newStopCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stops the Kafka cluster",
		Long:  "Stops the Kafka cluster.",
		RunE:  runStopCommand,
	}
}

// stopCmd represents the stop command
var stopCmd = newStopCommand()

func runStopCommand(cmd *cobra.Command, args []string) error {
	opts, err := stopOptionsFromCmd(cmd)
	if err != nil {
		return err
	}

	return runStop(opts)
}

func stopOptionsFromCmd(cmd *cobra.Command) (stopOptions, error) {
	timeout, _ := cmd.Flags().GetUint32("timeout")

	name := cmd.Flag("name").Value.String()
	if name == "" {
		return stopOptions{}, fmt.Errorf("--name option is required")
	}

	return stopOptions{
		name:       name,
		namespace:  cmd.Flag("namespace").Value.String(),
		kubeconfig: cmd.Flag("kubeconfig").Value.String(),
		timeout:    timeout,
	}, nil
}

func runStop(opts stopOptions) error {
	kubeConfig, kubeConfigNamespace, err := kubeConfigAndNamespace(opts.kubeconfig)
	if err != nil {
		return err
	}

	namespace, err := determineNamespace(opts.namespace, kubeConfigNamespace)
	if err != nil {
		return err
	}

	kube, err := kubeClient(kubeConfig)
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	strimziClient, err := strimziClient(kubeConfig)
	if err != nil {
		return fmt.Errorf("failed to create Strimzi client: %w", err)
	}

	return runStopWithClients(opts.name, namespace, opts.timeout, kube, strimziClient)
}

func runStopWithClients(name string, namespace string, timeout uint32, kube kubeClients, strimziClient strimzi.Interface) error {
	kafka, err := strimziClient.KafkaV1().Kafkas(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("Kafka cluster %v in namespace %s not found: %w", name, namespace, err)
	}

	if !isReconciliationPaused(kafka) {
		pausedKafka := kafka.DeepCopy()
		if pausedKafka.Annotations == nil {
			pausedKafka.Annotations = map[string]string{"strimzi.io/pause-reconciliation": "true"}
		} else {
			pausedKafka.Annotations["strimzi.io/pause-reconciliation"] = "true"
		}

		log.Printf("Pausing reconciliation of Kafka cluster %s in namespace %s", name, namespace)
		_, err = strimziClient.KafkaV1().Kafkas(namespace).Update(context.TODO(), pausedKafka, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("failed to pause Kafka cluster %s in namespace %s: %w", name, namespace, err)
		}

		log.Printf("Waiting for Kafka cluster %s in namespace %s reconciliation to be paused", name, namespace)
		_, err = waitUntilReconciliationPausedFn(strimziClient, name, namespace, timeout)
		if err != nil {
			return err
		}

		log.Printf("Reconciliation of Kafka cluster %s in namespace %s is paused", name, namespace)
	} else {
		log.Printf("Kafka cluster %s in namespace %s has already paused reconciliation", name, namespace)
	}

	var deploymentDeletes []func() error
	if kafka.Spec.CruiseControl != nil {
		deploymentDeletes = append(deploymentDeletes, func() error {
			return deleteDeploymentFn(kube.AppsV1(), name, "cruise-control", namespace, timeout)
		})
	}
	if kafka.Spec.KafkaExporter != nil {
		deploymentDeletes = append(deploymentDeletes, func() error {
			return deleteDeploymentFn(kube.AppsV1(), name, "kafka-exporter", namespace, timeout)
		})
	}
	if kafka.Spec.EntityOperator != nil {
		deploymentDeletes = append(deploymentDeletes, func() error {
			return deleteDeploymentFn(kube.AppsV1(), name, "entity-operator", namespace, timeout)
		})
	}

	if err = runDeleteWorkers(deploymentDeletes); err != nil {
		return err
	}

	nodePools, err := strimziClient.KafkaV1().KafkaNodePools(namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: "strimzi.io/cluster=" + name})
	if err != nil {
		return fmt.Errorf("failed to get KafkaNodePools belonging to Kafka cluster %s in namespace %s: %w", name, namespace, err)
	}

	log.Printf("Stopping all Kafka nodes with broker role only")
	var brokerDeletes []func() error
	for _, pool := range nodePools.Items {
		if !slices.Contains(pool.Spec.Roles, kafkaapi.CONTROLLER_PROCESSROLES) {
			poolName := pool.Name
			brokerDeletes = append(brokerDeletes, func() error {
				return deletePodSetFn(kube.CoreV1(), strimziClient, name, poolName, namespace, timeout)
			})
		}
	}
	if err = runDeleteWorkers(brokerDeletes); err != nil {
		return err
	}

	log.Printf("Stopping all remaining Kafka nodes")
	var controllerDeletes []func() error
	for _, pool := range nodePools.Items {
		if slices.Contains(pool.Spec.Roles, kafkaapi.CONTROLLER_PROCESSROLES) {
			poolName := pool.Name
			controllerDeletes = append(controllerDeletes, func() error {
				return deletePodSetFn(kube.CoreV1(), strimziClient, name, poolName, namespace, timeout)
			})
		}
	}
	if err = runDeleteWorkers(controllerDeletes); err != nil {
		return err
	}

	log.Printf("Kafka cluster %s in namespace %s has been stopped", name, namespace)
	return nil
}

func runDeleteWorkers(workers []func() error) error {
	var wg sync.WaitGroup
	errCh := make(chan error, len(workers))

	for _, worker := range workers {
		worker := worker
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := worker(); err != nil {
				errCh <- err
			}
		}()
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		if err != nil {
			return err
		}
	}

	return nil
}

func init() {
	rootCmd.AddCommand(stopCmd)
}
