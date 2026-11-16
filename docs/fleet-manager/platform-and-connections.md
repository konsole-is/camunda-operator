# Platform config and connections

A platform config holds the authentication, the license, and the image repositories of the clusters, management planes, and database servers that name it. The page **Setup > Platform** of the fleet manager lists each `CamundaPlatformConfig` and edits it. The [CamundaPlatformConfig](../crds/camundaplatformconfig.md) reference lists the fields.

The list shows each platform config with its authentication method, the number of its users, and its status. Select a platform config to open its page.

The fleet manager does not create a platform config. Apply one with `kubectl apply` or your GitOps tool, for example:

```yaml
apiVersion: core.camunda.io/v1
kind: CamundaPlatformConfig
metadata:
  name: camundaplatformconfig-sample
spec:
  auth:
    method: oidc
    oidc:
      issuerUrl: "https://login.example.com/realms/camunda"
      clientId: "camunda-orchestration"
      audience: "camunda-orchestration"
      clientSecretRef:
        name: "oidc-credentials"
        namespace: "camunda-system"
        key: "client-secret"
  licenseSecretRef:
    name: "camunda-license"
    namespace: "camunda-system"
    key: "license-key"
  images:
    camunda: "registry.example.com/camunda/camunda"
    optimize: "registry.example.com/camunda/optimize"
```

If the fleet manager cannot read its settings, the page shows `Changes are off: the fleet manager could not read its settings.` and accepts no change.

## Users

The section **Used by** of the page lists each user of the platform config. The column **Users** of the list counts the same users.

| User | When it uses the platform config |
| --- | --- |
| Cluster | Its `spec.platformConfigRef` names the platform config. |
| Management plane | Its `spec.platformConfigRef` names the platform config. |
| Database server | Its own `spec.platformConfigRef` names the platform config, or its preset names it and the server sets no other one. The page then shows the preset, as in `team-a/orders-db from the preset pg-standard`. |
| Claim | Its `spec.platformConfigRef` names the platform config. The cluster or management plane of the claim has the same namespace and name. It does not name the platform config yet, for example because it does not exist yet. When the cluster names the platform config too, the pair counts once, as the cluster. |
| Environment | Its `spec.platformConfigRef` names the platform config, so each claim of the environment without its own platform config takes it. The environment counts as one user. |

## Change a platform config

The page edits the authentication method, the OIDC fields, the management plane clients, the license Secret, and the image repositories. The management plane clients are the OIDC clients of Management Identity, Optimize, Web Modeler, and Console, as [The clients of the management plane](../crds/camundaplatformconfig.md#the-clients-of-the-management-plane) describes. The field **PostgreSQL** of the image repositories is the image of the database servers. A note above the form says what a change does:

`A change rolls the pods of every cluster and management plane that uses this config. A database server changes only when the postgres image changes.`

Each Secret reference names a namespace, a Secret, and a key. The page shows the names only, never a Secret value. A reference can name a Secret in any namespace.

When you change a field, a bar at the bottom of the page offers **Review**. The review shows the change and a dry run of it. Select **Save** to write it. In Git mode, the button reads **Push to Git**, and the page shows `Waiting for GitOps to apply` until your GitOps tool applies the commit. [Fleet manager settings](settings.md#git-mode) says how to turn on Git mode.

To turn off OIDC, select **Basic**. The save sends the basic method and no OIDC block. When another tool, such as `kubectl apply` or your GitOps tool, set the OIDC block, the Kubernetes API keeps that block. A platform config with the basic method and an OIDC block is not valid, so the dry run fails, and the review shows the error. Remove the OIDC block with the tool that set it, then save again.

When a referenced Secret or key is missing, the platform config is not ready, and the page shows the message of its `Ready` condition. The message starts with the spec path of the reference:

```yaml
status:
  conditions:
  - type: Ready
    status: "False"
    reason: MissingSecret
    message: "spec.licenseSecretRef: Secret camunda-system/camunda-license not found"
```

The Secret stays picked and shows as `camunda-license (not found)`. A key that the Secret does not hold shows the same way, as `license-key (not found)`. A save keeps the reference. Create the Secret with the named key, or pick another one. When the fleet manager cannot read the Secrets of a namespace, the picker shows the read error and marks no Secret or key as not found.

## Default images

The section **Image repositories** shows the default repository under each field. A field that you leave empty uses its default. The page shows repositories only, never a tag. The tag always comes from the version of the cluster, management plane, or database server that runs the image.

The defaults come from the images table of the operator, for each Camunda minor that the clusters and management planes of the platform config run. When they run more than one minor, each default shows the minors that it covers. For example, from Camunda 8.10 the Web Modeler images move to `camunda/hub` and `camunda/hub-websockets`:

```text
camunda/web-modeler-restapi for Camunda 8.9
camunda/hub for Camunda 8.10
```

A database server runs a PostgreSQL version, not a Camunda minor, so it adds no minor. When no cluster or management plane uses the platform config, the page shows the defaults of the newest minor of the images table. When the images table has no defaults for a minor that a user runs, a warning above the fields names that minor. The fields still show the defaults of the other minors.

## Delete a platform config

The page offers the delete only when nothing uses the platform config. To delete, type the name of the platform config and select **Review the delete**. The review says:

`Users of a deleted config report InvalidReference and keep running.`

Select **Delete the platform config** to delete it. In Git mode, the button reads **Push the delete to Git**, and the commit removes the file of the platform config. Your GitOps tool must prune to delete it from the cluster. Nothing uses the platform config when you delete it. The warning of the review is for a cluster or claim that names it later. For example, someone applies an old manifest after the delete. Such a user reports `InvalidReference`. The [Deletion](../crds/camundaplatformconfig.md#deletion) section of the reference says more.

The fleet manager must read every kind that can use a platform config to know that nothing uses it. For example, it needs permission to list database servers. When it cannot read one of these kinds, the page offers no delete, and an open delete review closes. The section **Used by** and the column **Users** of the list then say that the users are not known. The page names the kinds that it cannot read:

> Delete is possible when the fleet manager can read DatabaseServer.

The page also shows this line for a short time after it opens or connects again.
