import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { fn } from "storybook/test";
import type { Resource } from "@/lib/k8s/kinds";
import { presetFixtures } from "./fixtures";
import { RotateAction } from "./rotate-action";
import type { PresetSources } from "./sources";

const sources = presetFixtures();

function cluster(namespace: string, name: string): Resource<"CamundaCluster"> {
  return {
    ...sources.clusters[0],
    metadata: { namespace, name },
  } as Resource<"CamundaCluster">;
}

const three: PresetSources = {
  ...sources,
  clusters: [
    sources.clusters[0],
    cluster("team-billing", "billing"),
    cluster("team-search", "search"),
  ],
};

const meta = {
  title: "setup/presets/RotateAction",
  component: RotateAction,
  args: {
    preset: sources.clusterPresets[0],
    sources: three,
    unread: [],
    mode: "direct",
    disabled: false,
    actions: {
      previewChange: async () => ({
        objects: [
          {
            ref: {
              apiVersion: "core.camunda.io/v1",
              kind: "CamundaClusterPreset",
              name: "medium",
            },
            diff: [
              { op: "same", text: "auth:" },
              { op: "same", text: "  basic:" },
              { op: "added", text: "    passwordRotation: 2026-11-15T10:42Z" },
            ],
          },
        ],
        errors: [],
      }),
      applyChange: async () => ({ ok: true, mode: "direct" }),
    },
    show: fn(),
  },
} satisfies Meta<typeof RotateAction>;
export default meta;
type Story = StoryObj<typeof meta>;

export const ThreeClusters: Story = { name: "Three clusters" };

export const GitMode: Story = { name: "Git mode", args: { mode: "git" } };

export const NoClusterToRotate: Story = {
  name: "No cluster to rotate",
  args: { sources: { ...sources, clusters: [] } },
};

export const ClustersNotRead: Story = {
  name: "Clusters that cannot be read",
  args: { unread: ["CamundaCluster"] },
};
