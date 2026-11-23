# Install the fleet manager

The fleet manager is a web app for the Camunda clusters of one Kubernetes cluster. You create, change, back up, and delete clusters in it. Each version of the operator chart ships it as its own image. The chart does not install it by default.

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
| **Plugins** | [Plugins](registrations.md) |
| **Settings** | [Fleet manager settings](settings.md) |

The three buttons at the right of the top bar select the theme: **System**, **Light**, or **Dark**. The browser keeps the choice.

## What the fleet manager writes

When you create a cluster, the fleet manager writes a **claim**, a [CamundaClusterClaim](../crds/camundaclusterclaim.md) with the name of the cluster. A claim is a request for a cluster. It names a preset, a release, the storage and access options that you picked, and your changes to the settings of the preset. The operator builds the cluster from the claim.

Each change that you make to the cluster in the fleet manager is a change of its claim. A cluster without a claim shows **Not managed**, and you can only read it here.

- A **preset** holds the shared size and settings of a cluster, and a **release** holds the Camunda version. See [Presets and releases](presets-and-releases.md).
- An **option** gives the cluster its storage or its access, for example **New Elasticsearch cluster**. See [Plugins](registrations.md).
- An **environment** sets the presets, the releases, and the options that its namespaces can use. See [Environments](environments.md).
- A **management plane** runs Management Identity, Console, and Web Modeler for many clusters. See [Management planes](management-planes.md).

## Trust a private CA

If a private CA signs your Git or Prometheus server certificate, give the CA bundle to the fleet manager. Put the PEM bundle in a ConfigMap in the release namespace:

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

The fleet manager reads the key `ca.crt`. For another key, add `--set fleetManager.extraCACerts.key=<key>`. If the ConfigMap or its key does not exist, the pod does not start. The fleet manager reads the bundle when its pod starts, so restart it after you change the ConfigMap:

```bash
kubectl -n camunda-operator-system rollout restart deployment/camunda-operator-fleet-manager
```

## Who can reach it

!!! warning "Give `pods/portforward`, `pods/proxy`, and `services/proxy` only to the people who manage the fleet"
    A person who has one of these rights in the release namespace can reach the fleet manager and use every right of it. The fleet manager can create and change the claims, environments, presets, releases, platform configs, contracts, backups, restores, namespaces, and Secrets of the Kubernetes cluster.

The chart adds a NetworkPolicy that allows no traffic from other pods to the fleet manager. Another NetworkPolicy that selects this pod, such as one that allows all traffic in the namespace, lets other pods in again. Make sure that no other NetworkPolicy of the release namespace selects the pod of the fleet manager.

## Size

The fleet manager requests `50m` CPU and `256Mi` memory, with a memory limit of `512Mi`. It needs more memory when the Kubernetes cluster holds more resources. If the pod restarts with `OOMKilled`, raise the limit with `--set fleetManager.resources.limits.memory=1Gi`.

## Turn it off

Upgrade the release with `--set fleetManager.enabled=false`. The chart then removes the fleet manager. The operator and your Camunda clusters do not change.
