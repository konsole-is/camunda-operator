# Presets and releases

A preset holds the shared settings of the resources that name it. A release says what a platform runs: the versions, the pinned images, and the environment variables. The pages **Setup > Presets** and **Setup > Releases** of the fleet manager show them.

If the fleet manager cannot read its settings, the pages show `Changes are off: the fleet manager could not read its settings.` and accept no change. Reload the page when the settings can be read again.

## Presets

The page **Setup > Presets** of the fleet manager lists the presets of three kinds:

| Kind on the page | Preset | Used by |
| --- | --- | --- |
| Cluster | [CamundaClusterPreset](../crds/camundaclusterpreset.md) | `CamundaCluster` |
| Elasticsearch | [ElasticsearchClusterPreset](../crds/elasticsearchclusterpreset.md) | `ElasticsearchCluster` |
| PostgreSQL | [DatabaseServerPreset](../crds/databaseserverpreset.md) | `DatabaseServer` |

The page shows who uses each preset and how each field of it merges. It edits a preset, with a preview of the resources whose pods roll. It deletes a preset that nothing uses. It does not create a preset. Apply a new preset with `kubectl` or through your Git repository.

## The Presets page

The list has one row for each preset, sorted by kind and then by name. To show one kind, select **Cluster**, **Elasticsearch**, or **PostgreSQL** in the filter **Kind**. The columns are:

| Column | What it shows |
| --- | --- |
| Name | The name of the preset, with its description below it. |
| Kind | **Cluster**, **Elasticsearch**, or **PostgreSQL**. |
| Key facts | The size that the preset sets: brokers and partitions for a cluster, nodes for Elasticsearch, instances for PostgreSQL. Then the CPU and memory requests and the storage size. A fact that the preset does not set is not shown. |
| Allowed in | The [environments](environments.md#platform-config-presets-and-releases) that list the preset, or **Nowhere**. The operator refuses a claim that names a preset that its environment does not list. The column shows **Unknown** while the fleet manager cannot read the environments. |
| Used by | The number of users of each kind, for example `1 cluster, 1 claim`, or **Unused**. [Who uses a preset](#who-uses-a-preset) says what counts as a user. The column shows **Unknown** while the fleet manager cannot read a list that it needs to find the users. [Delete a preset](#delete-a-preset) names these lists. |
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

If the fleet manager cannot read a kind, the page shows `Could not read <kind>: <message>` above the table.

The users of a preset count only for the preset of their kind. A PostgreSQL preset `standard` that is in use does not stop the delete of an unused Elasticsearch preset `standard`.

## Who uses a preset

The column **Used by** counts each object that names the preset:

- A `CamundaCluster`, `ElasticsearchCluster`, or `DatabaseServer` whose `spec.presetRef` names it.
- A [CamundaClusterClaim](../crds/camundaclusterclaim.md) whose `spec.presetRef` names a cluster preset.
- A claim that selects the built-in Elasticsearch or PostgreSQL option, with the preset in the parameter `preset`. The claim can set the parameter, or its environment or the option can give it as a default. This counts for a cluster claim and for a management plane claim.
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
- The fleet manager cannot read a list that it needs for this check. The page shows `Could not read <kind>: <message>` for that list. The lists are:
    - For a cluster preset: `Environment`, `CamundaCluster`, and `CamundaClusterClaim`.
    - For an Elasticsearch or PostgreSQL preset: `Environment`, `Namespace`, `PluginRegistration`, `ElasticsearchCluster` or `DatabaseServer`, `CamundaClusterClaim`, `CamundaManagementClusterClaim`, and `PluginRequest`.
- The fleet manager does not have the current version of such a list yet. This occurs just after the page opens and after a lost connection.

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

### Change a preset

The page edits the preset. The form starts with the **Description**, which is the annotation `kubernetes.io/description`. A blank description removes the annotation.

A cluster preset has two sections:

- **Settings** has one group of field editors for **General** and one for each process, such as **Brokers** or **Connectors**. A field that the preset sets shows **From preset**. Select **Remove from the preset** to take the field out of the preset. The fields that a preset cannot set have no editor.
- **Environment variables** has the same groups as a release: **Every workload** and one for each process. See [Environment variables](#environment-variables).

An Elasticsearch preset and a PostgreSQL preset have one YAML editor, labelled `spec.cluster` or `spec.server`. The editor holds the body of the preset. While the YAML does not parse, the editor shows the error and **Review** stays disabled. The review checks each field against the CRD. A misspelled field gets an error from the API server, and the preset does not change.

To change a field of a cluster preset, for example the brokers of `medium`:

1. Open the page of the preset from the list.
2. In **Settings**, go to the group **Brokers** and type the new value in `replicas`. A field that the preset does not set yet has an empty editor in its group.
3. Select **Review** in the bar at the bottom of the page. The bar counts the unsaved changes. **Discard** starts again.
4. Read the diff and the impact on the inheriting resources. See [The impact of a change](#the-impact-of-a-change).
5. Select **Save**, or **Push to Git** in Git mode.

The Secret lists hold the Secrets of the namespaces of the users of the preset. A preset without a user takes the namespaces of the environments that allow it. A Secret key that one of these namespaces does not hold shows as missing. An environment variable of a cluster preset shows `Missing in <namespaces>` on its row. Every other key shows `<name>/<key> is missing in <namespaces>.` under the form, for example a key in the YAML of an Elasticsearch preset. Create the Secret in each of these namespaces before a resource there uses the preset. The fleet manager reads the Secret lists when the page opens, so reload the page after you create a Secret. If the fleet manager cannot read a list that it needs to find these namespaces, the page names the list. It then shows no Secret list and no missing key. Reload the page when it can read the list.

If you clear a field that a tool other than the fleet manager set, for example `kubectl apply`, the field stays. The review names it: `<field> stays, because another field manager set it.` Remove the field with the tool that set it.

### The impact of a change

The review lists each resource that inherits the preset, as `<namespace>/<name>`, with one mark:

| Mark | Meaning |
| --- | --- |
| `Pods roll` | The change rolls the workloads of the cluster. A change of a cluster preset rolls every cluster that inherits it, also a cluster that sets the changed field itself. The fields of `Connectors restart` and `No roll` are the only exceptions. |
| `Connectors restart` | The change reaches the cluster and restarts only its Connectors pods: a new `auth.basic.passwordRotation`, or a change of a field under `connectors.serviceAccount`. |
| `No roll` | The change reaches the cluster but restarts no pods: a change of `auth.basic.adminEmail`. |
| `Unchanged` | The change does not reach the resource. For a cluster, this occurs only when the change touches no field of `spec.cluster` other than the fields of `Connectors restart` and `No roll`. A change of the description only is an example. A cluster that sets its own `auth.basic` does not take a new `passwordRotation` of the preset. |
| `Pods may roll` | The merged spec of an Elasticsearch cluster or a database server changes. The fleet manager cannot tell which of these changes roll pods. A cluster never gets this mark. |

The API server checks the change before the review shows it, and the preset does not change during this check. The marks come from the preset as this check returns it. A field that stays, because another field manager set it, does not count as a change.

Under the marks, the review shows a note for each rule that the change touches:

- `The brokers restart one at a time. The other processes restart.` when a cluster rolls.
- `Connectors pods restart.` when a new password rotation or a change under `connectors.serviceAccount` reaches a cluster.
- A note about a smaller storage size, and for a cluster or an Elasticsearch preset a note about a new storage class. A resource that already has larger volumes keeps them, and a resource with existing volumes keeps its storage class.
- For a PostgreSQL preset, a note about a cleared `walStorageSize`. A running server keeps its write-ahead log volume.

The rules of each kind are in [CamundaClusterPreset: Changes](../crds/camundaclusterpreset.md#changes), [ElasticsearchClusterPreset: Changes](../crds/elasticsearchclusterpreset.md#changes), and [DatabaseServerPreset: Changes](../crds/databaseserverpreset.md#changes).

The marks need the list of the resources of the preset kind, and for a cluster preset the list of releases. If the fleet manager cannot read one of these lists, the review shows no mark and **Save** stays disabled. The same applies until the fleet manager has the current version of each list. The review names the lists:

> The impact is not known while the fleet manager cannot read CamundaCluster. Save is possible when it can read them.

### Settings and merge rules

Each field that the preset sets has a merge rule. For a cluster preset, the rules show under each group of **Settings**. For an Elasticsearch or a PostgreSQL preset, the page shows each field with its value and its rule under the YAML editor.

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

The section **Inheriting resources** lists each `CamundaCluster`, `ElasticsearchCluster`, or `DatabaseServer` whose `spec.presetRef` names the preset. Each row shows the environment of its namespace, the namespace, its own fields, and its `Ready` condition. A cluster row links to the cluster page. If the fleet manager cannot read the list of that kind, the section shows no row and names the list.

An own field is a field that the resource sets itself, so its value does not come from the preset. A cluster made from a claim can also get a field from an option that the claim selects. Such a field shows **Filled by** and the label of the section of that option. For example, an option of the section **Volume encryption** sets the storage class, and the row shows `zeebe.storageClassName Filled by Volume encryption`. The fields that a preset cannot set are not listed as own fields.

### Delete from the preset page

The section **Delete the preset** says why a delete is not possible, or offers it. To delete the preset, type its name and select **Review the delete**. Then select **Delete the preset** in the review, or **Push the delete to Git** in Git mode. After the delete, the page goes back to the list.

### Rotate the admin password

The page of a cluster preset has the section **Admin password**. Its button rotates the admin password of each cluster that takes the rotation from the preset, for example **Rotate the admin password of 3 clusters**. A cluster counts when its platform config uses basic authentication and it does not set its own `auth.basic`. A cluster whose platform config uses OIDC does not count.

The rotation writes the UTC date and time to the minute into the preset:

```yaml
apiVersion: core.camunda.io/v1
kind: CamundaClusterPreset
metadata:
  name: medium
spec:
  cluster:
    auth:
      basic:
        passwordRotation: "2026-11-15T10:42Z"
    # ... the rest of your preset
```

The review names each cluster whose password changes and says that its Connectors pods restart. Select **Rotate**, or **Push to Git** in Git mode. The operator never rotates the same value twice. A second rotation in the same minute writes nothing, so **Rotate** stays disabled. Each cluster shows the value in `status.adminPassword.rotation` when its rotation is complete. See [Authentication: Rotate the password](../guides/authentication.md#rotate-the-password).

The button is disabled while the form has unsaved changes. Save or discard them first. While the fleet manager cannot read the clusters or the platform configs, the section offers no rotation and names those lists.

## Releases

Clusters, Elasticsearch clusters, and database servers name a release in `spec.releaseRef`. A claim, which asks for a cluster, names one too. These resources are the users of the release. The page **Setup > Releases** of the fleet manager lists the releases and edits each one. The [CamundaRelease](../crds/camundarelease.md) reference lists the fields.

## The Releases list

The list shows one row for each release. The columns are the name, the Camunda version, the Connectors version, the Elasticsearch version, the PostgreSQL version, **Default in**, **Allowed in**, and **Used by**. A dash means that the release does not set that version. Select a name to open the page of the release.

Each row states facts only. These are the four facts:

| Fact | Meaning |
| --- | --- |
| `Unused` | No cluster, Elasticsearch cluster, database server, or claim names the release. |
| `Default in <environments>` | These environments give the release to a new cluster that names no release. |
| `Allowed in <environments>` | A claim in these environments can name the release. The default release of an environment is always one of its allowed releases. |
| `Not offered to new clusters` | No environment allows the release, so no new claim can name it. |

To offer a release to new clusters, add it to the allowed releases of an environment on the [Environments](environments.md) page.

**Used by** counts the clusters, the Elasticsearch clusters, and the database servers that name the release. A claim names a release before its cluster exists. When only claims name the release, **Used by** counts the claims, such as `1 claim`. A release that a claim names is not unused.

An environment shows by its display name, or by its name when it has none.

Two filters narrow the list. **Environment** shows the releases that an environment allows or has as its default. **Usage** shows the used or the unused releases. The search box matches the name.

## Delete a release

You can delete a release only when it is unused and no environment allows it. The list shows a checkbox only on such a row. **Select all that can be deleted** selects every such row that the search and the filters show. Then select **Delete 1 release**, or **Delete 2 releases** for more. The review lists the releases, and **Delete** removes them in one change.

If a claim or an environment names a selected release while the review is open, the review closes. Start the delete again.

The fleet manager must read every list of the page to know that a release is unused. When it cannot read one, no release can be deleted and an open delete review closes. The page then names the kinds that it cannot read:

> Delete is possible when the fleet manager can read CamundaClusterClaim.

The page also shows this line for a short time after it opens or connects again.

On the page of a release, the section **Delete the release** offers the same delete. Type the name of the release to confirm it.

In Git mode, the delete is a commit that removes the files of the releases. GitOps must prune the `CamundaRelease` objects to delete them from the cluster.

## New release

A new release usually starts as a copy of another one. Select **New release** on the list, or **Copy this release** on the page of a release. In **Copy from**, pick the release to copy. The form takes its versions, images, and environment variables. Type the **Release name** and change what differs, then select **Review** and **Create**. A change of **Copy from** replaces the form and keeps the name. The page of the new release opens when the list shows it.

A new release has no user, and no environment allows it yet, so the list shows it as `Unused` and `Not offered to new clusters`.

## Release settings

The **Settings** tab of a release page edits the release. The fragment below holds each field that the tab edits:

```yaml
apiVersion: core.camunda.io/v1
kind: CamundaRelease
metadata:
  name: camunda-8-9-4
spec:
  version: "8.9.4"
  connectors:
    version: "8.9.7"
  elasticsearch:
    version: "9.2.8"
  databaseServer:
    version: "17"
  images:
    camunda: "mirror.example.com/camunda/camunda:8.9.4-patched"
  extraEnv:
    - name: CAMUNDA_SOME_NEW_FLAG
      value: "true"
    - name: ORDERS_DB_PASSWORD
      valueFrom:
        secretKeyRef:
          name: orders-db
          key: password
  zeebe:
    extraEnvFrom:
      - configMapRef:
          name: zeebe-tuning
```

### Versions

**Camunda version** and **Connectors version** take a version such as `8.9.4`. **Elasticsearch version** takes a version such as `9.2.8`. **PostgreSQL version** takes a major version such as `17`. Only the Camunda version is required. If the Connectors version is empty, each cluster sets its own connectors version.

### Images

The **Images** section shows each image of the release with two values:

- **Pinned by the release** is the image that the release pins. A release can pin only the Camunda image and the Connectors image. Leave the field empty to pin nothing.
- **Default for the version** is the image that a cluster pulls when the release pins none. The tag of the Camunda and the Optimize image is the Camunda version of the release. The tag of the Connectors image is the Connectors version of the release. The other tags come from the images table that ships with the fleet manager.

The repository is the one that the images table gives. The platform config of each cluster sets the repository that the cluster pulls from, as [CamundaPlatformConfig](../crds/camundaplatformconfig.md#images) describes. If the images table has no entry for the minor of the release, the section says so and offers only the two pinned images.

### Environment variables

The **Environment variables** section has one group for each place in the spec. **Every workload** is the top level. **Connectors**, **Brokers**, **Gateway**, **Operate**, **Tasklist**, and **Admin** are the blocks `connectors`, `zeebe`, `gateway`, `operate`, `tasklist`, and `admin`. A group with entries is open when the page loads. The number next to the name of a group is its count of entries.

Each variable is a row with a name and a source. The source is a value, or a key of a Secret. The form never shows the value of a Secret. Only `extraEnvFrom` is YAML.

The Secret lists hold the Secrets of the namespaces of the users of the release. A namespace holds a user when a cluster, an Elasticsearch cluster, a database server, or a claim in it names the release. A release without a user takes the namespaces of the environments that allow it. When some of these namespaces do not hold a picked Secret key, the row shows `Missing in <namespaces>`. Create the Secret in each of these namespaces before a resource there uses the release. The fleet manager reads the Secret lists when the page opens.

## What a change does

Above the save, the page shows this notice:

> A change reaches every resource that uses this release. Their pods roll. A version change moves each user that does not set its own version. The Rollout tab moves clusters one by one.

Every cluster, Elasticsearch cluster, and database server that names the release takes the change, and its pods roll. A resource that sets its own version keeps it. The review of a save names the users and each version that changes. A version line names the kind of resource that runs the version. Clusters run the Camunda and Connectors versions. Elasticsearch clusters run the Elasticsearch version, and database servers run the PostgreSQL version. The rules about a version that a resource refuses are in the [CamundaRelease](../crds/camundarelease.md#versions) reference.

## Environments of a release

The **Environments** card of a release page shows each environment with its state for the release:

| State | Meaning |
| --- | --- |
| Default | A new cluster in the environment that names no release gets this release. |
| Allowed | A claim in the environment can name this release. |
| Not allowed | A claim in the environment cannot name this release. |

Select an environment to open its page and change its releases.
