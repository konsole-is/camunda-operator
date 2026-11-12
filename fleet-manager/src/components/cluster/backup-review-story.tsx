"use client";

import { ClaimReviewStory } from "@/components/fleet/claim-review-story";
import type { Step } from "@/lib/fields";
import type { Resource } from "@/lib/k8s/kinds";
import { CLAIM_LABEL } from "@/lib/labels";
import type { RollBackupLike } from "@/lib/policy/backups";
import { openSession, setValue, type Session } from "@/lib/session";
import type { WriteMode } from "@/lib/write/types";
import { backupReview } from "./backup-caution";

type Claim = Resource<"CamundaClusterClaim">;

const REFS = { presetRef: "medium", releaseRef: "camunda-8-9-5" };
const SPEC = {
  zeebe: { replicas: 3, storageSize: "32Gi" },
  gateway: { replicas: 2 },
};
const claim = {
  apiVersion: "fleet.camunda.io/v1alpha1",
  kind: "CamundaClusterClaim",
  metadata: {
    name: "orders",
    namespace: "team-payments",
    resourceVersion: "41",
  },
  spec: { ...REFS, cluster: SPEC },
} as unknown as Claim;
const live = {
  apiVersion: "core.camunda.io/v1",
  kind: "CamundaCluster",
  metadata: {
    name: "orders",
    namespace: "team-payments",
    labels: { [CLAIM_LABEL]: "orders" },
  },
  spec: { ...REFS, ...SPEC },
} as unknown as Resource<"CamundaCluster">;
const nightly = {
  kind: "LogicalBackupRDBMS",
  metadata: { namespace: "team-payments", name: "nightly-20261110" },
  spec: { clusterRef: { name: "orders" } },
  status: { phase: "Running" },
} as RollBackupLike;

const resources: Step[] = ["spec", "cluster", "zeebe", "resources"].map(
  (field) => ({ field }),
);
const deletionPolicy: Step[] = ["spec", "deletionPolicy"].map((field) => ({
  field,
}));

type Change = { steps: Step[]; value: unknown };

const CHANGES: Record<"rolls" | "keeps", Change[]> = {
  rolls: [{ steps: resources, value: { limits: { cpu: "2" } } }],
  keeps: [{ steps: deletionPolicy, value: "Orphan" }],
};

/**
 * BackupReviewStory shows ClaimReview with the props of backupReview for a
 * change of claim orders while backup nightly-20261110 runs. rolls changes
 * the resources of the brokers, keeps the deletion policy.
 */
export function BackupReviewStory({
  change,
  mode,
}: {
  change: "rolls" | "keeps";
  mode: WriteMode;
}) {
  // ClaimReview previews each new session, so changes keeps its identity.
  const changes = CHANGES[change];
  const [{ steps, value }] = changes;
  let session = openSession({
    claim,
    resource: { metadata: { labels: { [CLAIM_LABEL]: "orders" } } },
    locks: () => undefined,
  })!;
  const next = setValue(session, steps, value);
  if ("session" in next) session = next.session as Session<Claim>;
  const props = backupReview(
    {
      session,
      live,
      presets: [
        {
          apiVersion: "core.camunda.io/v1",
          kind: "CamundaClusterPreset",
          metadata: { name: "medium" },
          spec: { cluster: {} },
        } as unknown as Resource<"CamundaClusterPreset">,
      ],
      releases: [
        {
          apiVersion: "core.camunda.io/v1",
          kind: "CamundaRelease",
          metadata: { name: "camunda-8-9-5" },
          spec: { version: "8.9.5" },
        } as unknown as Resource<"CamundaRelease">,
      ],
      sections: [],
    },
    [nightly],
    mode,
    { backupsHref: "?tab=backups" },
  );
  return (
    <ClaimReviewStory claim={claim} changes={changes} mode={mode} {...props} />
  );
}
