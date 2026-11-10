# Write a plugin

This guide is for the author of a plugin. A plugin is a Helm chart and a plugin controller that offer an option in a section of a claim. You can write the controller in any language. The guide tells you which objects the controller writes, with which labels, and what it does when a request is deleted. [Plugins](../architecture.md#plugins) on the architecture page shows how the parts fit together.

A plugin controller in Go can use pluginkit, which does the steps of this guide for you. See [pluginkit](https://github.com/konsole-is/camunda-operator-plugins/blob/main/docs/pluginkit.md).

The examples use the plugin `s3-backup-bucket`, which creates one S3 bucket for the backups of each cluster. Its controller runs in the namespace `camunda-plugins` under the ServiceAccount `s3-backup-bucket-controller`.

## Publish a registration

The controller writes its [PluginRegistration](../crds/pluginregistration.md) when it starts, from its chart values. Then it sets the `Ready` condition on the registration. It serves no request before `Ready` is `True` for the current generation of the registration.

```yaml
apiVersion: plugins.camunda.io/v1alpha1
kind: PluginRegistration
metadata:
  name: s3-backup-bucket
spec:
  sections:
    - backup-storage
  label: New S3 bucket
  mode: Optional
  controller:
    name: s3-backup-bucket-controller
    version: 0.1.0
  serviceAccount:
    namespace: camunda-plugins
    name: s3-backup-bucket-controller
  outputs:
    - key: objectStorageConfig
      description: The name of the ObjectStorageConfig that points at the bucket.
  deletionPolicies:
    - Delete
    - Orphan
```

`outputs` must cover every output of each section in `sections`. The [PluginRegistration](../crds/pluginregistration.md) page describes every field, and the [API reference](../crds/api-reference.md#pluginregistration) lists them.

## Give the controller the verb serve

The chart gives the ServiceAccount of the controller the RBAC verb `serve` on its own registration. This ServiceAccount is the one that `spec.serviceAccount` of the registration names. A `PluginRegistration` is cluster-scoped, so the chart binds a ClusterRole with a ClusterRoleBinding. The repository of the operator holds this example as `config/plugins/example_serve_clusterrole.yaml`:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: s3-backup-bucket-controller
rules:
- apiGroups: ["plugins.camunda.io"]
  resources: ["pluginregistrations"]
  resourceNames: ["s3-backup-bucket"]
  verbs: ["serve"]
- apiGroups: ["plugins.camunda.io"]
  resources: ["grants"]
  verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: s3-backup-bucket-controller
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: s3-backup-bucket-controller
subjects:
- kind: ServiceAccount
  namespace: camunda-plugins
  name: s3-backup-bucket-controller
```

The controller also needs access to its registration, to the requests, and to events. `resourceNames` keeps the controller away from the registrations of other plugins. The verb `create` cannot have `resourceNames`, so it stays in a rule of its own. Add these rules to the same ClusterRole. Also add the rules for the objects that the controller writes, for example ConfigMaps:

```yaml
- apiGroups: ["plugins.camunda.io"]
  resources: ["pluginregistrations"]
  verbs: ["list", "watch", "create"]
- apiGroups: ["plugins.camunda.io"]
  resources: ["pluginregistrations", "pluginregistrations/status"]
  resourceNames: ["s3-backup-bucket"]
  verbs: ["get", "update", "patch"]
- apiGroups: ["plugins.camunda.io"]
  resources: ["pluginrequests"]
  verbs: ["get", "list", "watch", "update", "patch"]
- apiGroups: ["plugins.camunda.io"]
  resources: ["pluginrequests/status", "pluginrequests/finalizers"]
  verbs: ["get", "update", "patch"]
- apiGroups: ["events.k8s.io"]
  resources: ["events"]
  verbs: ["create", "patch"]
```

Without `serve`, the API server refuses each grant that the controller writes. The message is `the requester <user> does not have the verb serve on the PluginRegistration <name>, so it cannot write a Grant for it`. See [Who can write a grant](../crds/grant.md#who-can-write-a-grant).

## Keep Accepted

The operator writes the condition `Accepted` on the status of your registration. The controller writes `Ready` on the same status. When the controller writes the status, it sends `Accepted` back as it found it. For example, read the registration, set `Ready` in its list of conditions, and update the status with the full list.

The fleet manager offers the option only when `Accepted` and `Ready` are both `True`. When the controller drops `Accepted`, the option leaves the fleet manager until the operator writes `Accepted` again.

## Serve a request

A claim that selects the option writes a [PluginRequest](../crds/pluginrequest.md) with the label `plugins.camunda.io/registration: s3-backup-bucket`. The controller serves each request with this label. On each request, it writes in this order:

1. The finalizer `plugins.camunda.io/cleanup`, before the first object for the request.
2. `status.outputs`, with a value for each output of the section, and the condition `OutputsReady=True`. The `observedGeneration` of the condition is the current generation of the request.
3. The condition `Ready`, when the resources of the request are ready. When `status.podSelector` is set, at least one pod that it selects is Running and Ready by then.

The status of a ready request, as `kubectl get pluginrequest orders-backup-storage -n my-cluster-ns -o yaml` prints it:

```yaml
status:
  observedGeneration: 1
  conditions:
    - type: OutputsReady
      status: "True"
      reason: Healthy
      observedGeneration: 1
    - type: Ready
      status: "True"
      reason: Healthy
      observedGeneration: 1
  outputs:
    objectStorageConfig: orders-backup-storage
  summary:
    - label: Bucket
      value: orders-backup-storage
      severity: Info
  summaryTime: "2026-10-15T09:00:00Z"
  podSelector:
    matchLabels:
      plugins.camunda.io/request: orders-backup-storage
  resources:
    - kind: S3Bucket
      name: orders-backup-storage
      id: arn:aws:s3:::orders-backup-storage
      location: eu-west-1
      role: Backup bucket
      onOrphan: Kept
```

Five reasons of `Ready` have a fixed meaning. Use them only for that meaning:

| Reason | When to set it |
| --- | --- |
| `GrantRejected` | A grant of the request has `Ready=False`. |
| `WaitingForGrants` | No grant of the request has `Ready=False`, and a grant does not have `Ready=True` yet. |
| `WaitingForEndpoints` | A request of the section `access` waits for the endpoints in the status of its target. `OutputsReady` stays `True`. |
| `InvalidParameters` | The parameters do not pass the schema or the rules of the registration. The message names each field that fails. |
| `MissingInput` | A required input of the registration has no value in `spec.inputs`. Set `OutputsReady` to `False` with the same reason, and write nothing for the request. The message names the input. |

For any other state, choose your own reason. Record an event on the request for each external action, for example a bucket that the controller created.

## Label what you write

Each object that the controller writes for a request carries these labels:

| Label | On | Value |
| --- | --- | --- |
| `plugins.camunda.io/registration` | Every object | The name of the registration. |
| `plugins.camunda.io/request` | Every object | The name of the request. |
| `plugins.camunda.io/request-namespace` | A cluster-scoped object | The namespace of the request. |
| `plugins.camunda.io/cluster` | A grant | `spec.target.name` of the request. |

A namespaced object also gets an owner reference to the request. A cluster-scoped object gets no owner reference. A ConfigMap that the controller writes for the request `orders-backup-storage`:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: orders-backup-storage-settings
  namespace: my-cluster-ns
  labels:
    plugins.camunda.io/registration: s3-backup-bucket
    plugins.camunda.io/request: orders-backup-storage
  ownerReferences:
    - apiVersion: plugins.camunda.io/v1alpha1
      kind: PluginRequest
      name: orders-backup-storage
      uid: 4b8d5f3e-1c2a-4e7f-9a0b-6d3c2e1f0a9b
```

Each object, and each resource outside the cluster, gets a row in `status.resources` with `kind`, `name`, and `onOrphan`. `onOrphan` is `Kept` or `Deleted`. The plugin chooses it for each row. It says what the plugin does with the object when the request is deleted with the policy `Orphan`.

## Delete and Orphan

`spec.deletionPolicies` of the registration lists the policies that the plugin supports. `Delete` is mandatory. When the request is deleted, the controller reads `spec.deletionPolicy` of the request and applies it:

- `Delete`: delete every object and every resource outside the cluster of the request. Then remove the finalizer.
- `Orphan`: for each row with `onOrphan: Kept`, remove the owner reference to the request and keep both labels `plugins.camunda.io/registration` and `plugins.camunda.io/request`. Delete each row with `onOrphan: Deleted`. Then remove the finalizer.

A plugin of the section `access` writes routes to the Services of the cluster. With both policies, it deletes a route whose Service no longer exists. The request goes away when the finalizer is gone. See [Deletion](../crds/pluginrequest.md#deletion).

## Write a grant

When the registration has [grant templates](../crds/pluginregistration.md#grant-templates), the controller writes one [Grant](../crds/grant.md) for each template whose required inputs have values. The grant lives in the namespace of the request. It carries the labels of [Label what you write](#label-what-you-write) and an owner reference to the request. `spec.fragment` is the template with its placeholders filled in:

```yaml
apiVersion: plugins.camunda.io/v1alpha1
kind: Grant
metadata:
  name: orders-backup-storage-bucket-access
  namespace: my-cluster-ns
  labels:
    plugins.camunda.io/registration: s3-backup-bucket
    plugins.camunda.io/request: orders-backup-storage
    plugins.camunda.io/cluster: orders
  ownerReferences:
    - apiVersion: plugins.camunda.io/v1alpha1
      kind: PluginRequest
      name: orders-backup-storage
      uid: 4b8d5f3e-1c2a-4e7f-9a0b-6d3c2e1f0a9b
spec:
  cluster:
    name: orders
  template: bucket-access
  format: aws-iam-statement
  fragment:
    Effect: Allow
    Action: ["s3:GetObject", "s3:PutObject"]
    Resource: ["arn:aws:s3:::orders-backup-storage/*"]
```

The identity extension of the cluster applies the grant and writes its status. The controller reads the `Ready` condition of its grants to set `GrantRejected` or `WaitingForGrants` on the request.

## Log JSON lines with the request key

The controller writes each log line as one JSON object. A line about a request carries the key `request`. Its value is an object with the `name` and the `namespace` of the request.

The controller also logs the message `Set Ready of PluginRequest` each time the `Ready` condition of a request changes its status or its reason. The line has the keys `status` and `reason` with the new values. A request has one such line from its first reconcile on.

```json
{"level":"info","ts":"2026-10-15T09:00:04Z","msg":"Set Ready of PluginRequest","request":{"name":"orders-backup-storage","namespace":"my-cluster-ns"},"status":"True","reason":"Healthy"}
```

The fleet manager shows the logs of a request from these lines. The conformance check `REQ-LOGS` reads the log of the controller pods. It fails when a line is not JSON. It also fails when no line carries the `request` key for a request.

A controller built with pluginkit writes these lines for you.

## Run the conformance suite

The conformance suite checks that a plugin keeps the rules of this guide. Each release `vX.Y.Z` of the operator publishes the suite in three forms:

- The Go module `github.com/konsole-is/camunda-operator/conformance`, at the Go tag `conformance/vX.Y.Z`.
- The binaries `plugin-conformance-<os>-<arch>`, as assets of the release, for `linux-amd64`, `linux-arm64`, `darwin-amd64`, and `darwin-arm64`.
- The image `ghcr.io/konsole-is/camunda-operator-plugin-conformance:X.Y.Z`.

Use the suite of the operator release that your plugin builds against.

### What the cluster needs

The suite runs against a cluster that has these parts:

- The operator chart of the same release. It installs the plugin kinds, the operator, and the grant admission policy.
- The chart of your plugin.
- The registration of your plugin, with `Accepted=True` and `Ready=True`. The suite reads the registration one time, when it starts. Wait for both conditions before you start the suite.

The suite creates the namespace of the run and writes one request for each case into it. At the end, it deletes the requests and the namespace.

### Run the binary

Write a config file, for example `conformance.yaml`:

```yaml
registration: s3-backup-bucket
namespace: conformance-s3-backup-bucket
timeouts:
  outputsReady: 2m
  ready: 5m
  deletion: 90s
cases:
  - name: default
    inputs:
      bucket-region: eu-west-1
    parameters:
      retentionDays: 30
    target:
      kind: CamundaCluster
      name: orders
      serviceAccountName: orders-camunda
```

`registration` is the name of your registration. Each case is one request, with the inputs, the parameters, and the target that a claim gives it. A case without `deletionPolicy` runs one time for each deletion policy of the registration. The `timeouts` are optional. Without them, the suite waits 5 minutes for `OutputsReady`, 10 minutes for `Ready`, and 5 minutes for each deletion.

Run the binary against the cluster of your kubeconfig:

```bash
plugin-conformance --config conformance.yaml --report plugin-conformance-report.xml
```

The binary prints one line for each check, `PASS`, `FAIL`, or `SKIP`, and writes a JUnit report. Its exit code tells the result of the run:

| Exit code | Meaning |
| --- | --- |
| `0` | No check failed. |
| `1` | At least one check failed. |
| `2` | The run did not start, the binary did not write the report, or the cleanup at the end failed. |

### Run the suite in GitHub Actions

This job creates a kind cluster, installs the operator and the plugin, and runs the binary of the release `v0.3.0`. It reads the config file `conformance.yaml` from the root of your repository. `OPERATOR_VERSION` is the release version without the `v`.

```yaml
jobs:
  conformance:
    runs-on: ubuntu-latest
    env:
      OPERATOR_VERSION: 0.3.0
    steps:
      - uses: actions/checkout@v7

      - uses: azure/setup-helm@v5.0.1
        with:
          version: v4.3.1

      - name: Create a kind cluster
        run: |
          curl -fsSLo kind https://kind.sigs.k8s.io/dl/v0.33.0/kind-linux-amd64
          chmod +x kind && sudo mv kind /usr/local/bin/kind
          kind create cluster --image kindest/node:v1.37.0 --wait 120s

      - name: Install the operator
        run: |
          helm install camunda-operator oci://ghcr.io/konsole-is/charts/camunda-operator \
            --version "${OPERATOR_VERSION}" \
            --namespace camunda-operator-system --create-namespace --wait

      - name: Install the plugin
        run: |
          helm install s3-backup-bucket ./charts/s3-backup-bucket \
            --namespace camunda-plugins --create-namespace --wait

      - name: Wait for the registration
        run: |
          kubectl wait pluginregistration/s3-backup-bucket --for=create --timeout=120s
          kubectl wait pluginregistration/s3-backup-bucket --for=condition=Accepted --timeout=120s
          kubectl wait pluginregistration/s3-backup-bucket --for=condition=Ready --timeout=120s

      - name: Run the conformance suite
        run: |
          curl -fsSLo plugin-conformance \
            "https://github.com/konsole-is/camunda-operator/releases/download/v${OPERATOR_VERSION}/plugin-conformance-linux-amd64"
          chmod +x plugin-conformance
          ./plugin-conformance --config conformance.yaml --report plugin-conformance-report.xml

      - name: Keep the report
        if: ${{ !cancelled() }}
        uses: actions/upload-artifact@v7
        with:
          name: plugin-conformance-report
          path: plugin-conformance-report.xml
```

The plugin writes its registration after it starts, so the first `kubectl wait` waits until the registration exists. A failed check fails the job, because the binary exits with `1`.

### Skip a check

A run can skip only a check that has `Yes` in the column "Skippable" of [The checks](#the-checks). Each skip needs a reason, and the report shows it. Add the skips to the config file:

```yaml
skip:
  - id: REQ-LOGS
    reason: the controller writes plain text logs
```

If the config skips a check that a run cannot skip, the run does not start, and the exit code is `2`.

### The checks

The suite runs the checks in this order:

| Check | What it checks | Skippable |
| --- | --- | --- |
| `REG-ACCEPTED` | The registration has Accepted=True and Ready=True. | No |
| `REG-SCHEMA` | `spec.parameters` is a valid OpenAPI v3 schema, and the parameters of every case pass it. | No |
| `REG-OUTPUTS` | `spec.outputs` covers every output of each section in `spec.sections`. | No |
| `REG-INPUTS` | Each input names `<section>/<output>` of an installed section. | No |
| `REG-GRANTS` | Each grant template has a supported format and uses only the placeholders of the contract. | No |
| `REG-TEMPLATES` | Each default of an `x-fleet-template` parameter uses only the placeholders of the contract. | No |
| `REG-DELETION` | `spec.deletionPolicies` holds Delete. | No |
| `REG-RBAC` | The ServiceAccount of the registration has the verb serve on the registration name. | No |
| `REG-METRICS` | Each metric query uses only the placeholders of the contract and `.pods`. | No |
| `REQ-FINALIZER` | The request gets the finalizer `plugins.camunda.io/cleanup` before any object that the plugin writes for it. | No |
| `REQ-OUTPUTS-READY` | The request gets OutputsReady=True for its current generation within the OutputsReady timeout. | No |
| `REQ-OUTPUTS` | `status.outputs` has exactly the output keys of the registration, and each value is not empty. | No |
| `REQ-READY` | Within the Ready timeout, the request gets Ready=True, or Ready=False with the reason GrantRejected or WaitingForGrants. Ready=True fails the check while a Grant has a Ready condition and another Grant does not have Ready=True. The reason GrantRejected needs a Grant with Ready=False. The reason WaitingForGrants needs a Grant without Ready=True, and no Grant with Ready=False. | No |
| `REQ-LABELS` | Each object of the request carries both writer labels. A cluster-scoped object also carries `plugins.camunda.io/request-namespace` with the namespace of the request, and a namespaced object has an owner reference to the request. | No |
| `REQ-RESOURCES` | `status.resources` lists each object of the request, and each row has kind, name, and `onOrphan`. | No |
| `REQ-EVENTS` | The request has at least one event for each row of `status.resources`. | No |
| `REQ-DISPLAY` | The `summary`, `summaryTime`, `links`, and `podSelector` of the request status are complete and valid. | No |
| `REQ-LOGS` | The controller pods log JSON lines, and a line carries the key request with the name of each request. | Yes |
| `REQ-ACCEPTED-KEPT` | The registration keeps the Accepted condition of the registry controller while the plugin serves requests. | No |
| `GRANT-MATCH` | Each grant template whose required inputs have values has one Grant of the request. The Grant has the labels, the owner, the format, and the fragment of the filled-in template. | No |
| `GRANT-REQUIRES` | A grant template with a required input without a value has no Grant. | No |
| `GRANT-ADMISSION` | The API server refuses a Grant of the registration from a requester without the verb serve on it. It skips itself when the Grant admission policy is not installed. | Yes |
| `GRANT-READY` | A request is not Ready=True while one of its Grants has no Ready condition. It gets Ready=True within the Ready timeout after the suite sets Ready=True on its Grants. | Yes |
| `ORDER-REGISTRATION-FIRST` | The suite scales the plugin Deployment to zero, deletes the registration, and creates a request. The plugin must write its registration again with Ready=True when it starts, before the request gets a finalizer or an output. | Yes |
| `DELETE-DELETE` | With the deletion policy Delete, the request is gone within the Deletion timeout, and so is each of its objects. | No |
| `DELETE-ORPHAN` | With the deletion policy Orphan, each row with `onOrphan` Kept stays, with both writer labels and no owner reference to the request. Each row with `onOrphan` Deleted is gone. | No |
| `DELETE-ROUTES` | For the access section, a route to a Service that is gone is deleted under both deletion policies. | No |

To run the suite from `go test`, see the [README of the conformance module](https://github.com/konsole-is/camunda-operator/blob/main/conformance/README.md).

## Related

- [PluginSection](../crds/pluginsection.md): the sections and their outputs.
- [PluginRegistration](../crds/pluginregistration.md): the option that the plugin publishes.
- [PluginRequest](../crds/pluginrequest.md): the request that the plugin serves.
- [Grant](../crds/grant.md): the permissions that the plugin asks for.
- [API reference](../crds/api-reference.md): every field of the plugin kinds.
- [Claims](claims.md): how a claim selects the options.
