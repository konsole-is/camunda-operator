# Install the fleet manager

The fleet manager is a web app for the Camunda clusters of one Kubernetes cluster. Each version of the operator chart ships it as its own image. The chart does not install it by default.

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

Each `helm upgrade` on this page sets each value that it does not name back to the default of the chart. If your release has other values, add your values files and `--set` options to each command. Do not use `--reuse-values`. It keeps the defaults of the old chart, which do not have the values of the fleet manager.

The chart adds the Deployment and the Service `camunda-operator-fleet-manager` to the release namespace. Wait until the Deployment is ready:

```bash
kubectl -n camunda-operator-system rollout status deployment/camunda-operator-fleet-manager
```

These names are for the release `camunda-operator`. For a release whose name does not contain `camunda-operator`, the names start with `<release>-camunda-operator`. With the release `ops`, the Deployment is `ops-camunda-operator-fleet-manager`.

## Open it

The fleet manager has no Ingress and no login. Open a port to the Service:

```bash
kubectl -n camunda-operator-system port-forward service/camunda-operator-fleet-manager 3000:3000
```

Then open <http://localhost:3000> in a browser.

In this version, the Fleet page and the Setup page show only their title. This is expected, and the install works.

To set the cluster name, the Git repository, and the Prometheus address, see [Fleet manager settings](settings.md).

## Trust a private CA

If a private CA signs the certificate of your Git server or your Prometheus server, the fleet manager does not trust it by default. Give the fleet manager the CA bundle in a ConfigMap. The bundle is a PEM file, and it can hold more than one CA. Put the ConfigMap in the release namespace:

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

The fleet manager then trusts the CAs of the bundle in addition to the public CAs. It reads the bundle from the key `ca.crt`. If your ConfigMap uses another key, add `--set fleetManager.extraCACerts.key=<key>` to the command.

The fleet manager reads the bundle when its pod starts. After you change the ConfigMap, restart the Deployment:

```bash
kubectl -n camunda-operator-system rollout restart deployment/camunda-operator-fleet-manager
```

If the ConfigMap or its key does not exist, the pod does not start. `kubectl -n camunda-operator-system describe pod` shows the missing ConfigMap or key in the events of the pod.

## Who can reach it

!!! warning "Give `pods/portforward`, `pods/proxy`, and `services/proxy` only to the people who manage the fleet"
    A person who has one of these rights in the release namespace can reach the fleet manager and use every right of it. Do not count on the NetworkPolicy of the chart to stop this traffic. The fleet manager can create and change the claims, environments, presets, releases, platform configs, contracts, backups, restores, namespaces, and Secrets of the Kubernetes cluster.

The chart adds a NetworkPolicy that allows no traffic from other pods to the pod of the fleet manager. This includes the operator and your monitoring. Kubernetes adds up the traffic that each NetworkPolicy allows. So another policy that selects this pod, such as a policy that allows all traffic in the namespace, still lets other pods in. Make sure that no other NetworkPolicy of the release namespace selects the pod of the fleet manager. A NetworkPolicy takes effect only with a network plugin that enforces it. `kubectl port-forward` still works, because it does not go through the pod network.

## Size

The fleet manager requests `50m` CPU and `256Mi` memory, with a memory limit of `512Mi`. It needs more memory when the Kubernetes cluster holds more resources. If the pod restarts with `OOMKilled`, raise the limit:

```bash
helm upgrade camunda-operator \
  oci://ghcr.io/konsole-is/charts/camunda-operator \
  --version <version> \
  --namespace camunda-operator-system \
  --set fleetManager.enabled=true \
  --set fleetManager.resources.limits.memory=1Gi
```

## Turn it off

Upgrade the release with `fleetManager.enabled=false`:

```bash
helm upgrade camunda-operator \
  oci://ghcr.io/konsole-is/charts/camunda-operator \
  --version <version> \
  --namespace camunda-operator-system \
  --set fleetManager.enabled=false
```

The chart then removes the objects of the fleet manager. The operator and your Camunda clusters do not change.
