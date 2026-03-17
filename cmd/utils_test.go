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
	"errors"
	"strings"
	"testing"
	"time"

	coreapi "github.com/scholzj/strimzi-go/pkg/apis/core.strimzi.io/v1"
	kafkaapi "github.com/scholzj/strimzi-go/pkg/apis/kafka.strimzi.io/v1"
	strimzifake "github.com/scholzj/strimzi-go/pkg/client/clientset/versioned/fake"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

func TestDetermineNamespace_UsesExplicitNamespace(t *testing.T) {
	namespace, err := determineNamespace("explicit", "from-kubeconfig")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if namespace != "explicit" {
		t.Fatalf("expected explicit namespace, got %q", namespace)
	}
}

func TestDetermineNamespace_FallsBackToKubeconfigNamespace(t *testing.T) {
	namespace, err := determineNamespace("", "from-kubeconfig")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if namespace != "from-kubeconfig" {
		t.Fatalf("expected kubeconfig namespace, got %q", namespace)
	}
}

func TestDetermineNamespace_ReturnsErrorWhenMissing(t *testing.T) {
	_, err := determineNamespace("", "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !strings.Contains(err.Error(), "namespace has to be specified") {
		t.Fatalf("expected missing namespace error, got %v", err)
	}
}

func TestIsReady(t *testing.T) {
	tests := []struct {
		name  string
		kafka *kafkaapi.Kafka
		want  bool
	}{
		{
			name:  "returns true for ready condition with observed generation match",
			kafka: newKafkaWithConditions(3, 3, newCondition("Ready", metav1.ConditionTrue)),
			want:  true,
		},
		{
			name:  "returns false when ready condition missing",
			kafka: newKafkaWithConditions(3, 3, newCondition("Warning", metav1.ConditionTrue)),
			want:  false,
		},
		{
			name:  "returns false when ready condition false",
			kafka: newKafkaWithConditions(3, 3, newCondition("Ready", metav1.ConditionFalse)),
			want:  false,
		},
		{
			name:  "returns false when observed generation is stale",
			kafka: newKafkaWithConditions(3, 2, newCondition("Ready", metav1.ConditionTrue)),
			want:  false,
		},
		{
			name:  "returns false when status is nil",
			kafka: &kafkaapi.Kafka{ObjectMeta: metav1.ObjectMeta{Generation: 3}},
			want:  false,
		},
		{
			name:  "returns false when conditions are empty",
			kafka: newKafkaWithConditions(3, 3),
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isReady(tt.kafka)
			if got != tt.want {
				t.Fatalf("expected %t, got %t", tt.want, got)
			}
		})
	}
}

func TestIsReconciliationPaused(t *testing.T) {
	tests := []struct {
		name  string
		kafka *kafkaapi.Kafka
		want  bool
	}{
		{
			name:  "returns true for paused condition",
			kafka: newKafkaWithConditions(3, 3, newCondition("ReconciliationPaused", metav1.ConditionTrue)),
			want:  true,
		},
		{
			name:  "returns false when paused condition missing",
			kafka: newKafkaWithConditions(3, 3, newCondition("Ready", metav1.ConditionTrue)),
			want:  false,
		},
		{
			name:  "returns false when paused condition false",
			kafka: newKafkaWithConditions(3, 3, newCondition("ReconciliationPaused", metav1.ConditionFalse)),
			want:  false,
		},
		{
			name:  "returns false when status is nil",
			kafka: &kafkaapi.Kafka{ObjectMeta: metav1.ObjectMeta{Generation: 3}},
			want:  false,
		},
		{
			name:  "returns false when conditions are empty",
			kafka: newKafkaWithConditions(3, 3),
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isReconciliationPaused(tt.kafka)
			if got != tt.want {
				t.Fatalf("expected %t, got %t", tt.want, got)
			}
		})
	}
}

func TestWaitUntilReady_ReturnsTrueWhenReadyEventArrives(t *testing.T) {
	strimziClient := strimzifake.NewSimpleClientset()
	fakeWatch := watch.NewFake()

	strimziClient.PrependWatchReactor("kafkas", func(action k8stesting.Action) (bool, watch.Interface, error) {
		return true, fakeWatch, nil
	})

	errCh := make(chan error, 1)
	go func() {
		_, err := waitUntilReady(strimziClient, "my-cluster", "ns", 100)
		errCh <- err
	}()

	fakeWatch.Modify(newKafkaResource("my-cluster", "ns", 3, 3, nil, newCondition("Ready", metav1.ConditionTrue)))

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for ready result")
	}
}

func TestWaitUntilReady_IgnoresNonReadyEvents(t *testing.T) {
	strimziClient := strimzifake.NewSimpleClientset()
	fakeWatch := watch.NewFake()

	strimziClient.PrependWatchReactor("kafkas", func(action k8stesting.Action) (bool, watch.Interface, error) {
		return true, fakeWatch, nil
	})

	errCh := make(chan error, 1)
	go func() {
		_, err := waitUntilReady(strimziClient, "my-cluster", "ns", 100)
		errCh <- err
	}()

	fakeWatch.Modify(newKafkaResource("my-cluster", "ns", 3, 3, nil, newCondition("Ready", metav1.ConditionFalse)))
	fakeWatch.Modify(newKafkaResource("my-cluster", "ns", 3, 3, nil, newCondition("Ready", metav1.ConditionTrue)))

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for ready result")
	}
}

func TestWaitUntilReady_ReturnsErrorOnTimeout(t *testing.T) {
	strimziClient := strimzifake.NewSimpleClientset()
	fakeWatch := watch.NewFake()
	defer fakeWatch.Stop()

	strimziClient.PrependWatchReactor("kafkas", func(action k8stesting.Action) (bool, watch.Interface, error) {
		return true, fakeWatch, nil
	})

	_, err := waitUntilReady(strimziClient, "my-cluster", "ns", 10)
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}

	if !strings.Contains(err.Error(), "timed out waiting for the Kafka cluster my-cluster in namespace ns to be ready") {
		t.Fatalf("expected timeout error, got %v", err)
	}
}

func TestWaitUntilReconciliationPaused_ReturnsTrueWhenPausedEventArrives(t *testing.T) {
	strimziClient := strimzifake.NewSimpleClientset()
	fakeWatch := watch.NewFake()

	strimziClient.PrependWatchReactor("kafkas", func(action k8stesting.Action) (bool, watch.Interface, error) {
		return true, fakeWatch, nil
	})

	errCh := make(chan error, 1)
	go func() {
		_, err := waitUntilReconciliationPaused(strimziClient, "my-cluster", "ns", 100)
		errCh <- err
	}()

	fakeWatch.Modify(newKafkaResource("my-cluster", "ns", 3, 3, nil, newCondition("ReconciliationPaused", metav1.ConditionTrue)))

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for reconciliation pause result")
	}
}

func TestWaitUntilReconciliationPaused_IgnoresNonPausedEvents(t *testing.T) {
	strimziClient := strimzifake.NewSimpleClientset()
	fakeWatch := watch.NewFake()

	strimziClient.PrependWatchReactor("kafkas", func(action k8stesting.Action) (bool, watch.Interface, error) {
		return true, fakeWatch, nil
	})

	errCh := make(chan error, 1)
	go func() {
		_, err := waitUntilReconciliationPaused(strimziClient, "my-cluster", "ns", 100)
		errCh <- err
	}()

	fakeWatch.Modify(newKafkaResource("my-cluster", "ns", 3, 3, nil, newCondition("ReconciliationPaused", metav1.ConditionFalse)))
	fakeWatch.Modify(newKafkaResource("my-cluster", "ns", 3, 3, nil, newCondition("ReconciliationPaused", metav1.ConditionTrue)))

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for reconciliation pause result")
	}
}

func TestWaitUntilReconciliationPaused_ReturnsErrorOnTimeout(t *testing.T) {
	strimziClient := strimzifake.NewSimpleClientset()
	fakeWatch := watch.NewFake()
	defer fakeWatch.Stop()

	strimziClient.PrependWatchReactor("kafkas", func(action k8stesting.Action) (bool, watch.Interface, error) {
		return true, fakeWatch, nil
	})

	_, err := waitUntilReconciliationPaused(strimziClient, "my-cluster", "ns", 10)
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}

	if !strings.Contains(err.Error(), "timed out waiting for the Kafka cluster my-cluster in namespace ns to be paused") {
		t.Fatalf("expected timeout error, got %v", err)
	}
}

func TestWaitForPodSetPodsDeletion_ReturnsNilWhenNoPodsExist(t *testing.T) {
	kube := k8sfake.NewSimpleClientset()

	err := waitForPodSetPodsDeletion(kube.CoreV1(), "my-cluster", "pool-a", "my-cluster-pool-a", "ns", 50)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestWaitForPodSetPodsDeletion_ReturnsNilWhenPodsDisappear(t *testing.T) {
	kube := k8sfake.NewSimpleClientset(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod-0",
			Namespace: "ns",
			Labels: map[string]string{
				"strimzi.io/cluster":   "my-cluster",
				"strimzi.io/pool-name": "pool-a",
			},
		},
	})

	listCalls := 0
	kube.PrependReactor("list", "pods", func(action k8stesting.Action) (bool, runtime.Object, error) {
		listCalls++
		if listCalls == 1 {
			return true, &corev1.PodList{Items: []corev1.Pod{{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod-0",
					Namespace: "ns",
				},
			}}}, nil
		}

		return true, &corev1.PodList{}, nil
	})

	err := waitForPodSetPodsDeletion(kube.CoreV1(), "my-cluster", "pool-a", "my-cluster-pool-a", "ns", 1500)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestWaitForPodSetPodsDeletion_ReturnsErrorWhenPodListFails(t *testing.T) {
	kube := k8sfake.NewSimpleClientset()
	wantErr := errors.New("pod list failed")

	kube.PrependReactor("list", "pods", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, wantErr
	})

	err := waitForPodSetPodsDeletion(kube.CoreV1(), "my-cluster", "pool-a", "my-cluster-pool-a", "ns", 50)
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected %v, got %v", wantErr, err)
	}
}

func TestWaitForPodSetPodsDeletion_ReturnsErrorOnTimeout(t *testing.T) {
	kube := k8sfake.NewSimpleClientset(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod-0",
			Namespace: "ns",
			Labels: map[string]string{
				"strimzi.io/cluster":   "my-cluster",
				"strimzi.io/pool-name": "pool-a",
			},
		},
	})

	err := waitForPodSetPodsDeletion(kube.CoreV1(), "my-cluster", "pool-a", "my-cluster-pool-a", "ns", 10)
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}

	if !strings.Contains(err.Error(), "timed out waiting for Pod deletion") {
		t.Fatalf("expected timeout error, got %v", err)
	}
}

func TestWaitForDeploymentDeletion_ReturnsNilOnDeletedEvent(t *testing.T) {
	kube := k8sfake.NewSimpleClientset()
	fakeWatch := watch.NewFake()

	kube.PrependWatchReactor("deployments", func(action k8stesting.Action) (bool, watch.Interface, error) {
		return true, fakeWatch, nil
	})

	errCh := make(chan error, 1)
	go func() {
		errCh <- waitForDeploymentDeletion(kube.AppsV1(), "my-cluster-entity-operator", "ns", 100)
	}()

	fakeWatch.Delete(&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "my-cluster-entity-operator", Namespace: "ns"}})

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for deletion result")
	}
}

func TestWaitForDeploymentDeletion_ReturnsErrorOnTimeout(t *testing.T) {
	kube := k8sfake.NewSimpleClientset()
	fakeWatch := watch.NewFake()
	defer fakeWatch.Stop()

	kube.PrependWatchReactor("deployments", func(action k8stesting.Action) (bool, watch.Interface, error) {
		return true, fakeWatch, nil
	})

	err := waitForDeploymentDeletion(kube.AppsV1(), "my-cluster-entity-operator", "ns", 10)
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}

	if !strings.Contains(err.Error(), "timed out waiting for deletion of Deployment") {
		t.Fatalf("expected timeout error, got %v", err)
	}
}

func TestWaitForDeploymentDeletion_ReturnsErrorWhenWatchFails(t *testing.T) {
	kube := k8sfake.NewSimpleClientset()
	wantErr := errors.New("watch failed")

	kube.PrependWatchReactor("deployments", func(action k8stesting.Action) (bool, watch.Interface, error) {
		return true, nil, wantErr
	})

	err := waitForDeploymentDeletion(kube.AppsV1(), "my-cluster-entity-operator", "ns", 10)
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected %v, got %v", wantErr, err)
	}
}

func TestDeleteDeployment_ReturnsNilWhenDeploymentNotFound(t *testing.T) {
	kube := k8sfake.NewSimpleClientset()

	err := deleteDeployment(kube.AppsV1(), "my-cluster", "entity-operator", "ns", 10)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestDeleteDeployment_ReturnsWrappedErrorWhenDeleteFails(t *testing.T) {
	kube := k8sfake.NewSimpleClientset()
	wantErr := errors.New("delete failed")

	kube.PrependReactor("delete", "deployments", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, wantErr
	})

	err := deleteDeployment(kube.AppsV1(), "my-cluster", "entity-operator", "ns", 10)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !strings.Contains(err.Error(), "failed to delete Deployment my-cluster-entity-operator in namespace ns") {
		t.Fatalf("expected wrapped delete error, got %v", err)
	}

	if !strings.Contains(err.Error(), wantErr.Error()) {
		t.Fatalf("expected original error in %v", err)
	}
}

func TestDeleteDeployment_WaitsForDeletionAfterSuccessfulDelete(t *testing.T) {
	kube := k8sfake.NewSimpleClientset(&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "my-cluster-entity-operator", Namespace: "ns"}})
	fakeWatch := watch.NewFake()

	kube.PrependWatchReactor("deployments", func(action k8stesting.Action) (bool, watch.Interface, error) {
		return true, fakeWatch, nil
	})

	errCh := make(chan error, 1)
	go func() {
		errCh <- deleteDeployment(kube.AppsV1(), "my-cluster", "entity-operator", "ns", 100)
	}()

	time.Sleep(10 * time.Millisecond)
	fakeWatch.Delete(&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "my-cluster-entity-operator", Namespace: "ns"}})

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for deleteDeployment result")
	}
}

func TestDeletePodSet_ReturnsNilWhenPodSetNotFound(t *testing.T) {
	kube := k8sfake.NewSimpleClientset()
	strimziClient := strimzifake.NewSimpleClientset()

	err := deletePodSet(kube.CoreV1(), strimziClient, "my-cluster", "pool-a", "ns", 10)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestDeletePodSet_ReturnsWrappedErrorWhenDeleteFails(t *testing.T) {
	kube := k8sfake.NewSimpleClientset()
	strimziClient := strimzifake.NewSimpleClientset()
	wantErr := errors.New("delete failed")

	strimziClient.PrependReactor("delete", "strimzipodsets", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, wantErr
	})

	err := deletePodSet(kube.CoreV1(), strimziClient, "my-cluster", "pool-a", "ns", 10)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !strings.Contains(err.Error(), "failed to delete StrimziPodset my-cluster-pool-a in namespace ns") {
		t.Fatalf("expected wrapped delete error, got %v", err)
	}

	if !strings.Contains(err.Error(), wantErr.Error()) {
		t.Fatalf("expected original error in %v", err)
	}
}

func TestDeletePodSet_WaitsForPodsToDisappearAfterDelete(t *testing.T) {
	kube := k8sfake.NewSimpleClientset(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod-0",
			Namespace: "ns",
			Labels: map[string]string{
				"strimzi.io/cluster":   "my-cluster",
				"strimzi.io/pool-name": "pool-a",
			},
		},
	})
	strimziClient := strimzifake.NewSimpleClientset(&coreapi.StrimziPodSet{ObjectMeta: metav1.ObjectMeta{Name: "my-cluster-pool-a", Namespace: "ns"}})

	listCalls := 0
	kube.PrependReactor("list", "pods", func(action k8stesting.Action) (bool, runtime.Object, error) {
		listCalls++
		if listCalls == 1 {
			return true, &corev1.PodList{Items: []corev1.Pod{{ObjectMeta: metav1.ObjectMeta{Name: "pod-0", Namespace: "ns"}}}}, nil
		}

		return true, &corev1.PodList{}, nil
	})

	err := deletePodSet(kube.CoreV1(), strimziClient, "my-cluster", "pool-a", "ns", 1500)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func newKafkaWithConditions(generation int64, observedGeneration int64, conditions ...kafkaapi.Condition) *kafkaapi.Kafka {
	return &kafkaapi.Kafka{
		ObjectMeta: metav1.ObjectMeta{Generation: generation},
		Status: &kafkaapi.KafkaStatus{
			ObservedGeneration: observedGeneration,
			Conditions:         conditions,
		},
	}
}

func newCondition(conditionType string, status metav1.ConditionStatus) kafkaapi.Condition {
	return kafkaapi.Condition{
		Type:   conditionType,
		Status: string(status),
	}
}
