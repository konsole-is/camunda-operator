# Fleet manager settings

The settings of the fleet manager name this Kubernetes cluster, the Git repository and the Secret store for Git mode, and the Prometheus address. The fleet manager keeps them in the ConfigMap `fleet-manager-settings` in the release namespace. The release namespace is the namespace of the Helm release, `camunda-operator-system` in the examples on this page.

In Git mode, the fleet manager commits its changes to the Git repository and does not write them to the Kubernetes API. Git mode is on when `git.repository` is set. In this version, no page of the fleet manager shows the settings or saves a change yet.

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

An empty value means that the setting is not set. An address in `git.repository` or `prometheus.url` starts with `http://` or `https://`, and it must not hold a user name or a password, as in `https://user:token@git.example.com`. The fleet manager treats another address as not set. The pages show the settings, so put the credentials in a Secret. The fleet manager drops a user name and a password from an address when it reads the settings.

## Change a setting

In this version, no page reads the settings, so the ConfigMap does not exist after the install. Create it before you change a setting. Each key that you leave out is not set:

```bash
kubectl -n camunda-operator-system create configmap fleet-manager-settings \
  --from-literal=kubernetesClusterName=prod-eu
```

When the ConfigMap exists, the fleet manager uses it and does not read the `fleetManager.settings.*` values. A ConfigMap that the fleet manager created holds one key for each setting:

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

The fleet manager connects to the Git repository over HTTPS. It does not send the credentials to an address that starts with `http://`. The Secret of `git.credentialsSecretRef` holds the keys `username` and `token`. Create it in the release namespace:

```bash
kubectl -n camunda-operator-system create secret generic fleet-git \
  --from-literal=username=<user> \
  --from-literal=token=<token>
```

Then set `git.credentialsSecretRef` to `fleet-git`. Without the setting, the fleet manager connects to the repository without credentials.

## Git mode

Set `git.repository` to turn on Git mode. Set the branch, the folder, and the Secret with the credentials in the same ConfigMap:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: fleet-manager-settings
  namespace: camunda-operator-system
data:
  git.repository: https://git.example.com/platform/fleet.git
  git.branch: main
  git.path: clusters/prod
  git.credentialsSecretRef: fleet-git
  # ... the rest of your settings
```

An empty `git.branch` uses the default branch of the repository. The branch must exist before the first save. An empty `git.path` puts the files at the root of the repository. `git.path` must be a folder inside the repository, so a save fails when the path holds `..`.

Each save in the fleet manager becomes one commit on the branch. The subject of the commit names the change, and the author is `fleet-manager`. When the repository already holds the same content, the save makes no commit. The fleet manager writes each object to one file under `git.path`:

```text
clusters/prod/
├── _cluster/
│   └── namespaces/
│       └── team-a.yaml
└── team-a/
    └── camundaclusterclaims/
        └── orders.yaml
```

A namespaced object goes to `<git.path>/<namespace>/<plural>/<name>.yaml`. A cluster-scoped object goes to `<git.path>/_cluster/<plural>/<name>.yaml`. `<plural>` is the plural resource name of the kind, as `kubectl api-resources` shows it. A file holds the object without `status` and without the fields that the API server sets. Each object carries the annotation `fleet.camunda.io/revision` with the SHA-256 of its content. A delete in the fleet manager removes the file of the object. Point your GitOps tool at the folder of `git.path`.

### Waiting for GitOps

After a save in Git mode, each page shows "Waiting for GitOps to apply" at the top, with one line for each pushed file. A line goes away when the object in the cluster carries the `fleet.camunda.io/revision` of the push. The list belongs to the browser tab, so a reload of the page clears it. If a line stays, make sure that your GitOps tool syncs the folder of `git.path`.

### Changes that go to the API in Git mode

One-off backups, restores, and backup deletions go to the Kubernetes API also in Git mode. The settings go to the API too, and the fleet manager never commits them. For Secrets, see [Secrets in Git mode](#secrets-in-git-mode).

### When a save fails

Before the commit, the fleet manager sends each object to the cluster as a dry run. A save that the cluster refuses makes no commit. The dry run keeps each field that only your GitOps tool set, also when the saved object leaves the field out. A validation rule that depends on such a field can still fail when GitOps applies the file. The cluster cannot check an `ExternalSecret`, or an object in a namespace that the same save creates. The cluster checks these objects only when GitOps applies them.

When someone pushed to the branch after the fleet manager read it, the save commits on top of that push. The save fails only when that push changed a file of the save.

| Message | Cause | What to do |
| --- | --- | --- |
| The message of the API server, for example `releases.default must be one of releases.allowed` | The cluster refuses the object. | Correct the value and save again. |
| `The <kind> <name> changed since you opened it.` | Someone changed or deleted the object in the cluster after the page read it. | Open the page again. |
| `<file> in <repository> holds a save that GitOps has not applied yet. Wait until GitOps applies it, then open the page again. If GitOps cannot apply it, correct or revert <file> in the repository.` | An earlier save of the same object is in the repository, and GitOps has not applied it. Each save of the object fails until GitOps applies the file. | Wait until GitOps applies the earlier save, then open the page again. If GitOps cannot apply it, correct or revert the file in the repository. |
| `Someone changed <file> in <repository> since the fleet manager read it. Open the page again.` | Someone pushed a change to the same file. | Open the page again. |
| `<repository> changed again while the fleet manager pushed. Save again.` | The branch changed again during the save. | Save again. |
| `<repository> refused the credentials.` | The username or the token of the Secret is wrong, or the token cannot push. | Correct the Secret of `git.credentialsSecretRef`. |
| `<repository> refused the push to the branch <branch>.` | The Git server accepts the credentials, but the branch is protected, or a rule of the server refuses the push. | Let the user of the token push to the branch, or set `git.branch` to a branch that accepts a push. |
| `<repository> must start with https:// to use the credentials of the Secret <secret>.` | `git.credentialsSecretRef` is set, and `git.repository` is not an `https://` address. | Set `git.repository` to the `https://` address of the repository. |
| `The Secret <secret> does not exist in namespace <namespace>.` | The Secret of `git.credentialsSecretRef` is not in the release namespace. | Create the Secret, as [Git credentials](#git-credentials) shows. |
| `The Secret <secret> has no key <key>.` | The Secret has no key `username` or no key `token`, or the value is empty. | Add the key to the Secret. |
| `<repository> has no branch <branch>.` | The branch of `git.branch` does not exist. | Create the branch, or correct `git.branch`. |
| `<file> is not in the repository. GitOps does not manage <kind> <name>.` | A delete of an object that has no file in the repository. | Delete the object in the cluster with `kubectl delete`. |
| `<file> is behind a symbolic link in the repository. The fleet manager does not write through a link.` | A folder or the file on the path of the object is a symbolic link in the repository. | Replace the link with a real folder or file in the repository. |

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
