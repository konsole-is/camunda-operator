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

The CRDs of this chart have no field descriptions. The validation and the defaults do not change. `kubectl explain` shows the fields and their types but no text. Helm stores each release in one Secret, and the API server limits a Secret to 1 MiB of data. The CRDs with their descriptions do not fit in that Secret. The API reference has the field descriptions:
<https://konsole-is.github.io/camunda-operator/latest/crds/api-reference/>. The CRDs of `install.yaml` and `crds.yaml` of each release keep them, so `kubectl explain` shows them on a cluster where you applied one of these files.

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
| `manager.env` | the entry `CAMUNDA_OPERATOR_SERVICE_ACCOUNT` | Environment variables. Helm replaces the list, so keep this entry when you set `manager.env`. |
| `manager.envOverrides` | `{}` | Environment variables as a map. A name here wins over the same name in `manager.env`. |
| `manager.imagePullSecrets` | `[]` | Image pull secrets of the manager Pod and of the fleet manager Pod. |
| `manager.podSecurityContext` | `runAsNonRoot: true`, `seccompProfile.type: RuntimeDefault` | Pod security context. |
| `manager.securityContext` | drops `ALL`, read-only root FS, no privilege escalation | Container security context. |
| `manager.resources` | requests `10m`/`64Mi`, limits `500m`/`128Mi` | Resource requests and limits. |
| `manager.affinity` | `{}` | Pod affinity. |
| `manager.nodeSelector` | `{}` | Pod node selector. |
| `manager.tolerations` | `[]` | Pod tolerations. |
| `crd.enabled` | `true` | Install the CRDs with the chart. Set `false` when you apply `crds.yaml` of the release yourself. |
| `crd.keep` | `true` | Annotate the CRDs with `helm.sh/resource-policy: keep`. Then `helm uninstall` keeps the CRDs and your custom resources. |
| `rbac.helpers.enabled` | `false` | Install admin, editor, and viewer `ClusterRole`s for each CRD. |
| `metrics.enabled` | `true` | Expose the RBAC-protected `/metrics` endpoint. |
| `metrics.port` | `8443` | Metrics server port. |
| `certManager.enabled` | `false` | Make the `ServiceMonitor` verify the metrics endpoint with the certificate in the Secret `metrics-server-cert`. The chart does not create that Secret. |
| `prometheus.enabled` | `false` | Install a `ServiceMonitor`. Needs the prometheus-operator CRDs. You apply the alert rules and dashboards yourself, see [Observability](https://github.com/konsole-is/camunda-operator/blob/main/docs/observability.md). |
| `fleetManager.enabled` | `false` | Install the fleet manager, a web app for the Camunda clusters of the Kubernetes cluster. See [Install the fleet manager](https://github.com/konsole-is/camunda-operator/blob/main/docs/fleet-manager/install.md). |
| `fleetManager.image.repository` | `ghcr.io/konsole-is/camunda-operator-fleet-manager` | Fleet manager image repository. |
| `fleetManager.image.tag` | the chart version | Fleet manager image tag. |
| `fleetManager.image.pullPolicy` | `IfNotPresent` | Image pull policy of the fleet manager. |
| `fleetManager.resources` | requests `50m`/`256Mi`, limit memory `512Mi` | Resource requests and limits of the fleet manager. |
| `fleetManager.extraCACerts.configMapName` | `""` | A ConfigMap in the release namespace with a CA bundle that the fleet manager trusts, for a Git or Prometheus server with a private CA. Empty adds no CA. See [Trust a private CA](https://github.com/konsole-is/camunda-operator/blob/main/docs/fleet-manager/install.md#trust-a-private-ca). |
| `fleetManager.extraCACerts.key` | `ca.crt` | The key of the PEM CA bundle in that ConfigMap. |
| `fleetManager.settings.*` | `""` | Settings of the fleet manager for its first start. See [Fleet manager settings](https://github.com/konsole-is/camunda-operator/blob/main/docs/fleet-manager/settings.md). |
| `nameOverride` | unset | Replace the chart name in the resource names and in the `app.kubernetes.io/name` label. |
| `fullnameOverride` | unset | Replace the release and chart name at the start of each resource name. |

The chart carries `values.schema.json`. Helm checks your values against it on `install`, `upgrade`, `lint`, and `template`. A key that the chart does not know, or a value of the wrong type, stops the command:

```text
Error: values don't meet the specifications of the schema(s) in the following chart(s):
camunda-operator:
- at '/prometheus': additional properties 'enabeld' not allowed
```

Some values go to a Kubernetes field as a whole: `manager.resources`, `manager.podSecurityContext`, `manager.securityContext`, `manager.strategy`, `fleetManager.resources`, and each value with an empty default, such as `manager.nodeSelector`. Helm accepts any key in these values. The Kubernetes API checks their content. If an image tag looks like a number, set it with `--set-string`, for example `--set-string manager.image.tag=8`. The same holds for `manager.cliImage.tag` and `fleetManager.image.tag`. Helm reads `--set manager.image.tag=8` as a number, and the schema expects a string.

`make helm-generate` writes `values.yaml` and `values.schema.json` from `config/`. Do not edit these files. Change the defaults in `config/`.

## Manager settings

The manager reads the flags and the environment variables in this table. A flag wins over its environment variable.

To set a flag, give the full list in `manager.args`. Helm replaces the list, so keep `--leader-elect` in it:

```bash
helm upgrade camunda-operator \
  oci://ghcr.io/konsole-is/charts/camunda-operator \
  --version <version> \
  --namespace camunda-operator-system \
  --set 'manager.args={--leader-elect,--zap-log-level=debug}'
```

To set an environment variable, use `manager.envOverrides.<NAME>=<value>`, for example `--set manager.envOverrides.CAMUNDA_OPERATOR_WORKLOAD_GRACE_PERIOD=60m`. The manager ignores a name that it does not know, and it gives no error. Copy the names from this table.

The chart sets some flags from its own values. The column "Set by the chart" names the value to change. The chart puts `manager.args` after its own flags, so a flag in `manager.args` wins over the same flag from the chart. For `--health-probe-bind-address`, `--metrics-bind-address`, and `--metrics-secure`, change the chart value, not `manager.args`. The probes, the metrics Service, the `ServiceMonitor`, and the metrics RBAC use the value, so a different flag breaks them.

| Flag | Environment variable | Default | Set by the chart | Description |
|---|---|---|---|---|
| `--camunda-operator-cli-image` | `CAMUNDA_OPERATOR_CLI_IMAGE` | none | `manager.cliImage` | The CLI image that the logical backup and restore Jobs of a PostgreSQL database run. The manager does not start without it. The chart always sets the flag, so the environment variable has no effect in a chart install. |
| `--namespace` | `CAMUNDA_OPERATOR_NAMESPACE` | the namespace of the manager Pod | no | The namespace of the operator. It holds the Leases that make sure that only one resource claims a logical database, a Keycloak realm, or the secondary storage of a cluster. |
| `--service-account` | `CAMUNDA_OPERATOR_SERVICE_ACCOUNT` | none | no | The name of the ServiceAccount that the manager runs as, in the namespace of the operator. The built-in `PluginRegistration`s name it. The manager does not start without it. The chart sets the environment variable to the ServiceAccount of the manager Pod, in the default of `manager.env`. If you set `manager.env`, copy this entry from the chart defaults into your list. `manager.envOverrides` cannot set it, because the entry reads the Pod. |
| `--workload-grace-period` | `CAMUNDA_OPERATOR_WORKLOAD_GRACE_PERIOD` | `30m` | no | How long a Camunda, Optimize, Keycloak, or exporter workload can stay not ready before its condition reports `Degraded` or `Down`. See [Grace periods](https://github.com/konsole-is/camunda-operator/blob/main/docs/installation.md#grace-periods). |
| `--datastore-grace-period` | `CAMUNDA_OPERATOR_DATASTORE_GRACE_PERIOD` | `30m` | no | How long an Elasticsearch or a PostgreSQL cluster can stay not ready before its condition reports `Degraded` or `Down`. See [Grace periods](https://github.com/konsole-is/camunda-operator/blob/main/docs/installation.md#grace-periods). |
| `--leader-elect` | none | `false` | `manager.args` | Only one replica of the manager acts at a time. The default of `manager.args` turns it on. |
| `--metrics-bind-address` | none | `0` | `metrics.enabled`, `metrics.port` | The address of the `/metrics` endpoint. `0` turns the endpoint off. The chart sets `:<metrics.port>`, or `0` when `metrics.enabled` is `false`. |
| `--metrics-secure` | none | `true` | `metrics.secure` | Serve `/metrics` over HTTPS, and only to a caller that the Kubernetes RBAC allows. `false` serves plain HTTP to every caller. |
| `--metrics-cert-path` | none | empty | no | The directory that holds the certificate of the metrics server. When it is empty and `--metrics-secure` is `true`, the manager makes a self-signed certificate. |
| `--metrics-cert-name` | none | `tls.crt` | no | The file name of the certificate in `--metrics-cert-path`. |
| `--metrics-cert-key` | none | `tls.key` | no | The file name of the key in `--metrics-cert-path`. |
| `--kubeconfig` | `KUBECONFIG` | empty | no | The kubeconfig file that the manager uses to connect to Kubernetes. When both are empty, the manager uses its service account in its own cluster. Set it only to run the manager against a different cluster. |
| `--health-probe-bind-address` | none | `:8081` | `manager.healthProbe.port` | The address of the `/healthz` and `/readyz` endpoints. |
| `--enable-http2` | none | `false` | no | Allow HTTP/2 on the metrics and webhook servers. It is off because of the HTTP/2 Rapid Reset vulnerabilities. |
| `--webhook-cert-path` | none | empty | no | The directory that holds the certificate of the webhook server. The operator has no webhooks, so this flag changes nothing. |
| `--webhook-cert-name` | none | `tls.crt` | no | The file name of the webhook certificate. It changes nothing, as above. |
| `--webhook-cert-key` | none | `tls.key` | no | The file name of the webhook key. It changes nothing, as above. |
| `--zap-devel` | none | `false` | no | Development logging: console format, `debug` level, and stack traces from `warn`. The default `false` gives JSON lines, `info` level, and stack traces from `error`. The fleet manager finds the log lines of a built-in `PluginRequest` only in JSON lines. |
| `--zap-log-level` | none | `info` | no | The lowest level that the manager logs: `debug`, `info`, `error`, or `panic`. An integer above 0 also works: `1` is the same as `debug`. A larger integer logs more detail. The default follows `--zap-devel`. |
| `--zap-encoder` | none | `json` | no | The log format: `json` or `console`. The default follows `--zap-devel`. |
| `--zap-stacktrace-level` | none | `error` | no | The lowest level that gets a stack trace: `info`, `error`, or `panic`. The default follows `--zap-devel`. The flag cannot set `warn`, so only `--zap-devel=true` gives stack traces from `warn`. |
| `--zap-time-encoding` | none | `rfc3339` | no | The time format of a log line: `epoch`, `millis`, `nanos`, `iso8601`, `rfc3339`, or `rfc3339nano`. The help text of the flag says `nano`, but the manager accepts only `nanos`. |
