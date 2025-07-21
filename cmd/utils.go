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
	kafkaapi "github.com/scholzj/strimzi-go/pkg/apis/kafka.strimzi.io/v1beta2"
	strimzi "github.com/scholzj/strimzi-go/pkg/client/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"log"
	"os"
	"path/filepath"
	"time"
)

func strimziClient(kubeConfig *rest.Config) *strimzi.Clientset {
	strimzi, err := strimzi.NewForConfig(kubeConfig)
	if err != nil {
		log.Fatalf("Failed to create Strimzi client: %v", err)
	}

	return strimzi
}

func kubeConfigAndNamespace(kubeConfigOption string) (*rest.Config, string, error) {
	kubeConfigPath := kubeConfigPath(kubeConfigOption)

	if kubeConfigPath != "" {
		// Create the config
		config, err := clientcmd.BuildConfigFromFlags("", kubeConfigPath)
		if err != nil {
			return nil, "", fmt.Errorf("failed to instantiate Kubernetes configuration from %v: %v", kubeConfigPath, err)
		}

		// Try to get the namespace -> we might not need it, so we silence the errors
		var namespace string
		fileConfig, err := clientcmd.LoadFromFile(kubeConfigPath)
		if err != nil {
			log.Printf("Failed to parse Kubernetes client configuration to get default namespace: %v", err)
		} else {
			ns := fileConfig.Contexts[fileConfig.CurrentContext].Namespace

			if ns != "" {
				namespace = fileConfig.Contexts[fileConfig.CurrentContext].Namespace
			}
		}

		return config, namespace, nil
	} else {
		// Create the in-cluster config
		config, err := rest.InClusterConfig()
		if err != nil {
			return nil, "", fmt.Errorf("failed to instantiate Kubernetes in-cluster configuration: %v", err)
		}

		// Try to get the namespace -> we might not need it, so we silence the errors
		var namespace string
		namespaceBytes, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
		if err != nil {
			log.Printf("Failed to read namespace from /var/run/secrets/kubernetes.io/serviceaccount/namespace: %v", err)
			namespace = ""
		} else {
			namespace = string(namespaceBytes)
		}

		return config, namespace, nil
	}
}

func kubeConfigPath(kubeConfigOption string) string {
	if kubeConfigOption == "" {
		var kubeConfigPath string

		if os.Getenv("KUBECONFIG") != "" {
			kubeConfigPath = os.Getenv("KUBECONFIG")
			log.Printf("Using kubeconfig %s", kubeConfigPath)
			return kubeConfigPath
		} else if home := homedir.HomeDir(); home != "" {
			kubeConfigPath = filepath.Join(home, ".kube", "config")
			log.Printf("Using kubeconfig %s", kubeConfigPath)
		} else {
			log.Printf("Could not find Kubernetes configuration file. In-cluster configuration will be used.")
		}

		return kubeConfigPath
	} else {
		return kubeConfigOption
	}
}

func determineNamespace(namespaceOption string, kubeConfigNamespace string) (string, error) {
	if namespaceOption != "" {
		return namespaceOption, nil
	} else if kubeConfigNamespace != "" {
		return kubeConfigNamespace, nil
	} else {
		return "", fmt.Errorf("namespace has to be specified using the --namespace option or as part of the Kubernetes client configuration")
	}
}

func waitUntilReady(client *strimzi.Clientset, name string, namespace string) (bool, error) {
	watchContext, watchContextCancel := context.WithTimeout(context.Background(), time.Minute*5)
	defer watchContextCancel()

	watcher, err := client.KafkaV1beta2().Kafkas(namespace).Watch(watchContext, metav1.ListOptions{FieldSelector: fields.OneTermEqualSelector(metav1.ObjectNameField, name).String()})
	if err != nil {
		panic(err)
	}

	defer watcher.Stop()

	for {
		select {
		case event := <-watcher.ResultChan():
			if isReady(event.Object.(*kafkaapi.Kafka)) {
				return true, nil
			}
		case <-watchContext.Done():
			return false, fmt.Errorf("timed out waiting for the Kafka cluster %s in namespace %s to be ready", name, namespace)
		}
	}
}

func isReady(k *kafkaapi.Kafka) bool {
	if k.Status != nil && k.Status.Conditions != nil && len(k.Status.Conditions) > 0 {
		for _, condition := range k.Status.Conditions {
			if condition.Type == "Ready" && condition.Status == "True" {
				if k.Status.ObservedGeneration == k.ObjectMeta.Generation {
					//log.Print("The Kafka cluster is ready and up-to-date")
					return true
				}
			}
		}

		//log.Print("The Kafka cluster has conditions but is not ready")
		return false
	} else {
		//log.Print("The Kafka cluster has no conditions")
		return false
	}
}

func waitUntilReconciliationPaused(client *strimzi.Clientset, name string, namespace string) (bool, error) {
	watchContext, watchContextCancel := context.WithTimeout(context.Background(), time.Minute*5)
	defer watchContextCancel()

	watcher, err := client.KafkaV1beta2().Kafkas(namespace).Watch(watchContext, metav1.ListOptions{FieldSelector: fields.OneTermEqualSelector(metav1.ObjectNameField, name).String()})
	if err != nil {
		panic(err)
	}

	defer watcher.Stop()

	for {
		select {
		case event := <-watcher.ResultChan():
			if isReconciliationPaused(event.Object.(*kafkaapi.Kafka)) {
				return true, nil
			}
		case <-watchContext.Done():
			return false, fmt.Errorf("timed out waiting for the Kafka cluster %s in namespace %s to be paused", name, namespace)
		}
	}
}

func isReconciliationPaused(k *kafkaapi.Kafka) bool {
	if k.Status != nil && k.Status.Conditions != nil && len(k.Status.Conditions) > 0 {
		for _, condition := range k.Status.Conditions {
			if condition.Type == "ReconciliationPaused" && condition.Status == "True" {
				//log.Print("The Kafka cluster is ready and up-to-date")
				return true
			}
		}

		//log.Print("The Kafka cluster has conditions but is not ready")
		return false
	} else {
		//log.Print("The Kafka cluster has no conditions")
		return false
	}
}

func deletePodSet(strimzi *strimzi.Clientset, clusterName string, poolName string, namespace string) error {
	podSetName := clusterName + "-" + poolName
	log.Printf("Deleting StrimziPodSet %s for KafkaNodePool %s", podSetName, poolName)

	err := strimzi.CoreV1beta2().StrimziPodSets(namespace).Delete(context.TODO(), podSetName, metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete StrimziPodset %s in namespace %s: %v", podSetName, namespace, err)
	} else {
		return nil
	}
}
