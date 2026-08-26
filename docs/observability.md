# Observability

The manager serves Prometheus metrics on `/metrics`. Two dashboards and three sets of alert rules ship with the operator. Together they answer the two questions of the person on call: is the operator healthy, and which custom resources are not `Ready`.

## Metrics

Every series carries a `controller` label. Its value is the lower-cased kind of the custom resource that the controller reconciles, for example `camundacluster`. The same value labels the `controller_runtime_*` and `workqueue_*` series of that controller.

| Metric | What it reports |
| --- | --- |
| `camunda_operator_controller_condition` | One series per condition of every custom resource. The value is the `lastTransitionTime` of the condition as a Unix timestamp. The series of a deleted custom resource disappear with it. Labels, besides `controller`: `kind`, `name`, `namespace` (`exported_namespace` after a scrape, see below), `condition`, `status`, `reason`, `id`. |
| `ocf_resource_apply_total` | Counter of the resources that the operator applied, by `controller`, `owner_kind`, `component`, `resource`, `kind`, and `operation`. The operation is `none` when the apply changed nothing. |
| `ocf_resource_apply_errors_total` | Counter of the applies that failed, with the same labels without `operation`. |
| `controller_runtime_*`, `workqueue_*`, `rest_client_*`, `process_*` | The standard series of every controller-runtime operator. |

The manager serves the namespace of a custom resource as the label `namespace`. A scrape through the `ServiceMonitor` stamps the namespace of the manager on every series as `namespace` too. Prometheus keeps the target label and renames the served one to `exported_namespace`, because the `ServiceMonitor` does not set `honorLabels`. Query `exported_namespace` for the namespace of a custom resource. The shipped dashboards and rules do the same.

## Install the dashboards and the alert rules

The chart value `prometheus.enable=true` installs the `ServiceMonitor`. The dashboards and the alert rules are not part of the chart. Apply them into the namespace that your Prometheus Operator and your Grafana sidecar watch:

```bash
kubectl apply -n monitoring -k "https://github.com/konsole-is/camunda-operator//config/prometheus/observability?ref=<version>"
```

This creates:

| Object | Kind | Content |
| --- | --- | --- |
| `camunda-operator-dashboards` | `ConfigMap` with the label `grafana_dashboard: "1"` | The dashboards `OCF Operator` and `CRD Conditions Browser`. |
| `camunda-operator-crd-conditions` | `PrometheusRule` | The alerts on the conditions of the custom resources. |
| `ocf-managed-resources` | `PrometheusRule` | The alerts on the resources that the operator applies. |
| `ocf-controller-runtime` | `PrometheusRule` | The alerts on reconciliation, the work queue, and leader election. |

If a Prometheus Operator selects rules by a label, for example the `release` label of kube-prometheus-stack, add that label with an overlay:

```yaml
# kustomization.yaml
resources:
- https://github.com/konsole-is/camunda-operator//config/prometheus/observability?ref=<version>
labels:
- pairs:
    release: kube-prometheus-stack
```

```bash
kubectl apply -n monitoring -k .
```

Apply this directory once per cluster. If you run the operator in two namespaces, both installs share the one copy: every rule tells the installs apart by their scrape job. The two `ocf-*` rule sets are shared by every operator that is built on the same framework. If two of these operators run in one cluster, keep one copy of the `ocf-*` rules. A second copy in another namespace fires every shared alert twice, once from each copy.

The dashboards are Grafana JSON. Without the sidecar, import the two files under [`config/prometheus/observability/dashboards`](https://github.com/konsole-is/camunda-operator/tree/<version>/config/prometheus/observability/dashboards) through the Grafana UI.

## Dashboards

The `OCF Operator` dashboard shows one install of the operator. Select the namespace and the scrape job of the manager, then narrow to one controller. The rows answer four questions, from the top. Is the operator healthy? Which custom resources are not `Ready`? Is any resource rewritten or failing on every apply? How do the reconciliation and the work queue perform?

The `CRD Conditions Browser` lists the custom resources of one kind by condition, status, and reason. Every condition alert links to this dashboard, narrowed to the one custom resource that fired it.

## Alerts

Every rule ships with `severity: warning`. The thresholds are the defaults of the framework. To change a threshold, a duration, or the severity, add a patch to the overlay above. This example makes `CustomResourceNotReady` wait one hour:

```yaml
# kustomization.yaml
resources:
- https://github.com/konsole-is/camunda-operator//config/prometheus/observability?ref=<version>
patches:
- target:
    kind: PrometheusRule
    name: camunda-operator-crd-conditions
  patch: |
    - op: replace
      path: /spec/groups/0/rules/0/for
      value: 1h
```

### Conditions

| Alert | Fires when | What to do |
| --- | --- | --- |
| `CustomResourceNotReady` | The `Ready` condition of a custom resource is `False` for 30 minutes. | Read the reason and the message of the condition with `kubectl get <kind> <name> -o yaml`. The [CRD reference](crds/index.md) lists the reasons of each kind and the step for each. |
| `CustomResourceConditionUnknown` | Any condition of a custom resource is `Unknown` for 30 minutes. | The operator did not report on this custom resource. Make sure that the manager runs and that its logs show no error for this resource. |
| `CustomResourceConditionStuck` | The `Ready` condition of a custom resource is not `True` for 6 hours. | Same as `CustomResourceNotReady`. This alert also fires when the condition is `Unknown`, and it survives a restart of Prometheus. |

### Applied resources

Both alerts fire per resource type of one controller, not per custom resource. The message names the owner kind, the component, and the resource.

| Alert | Fires when | What to do |
| --- | --- | --- |
| `ManagedResourceNotConverging` | More than half of the applies of one resource type rewrite the object, over 15 minutes, and more than 15 rewrites happened. | Another writer changes the object after every apply. Compare the managed fields of the object with `kubectl get <kind> <name> --show-managed-fields -o yaml` and remove the other writer. |
| `ManagedResourceApplyFailing` | More than half of the apply attempts of one resource type fail, over 15 minutes, and more than 5 failed. | The `Ready` condition of the affected custom resources carries the error message. Read it with `kubectl get <owner-kind> -A -o yaml`. |

### Reconciliation

| Alert | Fires when | What to do |
| --- | --- | --- |
| `ControllerReconcileErrors` | More than a quarter of the reconciles of one controller return an error, over 10 minutes, for 15 minutes. | Read the manager logs: `kubectl logs -n camunda-operator-system deployment/camunda-operator-controller-manager`. |
| `ControllerReconcilePanics` | A reconcile panicked in the last 10 minutes. | Read the manager logs and report the stack trace as a bug. |
| `ControllerWorkqueueBacklog` | Items wait more than 100 seconds for a worker, at the 99th percentile, for 15 minutes. | The manager is too slow for the number of custom resources. Give it more CPU. |
| `ControllerReconcileLatencyHigh` | A reconcile takes more than 30 seconds, at the 99th percentile, for 15 minutes. | A call to the Kubernetes API or to a Camunda cluster is slow. Read the manager logs for the resource that is slow. |
| `OperatorLeaderMissing` | No replica of the manager holds the leader lease for 5 minutes. | Make sure that a manager pod runs: `kubectl get pods -n camunda-operator-system`. |

## Related

- [Installation](installation.md): the `prometheus.enable` value.
- [Operations](guides/operations.md#monitor): the metrics of the Camunda clusters that the operator runs.
