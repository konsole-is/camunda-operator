# Claims

A claim asks for a Camunda cluster or a management plane under the policy of an environment. The [fleet manager](../fleet-manager/install.md) writes claims from its create flow. This guide writes one with `kubectl`, so you can see what the operator creates from it.

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

## Create a management plane from a claim

This part creates the management plane `my-plane` in the namespace `my-plane-ns`. The plane serves the clusters of the environment `production`. Users sign in through the identity provider of the platform config. The plane uses a database that you already have.

You need:

- The `Environment` `production` of the first part, and the namespace `my-plane-ns` with the label `fleet.camunda.io/environment=production`.
- A [CamundaPlatformConfig](../crds/camundaplatformconfig.md) `my-platform-config` with an OIDC identity provider. See [Management plane](management-plane.md).
- A [DatabaseConfig](../crds/databaseconfig.md) `my-identity-db` in the namespace `my-plane-ns`. See [Management plane](management-plane.md#step-1-the-databases).

1. Add the sections of the plane to the `Environment`. The parameter policy lets a claim name its `DatabaseConfig`:

    ```yaml
    apiVersion: fleet.camunda.io/v1alpha1
    kind: Environment
    metadata:
      name: production
    spec:
      sections:
        management-database:
          allowed:
            - existing-database-configs
      parameters:
        existing-database-configs:
          identity:
            claimEditable: true
      # ... the rest of your environment
    ```

2. Apply the claim. It takes the platform config from the environment. `clusterSelector: {}` selects every cluster in the served namespaces. Without an option of `access`, the claim sets the URL of Management Identity itself:

    ```yaml
    apiVersion: fleet.camunda.io/v1alpha1
    kind: CamundaManagementClusterClaim
    metadata:
      name: my-plane
      namespace: my-plane-ns
    spec:
      environments:
        - production
      clusterSelector: {}
      management:
        identityProvider:
          oidc: {}
        identity:
          externalUrl: https://plane.example.com/identity
          admin:
            claimName: groups
            claimValue: camunda-admins
      versions:
        identity: 8.9.10
      plugins:
        - section: management-database
          registration: existing-database-configs
          parameters:
            identity: my-identity-db
    ```

3. Wait until the claim is ready:

    ```bash
    kubectl wait --for=condition=Ready camundamanagementclusterclaim/my-plane -n my-plane-ns --timeout=30m
    ```

4. Read the status of the claim:

    ```bash
    kubectl get camundamanagementclusterclaim my-plane -n my-plane-ns -o yaml
    ```

    ```yaml
    status:
      environment: production
      managementCluster:
        name: my-plane
      requests:
        - name: my-plane-management-database
          section: management-database
          registration: existing-database-configs
          outputsReady: "True"
          ready: "True"
    ```

The claim created the [CamundaManagementCluster](../crds/camundamanagementcluster.md) `my-plane`. It serves the clusters in the namespaces with the label `fleet.camunda.io/environment=production`, and its Management Identity uses `my-identity-db`. If `Ready` stays `False`, the [CamundaManagementClusterClaim](../crds/camundamanagementclusterclaim.md#status) page lists each reason and what to do.

## Change a cluster claim

This part changes the claim `my-cluster` of [Create a cluster from a claim](#create-a-cluster-from-a-claim). The claim sends each change to its `CamundaCluster`.

1. To move the cluster to a new release, add the release to `spec.releases.allowed` of the environment first. Then set the release on the claim:

    ```bash
    kubectl patch camundaclusterclaim my-cluster -n my-cluster-ns --type merge -p '{"spec":{"releaseRef":"camunda-8-9-1"}}'
    ```

2. Make sure that the cluster got the release:

    ```bash
    kubectl get camundacluster my-cluster -n my-cluster-ns -o jsonpath='{.spec.releaseRef}'
    ```

    If the environment does not allow the release, `Ready` of the claim is `False` with reason `NotAllowed`, and the cluster keeps its release.

3. To stop the cluster and keep its data, suspend the claim:

    ```bash
    kubectl patch camundaclusterclaim my-cluster -n my-cluster-ns --type merge -p '{"spec":{"suspend":true}}'
    ```

4. Make sure that the cluster is suspended. The command prints `true`:

    ```bash
    kubectl get camundacluster my-cluster -n my-cluster-ns -o jsonpath='{.spec.suspend}'
    ```

5. To start the cluster again, resume the claim:

    ```bash
    kubectl patch camundaclusterclaim my-cluster -n my-cluster-ns --type merge -p '{"spec":{"suspend":false}}'
    ```

The [CamundaClusterClaim](../crds/camundaclusterclaim.md) page describes [Suspend](../crds/camundaclusterclaim.md#suspend) and a move to an older release in [Downgrade](../crds/camundaclusterclaim.md#downgrade).

## Delete a claim

The deletion policy of the claim decides what the plugins do with the resources that they created, such as buckets and keys. `Delete` deletes them. `Orphan` keeps them. The [CamundaClusterClaim](../crds/camundaclusterclaim.md#deletion) and [CamundaManagementClusterClaim](../crds/camundamanagementclusterclaim.md#deletion) pages say what each claim always deletes. The steps below delete a cluster claim. For a plane claim, use `camundamanagementclusterclaim` and its names.

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

    The message names each object that the claim waits for. When the reason is `NotAllowed`, the claim deletes nothing. A selected registration does not support `Orphan`. To continue, set `spec.deletionPolicy: Delete`. The plugins then delete their resources.

4. Wait until the claim is gone:

    ```bash
    kubectl wait --for=delete camundaclusterclaim/my-cluster -n my-cluster-ns --timeout=30m
    ```

The claim deletes the `CamundaCluster` `my-cluster` with both policies. If the claim stays, read the status of the request that the message names:

```bash
kubectl get pluginrequest my-cluster-secondary-storage -n my-cluster-ns -o yaml
```
