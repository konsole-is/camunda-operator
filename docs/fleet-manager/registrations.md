# Registrations

A claim asks for a Camunda cluster or a management plane. A registration is one option that a claim can pick in a section, for example **Existing bucket** in the section **Backup storage**. The operator writes the built-in registrations, and each plugin chart writes its own. The page **Setup > Plugins** of the fleet manager lists every registration of the cluster with its status, and the official charts that are not installed. The [PluginRegistration](../crds/pluginregistration.md) reference lists the fields.

The page is read only. You install, upgrade, and remove a chart with Helm. An environment sets the options that its claims can use, and the values of their parameters, on the [Environments](environments.md) page.

## Sections and options

The page shows one group for each section, in the order of the create flow of a cluster. A group has a row for each option of the section. A registration that serves two sections shows in both groups. A registration shows only in the groups of its sections that exist.

Above its options, each group says three things about its section:

- The field that the claim sets from the option that it picks, for example `Fills CamundaCluster spec.backupStorageRef`. A field of a management plane ends with `for a management plane`. A section that only gives values to other sections, such as **Encryption key**, shows `Fills no field`.
- Whether a claim picks one option or many: `Pick one` or `Pick many`.
- The empty option that a claim can pick in place of an option, for example `Empty option: No backups`. A section without one shows `No empty option`, and a claim must pick an option there.

A section without a registration shows `No option is installed for this section.`

Each row shows these columns:

| Column | What it shows |
| --- | --- |
| Option | The label of the registration, and its name under it. |
| Controller | The controller that serves the option, and its version. The built-in options show `camunda-operator`. |
| Mode | `Default`: a new claim picks the option when no environment applies. `Optional`: a new claim does not pick the option, and a person can pick it. |
| Accepted | The `Accepted` condition, with its reason when it is not `True`. |
| Ready | The `Ready` condition, and how long the option is healthy. |
| Requests | The number of requests of the option, and the number in each environment. A claim writes one request for each option that it picks. |
| Operator versions | The range of operator versions that the option supports, for example `>=0.3.0 <1.0.0`, or `Any`. |
| Orphan | `Supported` when a claim can ask for the deletion policy `Orphan` with this option, else `Not supported`. See [When a claim is deleted](environments.md#when-a-claim-is-deleted). |

The line under the page title counts each registration once, for example `6 healthy, 0 not healthy`. A registration is healthy when `Accepted` and `Ready` are both `True`.

## Status of an option

`Accepted` says that the operator accepted the registration. `Ready` says that the controller of the option is healthy. A claim can pick an option only while both are `True`. A registration that is new and has no status yet shows `No status` in both columns.

`Healthy since 3d ago` under `Ready` is the time since `Ready` became `True`. A row whose `Ready` is not `True` shows no such time. Hold the pointer on a status to read its message.

| Reason of `Accepted` | Meaning | What to do |
| --- | --- | --- |
| `SectionMissing` | A section of the registration does not exist. When none of its sections exists, the registration shows in the last group, **Not in a section**. | Install the chart that brings the section, or correct the name of the section in the registration. |
| `OperatorVersionUnsupported` | The version of the operator is not in the operator versions of the registration. | Install a version of the chart that supports this operator, or upgrade the operator. |
| `DefaultConflict` | Another registration of the same section also has the mode `Default`. | Set the mode `Optional` on all but one of them. |

A reason of `Ready` comes from the controller of the option. Read its message, and the logs of that controller.

## Install an official chart

The panel **Charts you can install** lists the official plugin charts that are not installed on the cluster. Each chart shows its name, what it does, the sections that it brings options to, and the Helm command that installs it. Select **Copy the command** to copy the command. Then run it with access to the cluster. If the browser does not let the page copy, select the command and copy it by hand.

A chart counts as installed when a registration names the chart as its controller. The registration of the chart `aws-kms` has this field:

```yaml
apiVersion: plugins.camunda.io/v1alpha1
kind: PluginRegistration
metadata:
  name: aws-kms
spec:
  controller:
    name: aws-kms
    version: 0.1.0
  # ... the rest of your registration
```

The controller name is the chart name, unless you change it at install. If you install a chart with another controller name, the panel still lists that chart as not installed. Keep the default controller name if you want the panel to know the chart. When every official chart is installed, the panel does not show.
