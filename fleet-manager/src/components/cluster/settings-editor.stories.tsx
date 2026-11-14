import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { fn } from "storybook/test";
import type { CamundaClusterSpec } from "@/generated/crds/camundacluster";
import { clusterFields } from "@/lib/fields";
import type { Resource } from "@/lib/k8s/kinds";
import { CLAIM_LABEL } from "@/lib/labels";
import { mergeCluster } from "@/lib/merge";
import { settingsOf } from "./editable-settings";
import { secrets } from "./fixtures";
import { SettingsEditor } from "./settings-editor";
import { componentSettings } from "./settings";
import type { Process } from "./topology";

const preset: CamundaClusterSpec = {
  zeebe: {
    replicas: 3,
    partitions: 3,
    storageSize: "32Gi",
    resources: {
      requests: { cpu: "1", memory: "4Gi" },
      limits: { memory: "4Gi" },
    },
  },
  gateway: { replicas: 2 },
};

type State = {
  host: Process | "optimize";
  process: Process | "optimize";
  cluster: CamundaClusterSpec;
  live?: CamundaClusterSpec;
  optimize?: {
    managementAuthConfigRef: string;
    importer: { replicas: number };
  };
};

// argsOf gives the settings of the process of state for the cluster orders,
// whose claim sets the cluster of state and live adds to the cluster.
function argsOf({ host, process, cluster, live, optimize }: State) {
  const spec = { ...cluster, ...live };
  const merged = mergeCluster({ cluster: preset }, undefined, spec);
  const fields = clusterFields({
    cluster: {
      kind: "CamundaCluster",
      metadata: {
        name: "orders",
        namespace: "team-a",
        labels: { [CLAIM_LABEL]: "orders" },
      },
      spec,
    } as unknown as Resource<"CamundaCluster">,
    merged,
    claim: {
      kind: "CamundaClusterClaim",
      metadata: { name: "orders", namespace: "team-a" },
      spec: {
        presetRef: "medium",
        cluster,
        ...(optimize && { attachments: { optimize } }),
      },
    } as unknown as Resource<"CamundaClusterClaim">,
    sections: [],
    registrations: [],
    optimize:
      optimize &&
      ({
        kind: "CamundaOptimize",
        metadata: { name: "orders", namespace: "team-a" },
        spec: {
          clusterRef: { name: "orders" },
          managementAuthRef: optimize.managementAuthConfigRef,
          importer: optimize.importer,
        },
      } as unknown as Resource<"CamundaOptimize">),
  });
  const groups = componentSettings(fields, host, merged.spec);
  const group = groups.find((g) => g.component === process) ?? groups[0];
  return {
    group,
    settings: settingsOf({
      group,
      spec: process === "optimize" ? optimize : merged.spec,
    }),
  };
}

const meta = {
  title: "cluster/SettingsEditor",
  component: SettingsEditor,
  args: { secrets, set: fn(), reset: fn() },
  // The fields keep their first value, so a change of the args remounts
  // the editor.
  render: (args) => (
    <SettingsEditor
      key={JSON.stringify([args.group, args.settings])}
      {...args}
    />
  ),
} satisfies Meta<typeof SettingsEditor>;
export default meta;
type Story = StoryObj<typeof meta>;

export const BrokersFromThePreset: Story = {
  name: "Brokers from the preset",
  args: argsOf({
    host: "zeebe",
    process: "zeebe",
    cluster: {
      zeebe: { replicas: 5 },
      extraEnv: [{ name: "TZ", value: "UTC" }],
    },
  }),
};

export const EmbeddedTasklist: Story = {
  name: "Embedded Tasklist",
  args: argsOf({
    host: "gateway",
    process: "tasklist",
    cluster: { gateway: { mode: "Standalone" }, tasklist: { replicas: 1 } },
  }),
};

export const LockedFields: Story = {
  name: "Locked fields",
  args: argsOf({
    host: "connectors",
    process: "connectors",
    cluster: {},
    live: {
      connectors: {
        version: "8.9.23",
        serviceAccount: { name: "orders-connectors" },
      },
    },
  }),
};

export const Optimize: Story = {
  name: "Optimize",
  args: argsOf({
    host: "optimize",
    process: "optimize",
    cluster: {},
    optimize: {
      managementAuthConfigRef: "keycloak",
      importer: { replicas: 1 },
    },
  }),
};
