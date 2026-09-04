# Management plane with your own OIDC provider

A complete management plane that authenticates against an identity provider
you run. The operator runs no Keycloak and issues no clients. You register the
clients, and the platform configuration names them.

The management plane is `my-management` in the namespace `my-management-ns`.
The manifests use the names of
[Management plane](https://konsole-is.github.io/camunda-operator/guides/management-plane/#step-3c-your-own-oidc-provider).

The `DatabaseServer` inherits its sizing from the preset `standard` in
[`config/example/presets`](../../presets). `CamundaManagementCluster` has no
preset kind, so every value on it is its own.

## Before you start

- Install the CloudNativePG operator and the Camunda operator. See
  [Installation](https://konsole-is.github.io/camunda-operator/installation/).
  This mode needs no Keycloak Operator.
- Register six clients in your identity provider:

    | Client | Kind | Used by |
    | --- | --- | --- |
    | `camunda-orchestration` | confidential | Every orchestration cluster |
    | `camunda-identity` | confidential | Management Identity |
    | `camunda-optimize` | confidential | Optimize |
    | `camunda-console` | public | Console |
    | `camunda-web-modeler` | public | Web Modeler |
    | `camunda-web-modeler-api` | confidential | The Web Modeler API |

- Replace the placeholder values in `02-secrets.yaml`: the secrets of the four
  confidential clients, the SMTP credentials of Web Modeler, and your Camunda
  license key.
- Replace `claimValue` in `06-management-cluster.yaml` with the value that
  identifies your first administrator. The claim `oid` is an example. Use a
  claim your provider puts in the token.
- Replace the four `login.example.com` URLs with those of your provider.
- Replace every `camunda.example.com` hostname with your own domain, and route
  each URL to the Service of its component.

## Apply

One command applies the whole inventory, presets included:

```sh
kubectl apply -k config/example/camunda-management-cluster/oidc
```

To see each resource become ready, apply the presets once, then the files in
their number order:

1. The shared presets, once per Kubernetes cluster:

    ```sh
    kubectl apply -k config/example/presets
    ```

2. `01-namespace.yaml` creates the namespace `my-management-ns`.
3. `02-secrets.yaml` creates the four client Secrets, the SMTP Secret, and the
   license Secret.
4. `03-database-server.yaml` creates the `DatabaseServer` `my-db`, which
   inherits the preset `standard`. Wait for it:

    ```sh
    kubectl wait databaseserver/my-db -n my-management-ns \
      --for=condition=Ready --timeout=10m
    ```

5. `04-databases.yaml` creates the two `Database` resources.
6. `05-platform-config.yaml` creates the cluster-scoped
   `CamundaPlatformConfig` `my-platform-config`.
7. `06-management-cluster.yaml` creates the `CamundaManagementCluster`
   `my-management`. Wait for it:

    ```sh
    kubectl wait camundamanagementcluster/my-management -n my-management-ns \
      --for=condition=Ready --timeout=15m
    ```

## What you get

- `my-management` publishes the cluster-scoped `ManagementAuthConfig`
  `my-management`. A `CamundaOptimize` names it in `spec.managementAuthRef`.
- `my-management-identity`, `my-management-console`, and the Web Modeler
  Services serve the three components.
- The condition `SecretsReady` reports the reason `Disabled`. The operator
  issues no credentials in this mode, so there is nothing for it to write.

## Add an orchestration cluster

This inventory holds no `CamundaCluster`. `clusterSelector` is empty, so the
plane serves every cluster on the Kubernetes cluster.

CAUTION: Do not apply
[Camunda cluster on Elasticsearch](../../camunda-cluster/elasticsearch) with
`-k`. That inventory carries a `CamundaPlatformConfig` of its own, under the
same cluster-scoped name `my-platform-config`, with `method: basic`. An apply
of it replaces the OIDC configuration here, and the management plane loses its
identity provider.

Take the namespace and the Elasticsearch cluster from it:

```sh
kubectl apply -f config/example/camunda-cluster/elasticsearch/01-namespace.yaml
kubectl apply -f config/example/camunda-cluster/elasticsearch/02-elasticsearch-cluster.yaml
```

That `ElasticsearchCluster` needs the prerequisites in
[its README](../../camunda-cluster/elasticsearch).

Its `04-camunda-cluster.yaml` is written for basic authentication. Copy that
file, and add the two fields that an OIDC cluster needs:

```yaml
apiVersion: core.camunda.io/v1
kind: CamundaCluster
metadata:
  name: my-cluster
  namespace: my-cluster-ns
spec:
  platformConfigRef: my-platform-config
  storageRef: my-storage-config
  externalUrl: "https://my-cluster.camunda.example.com"
  auth:
    admin:
      users:
        - "ada@example.com"
  # ... the version, the zeebe block, and the gateway block of the file
```

- `spec.auth.admin` names the first administrator. A new OIDC cluster has none
  until this block names one. Each entry under `users` is a value of the claim
  `preferred_username`, because `05-platform-config.yaml` names that claim in
  `usernameClaim`. While no user is named, the web applications show the
  first-run setup page at `/admin/setup`.
- `spec.externalUrl` gives the browser login its redirect URI, which is
  `<externalUrl>/sso-callback`. Register
  `https://my-cluster.camunda.example.com/sso-callback` on the
  `camunda-orchestration` client. Without `externalUrl` the operator registers
  no redirect URI, and the orchestration cluster falls back to its own default.

That `CamundaCluster` names `my-platform-config`, so it reads the OIDC
configuration of this inventory.

[Authentication](https://konsole-is.github.io/camunda-operator/guides/authentication/#oidc)
explains both fields in full.

## Remove

```sh
kubectl delete camundamanagementcluster/my-management -n my-management-ns
kubectl delete database --all -n my-management-ns
kubectl delete databaseserver/my-db -n my-management-ns
kubectl delete camundaplatformconfig/my-platform-config
kubectl delete namespace my-management-ns
```

The presets are shared, so leave them unless no inventory uses them any more.

## Related

- [Presets](https://konsole-is.github.io/camunda-operator/guides/presets/)
- [Management plane](https://konsole-is.github.io/camunda-operator/guides/management-plane/#step-3c-your-own-oidc-provider)
- [The clients of the management plane](https://konsole-is.github.io/camunda-operator/crds/camundaplatformconfig/#the-clients-of-the-management-plane)
- [CamundaManagementCluster](https://konsole-is.github.io/camunda-operator/crds/camundamanagementcluster/)
