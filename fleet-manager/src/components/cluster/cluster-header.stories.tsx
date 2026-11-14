import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { ClusterHeader } from "./cluster-header";
import { launchLinks } from "./launch";

const meta = {
  title: "cluster/ClusterHeader",
  component: ClusterHeader,
} satisfies Meta<typeof ClusterHeader>;
export default meta;
type Story = StoryObj<typeof meta>;

export const ReadyWithLaunchLinks: Story = {
  name: "Ready with launch links",
  args: {
    name: "orders",
    namespace: "team-a",
    ready: { status: "True", reason: "Healthy" },
    environment: "Production",
    version: "8.9.23",
    preset: "medium",
    storage: "elasticsearch",
    links: launchLinks("https://orders.example.com", undefined),
  },
};

export const NotReadyWithoutExternalUrl: Story = {
  name: "Not ready without external URL",
  args: {
    name: "payments",
    namespace: "team-b",
    ready: {
      status: "False",
      reason: "ZeebeNotReady",
      message: "1 of 3 Zeebe brokers are ready",
    },
    storage: "rdbms",
    links: [],
  },
};
