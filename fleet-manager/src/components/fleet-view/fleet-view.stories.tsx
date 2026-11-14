import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { fn } from "storybook/test";
import type { PendingPush } from "@/components/fleet/git-pushes";
import type { PodWarnings } from "@/lib/warnings";
import { FleetView } from "./fleet-view";
import { NOW, empty, environment, fleet, planes } from "./fixtures";

const pushes: PendingPush[] = [
  {
    key: "CamundaClusterClaim/team-search/payments",
    ref: {
      kind: "CamundaClusterClaim",
      namespace: "team-search",
      name: "payments",
    },
    path: "clusters/team-search/payments.yaml",
    revision: "3f2c1a",
    pushedAt: "2026-11-10T11:58:00Z",
  },
];

const podWarnings: PodWarnings = {
  clusters: {
    "team-orders/orders": [
      {
        kind: "restarts",
        severity: "Warning",
        label: "Restarts",
        detail: "Container zeebe of pod orders-zeebe-2 restarted 6 times",
        since: "2026-11-10T11:41:00Z",
      },
      {
        kind: "memory",
        severity: "Error",
        label: "Near memory limit",
        detail: "Pod orders-zeebe-2 uses 93 % of its memory limit",
      },
      {
        kind: "backpressure",
        severity: "Warning",
        label: "Backpressure",
        detail:
          "4 % of the requests were dropped by backpressure in the last 5 minutes",
      },
    ],
    "team-orders/returns": [
      {
        kind: "oom",
        severity: "Error",
        label: "Out of memory",
        detail: "Container zeebe of pod returns-zeebe-1 ran out of memory",
        since: "2026-11-10T11:48:00Z",
      },
    ],
  },
  missing: [],
};

const meta = {
  title: "fleet-view/FleetView",
  component: FleetView,
  args: {
    sources: fleet,
    planes,
    pushes: [],
    now: NOW,
    onStopWaiting: fn(),
  },
} satisfies Meta<typeof FleetView>;
export default meta;
type Story = StoryObj<typeof meta>;

export const Warnings: Story = { name: "Warnings", args: { podWarnings } };

export const MissingSources: Story = {
  name: "Missing sources",
  args: {
    podWarnings: { ...podWarnings, missing: ["metrics-server", "prometheus"] },
  },
};

export const Environments: Story = { name: "Environments" };

export const LongEnvironmentNames: Story = {
  name: "Long environment names",
  args: {
    sources: {
      ...fleet,
      environments: [
        environment(
          "production",
          "Production in the European Union, west region, blue and green",
          "#3dd68c",
        ),
        environment(
          "staging",
          "Staging for the payments and the orders teams, shared cluster",
          "#ffc14d",
        ),
        environment("development", "Development", "#5b9bff"),
      ],
    },
  },
};

export const NoEnvironments: Story = {
  name: "No environments",
  args: { sources: { ...fleet, environments: [] }, planes: [] },
};

export const WaitingForGitOps: Story = {
  name: "Waiting for GitOps",
  args: { pushes },
};

export const EmptyFleet: Story = {
  name: "Empty fleet",
  args: { sources: empty, planes: [] },
};
