"use client";

import { useEffect, useRef } from "react";
import type { Step } from "@/lib/fields";
import type { Resource } from "@/lib/k8s/kinds";
import { CLAIM_LABEL } from "@/lib/labels";
import { mergeCluster } from "@/lib/merge";
import type { ClusterSlotContext } from "../slots";
import type { ClaimPanel } from "../tabs";
import { ClaimEditorPanels } from "./claim-editor";
import {
  type ClaimActions,
  ClusterEditProvider,
  useClusterEdit,
} from "./cluster-edit";

const ACTIONS: ClaimActions = {
  preview: async () => ({ objects: [], errors: [] }),
  apply: async () => ({ ok: true, mode: "direct" }),
};

const release = (version: string) =>
  ({
    kind: "CamundaRelease",
    metadata: { name: `camunda-${version.replaceAll(".", "-")}` },
    spec: { version },
  }) as unknown as Resource<"CamundaRelease">;

const preset = (name: string) =>
  ({
    kind: "CamundaClusterPreset",
    metadata: { name },
    spec: { cluster: {} },
  }) as unknown as Resource<"CamundaClusterPreset">;

/**
 * StoryCluster changes the cluster and the claim of a story. version is the
 * reported version, none when empty. suspended suspends both.
 */
export type StoryCluster = {
  version?: string;
  suspended?: boolean;
  claimSpec?: Record<string, unknown>;
  over?: Partial<ClusterSlotContext>;
};

function contextOf({
  version = "8.9.3",
  suspended = false,
  claimSpec = {},
  over = {},
}: StoryCluster): ClusterSlotContext {
  const metadata = { namespace: "team-payments", name: "orders" };
  const spec = {
    presetRef: "medium",
    releaseRef: "camunda-8-9-3",
    zeebe: { replicas: 3 },
    ...(suspended && { suspend: true }),
  };
  const cluster = {
    apiVersion: "core.camunda.io/v1",
    kind: "CamundaCluster",
    metadata: { ...metadata, labels: { [CLAIM_LABEL]: "orders" } },
    spec,
    status: version ? { management: { version } } : {},
  } as unknown as Resource<"CamundaCluster">;
  const claim = {
    apiVersion: "fleet.camunda.io/v1alpha1",
    kind: "CamundaClusterClaim",
    metadata: { ...metadata, resourceVersion: "41" },
    spec: {
      presetRef: "medium",
      releaseRef: "camunda-8-9-3",
      cluster: { zeebe: { replicas: 3 } },
      ...(suspended && { suspend: true }),
      ...claimSpec,
    },
    status: { environment: "prod" },
  } as unknown as Resource<"CamundaClusterClaim">;
  const environment = {
    kind: "Environment",
    metadata: { name: "prod" },
    spec: {
      displayName: "Production",
      presets: { camundaCluster: ["medium", "large"] },
      releases: {
        allowed: ["camunda-8-9-1", "camunda-8-9-3", "camunda-8-11-0"],
        default: "camunda-8-9-3",
      },
    },
  } as unknown as Resource<"Environment">;
  const releases = [release("8.9.1"), release("8.9.3"), release("8.11.0")];
  const presets = [preset("medium"), preset("large")];
  return {
    cluster,
    claim,
    merged: mergeCluster(presets[0].spec, releases[1].spec, spec),
    environment,
    environments: [environment],
    presets,
    releases,
    platformConfigs: [],
    managementAuthConfigs: [],
    sections: [],
    registrations: [],
    requests: [],
    backups: [],
    restores: [],
    backupStorage: { kind: "none", schedules: [] },
    schedules: [],
    fields: [],
    writeMode: "direct",
    now: "2026-11-10T12:00:00Z",
    ...over,
  };
}

// Changes applies changes to the session of the page once, as a user would
// before the screenshot.
function Changes({ changes }: { changes: [Step[], unknown][] }) {
  const { set } = useClusterEdit();
  const done = useRef(false);
  useEffect(() => {
    if (done.current) return;
    done.current = true;
    for (const [steps, value] of changes) set(steps, value);
  }, [changes, set]);
  return null;
}

/**
 * ClaimEditorStory shows the Claim editor of the cluster orders in the
 * gallery, inside an edit host whose preview and write succeed. cluster
 * changes the cluster and its claim, panel opens one panel, and changes
 * are pending changes made after the first render.
 */
export function ClaimEditorStory({
  cluster = {},
  panel,
  changes = [],
}: {
  cluster?: StoryCluster;
  panel?: ClaimPanel;
  changes?: [Step[], unknown][];
}) {
  const context = contextOf(cluster);
  return (
    <div className="max-w-5xl">
      <ClusterEditProvider context={context} actions={ACTIONS}>
        <Changes changes={changes} />
        <ClaimEditorPanels context={context} requested={panel} />
      </ClusterEditProvider>
    </div>
  );
}
