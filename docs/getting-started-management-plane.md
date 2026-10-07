# Getting started with the management plane

This guide takes you from an empty [kind](https://kind.sigs.k8s.io/) cluster to a sign-in to Console. Console then lists one orchestration cluster.
The management plane is a `CamundaManagementCluster`. It runs Management Identity, Console, and a Keycloak that the operator creates for you. Management Identity is the identity system that Console, Web Modeler, and Optimize sign in through. Camunda explains it in [Management Identity](https://docs.camunda.io/docs/self-managed/components/management-identity/overview/).

The guide follows one path. CloudNativePG runs the PostgreSQL server for the databases. You reach each component through a port forward, not an Ingress. The sizes fit a local kind cluster. They are not production sizes.

The [Management plane](guides/management-plane.md) guide covers the other identity providers, Web Modeler, and the routing for a real domain.

## The ready-made manifests

The manifests on this page come from [`config/example/camunda-management-cluster/keycloak`](https://github.com/konsole-is/camunda-operator/tree/main/config/example/camunda-management-cluster/keycloak). The link shows the `main` branch. Select the tag of your release on GitHub to get the manifests of that version.

That directory is written for a real domain, so this page changes some values. The changes are:

| File | On this page |
| --- | --- |
| `02-secrets.yaml` | Not used. The page sets no license key and runs no Web Modeler, so it needs neither Secret. |
| `05-platform-config.yaml` | No `licenseSecretRef`. |
| `06-management-cluster.yaml` | The Keycloak `externalUrl` is the address of the Keycloak Service inside the cluster, `http://my-management-keycloak-service.my-management-ns.svc:8080/auth`. The `externalUrl` of Management Identity and of Console is a `localhost` address of a port forward. No `webModeler` block. |
| `08-camunda-cluster.yaml` | `externalUrl` is `http://localhost:8088`. |
| `09-optimize.yaml` | An optional last step. `externalUrl` is `http://localhost:8090`. |
| `config/example/presets` | The page sets 16Gi for the Zeebe broker and for Elasticsearch. Earlier versions of the presets set 1Gi. |

To apply the directory in one command instead, clone the repository at your release tag. Replace the placeholders in `02-secrets.yaml`, and route each `camunda.example.com` URL to its Service. The [README of the directory](https://github.com/konsole-is/camunda-operator/tree/main/config/example/camunda-management-cluster/keycloak) lists these steps. If `config/example/presets` of your copy sets 1Gi for the Zeebe broker or for Elasticsearch, change both to 16Gi. Then run this command after step 3 of this page:

```bash
kubectl apply -k config/example/camunda-management-cluster/keycloak
```

## Before you start

You need:

- `kubectl`, `helm` 3.8 or later, and `kind`
- Docker or Podman, which runs the kind node
- about 10 GB of free memory and 4 CPUs for the kind node. This is an estimate from the requests of the pods, not a measured value.
- about 40Gi of free disk for the volumes: 16Gi for the Zeebe broker, 16Gi for Elasticsearch, and 8Gi for PostgreSQL
- a node that can pull images from Docker Hub, `docker.elastic.co`, `ghcr.io`, and `quay.io`

Create the kind cluster. Its default StorageClass `standard` binds the volumes of this guide.

```bash
kind create cluster --name camunda
kubectl version
```

The server version must be 1.34 or later. See the [requirements](installation.md#requirements). If it is earlier, delete the cluster and create it again with a newer node image:

```bash
kind delete cluster --name camunda
kind create cluster --name camunda --image kindest/node:v1.34.0
```

Elasticsearch needs `vm.max_map_count` of at least 262144 in the kernel that runs the kind node. On a Linux host, set it on the host:

```bash
sudo sysctl -w vm.max_map_count=262144
```

On macOS and Windows, Docker Desktop and Podman run the kind node in a virtual machine. Set the value in the node instead, and set it again after the virtual machine restarts:

```bash
docker exec camunda-control-plane sysctl -w vm.max_map_count=262144
```

With Podman, use `podman exec`.

Each step shows a manifest. Save it to a file and apply it with `kubectl apply -f <file>`.

## 1. Create the namespaces

The management plane runs in `my-management-ns`. The orchestration cluster runs in `my-cluster-ns`.

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: my-management-ns
---
apiVersion: v1
kind: Namespace
metadata:
  name: my-cluster-ns
```

Create them first, because the Keycloak Operator in step 2 runs in `my-management-ns`.

## 2. Install the operators that this guide needs

The operator runs Elasticsearch through ECK, PostgreSQL through CloudNativePG, and Keycloak through the Keycloak Operator. Install all three before the operator. The operator looks for their CRDs when it starts. If you install one of them later, restart the operator.

The versions below are the ones that the end-to-end suite of each release runs.

Install [ECK](https://www.elastic.co/guide/en/cloud-on-k8s/current/index.html) 3.5.0:

```bash
kubectl apply --server-side -f https://download.elastic.co/downloads/eck/3.5.0/crds.yaml
kubectl apply --server-side -f https://download.elastic.co/downloads/eck/3.5.0/operator.yaml
```

Install [CloudNativePG](https://cloudnative-pg.io/documentation/current/installation_upgrade/) 1.30.1:

```bash
kubectl apply --server-side -f https://raw.githubusercontent.com/cloudnative-pg/cloudnative-pg/release-1.30/releases/cnpg-1.30.1.yaml
kubectl rollout status deployment/cnpg-controller-manager -n cnpg-system
```

This guide needs no Barman Cloud plugin and no cert-manager. They are only for a PostgreSQL server that archives to a bucket.

Install the [Keycloak Operator](https://www.keycloak.org/operator/installation) 26.6.4. Its CRDs are cluster-wide. The Keycloak Operator itself runs Keycloak only in its own namespace, so install it into `my-management-ns`:

```bash
KC=https://raw.githubusercontent.com/keycloak/keycloak-k8s-resources/26.6.4/kubernetes
kubectl apply --server-side -f $KC/keycloaks.k8s.keycloak.org-v1.yml
kubectl apply --server-side -f $KC/keycloakrealmimports.k8s.keycloak.org-v1.yml
kubectl apply --server-side -n my-management-ns -f $KC/kubernetes.yml
kubectl rollout status deployment/keycloak-operator -n my-management-ns
```

The Keycloak Operator release must match the Keycloak `version` of step 6. With Camunda 8.9, keep that version below 26.7.0. [The operator runs Keycloak](crds/camundamanagementcluster.md#the-operator-runs-keycloak) gives the reason.

Use `--server-side` for each file above. Some of these files carry a CRD that is larger than the annotation that client-side apply writes. Server-side apply works for every file.

## 3. Install the operator

```bash
helm install camunda-operator \
  oci://ghcr.io/konsole-is/charts/camunda-operator \
  --version <version> \
  --namespace camunda-operator-system \
  --create-namespace
```

Replace `<version>` with a released version without the `v`, for example `0.1.0` for the release `v0.1.0`. Make sure that the manager is running:

```bash
kubectl get pods -n camunda-operator-system
```

## 4. Create the presets and the release

A preset holds the sizes of a resource. A release holds the versions. The `DatabaseServer`, the `ElasticsearchCluster`, and the `CamundaCluster` in the next steps each name one preset and the release, and set only their own values. All four are cluster-scoped. The [presets guide](guides/presets.md) explains both kinds.

```yaml
apiVersion: core.camunda.io/v1
kind: CamundaClusterPreset
metadata:
  name: small
spec:
  cluster:
    zeebe:
      replicas: 1
      partitions: 1
      replicationFactor: 1
      storageSize: 16Gi
      resources:
        requests:
          cpu: "1"
          memory: 1.5Gi
    gateway:
      resources:
        requests:
          cpu: 500m
          memory: 512Mi
---
apiVersion: core.camunda.io/v1
kind: ElasticsearchClusterPreset
metadata:
  name: standard
spec:
  cluster:
    replicas: 1
    storageSize: 16Gi
    resources:
      requests:
        cpu: 500m
        memory: 1Gi
---
apiVersion: core.camunda.io/v1
kind: DatabaseServerPreset
metadata:
  name: standard
spec:
  server:
    instances: 1
    storageSize: 8Gi
    resources:
      requests:
        cpu: 250m
        memory: 512Mi
---
apiVersion: core.camunda.io/v1
kind: CamundaRelease
metadata:
  name: camunda-8-9
spec:
  version: "8.9.22"
  connectors:
    version: "8.9.14"
  elasticsearch:
    version: "9.2.8"
  databaseServer:
    version: "17"
```

[`config/example/releases`](https://github.com/konsole-is/camunda-operator/tree/main/config/example/releases) holds the same release. [`config/example/presets`](https://github.com/konsole-is/camunda-operator/tree/main/config/example/presets) holds the same presets, except in older versions, which set 1Gi for the Zeebe broker and for Elasticsearch. If your copy sets 1Gi, use the 16Gi on this page.

## 5. Create the databases

Keycloak, Management Identity, and Web Modeler each need a PostgreSQL database of their own. A `DatabaseServer` runs the PostgreSQL server. The field `databaseServerConfig` names the contract that the server publishes, here `my-db-server`.

```yaml
apiVersion: core.camunda.io/v1
kind: DatabaseServer
metadata:
  name: my-db
  namespace: my-management-ns
spec:
  presetRef: standard
  releaseRef: camunda-8-9
  databaseServerConfig: my-db-server
```

Wait until it is ready. The first start pulls the PostgreSQL image.

```bash
kubectl wait databaseserver/my-db -n my-management-ns \
  --for=condition=Ready --timeout=10m
```

Then create one `Database` per component. Each one creates a `DatabaseConfig` of the same name, which the management plane names in step 6.

```yaml
apiVersion: core.camunda.io/v1
kind: Database
metadata:
  name: my-keycloak-db
  namespace: my-management-ns
spec:
  serverRef: my-db-server
  databaseName: keycloak
---
apiVersion: core.camunda.io/v1
kind: Database
metadata:
  name: my-identity-db
  namespace: my-management-ns
spec:
  serverRef: my-db-server
  databaseName: identity
---
apiVersion: core.camunda.io/v1
kind: Database
metadata:
  name: my-web-modeler-db
  namespace: my-management-ns
spec:
  serverRef: my-db-server
  databaseName: web_modeler
```

This guide runs no Web Modeler, so nothing uses `my-web-modeler-db` yet. It is there for the moment you add Web Modeler.

## 6. Create the management plane

A `CamundaPlatformConfig` holds the settings that all clusters share. This one selects basic authentication for the orchestration cluster. That choice is separate from the sign-in to Console, which goes through Keycloak.

```yaml
apiVersion: core.camunda.io/v1
kind: CamundaPlatformConfig
metadata:
  name: my-platform-config
spec:
  auth:
    method: basic
```

Without a license key, Camunda runs under its Non-Production License. `CamundaPlatformConfig` is cluster-scoped. It has no namespace.

Then create the management plane:

```yaml
apiVersion: core.camunda.io/v1
kind: CamundaManagementCluster
metadata:
  name: my-management
  namespace: my-management-ns
spec:
  platformConfigRef: my-platform-config
  clusterSelector:
    matchLabels:
      environment: production
  identityProvider:
    keycloak:
      version: "26.6.4"
      externalUrl: "http://my-management-keycloak-service.my-management-ns.svc:8080/auth"
      databaseConfigRef: my-keycloak-db
  identity:
    version: "8.9.10"
    externalUrl: "http://localhost:8084"
    databaseConfigRef: my-identity-db
    admin:
      username: admin
      email: admin@example.com
  console:
    version: "8.9.122"
    externalUrl: "http://localhost:8087"
```

Each `externalUrl` is the address that your browser opens. Step 8 forwards each of these ports to its Service.

The Keycloak `externalUrl` is different. The pods of the management plane reach Keycloak at this address too, so it is the address of the Keycloak Service inside the cluster. Its path must be exactly `/auth`. Step 8 makes your browser reach the same address.

`clusterSelector` selects the orchestration clusters that Console lists. This one selects every `CamundaCluster` with the label `environment: production`.

Wait until it is ready. The first start pulls the images of Keycloak, Management Identity, and Console, and can take a few minutes.

```bash
kubectl wait camundamanagementcluster/my-management -n my-management-ns \
  --for=condition=Ready --timeout=15m
kubectl get camundamanagementcluster -n my-management-ns
```

```
NAME            READY   REASON    AGE
my-management   True    Healthy   9m
```

If `Ready` stays `False`, the reason names the problem. [Status](crds/camundamanagementcluster.md#status) says what to do for each reason. For example, `KeycloakOperatorNotInstalled` means that the operator started before the Keycloak CRDs existed. Restart the operator.

## 7. Create the orchestration cluster

The orchestration cluster stores its data in Elasticsearch. Create the `ElasticsearchCluster` first. It publishes the contract `my-storage-config`, which the cluster names in `storageRef`.

```yaml
apiVersion: core.camunda.io/v1
kind: ElasticsearchCluster
metadata:
  name: my-cluster-es
  namespace: my-cluster-ns
spec:
  presetRef: standard
  releaseRef: camunda-8-9
  secondaryStorageConfig: my-storage-config
```

Then the cluster. The label `environment: production` is what the `clusterSelector` of step 6 matches.

```yaml
apiVersion: core.camunda.io/v1
kind: CamundaCluster
metadata:
  name: my-cluster
  namespace: my-cluster-ns
  labels:
    environment: production
spec:
  presetRef: small
  releaseRef: camunda-8-9
  platformConfigRef: my-platform-config
  storageRef: my-storage-config
  externalUrl: "http://localhost:8088"
```

Wait until it is ready. The cluster waits for Elasticsearch, and the first start pulls both images.

```bash
kubectl wait camundacluster/my-cluster -n my-cluster-ns \
  --for=condition=Ready --timeout=15m
```

Then make sure that the management plane serves the cluster:

```bash
kubectl get camundamanagementcluster my-management -n my-management-ns \
  -o jsonpath='{range .status.clusters[*]}{.name}{"\t"}{.attached}{"\t"}{.reason}{"\n"}{end}'
```

```
my-cluster	true
```

While the cluster starts, the line shows `false` and the reason `NotReady`. [Clusters](crds/camundamanagementcluster.md#clusters) lists the other reasons.

## 8. Sign in to Console

The operator creates the first administrator, `admin`. The Secret `my-management-identity-admin` holds the password:

```bash
kubectl get secret my-management-identity-admin -n my-management-ns \
  -o go-template='{{.data.password | base64decode}}'
```

Your browser must reach Keycloak at its `externalUrl` from step 6. Add this line to the hosts file of your computer. The file is `/etc/hosts` on Linux and macOS, and `C:\Windows\System32\drivers\etc\hosts` on Windows. Edit it as an administrator.

```
127.0.0.1 my-management-keycloak-service.my-management-ns.svc
```

Forward the ports of Keycloak, Management Identity, and Console. Each command runs until you stop it, so run each one in a terminal of its own:

```bash
kubectl port-forward svc/my-management-keycloak-service -n my-management-ns 8080:8080
kubectl port-forward svc/my-management-identity -n my-management-ns 8084:80
kubectl port-forward svc/my-management-console -n my-management-ns 8087:80
```

The local ports are the ports in the `externalUrl` fields of step 6. If you change the local port of Management Identity or Console, change its `externalUrl` too. Keep Keycloak on local port 8080, because its `externalUrl` names that port.

Open <http://localhost:8087>. Console sends you to the Keycloak sign-in page. Sign in as `admin` with the password above. Console lists `my-cluster`. Camunda marks the cluster list of Console as experimental in 8.9, under [experimental features](https://docs.camunda.io/docs/self-managed/components/console/configuration/#experimental-features).

Management Identity is at <http://localhost:8084>. Sign in there with the same user.

To open Operate on the cluster, forward the gateway port. Local port 8080 is in use by Keycloak, so this guide uses 8088, the port of the cluster `externalUrl` in step 7:

```bash
kubectl port-forward svc/my-cluster-gateway -n my-cluster-ns 8088:8080
```

Open <http://localhost:8088/operate/>.

Operate uses the basic authentication of the cluster, not Keycloak. Its user and password are not the ones of Management Identity. The username is `admin`, and the Secret `my-cluster-camunda-admin` holds the password:

```bash
kubectl get secret my-cluster-camunda-admin -n my-cluster-ns \
  -o go-template='{{.data.password | base64decode}}'
```

## 9. Optional: add Optimize

Optimize reads the data of one orchestration cluster and signs in through the management plane. It needs more memory on top of the sizes in [Before you start](#before-you-start).

```yaml
apiVersion: core.camunda.io/v1
kind: CamundaOptimize
metadata:
  name: my-cluster-optimize
  namespace: my-cluster-ns
spec:
  version: "8.9.22"
  managementAuthRef: my-management
  externalUrl: "http://localhost:8090"
  clusterRef:
    name: my-cluster
```

`managementAuthRef` names the contract that `my-management` publishes. Wait until it is ready, then forward its port:

```bash
kubectl wait camundaoptimize/my-cluster-optimize -n my-cluster-ns \
  --for=condition=Ready --timeout=15m
kubectl port-forward svc/my-cluster-optimize-webapp -n my-cluster-ns 8090:8090
```

The management plane also registers the sign-in address of Optimize in Keycloak and gives `admin` the Optimize role. Until it has done so, the sign-in to Optimize fails. Wait for it:

```bash
kubectl wait camundamanagementcluster/my-management -n my-management-ns \
  --for=condition=OptimizeCallbacksReady --timeout=5m
```

Open <http://localhost:8090> and sign in as `admin`. The Keycloak port forward and the hosts file line of step 8 must stay in place. See [CamundaOptimize](crds/camundaoptimize.md) for the rest.

This local setup has one limit for Optimize. The Optimize pod calls Management Identity at its `externalUrl`, `http://localhost:8084`, and that address does not answer inside the cluster. The parts of Optimize that read users or tenants from Management Identity can fail. The limit goes away when the `externalUrl` of Management Identity also answers inside the cluster, as a real domain usually does.

Web Modeler needs an SMTP server, so this guide leaves it out. [Step 3d of the Management plane guide](guides/management-plane.md#step-3d-web-modeler) adds it.

## 10. Clean up

```bash
kubectl delete camundaoptimize my-cluster-optimize -n my-cluster-ns --ignore-not-found
kubectl delete camundacluster my-cluster -n my-cluster-ns
kubectl delete elasticsearchcluster my-cluster-es -n my-cluster-ns
kubectl delete camundamanagementcluster my-management -n my-management-ns
kubectl delete database --all -n my-management-ns
kubectl delete databaseserver my-db -n my-management-ns
kubectl delete camundaplatformconfig my-platform-config
kubectl delete namespace my-management-ns my-cluster-ns
```

Deleting the namespace `my-management-ns` also removes the Keycloak Operator. The presets and the release stay. Delete them when nothing else uses them:

```bash
kubectl delete camundaclusterpreset small
kubectl delete elasticsearchclusterpreset standard
kubectl delete databaseserverpreset standard
kubectl delete camundarelease camunda-8-9
```

To remove everything at once, delete the kind cluster:

```bash
kind delete cluster --name camunda
```

## Next steps

- [Management plane](guides/management-plane.md): run the management plane on your own domain, with your own Keycloak or OIDC provider, and with Web Modeler.
- [CamundaManagementCluster reference](crds/camundamanagementcluster.md): every field, condition, and generated Secret.
- [Presets](guides/presets.md): write the sizes once and create each cluster in a few lines.
- [Getting started](getting-started.md): an orchestration cluster alone, without a management plane.
