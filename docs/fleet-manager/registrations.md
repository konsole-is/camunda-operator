# Registrations

A claim asks for a Camunda cluster or a management plane. A registration is one option that a claim can pick in a section, for example **Existing bucket** in the section **Backup storage**. The operator writes the built-in registrations, and each plugin chart writes its own. The page **Setup > Plugins** of the fleet manager lists every registration of the cluster with its status, and the official charts that are not installed. The [PluginRegistration](../crds/pluginregistration.md) reference lists the fields.

The page is read only. You install, upgrade, and remove a chart with Helm. An environment sets the options that its claims can use, and the values of their parameters, on the [Environments](environments.md) page.

## Sections and options

The page shows one group for each section, in the order of the create flow of a cluster. A group has a row for each option of the section. A registration that serves two sections shows in both groups. A registration shows only in the groups of its sections that exist.

Above its options, each group says three things about its section:

- The field that the claim sets from the option that it picks, for example `Fills CamundaCluster spec.backupStorageRef`. A field of a management plane ends with `for a management plane`. A section that only gives values to other sections, such as **Encryption key**, shows `Fills no field`.
- Whether a claim picks one option or many: `Pick one` or `Pick many`.
- The empty option that a claim can pick in place of an option, for example `Empty option: No backups`. A section without one shows `No empty option`. The setting **Required** of an environment decides if a claim must pick an option. See [Sections and options](environments.md#sections-and-options).

A section without a registration shows `No option is installed for this section.`

Each row shows these columns. Select the label of an option to open its page. [The page of an option](#the-page-of-an-option) describes that page.

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

`Healthy since 3d ago` under `Ready` is the time since `Ready` became `True`. A row whose `Ready` is not `True` shows no such time. To read the message of a status, open the option and select the tab **Health**.

| Reason of `Accepted` | Meaning | What to do |
| --- | --- | --- |
| `SectionMissing` | A section of the registration does not exist. When none of its sections exists, the registration shows in the last group, **Not in a section**. | Install the chart that brings the section, or correct the name of the section in the registration. |
| `OperatorVersionUnsupported` | The version of the operator is not in the operator versions of the registration. | Install a version of the chart that supports this operator, or upgrade the operator. |
| `DefaultConflict` | Another registration of the same section also has the mode `Default`. | Set the mode `Optional` on all but one of them. |

A reason of `Ready` comes from the controller of the option. Read its message, and the logs of that controller.

## The page of an option

The page of an option shows the label of the registration as its title, and its name under the title. The page has six tabs: **About**, **Contract**, **Parameters**, **Grants**, **Health**, and **Used by**. The address keeps the open tab, for example `/setup/plugins/elasticsearch?tab=used-by`. If the registration is removed while the page is open, the page says `This registration no longer exists`. It keeps saying so when it cannot read the registrations after that, and then adds `It was gone when the page last read it`.

### About

**About** shows the description of the option, a link to its documentation, its controller and version, and the ServiceAccount of the controller. **Other options of this controller** links the other registrations that the same controller serves.

The panel **Limits set at install** lists each limit of the chart by its label, with its value. No environment and no claim can change these limits. To change a limit, change the values of the chart and upgrade it with Helm. An option without limits shows `This option sets no limits at install.`

### Contract

**Contract** shows what the option takes and what it gives:

- **Sections**: each section that the option serves, with the field that it fills. A section that does not exist shows `Not a section on this cluster`.
- **Inputs**: each value that the option reads from an output of another section. **From** names that section and output, for example `Backup storage / objectStorageConfig`. **Formats** lists the formats that the option accepts, or `Any`. A `Required` input must have a value, or a claim cannot pick the option.
- **Outputs**: each value that the option writes for a request, with its description and format. For example, the built-in `elasticsearch` gives `secondaryStorageConfig`, the name of the SecondaryStorageConfig that the cluster reads.
- **Deletion**: `Delete` and `Orphan`, each `supported` or `not supported`. Every option supports `Delete`.

### Parameters

**Parameters** shows one panel for each parameter: its type, its range or its values, its default, and `A claim can change it`. The default is the value of the chart. The default and `A claim can change it` apply in a namespace without an environment.

Each environment adds a line with its value, or `No value`, and `A claim can change it: Yes` or `A claim can change it: No`. The line shows what a new claim in that environment gets. An environment can set a parameter that the chart does not have. Its panel then says `Only an environment sets this parameter.` A claim that exists keeps the values that it resolved when it was made.

The tab hides the parameters that the claim controller sets, such as `releaseRef` of the built-in `elasticsearch`. To change a value or `A claim can change it` for an environment, use the [Environments](environments.md) page.

### Grants

A Grant asks the cloud identity of a cluster for one permission, for one request of the option. **Grants** shows each grant template of the option, with its format `aws-iam-statement`, its statement, and the inputs that it needs. The statement keeps its placeholders as written, for example `{{ .outputs.bucket }}`. The controller of the option fills them for each request. The line **Grants** counts the Grants of the template that are ready and refused, and the Grants on an old template when there are any.

Each Grant shows its namespace and name, its cluster, its request, and three conditions: `Applied`, `MatchesTemplate`, and `Ready`. A refused Grant also says what its reason means:

| Reason | Meaning | What to do |
| --- | --- | --- |
| `OverCeiling` | The Grant asks for more than the ceiling of the identity allows. | Ask the admin of the identity extension to raise the ceiling, or change the parameters of the request. |
| `OverSizeLimit` | The Grant makes the policy of the identity larger than the cloud allows. | Pick fewer options that ask for permissions for this cluster. |
| `Rejected` | The identity or the cloud refused the Grant. The message says why. | Read the message. |
| `TemplateChanged` | The statement of the Grant is not the template of the registration now. The role still has the statement that the Grant holds, so the Grant is applied. This is a reason of `MatchesTemplate`. The `Ready` of the Grant is `False`, so it counts as on an old template, not as refused. | After a chart upgrade, wait. The controller of the option writes the Grant from its own template at its next reconcile. If the reason stays, the registration was changed outside its controller. The controller keeps its own template, and the old statement stays. |

The [Grant](../crds/grant.md#status) reference lists every condition of a Grant. An option without grant templates shows this line:

```text
This option has no grant template, so it writes no Grant.
```

A Grant whose template the registration no longer has shows in the last panel, **Grants of a template the registration no longer has**. The role keeps the statement of each of these Grants until the controller of the option deletes that Grant. A grant template that the page cannot read shows an error, with the Grants of that template under it.

### Health

**Health** shows the conditions `Accepted` and `Ready` of the registration with their reasons and messages, and the events of the registration, the newest first. The table in [Status of an option](#status-of-an-option) says what to do for each reason of `Accepted`.

### Used by

**Used by** shows one panel for each request of the option, named `<namespace>/<request>`. A panel shows:

- **Cluster** or **Management plane**: the claim that wrote the request, with a link to its page.
- **Environment**: the environment of the namespace, or `No environment`.
- **Outputs** and **Ready**: the conditions `OutputsReady` and `Ready` of the request. A condition that is not `True` shows its message. When `Ready` has the reason `GrantRejected`, the line `Grants with Ready False:` names each Grant of the request whose `Ready` is `False`. Open the tab **Grants** to read the reason of each one.
- **Headline**: the first fact that the controller of the option reports about the request, for example `SecondaryStorageConfig: orders-storage, Ready`.
- **Resources**: each resource that the controller made for the request, with its role and location. `On delete: Kept` or `On delete: Deleted` says what happens to the resource when the request is deleted with its **Deletion policy**. With `Delete`, every resource is deleted. With `Orphan`, a resource stays unless the controller of the option deletes it then too.
- **Grants**: each Grant of the request, with its `Ready` condition.
- **Deletion policy**: `Delete` or `Orphan`.

An option that no claim uses shows `No claim uses this option.`

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

If the fleet manager cannot read the catalog of charts in its image, the panel shows `Could not read the chart catalog:` and the error. The panel then lists no chart. This does not mean that every chart is installed. Reload the page to read the catalog again.

## When the page cannot read a list

The page **Plugins** reads five kinds: `PluginSection`, `PluginRegistration`, `PluginRequest`, `Namespace`, and `Environment`. If the fleet manager cannot read one of them, a red line under the title names the kind and the error. A line `Not read:` names each kind that the page has not read. When the page opens, this line can show for a moment, until the page has read each kind.

The page does not show an empty or healthy value for a kind that it has not read. It shows `not known` instead:

| Kind that is not read | What the page shows |
| --- | --- |
| `PluginRegistration` | `Health not known` in place of the count under the title. `Accepted: Not known` and `Ready: Not known` in each row, with no `Healthy since`. `Options not known` in a section without a row. `Which charts are installed is not known.` in the panel **Charts you can install**, with no chart. |
| `PluginSection` | A last group **Section not known** in place of **Not in a section**, with each registration that is in no section that the page knows. |
| `PluginRequest` | `Not known` in the column **Requests**. |
| `Namespace` or `Environment` | The number of requests in the column **Requests**, with `Environments not known` in place of the number in each environment. |

The page of an option shows the same red line and the same `Not read:` line. It also reads `Grant`, and the events. A tab that needs a kind that is not read shows `Not known:` and the kind, for example `Not known: the fleet manager has no current list of PluginRequest.` It does not show an empty list:

| Kind that is not read | What the page of an option shows |
| --- | --- |
| `PluginRegistration` | `Not known` on the tab **Health** in place of the conditions, and for **Other options of this controller** on the tab **About**. The tabs **About**, **Contract**, **Parameters**, and **Grants** show the registration as the page last read it, with the line `This is the registration as the page last read it. It can be old.` |
| `PluginSection` | `Not known` in place of `Not a section on this cluster` on the tab **Contract**. |
| `Environment` | `Values of the environments: Not known` on the tab **Parameters**, with no line for an environment. The tab shows no parameter that only an environment sets. If it has no other parameter, it shows `Not known` in place of `This option takes no parameters.` |
| `PluginRequest` | `Not known` in place of `No claim uses this option.` on the tab **Used by**. Each request that the page still shows has `Ready: Not known`, `OutputsReady: Not known`, and `Not known` for **Headline** and **Resources**. |
| `Namespace` or `Environment` | `Not known` for **Environment** on the tab **Used by**. |
| `Grant` | No count of ready and refused Grants on the tab **Grants**. Each Grant that the page still shows has `Applied: Not known`, `MatchesTemplate: Not known`, and `Ready: Not known`. `Not known` for **Grants** on the tab **Used by**. |
| `Event` | `Not known` in place of `No events` on the tab **Health**. Events that the page read before stay, with a line that the list can miss events. |

When the fleet manager can read the kind again, the page shows its values without a reload.
