# Getting started

This guide takes you from an empty Kubernetes cluster to a running Camunda orchestration cluster that you can log in to.
You create a namespace and three resources: an `ElasticsearchCluster` for secondary storage, a `CamundaPlatformConfig` for authentication, and a `CamundaCluster`.

The sizes in this guide fit a local [kind](https://kind.sigs.k8s.io/) cluster. They are not production sizes.

The same manifests are ready to apply in [`config/example/camunda-cluster/elasticsearch`](https://github.com/konsole-is/camunda-operator/tree/main/config/example/camunda-cluster/elasticsearch). The link shows the `main` branch. Select the tag of your release on GitHub to get the manifests of that version. Follow the steps below to learn what each one does, or apply that directory in one command.

## Before you start

You need:

- `kubectl` and `helm` 3.8 or later
- a Kubernetes cluster that meets the [requirements](installation.md#requirements), with a default StorageClass that can bind two volumes of 16Gi each, 32Gi in total
- about 4 GB of free memory on the nodes
- nodes that can pull images from Docker Hub, `docker.elastic.co`, and `ghcr.io`

On a kind cluster, Elasticsearch needs `vm.max_map_count` of at least 262144 on the host:

```bash
sudo sysctl -w vm.max_map_count=262144
```

## 1. Install the ECK operator

The operator runs Elasticsearch through [Elastic Cloud on Kubernetes (ECK)](https://www.elastic.co/guide/en/cloud-on-k8s/current/index.html). Install ECK before the operator. The operator looks for the ECK CRDs when it starts. If you install ECK later, restart the operator.

```bash
kubectl apply --server-side -f https://download.elastic.co/downloads/eck/3.5.0/crds.yaml
kubectl apply --server-side -f https://download.elastic.co/downloads/eck/3.5.0/operator.yaml
```

Use `--server-side`: the ECK CRD manifest is larger than the annotation that client-side apply writes.

This guide uses Elasticsearch. For PostgreSQL secondary storage, install CloudNativePG instead of ECK. [Installation](installation.md#install-cloudnativepg-and-the-barman-cloud-plugin) has the commands, and the [secondary storage guide](guides/secondary-storage.md#postgresql) has the resources.

## 2. Install the operator

```bash
helm install camunda-operator \
  oci://ghcr.io/konsole-is/charts/camunda-operator \
  --version <version> \
  --namespace camunda-operator-system \
  --create-namespace
```

Replace `<version>` with a released version. Make sure that the manager is running:

```bash
kubectl get pods -n camunda-operator-system
```

The [installation guide](installation.md) has the other install options.

## 3. Create the namespace

```bash
kubectl create namespace my-cluster-ns
```

## 4. Create the Elasticsearch cluster

Apply an `ElasticsearchCluster`. The operator creates an ECK `Elasticsearch` resource, a user for Camunda, and a `SecondaryStorageConfig` with the connection details. The field `secondaryStorageConfig` sets the name of that `SecondaryStorageConfig`, here `my-storage-config`.

```yaml
apiVersion: core.camunda.io/v1
kind: ElasticsearchCluster
metadata:
  name: my-cluster-es
  namespace: my-cluster-ns
spec:
  version: "9.2.8"
  replicas: 1
  storageSize: 16Gi
  resources:
    requests: { cpu: 500m, memory: 1Gi }
  secondaryStorageConfig: my-storage-config
```

Wait until it is ready. The first start pulls the Elasticsearch image and can take a few minutes.

```bash
kubectl wait elasticsearchcluster/my-cluster-es -n my-cluster-ns \
  --for=condition=Ready --timeout=15m
```

## 5. Create the platform configuration

A `CamundaPlatformConfig` holds the settings that all clusters share. This one selects basic authentication and sets no license key. Without a license key, Camunda runs under its Non-Production License, and Operate and Tasklist show a note that says so.

```yaml
apiVersion: core.camunda.io/v1
kind: CamundaPlatformConfig
metadata:
  name: my-platform-config
spec:
  auth:
    method: basic
```

`CamundaPlatformConfig` is cluster-scoped. It has no namespace.

## 6. Create the Camunda cluster

```yaml
apiVersion: core.camunda.io/v1
kind: CamundaCluster
metadata:
  name: my-cluster
  namespace: my-cluster-ns
spec:
  version: "8.9.21"
  platformConfigRef: my-platform-config
  storageRef: my-storage-config
  zeebe:
    storageSize: 16Gi
    resources:
      requests: { cpu: "1", memory: 1.5Gi }
  gateway:
    resources:
      requests: { cpu: 500m, memory: 512Mi }
```

By default, the Zeebe broker stops processing commands when its volume has less than 2GB free. Keep `zeebe.storageSize` well above that, also on kind. See [Disk space](https://docs.camunda.io/docs/self-managed/components/orchestration-cluster/zeebe/operations/disk-space/) in the Camunda docs.

This cluster runs the default topology of a `CamundaCluster`: one Zeebe broker, and one gateway that also serves Operate, Tasklist, and Admin.

In a shared environment, the sizing comes from a `CamundaClusterPreset`, the version from a `CamundaRelease`, and a cluster sets only `presetRef`, `releaseRef`, and its references. See the [presets guide](guides/presets.md).

Wait until it is ready:

```bash
kubectl wait camundacluster/my-cluster -n my-cluster-ns \
  --for=condition=Ready --timeout=15m
kubectl get camundacluster -n my-cluster-ns
```

```
NAME         READY   REASON    VERSION   AGE
my-cluster   True    Healthy   8.9.21    12m
```

If `Ready` stays `False`, read the conditions. The reason names the problem, for example a missing reference:

```bash
kubectl describe camundacluster my-cluster -n my-cluster-ns
```

## 7. Log in

With basic authentication, the operator creates the first administrator. The Secret `my-cluster-camunda-admin` holds the password:

```bash
kubectl get secret my-cluster-camunda-admin -n my-cluster-ns \
  -o go-template='{{.data.password | base64decode}}'
```

The username is `admin`. Forward the gateway port and open Operate:

```bash
kubectl port-forward svc/my-cluster-gateway -n my-cluster-ns 8080:8080
```

Open <http://localhost:8080/operate/> and log in. Tasklist is at `/tasklist/` and Admin at `/admin/`.

The REST API is on the same port. This call lists the brokers and partitions:

```bash
curl -u admin:<password> http://localhost:8080/v2/topology
```

## 8. Clean up

```bash
kubectl delete camundacluster my-cluster -n my-cluster-ns
kubectl delete elasticsearchcluster my-cluster-es -n my-cluster-ns
kubectl delete camundaplatformconfig my-platform-config
kubectl delete namespace my-cluster-ns
```

Deleting the `CamundaCluster` deletes the broker volumes. To keep them, set the retention policy before you delete:

```yaml
apiVersion: core.camunda.io/v1
kind: CamundaCluster
metadata:
  name: my-cluster
  namespace: my-cluster-ns
spec:
  # ... the rest of your cluster
  zeebe:
    persistentVolumeClaimRetentionPolicy:
      whenDeleted: Retain
```

## Next steps

- [Presets](guides/presets.md): write the sizing once and create each cluster in a few lines.
- [Secondary storage](guides/secondary-storage.md): run Camunda on PostgreSQL instead of Elasticsearch, or bring your own backend.
- [Authentication](guides/authentication.md): connect an OIDC identity provider and name administrators.
- [Backup](guides/backup.md): write backups of the cluster to a bucket.
- [Operations](guides/operations.md): read the status, suspend, grow storage, rotate passwords.
- [CamundaCluster reference](crds/camundacluster.md): every field.
- [Getting started with the management plane](getting-started-management-plane.md): add Console and sign in through Keycloak, on a kind cluster.
