import type { V1Namespace } from "@kubernetes/client-node";
import { BasicsStory } from "@/components/create/basics-story";
import { EnvironmentCards } from "@/components/create/environment-cards";
import { PresetCards } from "@/components/create/preset-cards";
import {
  DeletionChoiceStory,
  StorageStory,
} from "@/components/create/storage-story";
import type { Story } from "@/gallery/registry";
import type { CamundaClusterPreset } from "@/generated/crds/camundaclusterpreset";
import type { ElasticsearchClusterPreset } from "@/generated/crds/elasticsearchclusterpreset";
import type { Environment } from "@/generated/crds/environment";
import type {
  PluginRegistration,
  PluginRegistrationSpec,
} from "@/generated/crds/pluginregistration";
import type { PluginSection } from "@/generated/crds/pluginsection";
import { DESCRIPTION_ANNOTATION, ENVIRONMENT_LABEL } from "@/lib/labels";
import { NamespacePicker } from "./namespace-picker";
import { WizardStepper } from "./wizard-stepper";

const steps = (errors?: string[]) => [
  { id: "namespace", label: "Namespace" },
  { id: "storage", label: "Storage and access", errors },
  { id: "settings", label: "Settings" },
  { id: "review", label: "Review" },
];
const environment = {
  name: "prod",
  displayName: "Production",
  namespaceCreation: {
    allowed: true,
    labels: { "cost-center": "4711" },
    requiredLabelKeys: ["team"],
  },
};
const namespaces = [
  { name: "team-payments", environment: "prod" },
  { name: "team-orders", environment: "prod" },
];

const production: Environment = {
  apiVersion: "fleet.camunda.io/v1alpha1",
  kind: "Environment",
  metadata: { name: "prod" },
  spec: {
    displayName: "Production",
    color: "#2E7D32",
    namespaceCreation: environment.namespaceCreation,
    presets: { camundaCluster: ["small", "medium", "large"] },
  },
};
const development: Environment = {
  apiVersion: "fleet.camunda.io/v1alpha1",
  kind: "Environment",
  metadata: { name: "dev" },
  spec: {
    displayName: "Development",
    color: "#1565C0",
    presets: { camundaCluster: ["small"] },
  },
};
const legacy: Environment = {
  apiVersion: "fleet.camunda.io/v1alpha1",
  kind: "Environment",
  metadata: { name: "legacy" },
  spec: { displayName: "Legacy" },
  status: {
    conditions: [
      {
        type: "Ready",
        status: "False",
        reason: "InvalidReference",
        message: 'CamundaRelease "8.9-old" does not exist',
        lastTransitionTime: "2026-10-30T00:00:00Z",
      },
    ],
  },
};
const counts = {
  prod: { namespaces: 2, clusters: 3 },
  dev: { namespaces: 1, clusters: 0 },
  legacy: { namespaces: 0, clusters: 0 },
};
const clusterNamespaces: V1Namespace[] = [
  {
    metadata: {
      name: "team-payments",
      labels: { [ENVIRONMENT_LABEL]: "prod" },
    },
  },
  {
    metadata: { name: "team-orders", labels: { [ENVIRONMENT_LABEL]: "prod" } },
  },
  { metadata: { name: "sandbox", labels: { [ENVIRONMENT_LABEL]: "dev" } } },
];
const preset = (
  name: string,
  replicas: number,
  partitions: number,
  cpu: string,
  memory: string,
  description?: string,
): CamundaClusterPreset => ({
  apiVersion: "core.camunda.io/v1",
  kind: "CamundaClusterPreset",
  metadata: {
    name,
    ...(description && {
      annotations: { [DESCRIPTION_ANNOTATION]: description },
    }),
  },
  spec: {
    cluster: {
      zeebe: { replicas, partitions, resources: { requests: { cpu, memory } } },
    },
  },
});
const presets = [
  preset(
    "small",
    1,
    1,
    "500m",
    "1Gi",
    "For tests and demos. One broker, no replication.",
  ),
  preset(
    "medium",
    3,
    3,
    "1",
    "4Gi",
    "For production workloads with three brokers and replication.",
  ),
  preset("large", 5, 12, "4", "16Gi"),
];
const usage = {
  small: ["team-orders/search"],
  medium: ["team-orders/orders", "team-payments/billing"],
};

function storageSection(
  name: string,
  spec: Partial<PluginSection["spec"]> & { label: string },
): PluginSection {
  return {
    apiVersion: "plugins.camunda.io/v1alpha1",
    kind: "PluginSection",
    metadata: { name },
    spec: { appliesTo: ["CamundaClusterClaim"], selection: "One", ...spec },
  };
}

function storageRegistration(
  name: string,
  spec: Partial<PluginRegistrationSpec> & { label: string },
  ready: { status: "True" | "False"; message: string } = {
    status: "True",
    message: "",
  },
): PluginRegistration {
  return {
    apiVersion: "plugins.camunda.io/v1alpha1",
    kind: "PluginRegistration",
    metadata: { name },
    spec: {
      sections: [],
      mode: "Optional",
      deletionPolicies: ["Delete"],
      serviceAccount: { name, namespace: "plugins" },
      ...spec,
    },
    status: {
      conditions: [
        {
          type: "Accepted",
          status: "True",
          reason: "Valid",
          message: "",
          lastTransitionTime: "2026-11-10T00:00:00Z",
        },
        {
          type: "Ready",
          status: ready.status,
          reason: ready.status === "True" ? "Healthy" : "CredentialsExpired",
          message: ready.message,
          lastTransitionTime: "2026-11-10T00:00:00Z",
        },
      ],
    },
  };
}

const storageSections = [
  storageSection("encryption-key", {
    label: "Encryption key",
    description: "The key that encrypts the volumes and the backups.",
    order: 20,
    emptyOptionLabel: "No key of my own",
  }),
  storageSection("volume-encryption", {
    label: "Volume encryption",
    description: "How the volumes of the cluster are encrypted.",
    order: 30,
    emptyOptionLabel: "Default encryption of the cluster",
  }),
  storageSection("backup-storage", {
    label: "Backup storage",
    description: "Where the backups of the cluster go.",
    order: 40,
    emptyOptionLabel: "No backups",
  }),
  storageSection("secondary-storage", {
    label: "Secondary storage",
    description: "Where Operate, Tasklist, and Optimize read their data.",
    order: 50,
  }),
  storageSection("access", {
    label: "Access",
    description:
      "How users and clients reach the cluster or the management plane from outside.",
    order: 60,
    emptyOptionLabel: "No access from outside",
  }),
];

const BUILTIN = { name: "camunda-operator", version: "0.4.0" };

const storageRegistrations = [
  storageRegistration("key-service", {
    label: "Key from the key service",
    description: "A key for each cluster, in the key service of the cloud.",
    sections: ["encryption-key"],
  }),
  storageRegistration("encrypted-volumes", {
    label: "Volumes with my key",
    description: "A storage class that encrypts each volume with the key.",
    sections: ["volume-encryption"],
    inputs: [{ name: "keyRef", from: "encryption-key/keyRef", required: true }],
  }),
  storageRegistration("bucket", {
    label: "Bucket for each cluster",
    description: "A bucket of its own, with a retention rule.",
    sections: ["backup-storage"],
    parameters: {
      type: "object",
      properties: {
        retentionDays: {
          type: "integer",
          title: "Retention in days",
          minimum: 1,
          maximum: 365,
        },
      },
    },
  }),
  storageRegistration(
    "archive-bucket",
    {
      label: "Archive bucket",
      description: "A bucket in the archive tier.",
      sections: ["backup-storage"],
    },
    { status: "False", message: "The credentials of the plugin expired." },
  ),
  storageRegistration("external-search", {
    label: "External Elasticsearch",
    description: "An Elasticsearch cluster that runs outside Kubernetes.",
    sections: ["secondary-storage"],
  }),
  storageRegistration("elasticsearch", {
    label: "New Elasticsearch cluster",
    description:
      "Runs an Elasticsearch cluster for the cluster, from an Elasticsearch cluster preset.",
    sections: ["secondary-storage"],
    controller: BUILTIN,
    parameters: {
      type: "object",
      properties: {
        preset: { type: "string", title: "Preset" },
        releaseRef: { type: "string", title: "Release" },
        overrides: { type: "object", title: "Overrides" },
      },
    },
    inputs: [
      {
        name: "objectStorageConfig",
        from: "backup-storage/objectStorageConfig",
      },
    ],
  }),
  storageRegistration("gateway", {
    label: "Gateway",
    description: "A host for each cluster on the shared gateway.",
    sections: ["access"],
    parameterValues: { hostnamePattern: "{name}.{namespace}.example.com" },
  }),
];

function searchPreset(
  name: string,
  cluster: ElasticsearchClusterPreset["spec"]["cluster"],
): ElasticsearchClusterPreset {
  return {
    apiVersion: "core.camunda.io/v1",
    kind: "ElasticsearchClusterPreset",
    metadata: { name },
    spec: { cluster },
  };
}

const storageDraft = {
  environment: "prod",
  namespace: { kind: "existing" as const, name: "team-orders" },
  claim: { metadata: { name: "search" }, spec: { presetRef: "medium" } },
};

export const createFlowStories: Story[] = [
  {
    component: "WizardStepper",
    name: "Step without errors",
    render: () => (
      <WizardStepper steps={steps()} current="storage">
        <p className="text-sm text-text-2">
          The section cards of the step go here.
        </p>
      </WizardStepper>
    ),
  },
  {
    component: "WizardStepper",
    name: "Step with errors",
    render: () => (
      <WizardStepper
        steps={steps(["Choose a secondary storage option"])}
        current="storage"
      >
        <p className="text-sm text-text-2">
          The section cards of the step go here.
        </p>
      </WizardStepper>
    ),
  },
  {
    component: "WizardStepper",
    name: "Last step",
    render: () => (
      <WizardStepper steps={steps()} current="review">
        <p className="text-sm text-text-2">The review goes here.</p>
      </WizardStepper>
    ),
  },
  {
    component: "NamespacePicker",
    name: "Existing namespace",
    render: () => (
      <NamespacePicker
        namespaces={namespaces}
        environment={environment}
        defaultValue={{ kind: "existing", name: "team-orders" }}
      />
    ),
  },
  {
    component: "NamespacePicker",
    name: "New namespace",
    render: () => (
      <NamespacePicker
        namespaces={namespaces}
        environment={environment}
        defaultValue={{
          kind: "new",
          name: "team-search",
          labels: { team: "search" },
        }}
      />
    ),
  },
  {
    component: "NamespacePicker",
    name: "Creation not allowed",
    render: () => (
      <NamespacePicker
        namespaces={namespaces}
        environment={{ name: "prod", displayName: "Production" }}
      />
    ),
  },
  {
    component: "EnvironmentCards",
    name: "Two environments, one picked",
    render: () => (
      <EnvironmentCards
        environments={[production, development]}
        counts={counts}
        value="prod"
      />
    ),
  },
  {
    component: "EnvironmentCards",
    name: "Not ready",
    render: () => (
      <EnvironmentCards environments={[production, legacy]} counts={counts} />
    ),
  },
  {
    component: "EnvironmentCards",
    name: "Several picked",
    render: () => (
      <EnvironmentCards
        multiple
        environments={[production, development, legacy]}
        counts={counts}
        value={["dev", "prod"]}
      />
    ),
  },
  {
    component: "PresetCards",
    name: "Three presets",
    render: () => (
      <PresetCards
        presets={presets}
        usage={new Map(Object.entries(usage))}
        value="medium"
      />
    ),
  },
  {
    component: "ClusterBasicsStep",
    name: "Production picked",
    render: () => (
      <BasicsStory
        environments={[production, development, legacy]}
        namespaces={clusterNamespaces}
        presets={presets}
        usage={usage}
        initial={{
          environment: "prod",
          namespace: { kind: "existing", name: "team-orders" },
          claim: {
            metadata: { name: "search" },
            spec: { presetRef: "medium" },
          },
        }}
        hosts={[
          {
            parameter: "hostnamePattern",
            host: "search.team-orders.example.com",
          },
          {
            parameter: "grpcHostnamePattern",
            host: "grpc-search.team-orders.example.com",
          },
        ]}
      />
    ),
  },
  {
    component: "ClusterBasicsStep",
    name: "Taken host",
    render: () => (
      <BasicsStory
        environments={[production, development]}
        namespaces={clusterNamespaces}
        presets={presets}
        usage={usage}
        initial={{
          environment: "prod",
          namespace: { kind: "existing", name: "team-orders" },
          claim: {
            metadata: { name: "billing" },
            spec: { presetRef: "small" },
          },
        }}
        hosts={[
          {
            parameter: "hostnamePattern",
            host: "billing.team-orders.example.com",
          },
        ]}
        conflicts={[
          "The host billing.team-orders.example.com is taken by team-payments/billing. Choose another name.",
        ]}
      />
    ),
  },
  {
    component: "ClusterBasicsStep",
    name: "No environment",
    render: () => (
      <BasicsStory
        environments={[]}
        namespaces={clusterNamespaces}
        presets={presets}
        usage={usage}
        initial={{ claim: { metadata: { name: "" }, spec: {} } }}
      />
    ),
  },
  {
    component: "ClusterStorageStep",
    name: "Options with an unavailable one and a required section",
    render: () => (
      <StorageStory
        sections={storageSections}
        registrations={storageRegistrations}
        environment={{
          ...production,
          spec: {
            ...production.spec,
            sections: {
              "encryption-key": { allowed: ["key-service"] },
              "volume-encryption": { allowed: ["encrypted-volumes"] },
              "backup-storage": {
                allowed: ["archive-bucket", "bucket"],
                default: "archive-bucket",
              },
              "secondary-storage": {
                allowed: ["external-search"],
                required: true,
              },
              access: { allowed: ["gateway"], default: "gateway" },
            },
            parameters: { bucket: { retentionDays: { claimEditable: true } } },
          },
        }}
        initial={storageDraft}
      />
    ),
  },
  {
    component: "ClusterStorageStep",
    name: "Built-in storage with preset cards",
    render: () => (
      <StorageStory
        sections={storageSections.filter((s) =>
          ["backup-storage", "secondary-storage"].includes(
            s.metadata.name ?? "",
          ),
        )}
        registrations={storageRegistrations}
        environment={{
          ...production,
          spec: {
            ...production.spec,
            sections: {
              "backup-storage": { allowed: ["bucket"] },
              "secondary-storage": {
                allowed: ["elasticsearch"],
                default: "elasticsearch",
                required: true,
              },
            },
            presets: { elasticsearchCluster: ["search-small", "search-ha"] },
            parameters: { elasticsearch: { preset: { claimEditable: true } } },
          },
        }}
        objects={{
          ElasticsearchClusterPreset: [
            searchPreset("search-small", {
              replicas: 1,
              resources: { requests: { cpu: "1", memory: "4Gi" } },
              storageSize: "50Gi",
            }),
            searchPreset("search-ha", {
              replicas: 3,
              resources: { requests: { cpu: "2", memory: "8Gi" } },
              storageSize: "200Gi",
            }),
          ],
        }}
        initial={storageDraft}
      />
    ),
  },
  {
    component: "DeletionChoiceField",
    name: "Deletion choice with Keep refused",
    render: () => (
      <DeletionChoiceStory
        choice={{
          defaultPolicy: "Orphan",
          keepAllowed: false,
          alwaysDeleted: [
            {
              section: "backup-storage",
              label: "Archive bucket (Backup storage)",
            },
            { section: "access", label: "Gateway (Access)" },
          ],
          policy: "Delete",
        }}
      />
    ),
  },
];
