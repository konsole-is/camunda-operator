# camunda-operator Helm chart

The Kubernetes operator that runs Camunda 8.9+ orchestration clusters, their storage backends, backups, Optimize, and the management plane.

```bash
helm install camunda-operator \
  oci://ghcr.io/konsole-is/charts/camunda-operator \
  --version <version> \
  --namespace camunda-operator-system --create-namespace
```

The installation guide covers signature verification, CRDs installed separately, upgrades, and removal:
<https://github.com/konsole-is/camunda-operator/blob/main/docs/installation.md>

## Values

The defaults are the values of a released chart.

| Key | Default | Description |
|---|---|---|
| `manager.replicas` | `1` | Number of manager replicas. Leader election is on. |
| `manager.image.repository` | `ghcr.io/konsole-is/camunda-operator` | Manager image repository. |
| `manager.image.tag` | the chart version | Manager image tag. |
| `manager.image.pullPolicy` | `IfNotPresent` | Image pull policy of the manager. |
| `manager.cliImage.repository` | `ghcr.io/konsole-is/camunda-operator-cli` | Repository of the CLI image that the Jobs of the operator run. The manager gets it as `--camunda-operator-cli-image`. If you mirror the manager image, mirror this one too. |
| `manager.cliImage.tag` | the chart version | CLI image tag. |
| `manager.args` | `["--leader-elect"]` | Arguments of the manager. Keep `--leader-elect` when you run more than one replica. |
| `manager.env` | `[]` | Environment variables. |
| `manager.envOverrides` | `{}` | Environment variables as a map. A name here wins over the same name in `manager.env`. |
| `manager.imagePullSecrets` | `[]` | Image pull secrets. |
| `manager.podSecurityContext` | `runAsNonRoot: true`, `seccompProfile.type: RuntimeDefault` | Pod security context. |
| `manager.securityContext` | drops `ALL`, read-only root FS, no privilege escalation | Container security context. |
| `manager.resources` | requests `10m`/`64Mi`, limits `500m`/`128Mi` | Resource requests and limits. |
| `manager.affinity` | `{}` | Pod affinity. |
| `manager.nodeSelector` | `{}` | Pod node selector. |
| `manager.tolerations` | `[]` | Pod tolerations. |
| `crd.enable` | `true` | Install the CRDs with the chart. Set `false` when you apply `crds.yaml` of the release yourself. |
| `crd.keep` | `true` | Annotate the CRDs with `helm.sh/resource-policy: keep`. Then `helm uninstall` keeps the CRDs and your custom resources. |
| `rbacHelpers.enable` | `false` | Install admin, editor, and viewer `ClusterRole`s for each CRD. |
| `metrics.enable` | `true` | Expose the RBAC-protected `/metrics` endpoint. |
| `metrics.port` | `8443` | Metrics server port. |
| `certManager.enable` | `false` | Make the `ServiceMonitor` verify the metrics endpoint with the certificate in the Secret `metrics-server-cert`. The chart does not create that Secret. |
| `prometheus.enable` | `false` | Install a `ServiceMonitor`. Needs the prometheus-operator CRDs. You apply the alert rules and dashboards yourself, see [Observability](https://github.com/konsole-is/camunda-operator/blob/main/docs/observability.md). |
| `nameOverride` | unset | Replace the chart name in the resource names and in the `app.kubernetes.io/name` label. |
| `fullnameOverride` | unset | Replace the release and chart name at the start of each resource name. |

`make helm-generate` writes `values.yaml` from `config/`. Do not edit `values.yaml`. Change the defaults in `config/`.
