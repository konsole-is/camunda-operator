import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { ImpactLine } from "./impact-line";

const orders = {
  kind: "CamundaClusterClaim" as const,
  namespace: "team-a",
  name: "orders",
};

const meta = {
  title: "setup/environments/ImpactLine",
  component: ImpactLine,
} satisfies Meta<typeof ImpactLine>;
export default meta;
type Story = StoryObj<typeof meta>;

export const Warnings: Story = {
  name: "Warnings",
  args: {
    impact: {
      clusters: 2,
      planes: 1,
      warnings: [
        {
          claim: orders,
          kind: "CamundaRelease",
          name: "8.9.22",
          removed: true,
        },
        {
          claim: { ...orders, name: "billing" },
          kind: "CamundaClusterPreset",
          name: "large",
          removed: true,
        },
        {
          claim: { ...orders, name: "search" },
          kind: "CamundaRelease",
          name: "8.9.23",
          followsDefault: true,
        },
        {
          claim: { ...orders, name: "search" },
          kind: "ElasticsearchClusterPreset",
          name: "es-small",
          removed: true,
        },
      ],
    },
  },
};
