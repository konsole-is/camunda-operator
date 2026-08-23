# Installation

The operator ships as a Helm chart and as a plain Kubernetes manifest. Every release publishes both, together with two container images: the controller manager (`ghcr.io/konsole-is/camunda-operator`) and a companion CLI (`ghcr.io/konsole-is/camunda-operator-cli`). The chart and both images are signed with [cosign](https://docs.sigstore.dev/).

The manager never runs the CLI itself. The Jobs that the operator creates run it, for example the upload of a `LogicalBackupRDBMS` dump. Both install paths point the manager at the CLI image of the same release. If you mirror the manager image, mirror the CLI image too.

## Requirements

- Kubernetes 1.30 or later.
- Helm 3.8 or later, for the OCI registry.
- The [ECK operator](https://www.elastic.co/guide/en/cloud-on-k8s/current/k8s-deploy-eck.html), version 3.5 or later, if you use `ElasticsearchCluster`. The manager looks for the ECK CRDs when it starts and watches ECK `Elasticsearch` resources only when it finds them. If you install ECK after the manager, restart the manager. Until then, every `ElasticsearchCluster` reports `Ready=False` with reason `ECKNotInstalled`.
- A PostgreSQL server that a `DatabaseServerConfig` describes, if you use `Database`. The operator does not run PostgreSQL.
- The [Keycloak Operator](https://www.keycloak.org/operator/installation), if you use `CamundaManagementCluster` with `spec.identityProvider.keycloak`. The manager looks for the Keycloak CRDs when it starts. If it does not find them, every `CamundaManagementCluster` in that mode reports `Ready=False` with reason `KeycloakOperatorNotInstalled`. If you install the Keycloak Operator after the manager, restart the manager. Install the release of the Keycloak version you set on the resource, below 26.7 for Camunda 8.9. See [The operator runs Keycloak](crds/camundamanagementcluster.md#the-operator-runs-keycloak). The other two identity provider modes do not need it. Camunda documents the same prerequisite in [Keycloak deployment](https://docs.camunda.io/docs/self-managed/deployment/helm/configure/operator-based-infrastructure/#keycloak-deployment).

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
| `prometheus.enable` | `false` | Install a `ServiceMonitor` for the manager. Needs the prometheus-operator CRDs. |
| `rbacHelpers.enable` | `false` | Install admin, editor, and viewer `ClusterRole`s for every custom resource. |
| `crd.enable` | `true` | Install the CRDs with the chart. Set `false` to manage them yourself (below). |
| `certManager.enable` | `false` | Use a cert-manager certificate for the metrics endpoint. The chart does not create the certificate. Leave it `false` unless you provide the Secret `metrics-server-cert`. |

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

The manifest pins the CLI image in the manager's `CAMUNDA_OPERATOR_CLI_IMAGE` environment variable. To use a mirror, edit that value before you apply.

## Install the CRDs separately

Helm stores everything a chart renders in the release Secret, and etcd limits the size of that Secret. If your cluster also carries large custom resources, install the CRDs yourself and keep them out of the release:

```bash
kubectl apply --server-side -f https://github.com/konsole-is/camunda-operator/releases/download/<version>/crds.yaml

helm install camunda-operator \
  oci://ghcr.io/konsole-is/charts/camunda-operator \
  --version <version> \
  --namespace camunda-operator-system --create-namespace \
  --set crd.enable=false
```

With `crd.enable=false` you own the CRD lifecycle. Apply the new `crds.yaml` before you upgrade the chart.

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

The CRDs carry the annotation `helm.sh/resource-policy: keep`. Helm updates them in place and never deletes them.

## Uninstall

```bash
helm uninstall camunda-operator --namespace camunda-operator-system
```

Because of the `keep` policy, the CRDs and every custom resource stored in them survive the uninstall.

> **Caution:** Deleting the CRDs deletes every custom resource of the operator, and with them every Camunda cluster, Elasticsearch cluster, and backup that the operator manages. The data volumes follow the retention policy of each resource.

To remove the CRDs, delete them by name. The CRD manifests carry no labels, so a label selector does not match them:

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

`make helm-generate` renders `dist/chart/` from `config/`. The chart is generated, not checked in. `IMG` and `CLI_IMG` set the two images for `make deploy`, `make build-installer`, and `make helm-deploy` in the same way.
