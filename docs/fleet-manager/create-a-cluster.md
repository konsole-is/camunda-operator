# Create a cluster

The fleet manager creates a Camunda cluster as a [CamundaClusterClaim](../crds/camundaclusterclaim.md). The operator then creates the `CamundaCluster` and the requests of the options from the claim.

## Open the form

Open **Fleet** and select **New cluster**. The form has five steps: **Environment and name**, **Storage and access**, **Cluster settings**, **Attachments**, and **Review**. **Next** stays off while this step or an earlier step has an error. The errors show at the bottom of the form, each with the name of its step. You can go back to an earlier step at any time. The fleet manager keeps what you entered.

While you fill in the form, it shows the changes in the Kubernetes API at once. A namespace, a preset, or a host that changes shows in the errors of its step. While a step has an error, **Create** on the review stays off.

## Environment and name

Pick the environment of the cluster. The environments show only when at least one `Environment` exists. An environment that is not ready shows its message, and you cannot pick it. When you pick another environment, the form clears the namespace and the picks of **Storage and access**.

**Name** is the name of the claim and of the cluster. Use lowercase letters, digits, and `-`, at most 40 characters. The name starts with a letter and ends with a letter or a digit. The step refuses a name that a claim or a cluster of the namespace already has. It then shows `The name <name> is taken in this namespace.` Under the name, the step shows the hosts that the cluster gets from its access option.

**Namespace** is a namespace of the environment, or a new one when the environment allows **Create a namespace**. A new namespace gets the labels of the environment and a value for each label key that the environment asks for. [Namespace creation](environments.md#namespace-creation) lists the rules.

**Preset** shows the cluster presets that the environment allows. Each preset shows its brokers, partitions, CPU, and memory, and the number of clusters that use it.

## Storage and access

The step shows one group for each section that the environment allows, for example **Secondary storage**, **Backup storage**, and **Access**. Each option of a section is a card with its label. The option with the **Default** label is the default of the environment, and the form picks it for you. Under a picked option, the step shows each parameter that the environment lets the claim set. A parameter that names a config object lists the objects of that kind in the namespace of the cluster. An example is **Name** of **Existing secondary storage**.

When another cluster or plane already uses a host of the access option, the step shows the conflict under the access cards. Go back to **Environment and name** and pick another name.

**When the cluster is deleted** says what happens to the resources of the options. **Keep the resources** is the deletion policy `Orphan`, and **Delete the resources** is `Delete`. [When a claim is deleted](environments.md#when-a-claim-is-deleted) explains the default of the environment.

## Cluster settings

Pick the **Release** and the **Platform config**. Both start at the default of the environment.

Under them, a note names the preset that you picked in **Environment and name**. Your admin made this preset. It sets each value that shows **From preset**. You can change such a value here. The change applies to this cluster only and does not change the preset.

The settings come in groups: one group for the whole cluster and one group for each process. Each group starts closed. Select the name of a group to open it or close it. The number next to the name is the count of settings that you changed in the claim. A setting counts as soon as you change it. A value that you typed stays when you close the group.

The first line of an open group says how many settings the group has and where their values come from, for example `14 settings: 3 from the preset, 11 not set`. A setting is `not set` when the preset, the release, and the claim give it no value.

When a setting of a group shows an error, for example a Secret that the namespace does not hold, the group opens. It opens again each time that one more of its settings shows an error. A red count next to the name then says how many settings have an error, for example `1 error`. This count stays when you close the group.

A setting also shows an error when the value that the cluster gets is not valid. Requests and limits merge key by key, so a request or a limit that you leave empty keeps the value of the preset. When a limit is then below its request, the setting shows an error that names the preset. An example is `The CPU limit of 1 comes from the preset and is below the CPU request of 2`. When the preset alone gives a limit below its request, the error names the preset for both values.

**Next** stays off while a setting shows such an error, also when you changed nothing. When the preset changes to such a value after you leave this step, **Create** stays off and names this step. To correct it, set a limit that is not below the request, or a lower request. For an embedded process, such as Tasklist, or for Connectors that are off, the error shows as a warning and does not stop **Next**. Correct it before the process runs on its own, because its resources apply only then.

A resource such as `ephemeral-storage` has no field in the setting. An edit of another amount of the same setting writes its values into the claim too. **Reset to preset** removes them from the claim, together with the other amounts that the claim sets there. When a value comes from the preset, pick another preset, or ask your admin to correct the preset.

Labels, annotations, and environment variables merge key by key or name by name. The claim can add an entry or change its value, but it cannot remove an entry of the preset. When the preset holds an entry that the setting does not accept, the setting shows an error that names the entry and the preset. An example is `The label key "bad key" comes from the preset`. The error stays when you remove the row, and **Next** stays off. Pick another preset, or ask your admin to correct the preset. When only the value of a label is not valid, you can also set a valid value for that key. For a label or an annotation of an embedded process, or for a variable of Connectors that are off, the error shows as a warning. The variables of an embedded process apply to its host, so their error stops **Next**.

A setting that an option of **Storage and access** fills shows locked, with the name of its section. A setting that you leave as it is stays out of the claim, so the preset and the release decide its value.

A setting that names a Secret, such as an OIDC client secret, lists the Secrets of the namespace of the cluster. A new namespace holds no Secret yet. [A Secret that the namespace does not hold](#a-secret-that-the-namespace-does-not-hold) says what the step shows then.

## Attachments

**Attach Optimize** adds a `CamundaOptimize` for the cluster. Optimize needs Elasticsearch as secondary storage and a `ManagementAuthConfig`. Without them, the checkbox is off and says which one is missing.

**Back up on a schedule** adds a `BackupSchedule` for the cluster. It needs a backup storage option. [Change a cluster](change-a-cluster.md#change-the-claim-in-the-claim-tab) describes the fields of both attachments.

## Review

The review shows what the create writes and what the operator creates from it:

- The claim as YAML, at the top of the review.
- One group for each earlier step, with the values that you picked.
- **The new namespace**: its YAML. It shows only when the cluster gets a new namespace.
- **Permissions the options ask for**: the permissions of the chosen options. [Permissions](#permissions) explains them.
- **What this change does**: the objects that the create makes. These are the namespace, the claim, one request for each option, and the `CamundaCluster`. The `CamundaOptimize` and the `BackupSchedule` show when you attach them.

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
```

The claim names `platformConfigRef` only when you picked a platform config that is not the default of the environment. It names `deletionPolicy` only when your choice is not the default of the environment.

The review asks the Kubernetes API to check the claim, and saves nothing. When the Kubernetes API refuses the claim, the review shows its message, and **Create** stays off. The Kubernetes API cannot check a claim in a namespace that does not exist yet. So when the cluster gets a new namespace, the review shows the claim unchecked, and the check comes at **Create**. [When the write is refused](#when-the-write-is-refused) says what happens then.

### Permissions

A plugin can ask for permissions in the cloud for its option, for example an IAM policy for a bucket. The review shows each permission template of each chosen option as the plugin renders it for this claim:

- A choice that you picked shows by its label, for example `<policy: SQS access>`.
- An output that does not exist yet shows as a placeholder, for example `<bucket ARN from Backup storage>`.
- An ARN that the admin set shows as `<label: Set by your admin>`, with the label of the parameter. An ARN that you entered shows as `<label: An ARN that you entered>`. The review never shows an ARN.

The review leaves out a template that needs the output of a section that the claim does not select. The plugin asks for no such permission. A template that waits for the output of another chosen section says `Only when <section> gives its output`. When a template of an option cannot render, each template of that option shows the error. The plugin then asks for none of the permissions of that option. This error does not turn **Create** off.

## Direct mode and Git mode

Without Git mode, the fleet manager writes to the Kubernetes API directly. Select **Create** to write the cluster. The fleet manager creates the new namespace first, then the claim. The page then shows the progress of the cluster and the message `Created the cluster <name>.`

In Git mode, the button reads **Push to Git**. The fleet manager commits the claim to the repository, and the new namespace too when the cluster needs one, in one commit. The message then shows `Waiting for GitOps to apply` and the first seven characters of the commit. The message changes to `Created the cluster <name>.` after your GitOps tool applies the claim with the revision of the push. [Git mode](settings.md#git-mode) says how to turn on Git mode and where the files go.

## When a step cannot read a list

The form reads the lists that it needs from the Kubernetes API, for example the environments, the options, and the presets. When the fleet manager cannot read a list, the step that needs it shows `The fleet manager could not read the <kind> objects:` and the error. **Next** stays off, because the step cannot check your picks against that list. On the review, the line reads `The fleet manager could not read the <kind> objects. The review can miss what the create makes, so Create stays off.`

The error goes away when the fleet manager can read the list again. If it stays, make sure that the fleet manager can reach the API server. Also make sure that its role can list that kind. Then open the form again.

## A Secret that the namespace does not hold

A setting that names a Secret needs that Secret, with its key, in the namespace of the cluster. When it is not there, the errors at the bottom of the form show an error with the path of the setting, for example:

```text
auth.clientSecretRef: The namespace team-b holds no Secret orders-oidc. Pick a Secret of team-b, or reset the field.
```

A setting of **Cluster settings** also shows the error under its editor, without the path, and its group opens. This happens when you pick another namespace after you picked the Secret. Pick a Secret of the new namespace, or reset the field. A new namespace holds no Secret, so the step asks you to reset the field. A reference that sets `optional: true` gets no error.

## When the write is refused

A refused write shows its message and keeps the form as it is. Correct the cause and select **Create** again.

When the fleet manager created the new namespace and the claim was refused, the namespace stays. The form then uses that namespace as an existing namespace and says so. The next **Create** writes only the claim.

Another claim with the same name can appear while the review is open. The form then says `A claim <name> exists in <namespace> already. This create did not make it.` Go back to **Environment and name** and pick another name.

The answer of a write can get lost, for example when the connection to the fleet manager drops. The write can then have landed or not. **Create** stays off, and the form shows **Try again**. **Try again** asks the Kubernetes API to check the claim again. When the claim of your write shows up meanwhile, the form shows **Follow the claim**, which opens its progress.

## Follow the creation

After the write, the page shows the progress of the cluster. On top, it shows the `Ready` condition of the claim and its message. Then it shows one row for each option. A row shows the name of the request, the state of its outputs and of its `Ready` condition, and its first summary line. The rows **Requests**, **Cluster**, and **Attachments** follow. Each shows the state of its condition of the claim and its message. The row **Cluster** shows the reason and the message of the `CamundaCluster`, for example `StorageAlreadyAttached: …`.

While an option waits for the output of another section, the message of the claim names that section. The page shows that message on top. `kubectl get camundaclusterclaim my-cluster -n my-cluster-ns -o yaml` shows it in the status. This part of the status holds the message and the requests:

```yaml
status:
  conditions:
    - type: RequestsReady
      status: "False"
      reason: Blocked
      message: waiting for the outputs of encryption-key
      observedGeneration: 1
    - type: Ready
      status: "False"
      reason: PrerequisiteNotMet
      message: 'cluster: Prerequisite not met: waiting for condition "RequestsReady" to become True (currently False: waiting for the outputs of encryption-key)'
      observedGeneration: 1
  requests:
    - name: my-cluster-encryption-key
      section: encryption-key
      registration: aws-kms-key
    - name: my-cluster-volume-encryption
      section: volume-encryption
      registration: aws-encrypted-volumes
```

When the operator refuses the claim, the page shows the reason and the message, for example `NotAllowed`. The [Status](../crds/camundaclusterclaim.md#status) section of the claim lists each reason and what to do. A claim that the operator refuses at creation gets no `CamundaCluster`. The cluster then has no page, and the line has no link to the Claim tab. Correct the cause, for example in the `Environment`. The claim then checks itself again. When the `CamundaCluster` exists, the line also has the link **Open the Claim tab**. When the cluster is ready, the link **Open the cluster page** shows under the rows.

## Related

- [Change a cluster](change-a-cluster.md)
- [The cluster page](cluster-page.md)
- [CamundaClusterClaim](../crds/camundaclusterclaim.md)
