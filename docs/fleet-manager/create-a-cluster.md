# Create a cluster

The create form asks for the environment, the preset, the release, and the storage and access options of a new cluster. It then writes the claim of the cluster, and the operator builds the cluster from it. [What the fleet manager writes](install.md#what-the-fleet-manager-writes) explains these words.

## Open the form

Open **Fleet** and select **New cluster**. The form has five steps: **Environment and name**, **Storage and access**, **Cluster settings**, **Attachments**, and **Review**.

**Next** stays off while a step has an error. The errors show at the bottom of the form, each with the name of its step. You can go back to an earlier step at any time, and the form keeps what you entered.

## Environment and name

1. Pick the **Environment** of the cluster. An environment that is not ready shows its message, and you cannot pick it.
2. Type the **Name**. Use lowercase letters, digits, and `-`, at most 40 characters. The name starts with a letter and ends with a letter or a digit. It must be new in the namespace.
3. Pick the **Namespace**: a namespace of the environment, or a new one when the environment allows **Create a namespace**. A new namespace gets the labels that the environment sets, as [Namespace creation](environments.md#namespace-creation) shows.
4. Pick the **Preset**. Each preset shows its brokers, partitions, CPU, and memory.

Under the name, the step shows the hosts that the cluster gets from its access option.

## Storage and access

The step has one group for each storage and access choice that the environment allows, for example **Secondary storage**, **Backup storage**, and **Access**. Each option is a card. The form picks the card with the **Default** label for you. Under the picked card, the step shows the parameters that you can set.

If another cluster already uses a host of the access option, the step says so. Go back and pick another name.

**When the cluster is deleted** says what happens to the resources of these options, for example a bucket. Pick **Keep the resources** or **Delete the resources**. The default comes from the environment.

## Cluster settings

Pick the **Release** and the **Platform config**. Both start at the default of the environment.

Below them, the settings of the cluster come in groups: one for the whole cluster and one for each process, such as **Brokers**. Select the name of a group to open it. Each setting shows where its value comes from, for example **From preset**. A change applies to this cluster only, and does not change the preset. The number next to a group counts the settings that you changed.

A setting that an option fills, for example the storage class of an encrypted volume, shows locked with the name of its choice.

### Errors in the settings

A group with an error opens, and a red count next to its name says how many settings have an error. The most common error is a CPU or memory limit below its request. A request or a limit that you leave empty keeps the value of the preset, so the error can name the preset:

```text
The CPU limit of 1 comes from the preset and is below the CPU request of 2
```

Set a limit that is not below the request, or a lower request. If the preset itself holds a wrong value, pick another preset. You can also ask your admin to correct it.

### A Secret that the namespace does not hold

A setting that names a Secret, such as an OIDC client secret, lists the Secrets of the namespace of the cluster. If the Secret is not there, for example after you picked another namespace, the step shows an error:

```text
auth.clientSecretRef: The namespace team-b holds no Secret orders-oidc. Pick a Secret of team-b, or reset the field.
```

A new namespace holds no Secret yet, so reset the field.

## Attachments

- **Attach Optimize** adds Optimize to the cluster. Optimize needs Elasticsearch as secondary storage and a `ManagementAuthConfig`.
- **Back up on a schedule** adds a backup schedule. It needs a backup storage option.

A checkbox that is off says what is missing. [The Claim tab](change-a-cluster.md#change-the-claim-in-the-claim-tab) describes the fields of both.

## Review

The review shows the claim as YAML, the values of each step, and what the create makes:

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

The Kubernetes API checks the claim before you create it. If it refuses the claim, the review shows the message, and **Create** stays off.

**Permissions the options ask for** lists the cloud permissions of the options, for example an IAM policy for a bucket. A value that does not exist yet shows as a placeholder, such as `<bucket ARN from Backup storage>`.

## Create the cluster

Select **Create**. The page then shows the progress of the cluster, and `Created the cluster <name>.`

In Git mode, the button is **Push to Git**. The fleet manager commits the claim, and the new namespace when the cluster needs one. The page shows `Waiting for GitOps to apply` until your GitOps tool applies the commit. [Git mode](settings.md#git-mode) shows how to turn it on.

If the create fails, the form keeps your values and shows the message. Correct the cause and select **Create** again. A new namespace that the fleet manager already created stays, and the form uses it.

## Follow the creation

After **Create**, the page shows the `Ready` condition of the claim and its message. Then it shows one row for each option, and the rows **Requests**, **Cluster**, and **Attachments**. A request, a `PluginRequest`, is what the operator writes for each option so that the option builds its resources. When the cluster is ready, the link **Open the cluster page** shows.

While an option waits for another option, the message names the choice that it waits for.

If the operator refuses the claim, the page shows the reason, for example `NotAllowed`, and the message. Correct the cause, for example in the environment. The operator then checks the claim again. The [Status](../crds/camundaclusterclaim.md#status) section of `CamundaClusterClaim` lists each reason and what to do.

## Related

- [Change a cluster](change-a-cluster.md)
- [The cluster page](cluster-page.md)
- [CamundaClusterClaim](../crds/camundaclusterclaim.md)
