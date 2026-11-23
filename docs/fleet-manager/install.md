# Install the fleet manager

The fleet manager is a web app for the Camunda clusters of one Kubernetes cluster. You create, change, back up, and delete clusters in it. Each version of the operator chart ships it as its own image. The chart does not install it by default.

## What the fleet manager writes

The fleet manager does not write a `CamundaCluster` itself. When you create a cluster, it writes a **claim**. A claim is a request for one cluster. The operator reads the claim and builds the cluster from it.

A claim is a `CamundaClusterClaim` with the name of the cluster, in the namespace of the cluster. A management plane runs Management Identity, Console, and Web Modeler for many clusters. Its claim is a `CamundaManagementClusterClaim`. The claim holds what you chose in the create form:

- The **preset**: the shared size and settings of the cluster, such as the number of brokers, the CPU, and the memory. Your admin makes the presets.
- The **release**: the Camunda version and the images.
- The **platform config**, when you do not use the default: the sign-in method, the license, and the image repositories.
- One **option** for each storage and access choice, for example **Existing secondary storage** for **Secondary storage**. An option is a `PluginRegistration`, also called a registration. The operator brings the built-in options. A plugin is a Helm chart that adds more options, for example for a cloud provider.
- The settings that you changed on top of the preset, for example two Tasklist replicas.

```yaml
apiVersion: fleet.camunda.io/v1alpha1
kind: CamundaClusterClaim
metadata:
  name: my-cluster
  namespace: my-cluster-ns
spec:
  presetRef: small
  releaseRef: camunda-8-9-0
  plugins:
    - section: secondary-storage
      registration: existing-secondary-storage
      parameters:
        name: my-cluster-storage
  cluster:
    tasklist:
      replicas: 2
```

The **environment** of the namespace sets the rules for its claims. It names the presets, the releases, the platform config, and the options that a claim can use. If a claim breaks these rules, the operator does not build the cluster, and the status of the claim says why. [Environments](environments.md) shows how to set the rules.

Each change that you make to a cluster in the fleet manager is a change of its claim. You see the claim on the **Claim** tab of the cluster page and in the review before each save. A cluster that you applied without a claim, for example with `kubectl`, shows **Not managed**. You can read such a cluster in the fleet manager, but you cannot change it there.

In Git mode, the fleet manager commits the claim to a Git repository, and your GitOps tool applies it. [Git mode](settings.md#git-mode) shows how to turn it on.

[Claims](../guides/claims.md) writes a claim with `kubectl` and shows what the operator builds from it. [CamundaClusterClaim](../crds/camundaclusterclaim.md) and [CamundaManagementClusterClaim](../crds/camundamanagementclusterclaim.md) list the fields.

## Turn it on

Set `fleetManager.enabled=true` on the release of the operator:

```bash
helm upgrade --install camunda-operator \
  oci://ghcr.io/konsole-is/charts/camunda-operator \
  --version <version> \
  --namespace camunda-operator-system \
  --create-namespace \
  --set fleetManager.enabled=true
```

Each `helm upgrade` on this page sets the values that it does not name back to the chart defaults. If your release has other values, add your values files and `--set` options to each command. Do not use `--reuse-values`, because the old values do not hold the values of the fleet manager.

Wait until the Deployment is ready:

```bash
kubectl -n camunda-operator-system rollout status deployment/camunda-operator-fleet-manager
```

These names are for the release `camunda-operator`. For a release whose name does not contain `camunda-operator`, the names start with `<release>-camunda-operator`.

## Open it

The fleet manager has no Ingress and no login. Open a port to the Service:

```bash
kubectl -n camunda-operator-system port-forward service/camunda-operator-fleet-manager 3000:3000
```

Then open <http://localhost:3000> in a browser.

The **Fleet** page lists the clusters, as [The fleet view](fleet-view.md) shows. The **Setup** page has these pages at the side:

| Page | Guide |
| --- | --- |
| **Environments** | [Environments](environments.md) |
| **Presets** and **Releases** | [Presets and releases](presets-and-releases.md) |
| **Platform** and **Connections** | [Platform config and connections](platform-and-connections.md) |
| **Plugins** | [Registrations](registrations.md) |
| **Settings** | [Fleet manager settings](settings.md) |

The three buttons at the right of the top bar select the theme: **System**, **Light**, or **Dark**. The browser keeps the choice.

## Trust a private CA

A private CA can sign the certificate of your Git server or Prometheus server. Then give the CA bundle to the fleet manager. Put the PEM bundle in a ConfigMap in the release namespace:

```bash
kubectl -n camunda-operator-system create configmap fleet-manager-ca --from-file=ca.crt=<path to the PEM CA bundle>
```

Then name the ConfigMap in `fleetManager.extraCACerts.configMapName`:

```bash
helm upgrade camunda-operator \
  oci://ghcr.io/konsole-is/charts/camunda-operator \
  --version <version> \
  --namespace camunda-operator-system \
  --set fleetManager.enabled=true \
  --set fleetManager.extraCACerts.configMapName=fleet-manager-ca
```

The fleet manager reads the key `ca.crt`. For another key, add `--set fleetManager.extraCACerts.key=<key>`. The fleet manager reads the bundle when its pod starts, so restart the Deployment after you change the ConfigMap. If the ConfigMap or its key does not exist, the pod does not start.

## Who can reach it

!!! warning "Give `pods/portforward`, `pods/proxy`, and `services/proxy` only to the people who manage the fleet"
    A person who has one of these rights in the release namespace can reach the fleet manager and use every right of it. The fleet manager can create and change the claims, environments, presets, releases, platform configs, contracts, backups, restores, namespaces, and Secrets of the Kubernetes cluster.

The chart adds a NetworkPolicy that allows no traffic from other pods to the fleet manager. Another NetworkPolicy that selects this pod, such as one that allows all traffic in the namespace, lets other pods in again. Make sure that no other NetworkPolicy of the release namespace selects the pod of the fleet manager.

## Size

The fleet manager requests `50m` CPU and `256Mi` memory, with a memory limit of `512Mi`. It needs more memory when the Kubernetes cluster holds more resources. If the pod restarts with `OOMKilled`, raise the limit with `--set fleetManager.resources.limits.memory=1Gi`.

## Turn it off

Upgrade the release with `--set fleetManager.enabled=false`. The chart then removes the fleet manager. The operator and your Camunda clusters do not change.
