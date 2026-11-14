import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { useState } from "react";
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

type Pick = [ModeProcess, ComponentMode];

// propsOf gives the props of the panel for a change of the cluster orders
// with spec, after the user picked the modes of picks.
function propsOf(
  spec: Record<string, unknown>,
  picks: Pick[],
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

// TopologyChangePanelHarness keeps the picks of the user, as the edit
// session of the page does, so that a pick changes the panel.
function TopologyChangePanelHarness({
  spec,
  picks,
  locks,
  onPick,
}: {
  spec: Record<string, unknown>;
  picks: Pick[];
  locks?: LockProvider;
  onPick: (process: ModeProcess, mode: ComponentMode) => void;
}) {
  const [current, setCurrent] = useState(picks);
  return (
    <TopologyChangePanel
      {...propsOf(spec, current, locks)}
      onPick={(process, mode) => {
        onPick(process, mode);
        setCurrent((was) => [
          ...was.filter(([p]) => p !== process),
          [process, mode],
        ]);
      }}
    />
  );
}

const meta = {
  title: "cluster/TopologyChangePanel",
  component: TopologyChangePanelHarness,
  args: { onPick: fn() },
  render: (args) => (
    <TopologyChangePanelHarness
      key={JSON.stringify([args.spec, args.picks])}
      {...args}
    />
  ),
} satisfies Meta<typeof TopologyChangePanelHarness>;
export default meta;
type Story = StoryObj<typeof meta>;

export const GatewayBecomesEmbedded: Story = {
  name: "Gateway becomes embedded",
  args: { spec: {}, picks: [["gateway", "Embedded"]] },
};

export const OperateBecomesStandaloneWithPreparedSettings: Story = {
  name: "Operate becomes standalone with prepared settings",
  args: {
    spec: {
      operate: { replicas: 2, resources: { requests: { cpu: "1" } } },
    },
    picks: [["operate", "Standalone"]],
  },
};

export const LockedMode: Story = {
  name: "Locked mode",
  args: {
    spec: { connectors: { enabled: true } },
    picks: [["tasklist", "Standalone"]],
    locks: (path) =>
      path === "spec.cluster.gateway.mode" ? { reason: "admin" } : undefined,
  },
};
