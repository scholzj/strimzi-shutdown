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
	"testing"

	strimzi "github.com/scholzj/strimzi-go/pkg/client/clientset/versioned"
	strimzifake "github.com/scholzj/strimzi-go/pkg/client/clientset/versioned/fake"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestRunContinueCommand_ReturnsErrorWhenNameMissing(t *testing.T) {
	cmd := &cobra.Command{}
	addPersistentFlags(cmd)

	err := runContinueCommand(cmd, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if err.Error() != "--name option is required" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunContinueWithClients_UnpausesPausedClusterAndWaitsForReady(t *testing.T) {
	strimziClient := strimzifake.NewSimpleClientset(newKafkaResource(
		"my-cluster",
		"ns",
		3,
		3,
		map[string]string{"strimzi.io/pause-reconciliation": "true"},
		newCondition("ReconciliationPaused", metav1.ConditionTrue),
	))

	waitCalled := false
	oldWaitFn := waitUntilReadyFn
	waitUntilReadyFn = func(client strimzi.Interface, name string, namespace string, timeout uint32) (bool, error) {
		waitCalled = true
		if name != "my-cluster" || namespace != "ns" || timeout != 123 {
			t.Fatalf("unexpected wait args: %s %s %d", name, namespace, timeout)
		}
		return true, nil
	}
	t.Cleanup(func() {
		waitUntilReadyFn = oldWaitFn
	})

	err := runContinueWithClients("my-cluster", "ns", 123, strimziClient)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if !waitCalled {
		t.Fatal("expected waitUntilReady to be called")
	}

	updatedKafka, err := strimziClient.KafkaV1().Kafkas("ns").Get(context.TODO(), "my-cluster", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed to get updated Kafka resource: %v", err)
	}

	if updatedKafka.Annotations["strimzi.io/pause-reconciliation"] != "false" {
		t.Fatalf("expected reconciliation annotation to be false, got %q", updatedKafka.Annotations["strimzi.io/pause-reconciliation"])
	}
}

func TestRunContinueWithClients_ReturnsNilWhenClusterAlreadyReady(t *testing.T) {
	strimziClient := strimzifake.NewSimpleClientset(newKafkaResource(
		"my-cluster",
		"ns",
		3,
		3,
		nil,
		newCondition("Ready", metav1.ConditionTrue),
	))

	waitCalled := false
	oldWaitFn := waitUntilReadyFn
	waitUntilReadyFn = func(client strimzi.Interface, name string, namespace string, timeout uint32) (bool, error) {
		waitCalled = true
		return true, nil
	}
	t.Cleanup(func() {
		waitUntilReadyFn = oldWaitFn
	})

	err := runContinueWithClients("my-cluster", "ns", 123, strimziClient)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if waitCalled {
		t.Fatal("did not expect waitUntilReady to be called")
	}
}
