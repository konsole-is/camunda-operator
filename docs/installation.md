# Installation

The operator ships as a Helm chart and as a plain Kubernetes manifest. Every release publishes both, together with two container images: the controller manager (`ghcr.io/konsole-is/camunda-operator`) and a companion CLI (`ghcr.io/konsole-is/camunda-operator-cli`). The chart and both images are signed with [cosign](https://docs.sigstore.dev/).

The manager never runs the CLI itself. The Jobs that the operator creates run it, for example the upload of a `LogicalBackupRDBMS` dump. Both install paths point the manager at the CLI image of the same release. If you mirror the manager image, mirror the CLI image too.

## Requirements

- Kubernetes 1.30 or later.
- Helm 3.8 or later, for the OCI registry.
- The [ECK operator](https://www.elastic.co/guide/en/cloud-on-k8s/current/k8s-deploy-eck.html), version 3.5 or later, if you use `ElasticsearchCluster`. The manager looks for the ECK CRDs when it starts. If it does not find them, every `ElasticsearchCluster` reports `Ready=False` with reason `ECKNotInstalled`. If you install ECK after the manager, restart the manager.
- The [CloudNativePG operator](https://cloudnative-pg.io/documentation/current/installation_upgrade/), version 1.26 or later, if you use `DatabaseServer`. Use 1.27 or later with the Barman Cloud plugin. The manager looks for the CloudNativePG CRDs when it starts. If it does not find them, every `DatabaseServer` reports `Ready=False` with reason `CNPGNotInstalled`. If you install CloudNativePG after the manager, restart the manager.
- The [Barman Cloud plugin](https://cloudnative-pg.io/plugin-barman-cloud/docs/installation/), version 0.14 or later, and [cert-manager](https://cert-manager.io/docs/installation/), if you use `DatabaseServer` with `spec.archive`. Both install into the namespace of the CloudNativePG operator. Without the plugin, a `DatabaseServer` with an archive reports `Ready=False` with reason `BarmanPluginNotInstalled`. If you install the plugin after the manager, restart the manager.
- A PostgreSQL server that a `DatabaseServerConfig` describes, if you use `Database` without a `DatabaseServer`. The operator runs PostgreSQL only through `DatabaseServer`.
- The [Keycloak Operator](https://www.keycloak.org/operator/installation), if you use `CamundaManagementCluster` with `spec.identityProvider.keycloak`. The other two identity provider modes do not need it. The manager looks for the Keycloak CRDs when it starts. If it does not find them, every `CamundaManagementCluster` in that mode reports `Ready=False` with reason `KeycloakOperatorNotInstalled`. If you install the Keycloak Operator after the manager, restart the manager. Install the Keycloak Operator release that matches `spec.identityProvider.keycloak.version`. With Camunda 8.9, keep that version below 26.7.0. [The operator runs Keycloak](crds/camundamanagementcluster.md#the-operator-runs-keycloak) gives the reason. Camunda documents the same prerequisite in [Keycloak deployment](https://docs.camunda.io/docs/self-managed/deployment/helm/configure/operator-based-infrastructure/#keycloak-deployment).

The end-to-end suite of each release runs against ECK 3.5.0, CloudNativePG 1.30.1, the Barman Cloud plugin 0.14.0, and the Keycloak Operator 26.6.4.

### Install CloudNativePG and the Barman Cloud plugin

Install [cert-manager](https://cert-manager.io/docs/installation/) first. The plugin gets its certificates from cert-manager.

```bash
kubectl apply --server-side -f https://raw.githubusercontent.com/cloudnative-pg/cloudnative-pg/release-1.30/releases/cnpg-1.30.1.yaml
kubectl apply --server-side -f https://github.com/cloudnative-pg/plugin-barman-cloud/releases/download/v0.14.0/manifest.yaml
kubectl rollout status deployment/cnpg-controller-manager -n cnpg-system
kubectl rollout status deployment/barman-cloud -n cnpg-system
```

Both manifests install into the namespace `cnpg-system`. Use `--server-side`: each manifest carries a CRD larger than the annotation that client-side apply writes.

Skip the plugin when no `DatabaseServer` uses `spec.archive`. A server without an archive runs on CloudNativePG alone, and no point-in-time restore can reach it.

## Install with Helm

```bash
helm install camunda-operator \
  oci://ghcr.io/konsole-is/charts/camunda-operator \
  --version <version> \
  --namespace camunda-operator-system \
  --create-namespace
```

This installs the manager and every custom resource definition. Replace `<version>` with a released version, for example `0.1.0`. Release tags are plain SemVer. The chart version, the chart `appVersion`, and the image tag are always the same string.

### Values

The [chart README](https://github.com/konsole-is/camunda-operator/blob/main/dist/chart/README.md) documents every value. The values you are most likely to set:

| Value | Default | Effect |
| --- | --- | --- |
| `manager.replicas` | `1` | Number of manager replicas. Leader election is on. |
| `manager.cliImage.repository`, `manager.cliImage.tag` | the CLI image of the release | The image that the Jobs of the operator run. Point it at your mirror when you mirror the manager image. |
| `prometheus.enable` | `false` | Install a `ServiceMonitor` for the manager. Needs the prometheus-operator CRDs. The alert rules and dashboards are applied separately, see [Observability](observability.md). |
| `rbacHelpers.enable` | `false` | Install admin, editor, and viewer `ClusterRole`s for every custom resource. |
| `crd.enable` | `true` | Install the CRDs with the chart. Set `false` to manage them yourself (below). |
| `certManager.enable` | `false` | Make the `ServiceMonitor` verify the metrics endpoint with the certificate in the Secret `metrics-server-cert`. The chart does not create that Secret. |

To pull from a mirror, set `manager.image.repository` and `manager.cliImage.repository`, and put the pull Secret in `manager.imagePullSecrets`. That Secret applies to the manager Pod only, not to the Jobs that run the CLI image. Those Jobs and the Camunda pods run under the ServiceAccount of their `CamundaCluster`. Add the pull Secret to the `imagePullSecrets` of that ServiceAccount, see [Workload identity](crds/camundacluster.md#workload-identity). The [chart README](https://github.com/konsole-is/camunda-operator/blob/main/dist/chart/README.md) lists these values.

Example:

```bash
helm install camunda-operator \
  oci://ghcr.io/konsole-is/charts/camunda-operator \
  --version <version> \
  --namespace camunda-operator-system --create-namespace \
  --set manager.replicas=2 \
  --set prometheus.enable=true
```

## Install without Helm

Every release attaches a rendered manifest:

```bash
kubectl apply --server-side -f https://github.com/konsole-is/camunda-operator/releases/download/<version>/install.yaml
```

Use `--server-side`: the `CamundaCluster` CRD is larger than the annotation that client-side apply writes.

The manifest differs from the chart defaults in two ways. It creates the namespace `camunda-operator-system`. And it includes the admin, editor, and viewer `ClusterRole`s for every custom resource, the same set the chart renders with `rbacHelpers.enable=true`.

The manifest pins the CLI image in the manager's `CAMUNDA_OPERATOR_CLI_IMAGE` environment variable. To use a mirror, edit that value and the image of the manager Deployment before you apply.

## Install the CRDs separately

Install the CRDs yourself when you manage them outside Helm, for example with a GitOps tool. This also keeps them out of the Helm release Secret, which etcd limits to about 1 MB:

```bash
kubectl apply --server-side -f https://github.com/konsole-is/camunda-operator/releases/download/<version>/crds.yaml

helm install camunda-operator \
  oci://ghcr.io/konsole-is/charts/camunda-operator \
  --version <version> \
  --namespace camunda-operator-system --create-namespace \
  --set crd.enable=false
```

With `crd.enable=false` you own the CRD lifecycle. Apply the new `crds.yaml` before you upgrade the chart.

## Grace periods

A workload that is not ready reports `Creating`, `Updating`, `Scaling`, or `Failing` until its grace period ends. After that, its condition reports `Degraded` or `Down`, with one exception. The grace period starts when the workload begins to roll out. For example, it starts when the workload is first created, or when its condition changes from `True` to `False`. [Status conventions](architecture.md#status-conventions) describes the reasons, the exception, and the start of the grace period.

| Flag | Environment variable | Default | Applies to |
| --- | --- | --- | --- |
| `--workload-grace-period` | `CAMUNDA_OPERATOR_WORKLOAD_GRACE_PERIOD` | `30m` | The processes of a `CamundaCluster`. The webapp and importer of a `CamundaOptimize`. The Keycloak and the workloads of a `CamundaManagementCluster`. The exporter of an `ElasticsearchCluster`. |
| `--datastore-grace-period` | `CAMUNDA_OPERATOR_DATASTORE_GRACE_PERIOD` | `30m` | The Elasticsearch cluster of an `ElasticsearchCluster` and the PostgreSQL cluster of a `DatabaseServer`. |

A value is a Go duration, for example `20m` or `1h`. The flag wins over the environment variable. With `0`, the condition keeps its progress reason and never reports `Degraded` or `Down`. The manager does not start with a negative value. It also does not start with a value that is not a duration, unless a flag overrides that environment variable.

Set a value that is longer than your slowest rollout. Keep the workload grace period at or above the datastore grace period. A cluster on Elasticsearch and its Optimize are not ready until that Elasticsearch is. A shorter workload period reports them `Down` while Elasticsearch still starts. A value that is too short reports `Degraded` or `Down` for a workload that starts slowly but correctly. A rolling update of many brokers, or the first start of a large Elasticsearch cluster, can take longer than the default.

With Helm, set the environment variables in `manager.envOverrides`:

```bash
helm install camunda-operator \
  oci://ghcr.io/konsole-is/charts/camunda-operator \
  --version <version> \
  --namespace camunda-operator-system --create-namespace \
  --set manager.envOverrides.CAMUNDA_OPERATOR_WORKLOAD_GRACE_PERIOD=60m \
  --set manager.envOverrides.CAMUNDA_OPERATOR_DATASTORE_GRACE_PERIOD=45m
```

Without Helm, add the environment variables to the `manager` container of the Deployment `camunda-operator-controller-manager` in `install.yaml`, next to `CAMUNDA_OPERATOR_CLI_IMAGE`:

```yaml
env:
  - name: CAMUNDA_OPERATOR_WORKLOAD_GRACE_PERIOD
    value: 60m
  - name: CAMUNDA_OPERATOR_DATASTORE_GRACE_PERIOD
    value: 45m
```

## Verify the signatures

The chart and both images are signed with cosign keyless signatures. There is no public key. Verification checks the identity of the release workflow:

```bash
for artifact in \
  ghcr.io/konsole-is/charts/camunda-operator:<version> \
  ghcr.io/konsole-is/camunda-operator:<version> \
  ghcr.io/konsole-is/camunda-operator-cli:<version>; do
  cosign verify "$artifact" \
    --certificate-identity-regexp '^https://github\.com/konsole-is/camunda-operator/\.github/workflows/release\.yml@refs/tags/.+$' \
    --certificate-oidc-issuer https://token.actions.githubusercontent.com
done
```

A successful verification prints the certificate subject and the matched claims.

## Upgrade

```bash
helm upgrade camunda-operator \
  oci://ghcr.io/konsole-is/charts/camunda-operator \
  --version <new-version> \
  --namespace camunda-operator-system
```

Without `--set` or `-f`, Helm keeps the values of the last release. If you pass a value, Helm uses the chart defaults for all the values you do not pass. In that case, pass every value of the install again. Do not use `--reuse-values`, because it keeps the image tags of the old chart.

The CRDs carry the annotation `helm.sh/resource-policy: keep`. Helm updates them in place and never deletes them.

To upgrade a manifest install, apply the `install.yaml` of the new release with `--server-side`, as in [Install without Helm](#install-without-helm). Make the same mirror edits again before you apply.

## Uninstall

```bash
helm uninstall camunda-operator --namespace camunda-operator-system
```

Because of the `keep` policy, the CRDs and every custom resource stored in them survive the uninstall. The Camunda workloads keep running, but nothing manages them.

To remove everything, do these steps in this order:

1. Delete your custom resources while the manager runs. Some of them, for example a `CamundaCluster` or a backup, wait for the manager before they go away. Without the manager, they stay in deletion, and the delete of their CRD does not finish.
2. Uninstall the chart.
3. Delete the CRDs.

> **Caution:** When you delete a backup resource, the operator deletes the snapshots or the dump of that backup from the bucket. If you delete the cluster or its `ObjectStorageConfig` first, the backup goes and leaves its artifacts in the bucket. When you delete a `CamundaCluster`, the broker volumes follow `spec.zeebe.persistentVolumeClaimRetentionPolicy`. The default deletes them.

Delete the CRDs by name. The CRD manifests carry no labels, so a label selector does not match them:

```bash
kubectl delete -f https://github.com/konsole-is/camunda-operator/releases/download/<version>/crds.yaml
```

Without the release file, delete every CRD in the `core.camunda.io` group:

```bash
crds=$(kubectl get crd -o name | grep '\.core\.camunda\.io$')
if [ -n "$crds" ]; then kubectl delete $crds; fi
```

## Install from source

```bash
git clone https://github.com/konsole-is/camunda-operator
cd camunda-operator
make docker-build docker-push         IMG=<registry>/camunda-operator:<tag>
make docker-build-cli docker-push-cli CLI_IMG=<registry>/camunda-operator-cli:<tag>
make helm-generate IMG=<registry>/camunda-operator:<tag> CLI_IMG=<registry>/camunda-operator-cli:<tag>
make helm-deploy   IMG=<registry>/camunda-operator:<tag> CLI_IMG=<registry>/camunda-operator-cli:<tag>
```

This needs Docker, Go, and the `kubebuilder` CLI. `make helm-generate` renders `dist/chart/values.yaml` and `dist/chart/templates/` from `config/`. The repository holds only `Chart.yaml` and `README.md` of the chart. `IMG` and `CLI_IMG` set the two images for `make deploy`, `make build-installer`, and `make helm-deploy` in the same way.
