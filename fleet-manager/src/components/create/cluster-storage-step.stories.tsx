import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { type ComponentProps, useState } from "react";
import { fn } from "storybook/test";
import { WizardStepper } from "@/components/fleet/wizard-stepper";
import type { ElasticsearchClusterPreset } from "@/generated/crds/elasticsearchclusterpreset";
import type { Environment } from "@/generated/crds/environment";
import type {
  PluginRegistration,
  PluginRegistrationSpec,
} from "@/generated/crds/pluginregistration";
import type { PluginSection } from "@/generated/crds/pluginsection";
import { CLUSTER_STEPS, type ClusterDraft } from "@/lib/clusters/draft";
import {
  type StorageData,
  clusterDeletion,
  storageErrors,
  storageViews,
} from "@/lib/clusters/storage";
import type { BuiltinObjects } from "@/lib/policy";
import type { ValidationError } from "@/lib/schema";
import { ClusterStorageStep } from "./cluster-storage-step";
import { production } from "./fixtures";
import { useCreateFlow } from "./use-create-flow";

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

// ClusterStorageStep holds no draft, so the harness holds the draft, the card
// errors, and the check of step 2 as the create flow does.
function ClusterStorageStepHarness({
  sections,
  registrations,
  environment,
  objects = {},
  initial,
  onSection,
  onDeletion,
}: {
  sections: PluginSection[];
  registrations: PluginRegistration[];
  environment?: Environment;
  objects?: BuiltinObjects;
  initial: ClusterDraft;
  onSection: ComponentProps<typeof ClusterStorageStep>["onSection"];
  onDeletion: ComponentProps<typeof ClusterStorageStep>["onDeletion"];
}) {
  const data: StorageData = { environment, sections, registrations, objects };
  const [cardErrors, setCardErrors] = useState<
    Record<string, ValidationError[]>
  >({});
  const flow = useCreateFlow<ClusterDraft>({
    steps: CLUSTER_STEPS.filter((s) => s.id === "storage").map((s) => ({
      ...s,
      check: (draft: ClusterDraft) =>
        storageErrors(draft, data, cardErrors, []),
    })),
    initial,
  });
  return (
    <WizardStepper
      steps={CLUSTER_STEPS.map((s) => ({
        ...s,
        errors: s.id === "storage" ? flow.errors.storage : undefined,
      }))}
      current="storage"
    >
      <ClusterStorageStep
        draft={flow.draft}
        views={storageViews(flow.draft, data)}
        registrations={registrations}
        sections={sections}
        context={{ environment, objects }}
        hosts={[]}
        conflicts={[]}
        onSection={(name, selections, errors) => {
          onSection(name, selections, errors);
          flow.set(["storage", name], selections);
          setCardErrors((current) => ({ ...current, [name]: errors }));
        }}
        deletion={clusterDeletion(flow.draft, data)}
        onDeletion={(policy) => {
          onDeletion(policy);
          flow.set(["claim", "spec", "deletionPolicy"], policy);
        }}
      />
    </WizardStepper>
  );
}

const meta = {
  title: "create/ClusterStorageStep",
  component: ClusterStorageStepHarness,
  args: {
    sections: storageSections,
    registrations: storageRegistrations,
    initial: {
      environment: "prod",
      namespace: { kind: "existing", name: "team-orders" },
      claim: { metadata: { name: "search" }, spec: { presetRef: "medium" } },
    },
    onSection: fn(),
    onDeletion: fn(),
  },
  render: (args) => (
    <ClusterStorageStepHarness key={JSON.stringify(args)} {...args} />
  ),
} satisfies Meta<typeof ClusterStorageStepHarness>;
export default meta;
type Story = StoryObj<typeof meta>;

export const OptionsWithAnUnavailableOneAndARequiredSection: Story = {
  name: "Options with an unavailable one and a required section",
  args: {
    environment: {
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
    },
  },
};

export const BuiltInStorageWithPresetCards: Story = {
  name: "Built-in storage with preset cards",
  args: {
    sections: storageSections.filter((s) =>
      ["backup-storage", "secondary-storage"].includes(s.metadata.name ?? ""),
    ),
    environment: {
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
    },
    objects: {
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
    },
  },
};
