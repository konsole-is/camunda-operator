import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import type { CamundaClusterPresetSpec } from "@/generated/crds/camundaclusterpreset";
import { presetFixtures } from "./fixtures";
import { presetImpact } from "./impact";
import { ImpactPreview } from "./impact-preview";

const sources = presetFixtures();
const medium = sources.clusterPresets[0];
const es = sources.elasticsearchPresets[0];

function after(
  change: (c: CamundaClusterPresetSpec["cluster"]) => void,
): CamundaClusterPresetSpec {
  const spec = structuredClone(medium.spec);
  change(spec.cluster);
  return spec;
}

const cluster = (spec: CamundaClusterPresetSpec) =>
  presetImpact({
    kind: "CamundaClusterPreset",
    name: "medium",
    before: medium.spec,
    after: spec,
    sources,
  });

const meta = {
  title: "setup/presets/ImpactPreview",
  component: ImpactPreview,
  args: {
    impact: cluster(
      after((c) => {
        c.zeebe = { ...c.zeebe, replicas: 5 };
      }),
    ),
  },
} satisfies Meta<typeof ImpactPreview>;
export default meta;
type Story = StoryObj<typeof meta>;

export const PodsRoll: Story = { name: "Pods roll" };

export const ConnectorsRestart: Story = {
  name: "Connectors restart",
  args: {
    impact: presetImpact({
      kind: "CamundaClusterPreset",
      name: "medium",
      before: medium.spec,
      after: after((c) => {
        c.auth = {
          basic: { ...c.auth?.basic, passwordRotation: "2026-11-15T10:42Z" },
        };
      }),
      sources: {
        ...sources,
        clusters: sources.clusters.map((c) => ({
          ...c,
          spec: { ...c.spec, connectors: { enabled: true } },
        })),
      },
    }),
  },
};

export const Unchanged: Story = {
  name: "Unchanged",
  args: { impact: cluster(medium.spec) },
};

export const PodsMayRoll: Story = {
  name: "Pods may roll",
  args: {
    impact: presetImpact({
      kind: "ElasticsearchClusterPreset",
      name: "standard",
      before: es.spec,
      after: { cluster: { ...es.spec.cluster, replicas: 5 } },
      sources: {
        ...sources,
        elasticsearchClusters: [
          {
            apiVersion: "core.camunda.io/v1",
            kind: "ElasticsearchCluster",
            metadata: { namespace: "search", name: "logs" },
            spec: { presetRef: "standard" },
          } as (typeof sources.elasticsearchClusters)[number],
        ],
      },
    }),
  },
};

export const TheTwoStorageRules: Story = {
  name: "The two storage rules",
  args: {
    impact: cluster(
      after((c) => {
        c.zeebe = { ...c.zeebe, storageSize: "16Gi", storageClassName: "gp3" };
      }),
    ),
  },
};

export const UsersNotRead: Story = {
  name: "Users that cannot be read",
  args: {
    impact: presetImpact({
      kind: "CamundaClusterPreset",
      name: "medium",
      before: medium.spec,
      after: medium.spec,
      sources,
      unread: ["CamundaCluster"],
    }),
  },
};
