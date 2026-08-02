# Installation

The operator is distributed as a Helm chart and a plain Kubernetes manifest, both
published with every release. Charts and images live in GitHub Container Registry
and are signed with [cosign](https://docs.sigstore.dev/) keyless signatures.

**Requirements:** Kubernetes 1.30 or later, and Helm 3.8+ for the OCI registry
(the repository pins Helm 4.1.4).

## Install with Helm

```bash
helm install camunda-operator \
  oci://ghcr.io/konsole-is/charts/camunda-operator \
  --version <version> \
  --namespace camunda-operator-system \
  --create-namespace
```

This installs the controller manager and all 19 custom resource definitions.

Replace `<version>` with a released version — for example `0.1.0`. Release tags
are unprefixed SemVer, and the chart version, chart `appVersion`, and operator
image tag are always the same string.

### Configuration

The chart's values and their defaults are documented in the
[chart README](https://github.com/konsole-is/camunda-operator/blob/main/dist/chart/README.md).
The values you are most likely to touch:

```bash
helm install camunda-operator \
  oci://ghcr.io/konsole-is/charts/camunda-operator \
  --version <version> \
  --namespace camunda-operator-system --create-namespace \
  --set manager.replicas=2 \
  --set prometheus.enable=true \
  --set rbacHelpers.enable=true
```

- `prometheus.enable` installs a `ServiceMonitor`. It requires the
  prometheus-operator CRDs to already be present in the cluster.
- `rbacHelpers.enable` installs convenience admin, editor, and viewer
  `ClusterRole`s for each custom resource.
- `certManager.enable` switches the `ServiceMonitor`'s TLS config to a
  cert-manager-backed setup that expects a `metrics-server-cert` secret. The
  chart does not provision that certificate — leave this `false` unless you
  supply the secret yourself.

## Install without Helm

Every release attaches a rendered manifest:

```bash
kubectl apply -f https://github.com/konsole-is/camunda-operator/releases/download/<version>/install.yaml
```

This is the same content the chart renders at its defaults, with the namespace
fixed to `camunda-operator-system`.

## Installing CRDs out of band

By default the chart installs the CRDs. Everything a chart renders is stored in
the Helm release Secret, which is bounded by etcd's object size limit, so
clusters that also carry large custom resources may prefer to apply the CRDs
separately:

```bash
kubectl apply --server-side -f https://github.com/konsole-is/camunda-operator/releases/download/<version>/crds.yaml

helm install camunda-operator \
  oci://ghcr.io/konsole-is/charts/camunda-operator \
  --version <version> \
  --namespace camunda-operator-system --create-namespace \
  --set crd.enable=false
```

With `crd.enable=false`, CRD lifecycle is yours to manage: apply the updated
`crds.yaml` before upgrading the chart.

## Verifying signatures

Both the chart and the operator image are signed with cosign keyless signatures
tied to the release workflow's GitHub identity. There is no public key to
distribute — verification checks the signing identity instead:

```bash
cosign verify ghcr.io/konsole-is/charts/camunda-operator:<version> \
  --certificate-identity-regexp '^https://github\.com/konsole-is/camunda-operator/\.github/workflows/release\.yml@refs/tags/.+$' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com

cosign verify ghcr.io/konsole-is/camunda-operator:<version> \
  --certificate-identity-regexp '^https://github\.com/konsole-is/camunda-operator/\.github/workflows/release\.yml@refs/tags/.+$' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com
```

A successful verification prints the certificate subject and the matched claims.

## Upgrading

```bash
helm upgrade camunda-operator \
  oci://ghcr.io/konsole-is/charts/camunda-operator \
  --version <new-version> \
  --namespace camunda-operator-system
```

CRDs are annotated `helm.sh/resource-policy: keep` by default, so Helm updates
them in place and never deletes them.

## Uninstalling

```bash
helm uninstall camunda-operator --namespace camunda-operator-system
```

Because of the `keep` policy, the CRDs and every custom resource stored in them
survive an uninstall. To remove them — **this deletes every Camunda cluster the
operator manages** — delete the CRDs explicitly:

```bash
kubectl delete crd -l app.kubernetes.io/name=camunda-operator
```

## Installing from source

```bash
git clone https://github.com/konsole-is/camunda-operator
cd camunda-operator
make helm-generate IMG=<registry>/camunda-operator:<tag>
make helm-deploy   IMG=<registry>/camunda-operator:<tag>
```

`make helm-generate` regenerates `dist/chart/` from `config/`; the chart is not
checked into the repository.
