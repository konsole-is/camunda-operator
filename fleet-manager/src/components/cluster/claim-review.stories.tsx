import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { fn } from "storybook/test";
import { ClaimReviewHarness } from "@/components/fleet/fixtures";
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

// ROLLS changes the resources of the brokers, so they roll. KEEPS changes
// only the deletion policy.
const ROLLS: Change[] = [{ steps: resources, value: { limits: { cpu: "2" } } }];
const KEEPS: Change[] = [{ steps: deletionPolicy, value: "Orphan" }];

// argsOf gives the review of changes to the claim orders in mode while the
// backup nightly-20261110 runs.
function argsOf(changes: Change[], mode: WriteMode) {
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
    // A plain link to ?tab=backups would replace the story in its frame.
    { backupsHref: "?tab=backups", onOpenBackups: fn() },
  );
  return { claim, changes, mode, ...props };
}

const meta = {
  title: "cluster/ClaimReview",
  component: ClaimReviewHarness,
  args: {
    preview: fn(async () => ({ objects: [], errors: [] })),
    apply: fn(async () => ({ ok: true as const, mode: "direct" as const })),
    show: fn(),
    onWritten: fn(),
    onRefused: fn(),
    onCancel: fn(),
  },
} satisfies Meta<typeof ClaimReviewHarness>;
export default meta;
type Story = StoryObj<typeof meta>;

export const BackupCaution: Story = {
  name: "Backup caution",
  args: argsOf(ROLLS, "direct"),
};

export const BackupCautionInGitMode: Story = {
  name: "Backup caution in Git mode",
  args: argsOf(ROLLS, "git"),
};

export const BackupDoesNotRoll: Story = {
  name: "Backup does not roll",
  args: argsOf(KEEPS, "direct"),
};
