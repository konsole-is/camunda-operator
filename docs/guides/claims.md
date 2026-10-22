# Claims

A claim asks for a Camunda cluster under the policy of an environment. The [fleet manager](../fleet-manager/install.md) writes claims from its create flow. This guide writes one with `kubectl`, so you can see what the operator creates from it.

The kinds of this guide are an alpha API in the group `fleet.camunda.io/v1alpha1`. Their fields can change in a later release.

## Create a cluster from a claim

This part creates the cluster `my-cluster` in the namespace `my-cluster-ns`, with Elasticsearch that you already run as its secondary storage.

You need:

- The operator installed. See [Installation](../installation.md).
- A [CamundaClusterPreset](../crds/camundaclusterpreset.md) `small`, a [CamundaRelease](../crds/camundarelease.md) `camunda-8-9-0`, and a [CamundaPlatformConfig](../crds/camundaplatformconfig.md) `my-platform-config`.
- A [SecondaryStorageConfig](../crds/secondarystorageconfig.md) `my-storage-config` in the namespace `my-cluster-ns`. See [Secondary storage](secondary-storage.md).

1. Create an [Environment](../crds/environment.md) that allows the preset, the release, and the option `existing-secondary-storage`. The parameter policy lets a claim set the name of its `SecondaryStorageConfig`:

    ```yaml
    apiVersion: fleet.camunda.io/v1alpha1
    kind: Environment
    metadata:
      name: production
    spec:
      platformConfigRef: my-platform-config
      presets:
        camundaCluster:
          - small
      releases:
        allowed:
          - camunda-8-9-0
        default: camunda-8-9-0
      sections:
        secondary-storage:
          required: true
          default: existing-secondary-storage
          allowed:
            - existing-secondary-storage
      parameters:
        existing-secondary-storage:
          name:
            claimEditable: true
    ```

2. Put the namespace in the environment:

    ```bash
    kubectl label namespace my-cluster-ns fleet.camunda.io/environment=production
    ```

3. Apply the claim. It takes the release and the platform config from the environment:

    ```yaml
    apiVersion: fleet.camunda.io/v1alpha1
    kind: CamundaClusterClaim
    metadata:
      name: my-cluster
      namespace: my-cluster-ns
    spec:
      presetRef: small
      plugins:
        - section: secondary-storage
          registration: existing-secondary-storage
          parameters:
            name: my-storage-config
    ```

4. Wait until the claim is ready:

    ```bash
    kubectl wait --for=condition=Ready camundaclusterclaim/my-cluster -n my-cluster-ns --timeout=30m
    ```

    The claim is ready when its `CamundaCluster` is ready. That takes as long as the start of a cluster.

5. Read the status of the claim:

    ```bash
    kubectl get camundaclusterclaim my-cluster -n my-cluster-ns -o yaml
    ```

    ```yaml
    status:
      environment: production
      cluster:
        name: my-cluster
      requests:
        - name: my-cluster-secondary-storage
          section: secondary-storage
          registration: existing-secondary-storage
          outputsReady: "True"
          ready: "True"
    ```

The claim created two objects. The [PluginRequest](../crds/pluginrequest.md) `my-cluster-secondary-storage` asks the built-in option for the name of the `SecondaryStorageConfig`. The [CamundaCluster](../crds/camundacluster.md) `my-cluster` names it in `spec.storageRef`. Connect to the cluster as the [CamundaCluster](../crds/camundacluster.md) page describes.

If `Ready` stays `False`, its reason tells you what to correct. The [CamundaClusterClaim](../crds/camundaclusterclaim.md#status) page lists each reason and what to do.

## Delete a claim

The deletion policy of the claim decides what the plugins do with the resources that they created, such as buckets and keys. `Delete` deletes them. `Orphan` keeps them. The [CamundaClusterClaim](../crds/camundaclusterclaim.md#deletion) page says what the claim always deletes.

1. If you want to keep the resources of the plugins, set the policy `Orphan` before you delete the claim:

    ```yaml
    apiVersion: fleet.camunda.io/v1alpha1
    kind: CamundaClusterClaim
    metadata:
      name: my-cluster
      namespace: my-cluster-ns
    spec:
      deletionPolicy: Orphan
      # ... the rest of your claim
    ```

2. Delete the claim:

    ```bash
    kubectl delete camundaclusterclaim my-cluster -n my-cluster-ns --wait=false
    ```

3. Read the status of the claim while it waits for the plugins:

    ```bash
    kubectl get camundaclusterclaim my-cluster -n my-cluster-ns -o yaml
    ```

    ```yaml
    status:
      conditions:
        - type: Ready
          status: "False"
          reason: Deleting
          message: 'The claim is deleted after these objects are gone: PluginRequest my-cluster-secondary-storage'
          observedGeneration: 2
    ```

    The message names each object that the claim waits for. When the reason is `NotAllowed`, the claim deletes nothing. A selected registration does not support `Orphan`. Set `spec.deletionPolicy: Delete`, or select another registration.

4. Wait until the claim is gone:

    ```bash
    kubectl wait --for=delete camundaclusterclaim/my-cluster -n my-cluster-ns --timeout=30m
    ```

The claim deletes the `CamundaCluster` `my-cluster` with both policies. If the claim stays, read the status of the request that the message names:

```bash
kubectl get pluginrequest my-cluster-secondary-storage -n my-cluster-ns -o yaml
```
