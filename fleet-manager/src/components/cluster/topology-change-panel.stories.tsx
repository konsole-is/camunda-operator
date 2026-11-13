import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { fn } from "storybook/test";
import type { Resource } from "@/lib/k8s/kinds";
import { CLAIM_LABEL } from "@/lib/labels";
import {
  openSession,
  setValue,
  type LockProvider,
  type Session,
} from "@/lib/session";
import { clusterImpacts, type ClusterChangeInput } from "./impacts";
import type { ComponentMode } from "./topology";
import { modeSteps, topologyChange, type ModeProcess } from "./topology-change";
import { TopologyChangePanel } from "./topology-editor";

type Claim = Resource<"CamundaClusterClaim">;

// argsOf gives the props of the panel for a change of the cluster orders
// with spec, after the user picked the modes of picks.
function argsOf(
  spec: Record<string, unknown>,
  picks: [ModeProcess, ComponentMode][],
  locks: LockProvider = () => undefined,
) {
  const metadata = { name: "orders", namespace: "team-payments" };
  const live = {
    apiVersion: "core.camunda.io/v1",
    kind: "CamundaCluster",
    metadata: { ...metadata, labels: { [CLAIM_LABEL]: "orders" } },
    spec,
  } as unknown as Resource<"CamundaCluster">;
  let session = openSession({
    claim: {
      apiVersion: "fleet.camunda.io/v1alpha1",
      kind: "CamundaClusterClaim",
      metadata: { ...metadata, resourceVersion: "41" },
      spec: { cluster: spec },
    } as unknown as Claim,
    resource: live,
    locks,
  })!;
  for (const [p, mode] of picks) {
    const next = setValue(session, modeSteps(p), mode);
    if ("session" in next) session = next.session as Session<Claim>;
  }
  const input: ClusterChangeInput = {
    session,
    live,
    presets: [],
    releases: [],
    sections: [],
  };
  return {
    change: topologyChange(input),
    impacts: clusterImpacts(input),
    others: false,
  };
}

const meta = {
  title: "cluster/TopologyChangePanel",
  component: TopologyChangePanel,
  args: { onPick: fn() },
} satisfies Meta<typeof TopologyChangePanel>;
export default meta;
type Story = StoryObj<typeof meta>;

export const GatewayBecomesEmbedded: Story = {
  name: "Gateway becomes embedded",
  args: argsOf({}, [["gateway", "Embedded"]]),
};

export const OperateBecomesStandaloneWithPreparedSettings: Story = {
  name: "Operate becomes standalone with prepared settings",
  args: argsOf(
    { operate: { replicas: 2, resources: { requests: { cpu: "1" } } } },
    [["operate", "Standalone"]],
  ),
};

export const LockedMode: Story = {
  name: "Locked mode",
  args: argsOf(
    { connectors: { enabled: true } },
    [["tasklist", "Standalone"]],
    (path) =>
      path === "spec.cluster.gateway.mode" ? { reason: "admin" } : undefined,
  ),
};
