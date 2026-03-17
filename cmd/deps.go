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
	strimzi "github.com/scholzj/strimzi-go/pkg/client/clientset/versioned"
	appsv1client "k8s.io/client-go/kubernetes/typed/apps/v1"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
)

type kubeClients interface {
	AppsV1() appsv1client.AppsV1Interface
	CoreV1() corev1client.CoreV1Interface
}

var loadKubeConfigAndNamespace = kubeConfigAndNamespace

var newKubeClients = func(kubeConfig *rest.Config) (kubeClients, error) {
	return kubeClient(kubeConfig)
}

var newStrimziClients = func(kubeConfig *rest.Config) (strimzi.Interface, error) {
	return strimziClient(kubeConfig)
}

var waitUntilReadyFn = waitUntilReady
var waitUntilReconciliationPausedFn = waitUntilReconciliationPaused
var deleteDeploymentFn = deleteDeployment
var deletePodSetFn = deletePodSet
