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
	"reflect"
	"testing"

	kafkaapi "github.com/scholzj/strimzi-go/pkg/apis/kafka.strimzi.io/v1"
	strimzi "github.com/scholzj/strimzi-go/pkg/client/clientset/versioned"
	strimzifake "github.com/scholzj/strimzi-go/pkg/client/clientset/versioned/fake"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	appsv1client "k8s.io/client-go/kubernetes/typed/apps/v1"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
)

type fakeKubeClients struct {
	appsClient appsv1client.AppsV1Interface
	coreClient corev1client.CoreV1Interface
}

func (f fakeKubeClients) AppsV1() appsv1client.AppsV1Interface {
	return f.appsClient
}

func (f fakeKubeClients) CoreV1() corev1client.CoreV1Interface {
	return f.coreClient
}

func TestRunStopCommand_ReturnsErrorWhenNameMissing(t *testing.T) {
	cmd := &cobra.Command{}
	addPersistentFlags(cmd)

	err := runStopCommand(cmd, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if err.Error() != "--name option is required" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunStopWithClients_PausesClusterAndWaitsForReconciliationPause(t *testing.T) {
	kube := k8sfake.NewSimpleClientset()
	strimziClient := strimzifake.NewSimpleClientset(newKafkaResource("my-cluster", "ns", 3, 3, nil))

	waitCalled := false
	oldWaitFn := waitUntilReconciliationPausedFn
	oldDeletePodSetFn := deletePodSetFn
	waitUntilReconciliationPausedFn = func(client strimzi.Interface, name string, namespace string, timeout uint32) (bool, error) {
		waitCalled = true
		return true, nil
	}
	deletePodSetFn = func(kubeClient corev1client.CoreV1Interface, strimziClient strimzi.Interface, clusterName string, poolName string, namespace string, timeout uint32) error {
		return nil
	}
	t.Cleanup(func() {
		waitUntilReconciliationPausedFn = oldWaitFn
		deletePodSetFn = oldDeletePodSetFn
	})

	err := runStopWithClients("my-cluster", "ns", 123, fakeKubeClients{kube.AppsV1(), kube.CoreV1()}, strimziClient)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if !waitCalled {
		t.Fatal("expected waitUntilReconciliationPaused to be called")
	}

	updatedKafka, err := strimziClient.KafkaV1().Kafkas("ns").Get(context.TODO(), "my-cluster", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed to get updated Kafka resource: %v", err)
	}

	if updatedKafka.Annotations["strimzi.io/pause-reconciliation"] != "true" {
		t.Fatalf("expected reconciliation annotation to be true, got %q", updatedKafka.Annotations["strimzi.io/pause-reconciliation"])
	}
}

func TestRunStopWithClients_DeletesBrokerPoolsBeforeControllerPools(t *testing.T) {
	kube := k8sfake.NewSimpleClientset()
	strimziClient := strimzifake.NewSimpleClientset(
		newKafkaResource("my-cluster", "ns", 3, 3, nil, newCondition("ReconciliationPaused", metav1.ConditionTrue)),
		newKafkaNodePoolResource("broker-pool", "ns", "my-cluster", kafkaapi.BROKER_PROCESSROLES),
		newKafkaNodePoolResource("controller-pool", "ns", "my-cluster", kafkaapi.CONTROLLER_PROCESSROLES),
	)

	oldWaitFn := waitUntilReconciliationPausedFn
	oldDeletePodSetFn := deletePodSetFn
	waitUntilReconciliationPausedFn = func(client strimzi.Interface, name string, namespace string, timeout uint32) (bool, error) {
		t.Fatal("did not expect waitUntilReconciliationPaused to be called")
		return false, nil
	}
	var deletedPools []string
	deletePodSetFn = func(kubeClient corev1client.CoreV1Interface, strimziClient strimzi.Interface, clusterName string, poolName string, namespace string, timeout uint32) error {
		deletedPools = append(deletedPools, poolName)
		return nil
	}
	t.Cleanup(func() {
		waitUntilReconciliationPausedFn = oldWaitFn
		deletePodSetFn = oldDeletePodSetFn
	})

	err := runStopWithClients("my-cluster", "ns", 123, fakeKubeClients{kube.AppsV1(), kube.CoreV1()}, strimziClient)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if !reflect.DeepEqual(deletedPools, []string{"broker-pool", "controller-pool"}) {
		t.Fatalf("unexpected pool deletion order: %v", deletedPools)
	}
}

func newKafkaResource(name string, namespace string, generation int64, observedGeneration int64, annotations map[string]string, conditions ...kafkaapi.Condition) *kafkaapi.Kafka {
	return &kafkaapi.Kafka{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   namespace,
			Generation:  generation,
			Annotations: annotations,
		},
		Spec: &kafkaapi.KafkaSpec{},
		Status: &kafkaapi.KafkaStatus{
			ObservedGeneration: observedGeneration,
			Conditions:         conditions,
		},
	}
}

func newKafkaNodePoolResource(name string, namespace string, clusterName string, roles ...kafkaapi.ProcessRoles) *kafkaapi.KafkaNodePool {
	return &kafkaapi.KafkaNodePool{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				"strimzi.io/cluster": clusterName,
			},
		},
		Spec: &kafkaapi.KafkaNodePoolSpec{
			Roles: roles,
		},
	}
}
