# Presets and releases

A release says what a platform runs: the versions, the pinned images, and the environment variables. Clusters, Elasticsearch clusters, and database servers name a release in `spec.releaseRef`. A claim, which asks for a cluster, names one too. These resources are the users of the release. The page **Setup > Releases** of the fleet manager lists the releases and edits each one. The [CamundaRelease](../crds/camundarelease.md) reference lists the fields.

If the fleet manager cannot read its settings, the pages show `Changes are off: the fleet manager could not read its settings.` and accept no change. Reload the page when the settings can be read again.

## The Releases list

The list shows one row for each release. The columns are the name, the Camunda version, the Connectors version, the Elasticsearch version, the PostgreSQL version, **Default in**, **Allowed in**, and **Used by**. A dash means that the release does not set that version. Select a name to open the page of the release.

Each row states facts only. These are the four facts:

| Fact | Meaning |
| --- | --- |
| `Unused` | No cluster, Elasticsearch cluster, database server, or claim names the release. |
| `Default in <environments>` | These environments give the release to a new cluster that names no release. |
| `Allowed in <environments>` | A claim in these environments can name the release. |
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
