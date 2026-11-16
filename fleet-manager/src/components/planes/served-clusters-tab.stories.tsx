import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import type { ServedCluster } from "./served-clusters";
import { ServedClustersTab } from "./served-clusters-tab";

const row = (
  name: string,
  attached: boolean,
  extra: Partial<ServedCluster> = {},
): ServedCluster => ({
  namespace: `team-${name}`,
  name,
  environment: "production",
  attached,
  href: `/fleet/clusters/team-${name}/${name}`,
  ...extra,
});

const meta = {
  title: "planes/ServedClustersTab",
  component: ServedClustersTab,
} satisfies Meta<typeof ServedClustersTab>;
export default meta;
type Story = StoryObj<typeof meta>;

export const EachReason: Story = {
  name: "Each reason",
  args: {
    rows: [
      row("orders", true),
      row("modeler", true, {
        reason: "BasicAuthUserFailed",
        message:
          "The management plane serves the cluster, but the Web Modeler user on it is missing",
      }),
      row("billing", false, {
        reason: "ClaimedElsewhere",
        message:
          "The management plane platform/sandbox already serves this cluster",
      }),
      row("search", false, {
        reason: "NotReady",
        message: "The cluster publishes no gateway endpoints yet",
      }),
      row("ledger", false, {
        environment: "staging",
        reason: "InvalidReference",
        message:
          "The OIDC issuer of the cluster differs from the issuer of the management plane",
      }),
      row("reports", false, {
        reason: "WriteFailed",
        message:
          "The API server refused the write of the Console ping settings",
      }),
      row("lab", true, { environment: undefined }),
    ],
  },
};

export const NoClusters: Story = {
  name: "No clusters",
  args: { rows: [] },
};

export const PlaneNotRead: Story = {
  name: "Plane not read",
  args: { rows: [], unread: ["CamundaManagementCluster"] },
};
