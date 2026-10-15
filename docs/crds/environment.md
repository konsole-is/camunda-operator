# Environment

An `Environment` sets the policy for the claims in its namespaces. A claim is a request for a Camunda cluster or a management plane, in one namespace. The [fleet manager](../fleet-manager/install.md) is the web app that creates claims. A namespace belongs to the `Environment` that its label `fleet.camunda.io/environment` names. An `Environment` is cluster-scoped.

The policy names the releases and the presets that a claim can use. It also names the plugin options of each `PluginSection` of the create flow, for example "Backup storage". Each option is a `PluginRegistration`. The policy sets the parameter values of each option and the deletion default.

!!! note
    `fleet.camunda.io/v1alpha1` is an alpha API. Its fields can change in a later release. In this release, the operator reports the status of an environment. It does not create anything from a claim yet.

The smallest manifest:

```yaml
apiVersion: fleet.camunda.io/v1alpha1
kind: Environment
metadata:
  name: production
spec:
  releases:
    allowed:
      - camunda-8-9-0
    default: camunda-8-9-0
```

```mermaid
graph LR
    NS[Namespace] -.->|"label fleet.camunda.io/environment"| ENV[Environment]
    ENV -.->|releases| CR[CamundaRelease]
    ENV -.->|presets| CCP[CamundaClusterPreset]
    ENV -.->|presets| ESCP[ElasticsearchClusterPreset]
    ENV -.->|presets| DBSP[DatabaseServerPreset]
    ENV -.->|platformConfigRef| PFC[CamundaPlatformConfig]
    ENV -.->|sections| PS[PluginSection]
    ENV -.->|"sections, parameters"| PR[PluginRegistration]
```

## Namespaces

To put a namespace in an environment, give the namespace the label `fleet.camunda.io/environment` with the name of the environment:

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: my-cluster-ns
  labels:
    fleet.camunda.io/environment: production
```

`status.namespaces` is the number of namespaces that carry the label with the name of the environment. It is `0` when no namespace carries it. To move a namespace to another environment, change the value of the label. A namespace without the label belongs to no environment.

## References

The spec names cluster-scoped objects by their names:

| Field | Kind |
| --- | --- |
| `spec.platformConfigRef` | `CamundaPlatformConfig` |
| `spec.presets.camundaCluster` | `CamundaClusterPreset` |
| `spec.presets.elasticsearchCluster` | `ElasticsearchClusterPreset` |
| `spec.presets.databaseServer` | `DatabaseServerPreset` |
| `spec.releases.allowed`, `spec.releases.default` | `CamundaRelease` |
| the keys of `spec.sections` | `PluginSection` |
| `spec.sections.<section>.allowed`, `spec.sections.<section>.default`, the keys of `spec.parameters` | `PluginRegistration` |

When one of these objects does not exist, `Ready` is `False` with reason `InvalidReference`. The message names the kind and the name of each missing object:

```yaml
status:
  observedGeneration: 2
  namespaces: 3
  conditions:
    - type: Ready
      status: "False"
      reason: InvalidReference
      message: "The spec names objects that do not exist: CamundaRelease camunda-8-10-0, PluginSection backup-storage"
      observedGeneration: 2
      lastTransitionTime: "2026-10-15T09:12:44Z"
```

To correct it, create each missing object, or remove its name from the spec. You do not need to edit the environment after you create an object. `Ready` changes to `True` when the last missing object exists. When you delete an object that the spec names, `Ready` changes to `False` again.

## Policy

`spec.sections` holds the policy of each `PluginSection`, by section name. A claim can select only a registration in `allowed`. The create flow of the fleet manager preselects `default`. The empty option selects no registration of the section. `required: true` makes a claim select a registration, and the fleet manager then hides the empty option. A section that is not in the list offers only the empty option:

```yaml
apiVersion: fleet.camunda.io/v1alpha1
kind: Environment
metadata:
  name: production
spec:
  sections:
    backup-storage:
      required: true
      default: s3-backups
      allowed:
        - s3-backups
  # ... the rest of your environment
```

`spec.parameters` holds the policy of the parameters of each `PluginRegistration`, by registration name and then by parameter name. `value` replaces the value that the registration gives. `claimEditable: true` lets a claim change the parameter. `allowed` limits a claim to a list of choices, and a claim cannot pick a name outside the list. Each choice has a `name` that the claim stores, a `label` that the fleet manager shows, and a `value` that the plugin gets. A parameter without a policy keeps the value of its registration, and a claim cannot change it. In the example below, `aws-connectors` is the name of a registration, and `policies` is the name of one of its parameters:

```yaml
apiVersion: fleet.camunda.io/v1alpha1
kind: Environment
metadata:
  name: production
spec:
  parameters:
    aws-connectors:
      policies:
        claimEditable: true
        allowed:
          - name: sqs
            label: SQS access
            value: arn:aws:iam::aws:policy/AmazonSQSFullAccess
  # ... the rest of your environment
```

`spec.deletionPolicy` is the deletion policy of a claim that sets none itself. `Delete` deletes the resources that the plugins created for the claim when you delete the claim. `Orphan` keeps them. The default is `Delete`.

`spec.namespaceCreation` lets the create flow of the fleet manager create a namespace in the environment. `allowed: true` turns it on. `labels` are fixed labels of each new namespace. `requiredLabelKeys` are label keys that the create flow asks a value for. The create flow always sets the label `fleet.camunda.io/environment` on the new namespace:

```yaml
apiVersion: fleet.camunda.io/v1alpha1
kind: Environment
metadata:
  name: production
spec:
  namespaceCreation:
    allowed: true
    labels:
      owner: platform
    requiredLabelKeys:
      - team
  # ... the rest of your environment
```

A change of `sections`, `parameters`, `deletionPolicy`, or `platformConfigRef` applies to new claims only. An existing claim keeps the policy that it recorded at creation. The releases and the presets are different. A new claim, and a release or preset change of an existing claim, are checked against the current lists.

## Status

| Type | Reason | Meaning | What to do |
| --- | --- | --- | --- |
| `Ready` | `Healthy` | Every object that the spec names exists. | Nothing. |
| `Ready` | `InvalidReference` | The spec names an object that does not exist. The message names the kind and the name of each one. | Create the object, or remove its name from the spec. |

`status.namespaces` is the number of namespaces with the label of the environment. `status.observedGeneration` is the last generation of the spec that the status describes.

`kubectl get environments` shows `Ready`, the reason, the number of namespaces, and the age. `kubectl get environments -o wide` adds the display name.

## Spec reference

Every field, with its type, whether it is required, and its default:

```yaml
apiVersion: fleet.camunda.io/v1alpha1
kind: Environment
metadata:
  # Cluster-scoped: no namespace. At most 63 characters.
  name: production
spec:
  # string. Optional. The name that the fleet manager shows, at most 64 characters. It changes nothing for a claim.
  displayName: Production
  # string. Optional. The color that the fleet manager shows, as a hex color. It changes nothing for a claim.
  color: "#2E7D32"
  # string. Optional. The CamundaPlatformConfig of a claim that names none itself. A change applies to new claims only.
  platformConfigRef: my-platform-config
  # object. Optional. The presets that a claim can use, for each preset kind.
  presets:
    # []string. Optional. Names of the CamundaClusterPresets that a cluster claim can use.
    camundaCluster:
      - small
    # []string. Optional. Names of the ElasticsearchClusterPresets that the built-in Elasticsearch of a claim can use.
    elasticsearchCluster:
      - es-small
    # []string. Optional. Names of the DatabaseServerPresets that the built-in PostgreSQL of a claim can use.
    databaseServer:
      - pg-small
  # object. Optional. The releases that a claim can use.
  releases:
    # []string. Optional. Names of the CamundaReleases that a claim can use.
    allowed:
      - camunda-8-9-0
    # string. Optional. The release of a claim that names none. It is one of allowed.
    default: camunda-8-9-0
  # map[string]SectionPolicy. Optional. The policy of each PluginSection, by section name. A change applies to new claims only.
  sections:
    backup-storage:
      # boolean. Optional, default: false. A claim must select a registration of the section.
      required: true
      # string. Optional. The registration that the create flow preselects. It is empty or one of allowed.
      default: s3-backups
      # []string. Optional. Names of the PluginRegistrations of the section that a claim can select.
      allowed:
        - s3-backups
  # map[string]map[string]ParameterPolicy. Optional. The policy of each parameter, by registration name and then by parameter name. A change applies to new claims only.
  parameters:
    s3-backups:
      retentionDays:
        # JSON. Optional. Replaces the value that the registration gives the parameter.
        value: 30
        # boolean. Optional, default: false. A claim can change the parameter.
        claimEditable: true
        # []NamedValue. Optional. The choices that a claim can pick. Needs claimEditable.
        allowed:
          # string. Required. What the claim stores when it picks the choice.
          - name: month
            # string. Required. What the fleet manager shows for the choice.
            label: One month
            # JSON. Required. What the plugin gets.
            value: 30
  # string. Optional, default: Delete. The deletion policy of a claim that sets none: Delete or Orphan. A change applies to new claims only.
  deletionPolicy: Delete
  # object. Optional. Lets the create flow of the fleet manager create a namespace in the environment.
  namespaceCreation:
    # boolean. Optional, default: false. The create flow can create a namespace.
    allowed: true
    # map[string]string. Optional. Fixed labels of a new namespace.
    labels:
      owner: platform
    # []string. Optional. Label keys that the create flow asks a value for.
    requiredLabelKeys:
      - team
```

### Validation rules

- `metadata.name` has at most 63 characters.
- `releases.default` is empty or one of `releases.allowed`.
- In each section policy, `default` is empty or one of `allowed`. `required: true` needs at least one name in `allowed`.
- A parameter policy with `allowed` needs `claimEditable: true`. Each choice has a unique `name` of at most 63 characters and a `label` of at most 64 characters.
- `namespaceCreation.labels` and `namespaceCreation.requiredLabelKeys` cannot hold `fleet.camunda.io/environment`. Each key is a valid label key, and each label value is a valid label value.
- `displayName` has at most 64 characters. `color` is a hex color such as `#2E7D32`.
- `sections`, `parameters`, and `releases.allowed` have at most 64 entries. A parameter policy has at most 32 choices. `namespaceCreation` has at most 32 labels and 16 required label keys.

### A production-shaped example

```yaml
apiVersion: fleet.camunda.io/v1alpha1
kind: Environment
metadata:
  name: production
spec:
  displayName: Production
  color: "#2E7D32"
  platformConfigRef: my-platform-config
  presets:
    camundaCluster:
      - small
      - large
  releases:
    allowed:
      - camunda-8-9-0
    default: camunda-8-9-0
  sections:
    backup-storage:
      required: true
      default: s3-backups
      allowed:
        - s3-backups
  parameters:
    aws-connectors:
      policies:
        claimEditable: true
        allowed:
          - name: sqs
            label: SQS access
            value: arn:aws:iam::aws:policy/AmazonSQSFullAccess
  deletionPolicy: Delete
  namespaceCreation:
    allowed: true
    labels:
      owner: platform
    requiredLabelKeys:
      - team
```

## Related

- [CamundaRelease](camundarelease.md): named in `spec.releases`.
- [CamundaClusterPreset](camundaclusterpreset.md), [ElasticsearchClusterPreset](elasticsearchclusterpreset.md), [DatabaseServerPreset](databaseserverpreset.md): named in `spec.presets`.
- [CamundaPlatformConfig](camundaplatformconfig.md): named in `spec.platformConfigRef`.
- [API reference](api-reference.md#environment): every type and field of `fleet.camunda.io/v1alpha1`.
