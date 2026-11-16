# Presets and releases

A preset holds the shared settings of the resources that name it. The page **Setup > Presets** of the fleet manager lists the presets of three kinds:

| Kind on the page | Preset | Used by |
| --- | --- | --- |
| Cluster | [CamundaClusterPreset](../crds/camundaclusterpreset.md) | `CamundaCluster` |
| Elasticsearch | [ElasticsearchClusterPreset](../crds/elasticsearchclusterpreset.md) | `ElasticsearchCluster` |
| PostgreSQL | [DatabaseServerPreset](../crds/databaseserverpreset.md) | `DatabaseServer` |

The page shows who uses each preset and how each field of it merges. It deletes a preset that nothing uses. It does not create a preset. Apply a new preset with `kubectl` or through your Git repository.

If the fleet manager cannot read its settings, the page shows `Changes are off: the fleet manager could not read its settings.` and offers no delete.

## The Presets page

The list has one row for each preset, sorted by kind and then by name. To show one kind, select **Cluster**, **Elasticsearch**, or **PostgreSQL** in the filter **Kind**. The columns are:

| Column | What it shows |
| --- | --- |
| Name | The name of the preset, with its description below it. |
| Kind | **Cluster**, **Elasticsearch**, or **PostgreSQL**. |
| Key facts | The size that the preset sets: brokers and partitions for a cluster, nodes for Elasticsearch, instances for PostgreSQL. Then the CPU and memory requests and the storage size. A fact that the preset does not set is not shown. |
| Allowed in | The [environments](environments.md#platform-config-presets-and-releases) that list the preset, or **Nowhere**. The operator refuses a claim that names a preset that its environment does not list. The column shows **Unknown** while the fleet manager cannot read the environments. |
| Used by | The number of users of each kind, for example `1 cluster, 1 claim`, or **Unused**. [Who uses a preset](#who-uses-a-preset) says what counts as a user. The column shows **Unknown** while the fleet manager cannot read a list of objects that can use the preset. |
| Last changed | The time of the newest change to the preset, in UTC to the minute. In Git mode, this is the time that GitOps applied the file. |

The description comes from the annotation `kubernetes.io/description` of the preset:

```yaml
apiVersion: core.camunda.io/v1
kind: CamundaClusterPreset
metadata:
  name: medium
  annotations:
    kubernetes.io/description: "For production workloads with three brokers and replication."
spec:
  # ... the rest of your preset
```

A preset without the annotation shows no description. Two presets of different kinds can have the same name, for example an Elasticsearch preset and a PostgreSQL preset both named `standard`. The column **Kind** tells them apart, and the path of the preset page holds the kind, for example `/setup/presets/postgresql/standard`.

If the fleet manager cannot read a kind, the page shows `Could not read <kind>: <message>` above the table. It also shows this line while the connection to the cluster is lost and the list of a kind can be out of date.

## Who uses a preset

The column **Used by** counts each object that names the preset:

- A `CamundaCluster`, `ElasticsearchCluster`, or `DatabaseServer` whose `spec.presetRef` names it.
- A [CamundaClusterClaim](../crds/camundaclusterclaim.md) whose `spec.presetRef` names a cluster preset.
- A claim that selects the built-in Elasticsearch or PostgreSQL option, with the preset in the parameter `preset`. This counts for a cluster claim and for a management plane claim.
- A [PluginRequest](../crds/pluginrequest.md) of the built-in Elasticsearch or PostgreSQL option, with the preset in the parameter `preset`.

A claim counts before its cluster or its storage exists. For example, this claim uses the cluster preset `medium` and the PostgreSQL preset `small`:

```yaml
apiVersion: fleet.camunda.io/v1alpha1
kind: CamundaClusterClaim
metadata:
  name: my-cluster
  namespace: my-cluster-ns
spec:
  presetRef: medium
  plugins:
    - section: secondary-storage
      registration: database-server
      parameters:
        preset: small
  # ... the rest of your claim
```

## Delete a preset

The page offers a delete only for a preset that it can delete safely. The check box of a preset is disabled in each of these cases:

- An object uses the preset (see [Who uses a preset](#who-uses-a-preset)).
- An environment allows the preset.
- The fleet manager cannot read the environments, or a list of the objects that can use a preset of that kind. The page shows `Could not read <kind>: <message>` for that list.

To delete presets from the list:

1. Select the check box of each preset to delete. Nothing is selected when the page opens. **Select all unused** selects every preset whose check box is enabled.
2. Select the delete button under the table. It reads **Delete the preset** and the name for one preset, or **Delete 3 presets** for three.
3. Read the review. The API server checks the delete first, without a change, and the review shows the result. It names each preset.
4. Select **Delete**.

If a preset of the review gets a user, or an environment allows it, the review closes before the delete starts. Nothing is deleted, and the check box of that preset is disabled.

In Git mode the button of the review reads **Push the delete to Git**. The delete is a commit that removes the file of each preset. GitOps must prune the presets to delete them from the cluster.

The operator does not stop a delete of a preset that is in use, for example by `kubectl delete`. A cluster that still names a deleted preset reports `Ready: False` with reason `InvalidReference`, and its workloads keep their last configuration. A claim that names a deleted preset also reports `InvalidReference`. See [CamundaClusterPreset: Deletion](../crds/camundaclusterpreset.md#deletion).

## The preset page

Select a preset in the list to open its page. The header shows the kind, the description, the key facts, the environments that allow it, and each user as `<namespace>/<name>`. The header shows **Unknown** for the environments or the users in the same cases as the list.

### Settings and merge rules

The page shows each field that the preset sets, with its value and its merge rule. The fields are in parts: **General**, one part for each process such as **Brokers** or **Connectors**, **Authentication**, and **Backup**. An Elasticsearch preset and a PostgreSQL preset have the part **General** only.

| Merge rule | Meaning |
| --- | --- |
| A resource value replaces it | A resource that sets the field replaces the preset value. A resource that does not set it inherits the preset value. |
| Replaces whole | A block or list that the resource sets replaces the preset block or list as a whole. |
| Merged by key | The entries of the map merge by key. The resource wins on a conflict. |
| Merged by name | The environment variables merge by name. The resource wins on a conflict. |
| Resource entries follow | The entries of the resource come after the entries of the preset. |
| Merged by field | The preset sets an empty block. The block is kept, and each field that a resource sets in it merges by its own rule. |
| Replaced with the client | The client id, the audience, and the client secret go together. A cluster that sets `auth.clientId` replaces all three values of the preset. |

The merge rules of each kind are in [CamundaClusterPreset: Merge rules](../crds/camundaclusterpreset.md#merge-rules), [ElasticsearchClusterPreset: Merge rules](../crds/elasticsearchclusterpreset.md#merge-rules), and [DatabaseServerPreset: Merge rules](../crds/databaseserverpreset.md#merge-rules).

### Fields a preset cannot set

The section **Fields a preset cannot set** lists the fields that each resource sets itself. The API server refuses a preset that sets one of them. A resource takes its version from its release, or sets it itself. The lists are:

| Kind | Fields a preset cannot set |
| --- | --- |
| Cluster | `platformConfigRef`, `presetRef`, `releaseRef`, `externalUrl`, `serviceAccount`, `storageRef`, `backupStorageRef`, `documentStorageRef`, `monitoring`, `suspend`, `pause`, `version`, `connectors.version`, `connectors.serviceAccount.name` |
| Elasticsearch | `presetRef`, `releaseRef`, `secondaryStorageConfig`, `suspend`, `version` |
| PostgreSQL | `presetRef`, `releaseRef`, `databaseServerConfig`, `suspend`, `version` |

### Inheriting resources

The section **Inheriting resources** lists each `CamundaCluster`, `ElasticsearchCluster`, or `DatabaseServer` whose `spec.presetRef` names the preset. Each row shows the environment of its namespace, the namespace, its own fields, and its `Ready` condition. A cluster row links to the cluster page.

An own field is a field that the resource sets itself, so its value does not come from the preset. A cluster made from a claim can also get a field from an option that the claim selects. Such a field shows **Filled by** and the label of the section of that option. For example, an option of the section **Volume encryption** sets the storage class, and the row shows `zeebe.storageClassName Filled by Volume encryption`. The fields that a preset cannot set are not listed as own fields.

### Delete from the preset page

The section **Delete the preset** says why a delete is not possible, or offers it. To delete the preset, type its name and select **Review the delete**. Then select **Delete the preset** in the review, or **Push the delete to Git** in Git mode. After the delete, the page goes back to the list.
