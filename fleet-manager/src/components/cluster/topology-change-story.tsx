"use client";

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

type Fixture = {
  spec: Record<string, unknown>;
  picks: [ModeProcess, ComponentMode][];
  locks?: LockProvider;
};

const FIXTURES = {
  embedded: { spec: {}, picks: [["gateway", "Embedded"]] },
  standalone: {
    spec: {
      operate: { replicas: 2, resources: { requests: { cpu: "1" } } },
    },
    picks: [["operate", "Standalone"]],
  },
  locked: {
    spec: { connectors: { enabled: true } },
    picks: [["tasklist", "Standalone"]],
    locks: (path) =>
      path === "spec.cluster.gateway.mode" ? { reason: "admin" } : undefined,
  },
} satisfies Record<string, Fixture>;

function input({ spec, picks, locks }: Fixture): ClusterChangeInput {
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
    locks: locks ?? (() => undefined),
  })!;
  for (const [p, mode] of picks) {
    const next = setValue(session, modeSteps(p), mode);
    if ("session" in next) session = next.session as Session<Claim>;
  }
  return { session, live, presets: [], releases: [], sections: [] };
}

/**
 * TopologyChangeStory shows TopologyChangePanel for a change of the cluster
 * orders: embedded moves the gateway into the brokers, standalone gives
 * Operate its own Deployment with two prepared fields, and locked has a
 * gateway mode that the admin set.
 */
export function TopologyChangeStory({
  fixture,
}: {
  fixture: keyof typeof FIXTURES;
}) {
  const i = input(FIXTURES[fixture] as Fixture);
  return (
    <TopologyChangePanel
      change={topologyChange(i)}
      impacts={clusterImpacts(i)}
      others={false}
      onPick={() => {}}
    />
  );
}
