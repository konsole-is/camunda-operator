# Install the fleet manager

The fleet manager is a web app for the Camunda clusters of one Kubernetes cluster. Each version of the operator chart ships it as its own image. The chart does not install it by default.

## Turn it on

Set `fleetManager.enabled=true` on the release of the operator:

```bash
helm upgrade --install camunda-operator \
  oci://ghcr.io/konsole-is/charts/camunda-operator \
  --version <version> \
  --namespace camunda-operator-system \
  --set fleetManager.enabled=true
```

If the release already has other values, give them again, or add `--reuse-values`. Without them, `helm upgrade` sets those values back to their defaults.

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

## Who can reach it

!!! warning "Give `pods/portforward` only to the people who manage the fleet"
    A person who can create `pods/portforward` in the release namespace can use every right of the fleet manager. The fleet manager can create and change the claims, environments, presets, releases, platform configs, contracts, backups, restores, namespaces, and Secrets of the Kubernetes cluster.

The chart adds a NetworkPolicy that refuses all traffic from other pods to the pod of the fleet manager. This includes the operator and your monitoring. A NetworkPolicy takes effect only with a network plugin that enforces it. `kubectl port-forward` still works, because it does not go through the pod network.

## Size

The fleet manager requests `50m` CPU and `256Mi` memory, with a memory limit of `512Mi`. It needs more memory when the Kubernetes cluster holds more resources. If the pod restarts with `OOMKilled`, raise the limit:

```bash
helm upgrade camunda-operator \
  oci://ghcr.io/konsole-is/charts/camunda-operator \
  --version <version> \
  --namespace camunda-operator-system \
  --reuse-values \
  --set fleetManager.resources.limits.memory=1Gi
```

## Turn it off

Upgrade the release with `fleetManager.enabled=false`:

```bash
helm upgrade camunda-operator \
  oci://ghcr.io/konsole-is/charts/camunda-operator \
  --version <version> \
  --namespace camunda-operator-system \
  --reuse-values \
  --set fleetManager.enabled=false
```

The chart then removes the objects of the fleet manager. The operator and your Camunda clusters do not change.
