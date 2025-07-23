# Strimzi Shutdown

_Note: This is not part of the Strimzi CNCF project!_

Strimzi Shutdown is a simple utility to temporarily stop or restart your [Strimzi-based Apache Kafka cluster](https://strimzi.io).

## How to use Strimzi Shutdown?

### Installation

You can download one of the release binaries from one of the [GitHub releases](https://github.com/scholzj/strimzi-shutdown/releases) and use it.
Alternatively, you can also use the provided container image to run it from a Kubernetes Pod or locally as a container.

### Configuration options

Strimzi Shutdown supports several command line options:

| Option             | Description                                                                                                                                                | Default Value |
|--------------------|------------------------------------------------------------------------------------------------------------------------------------------------------------|---------------|
| `--help` / `-h`    | Help                                                                                                                                                       |               |
| `--kubeconfig`     | Path to the kubeconfig file to use for Kubernetes API requests. If not specified, `strimzi-shutdown` will try to auto-detect the Kubernetes configuration. |               |
| `--namespace`      | Namespace of the Kafka cluster. If not specified, defaults to the namespace from your Kubernetes configuration.                                            |               |
| `--name`           | Name of the Kafka cluster.                                                                                                                                 |               |
| `--timeout` / `-t` | Timeout for how long to wait for the Proxy Pod to become ready. In milliseconds.                                                                           | `300000`      |

### Stopping your Kafka cluster

You can stop your Strimzi-based Apache Kafka cluster using the `stop` command.
The `stop` command will execute the following steps:
1. Pause the reconciliation of the Kafka cluster and wait for the Strimzi Cluster Operator to confirm it
2. Stop all auxiliary tools (Cruise Control, Kafka Exporter, and Entity Operator)
3. Stop all Kafka nodes with the broker role only
4. Stop all Kafka nodes with the controller role

The following snippet shows an example of the `stop` command:

```
$ ./strimzi-shutdown stop --namespace myproject --name my-cluster
2025/07/22 11:43:08 Using kubeconfig /Users/scholzj/.kube/config
2025/07/22 11:43:08 Pausing reconciliation of Kafka cluster my-cluster in namespace myproject
2025/07/22 11:43:08 Waiting for Kafka cluster my-cluster in namespace myproject reconciliation to be paused
2025/07/22 11:43:08 Reconciliation of Kafka cluster my-cluster in namespace myproject is paused
2025/07/22 11:43:08 Deleting Deployment my-cluster-entity-operator in namespace myproject
2025/07/22 11:43:08 Deleting Deployment my-cluster-cruise-control in namespace myproject
2025/07/22 11:43:08 Deleting Deployment my-cluster-kafka-exporter in namespace myproject
2025/07/22 11:43:10 Deployment my-cluster-kafka-exporter in namespace myproject has been deleted
2025/07/22 11:43:10 Deployment my-cluster-entity-operator in namespace myproject has been deleted
2025/07/22 11:43:10 Stopping all Kafka nodes with broker role only
2025/07/22 11:43:10 Deleting StrimziPodSet my-cluster-witton for KafkaNodePool witton
2025/07/22 11:43:10 Deleting StrimziPodSet my-cluster-aston for KafkaNodePool aston
2025/07/22 11:43:10 Deleting StrimziPodSet my-cluster-bodymoor for KafkaNodePool bodymoor
2025/07/22 11:43:13 Stopping all remaining Kafka nodes
2025/07/22 11:43:13 Deleting StrimziPodSet my-cluster-controllers for KafkaNodePool controllers
2025/07/22 11:43:20 Kafka cluster my-cluster in namespace myproject has been stopped
```

### Restarting your Kafka cluster

You can restart your stopped Strimzi-based Apache Kafka cluster using the `continue` command.
The `continue` command will unpause the reconciliation of the Apache Kafka cluster, and the Strimzi Cluster Operator will restore it.
The `continue` command will wait until the Kafka cluster is ready before exiting.

The following snippet shows an example of the `continue` command:

```
$ ./strimzi-shutdown continue --namespace myproject --name my-cluster
2025/07/22 11:44:32 Using kubeconfig /Users/scholzj/.kube/config
2025/07/22 11:44:32 Reconciliation of Kafka cluster my-cluster in namespace myproject will be unpaused
2025/07/22 11:44:32 Unpausing reconciliation of Kafka cluster my-cluster in namespace myproject
2025/07/22 11:44:32 Waiting for Kafka cluster my-cluster in namespace myproject to get ready.
2025/07/22 11:47:02 Kafka cluster my-cluster in namespace myproject has been restarted and should be ready
```

### Scheduled shutdown and restart of a Kafka cluster using Kubernetes `CronJob`

You can use Kubernetes `CronJob` with Strimzi Shutdown to stop or restart your Kafka cluster at a specific time.
To do so, you can use the following `CronJob`:

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: strimzi-shutdown

---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: strimzi-shutdown
rules:
  - apiGroups: [""]
    resources: ["pods"]
    verbs: ["get", "list"]
  - apiGroups: ["apps"]
    resources: ["deployments"]
    verbs: ["get", "watch", "list", "delete"]
  - apiGroups: ["kafka.strimzi.io"]
    resources: ["kafkas", "kafkanodepools"]
    verbs: ["get", "watch", "list", "update"]
  - apiGroups: ["core.strimzi.io"]
    resources: ["strimzipodsets"]
    verbs: ["get", "list", "delete"]

---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: strimzi-shutdown
subjects:
  - kind: ServiceAccount
    name: strimzi-shutdown
roleRef:
  kind: Role
  name: strimzi-shutdown
  apiGroup: rbac.authorization.k8s.io

---
apiVersion: batch/v1
kind: CronJob
metadata:
  name: shutdown-my-cluster
spec:
  schedule: "00 20 * * 1-5"
  jobTemplate:
    spec:
      template:
        spec:
          serviceAccountName: strimzi-shutdown
          containers:
            - name: strimzi-shutdown
              image: ghcr.io/scholzj/strimzi-shutdown:0.1.0
              command:
                - /strimzi-shutdown
                - stop
                - --name=my-cluster
          restartPolicy: OnFailure

---
apiVersion: batch/v1
kind: CronJob
metadata:
  name: restart-my-cluster
spec:
  schedule: "00 8 * * 1-5"
  jobTemplate:
    spec:
      template:
        spec:
          serviceAccountName: strimzi-shutdown
          containers:
            - name: strimzi-shutdown
              image: ghcr.io/scholzj/strimzi-shutdown:0.1.0
              command:
                - /strimzi-shutdown
                - continue
                - --name=my-cluster
          restartPolicy: OnFailure
```

This example will stop the Kafka cluster named `my-cluster` every Monday to Friday at 8 pm.
And it will start the same Kafka cluster again every Monday to Friday at 8 am.

Note: If you want to deploy the `CronJob` into a different namespace from where the Kafka cluster is running, you would need to adjust the RBAC resources accordingly and use the `--namespace` option. 

## Frequently Asked Questions

### Does Strimzi Shutdown support ZooKeeper-based clusters?

No, Strimzi shutdown currently supports only KRaft-based Apache Kafka clusters.

### Any plans to support other Strimzi resources?

No, the other Strimzi operands, such as Kafka Connect, Mirror Maker, or Bridge are stateless and do not need any special process to be stopped.
If you want to stop them, you can scale them to `0` by editing the resource and changing the `.spec.replicas` field or using `kubectl scale`.
