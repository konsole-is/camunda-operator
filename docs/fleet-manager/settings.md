# Fleet manager settings

The settings of the fleet manager name this Kubernetes cluster, the Git repository and the Secret store for Git mode, and the Prometheus address. The fleet manager keeps them in the ConfigMap `fleet-manager-settings` in the release namespace. The release namespace is the namespace of the Helm release, `camunda-operator-system` in the examples on this page.

In Git mode, the fleet manager commits its changes to the Git repository and does not write them to the Kubernetes API. In this version, Git mode is not available yet, and no page of the fleet manager shows or uses the settings.

## Set the settings at install

The chart values `fleetManager.settings.*` give the first value of each setting:

```bash
helm upgrade --install camunda-operator \
  oci://ghcr.io/konsole-is/charts/camunda-operator \
  --version <version> \
  --namespace camunda-operator-system \
  --create-namespace \
  --set fleetManager.enabled=true \
  --set fleetManager.settings.kubernetesClusterName=prod-eu \
  --set fleetManager.settings.prometheus.url=http://prometheus.monitoring:9090
```

The fleet manager creates the ConfigMap `fleet-manager-settings` from these values the first time that it reads its settings. After that, the ConfigMap is the only source of the settings. A later `helm upgrade` with other `fleetManager.settings.*` values does not change the ConfigMap. To change a setting, edit the ConfigMap, as [Change a setting](#change-a-setting) shows.

## Settings

| Key in the ConfigMap | Chart value | What it does |
| --- | --- | --- |
| `kubernetesClusterName` | `fleetManager.settings.kubernetesClusterName` | The name of this Kubernetes cluster. |
| `git.repository` | `fleetManager.settings.git.repository` | The HTTPS address of the Git repository for Git mode. |
| `git.branch` | `fleetManager.settings.git.branch` | The branch of the repository. |
| `git.path` | `fleetManager.settings.git.path` | The folder in the repository for the files of the fleet manager. |
| `git.credentialsSecretRef` | `fleetManager.settings.git.credentialsSecretRef` | The name of the Secret with the Git credentials. See [Git credentials](#git-credentials). |
| `secretStore.kind` | `fleetManager.settings.secretStore.kind` | `SecretStore` or `ClusterSecretStore`, the kind of the External Secrets store for Git mode. |
| `secretStore.name` | `fleetManager.settings.secretStore.name` | The name of that store. |
| `prometheus.url` | `fleetManager.settings.prometheus.url` | The address of Prometheus. |
| `prometheus.credentialsSecretRef` | `fleetManager.settings.prometheus.credentialsSecretRef` | The name of the Secret with the Prometheus credentials. See [Prometheus credentials](#prometheus-credentials). |

An empty value means that the setting is not set. An address in `git.repository` or `prometheus.url` must not hold a user name or a password, as in `https://user:token@git.example.com`. The pages show the settings, so put the credentials in a Secret.

## Change a setting

The ConfigMap holds one key for each setting:

```bash
kubectl -n camunda-operator-system get configmap fleet-manager-settings -o yaml
```

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: fleet-manager-settings
  namespace: camunda-operator-system
data:
  git.branch: ""
  git.credentialsSecretRef: ""
  git.path: ""
  git.repository: ""
  kubernetesClusterName: prod-eu
  prometheus.credentialsSecretRef: ""
  prometheus.url: http://prometheus.monitoring:9090
  secretStore.kind: ""
  secretStore.name: ""
```

To change a setting, edit the ConfigMap:

```bash
kubectl -n camunda-operator-system edit configmap fleet-manager-settings
```

The fleet manager reads the ConfigMap each time that it needs a setting, so an edit takes effect without a restart. Git mode does not apply to the settings. The fleet manager never commits them to the repository.

## Git credentials

The fleet manager connects to the Git repository over HTTPS. The Secret of `git.credentialsSecretRef` holds the keys `username` and `token`. Create it in the release namespace:

```bash
kubectl -n camunda-operator-system create secret generic fleet-git \
  --from-literal=username=<user> \
  --from-literal=token=<token>
```

Then set `git.credentialsSecretRef` to `fleet-git`. Without the setting, the fleet manager connects to the repository without credentials.

## Prometheus credentials

The Secret of `prometheus.credentialsSecretRef` holds the keys `username` and `password` for basic authentication, or the key `token` for a bearer token. Create it in the release namespace:

```bash
kubectl -n camunda-operator-system create secret generic fleet-prometheus \
  --from-literal=token=<token>
```

Then set `prometheus.credentialsSecretRef` to `fleet-prometheus`. Without the setting, the fleet manager sends no credentials to Prometheus.

## Secret values

The fleet manager shows the names of Secrets and the names of their keys. It never shows the value of a key, and no page and no response of the fleet manager holds one.

## Secrets in Git mode

In Git mode, the fleet manager does not write a Secret value. It commits an `ExternalSecret` to the Git repository, and External Secrets makes the Secret from the store of `secretStore.kind` and `secretStore.name`. For the Secret `db` in the namespace `orders`, the commit holds this object:

```yaml
apiVersion: external-secrets.io/v1
kind: ExternalSecret
metadata:
  name: db
  namespace: orders
spec:
  refreshInterval: 1h
  secretStoreRef:
    kind: SecretStore
    name: vault
  target:
    name: db
    creationPolicy: Owner
  dataFrom:
    - extract:
        key: orders/db
```

Put each key of the Secret as a property of the store entry `<namespace>/<name>`, here `orders/db`. External Secrets copies each property of the entry into the Secret. If `secretStore.kind` or `secretStore.name` is not set, the fleet manager refuses to write a Secret in Git mode.
