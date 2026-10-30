import type { Story } from "@/gallery/registry";
import { objectDiff } from "@/lib/diff";
import type { Step } from "@/lib/fields";
import { STALE_CLAIM_MESSAGE } from "@/lib/session";
import { ClaimReviewStory } from "./claim-review-story";
import { DiffView } from "./diff-view";
import { GitWaitingLine } from "./git-waiting-line";
import { PrecheckLine } from "./precheck-line";
import { ReviewPanel } from "./review-panel";

const before = {
  apiVersion: "fleet.camunda.io/v1alpha1",
  kind: "CamundaClusterClaim",
  metadata: { name: "orders", namespace: "team-payments" },
  spec: { cluster: { zeebe: { replicas: 3, partitions: 3 } } },
};
const after = {
  ...before,
  spec: { cluster: { zeebe: { replicas: 5, partitions: 3 } } },
};
const lines = objectDiff(before, after);

const claim = {
  ...before,
  metadata: { ...before.metadata, resourceVersion: "41" },
};
const replicas: Step[] = [
  { field: "spec" },
  { field: "cluster" },
  { field: "zeebe" },
  { field: "replicas" },
];
const scale = [{ steps: replicas, value: 5 }];
const restart = ["Broker pods restart."];

export const reviewStories: Story[] = [
  {
    component: "DiffView",
    name: "Changed field",
    render: () => <DiffView label="Claim orders" lines={lines} />,
  },
  {
    component: "DiffView",
    name: "New object",
    render: () => (
      <DiffView label="Claim orders" lines={objectDiff(undefined, before)} />
    ),
  },
  {
    component: "DiffView",
    name: "No changes",
    render: () => (
      <DiffView label="Claim orders" lines={objectDiff(before, before)} />
    ),
  },
  {
    component: "ReviewPanel",
    name: "Direct mode",
    render: () => (
      <ReviewPanel
        lines={lines}
        mode="direct"
        confirmLabel="Change the claim"
        effects={[
          "Zeebe gets two more brokers.",
          "The partitions move to the new brokers.",
        ]}
      />
    ),
  },
  {
    component: "ReviewPanel",
    name: "Git mode",
    render: () => (
      <ReviewPanel
        lines={lines}
        mode="git"
        confirmLabel="Change the claim"
        effects={["Zeebe gets two more brokers."]}
      />
    ),
  },
  {
    component: "ReviewPanel",
    name: "With caution",
    render: () => (
      <ReviewPanel
        lines={lines}
        mode="direct"
        danger
        confirmLabel="Change anyway"
        effects={["Zeebe gets two more brokers."]}
        caution="A backup of orders runs now. The rollout makes it fail. A backup that starts after the change waits until the rollout is done."
      />
    ),
  },
  {
    component: "GitWaitingLine",
    name: "Waiting",
    render: () => (
      <GitWaitingLine
        pushed={{
          path: "clusters/team-a/camundaclusterclaims/orders.yaml",
          revision: "bbb",
          pushedAt: "2026-10-18T11:58:00Z",
        }}
        liveRevision="aaa"
        now="2026-10-18T12:00:00Z"
      />
    ),
  },
  {
    component: "GitWaitingLine",
    name: "Waiting for a delete",
    render: () => (
      <GitWaitingLine
        pushed={{
          path: "clusters/team-a/camundaclusterclaims/orders.yaml",
          pushedAt: "2026-10-18T11:58:00Z",
          deleted: true,
        }}
        now="2026-10-18T12:00:00Z"
      />
    ),
  },
  {
    component: "ClaimReview",
    name: "Direct",
    render: () => (
      <ClaimReviewStory
        claim={claim}
        changes={scale}
        mode="direct"
        impacts={restart}
      />
    ),
  },
  {
    component: "ClaimReview",
    name: "Git",
    render: () => (
      <ClaimReviewStory
        claim={claim}
        changes={scale}
        mode="git"
        impacts={restart}
      />
    ),
  },
  {
    component: "ClaimReview",
    name: "With caution",
    render: () => (
      <ClaimReviewStory
        claim={claim}
        changes={scale}
        mode="direct"
        impacts={restart}
        caution="A backup of orders runs now. The rollout makes it fail."
      />
    ),
  },
  {
    component: "ClaimReview",
    name: "Preview refused",
    render: () => (
      <ClaimReviewStory
        claim={claim}
        changes={[{ steps: replicas, value: 0 }]}
        mode="direct"
        preview={{
          objects: [],
          errors: [
            {
              code: "Invalid",
              message: "Invalid value: 0: must be greater than or equal to 1",
              path: "spec.cluster.zeebe.replicas",
            },
          ],
        }}
      />
    ),
  },
  {
    component: "ClaimReview",
    name: "Stale",
    render: () => (
      <ClaimReviewStory
        claim={claim}
        changes={scale}
        mode="direct"
        impacts={restart}
        preview={{
          objects: [],
          errors: [{ code: "Stale", message: STALE_CLAIM_MESSAGE }],
        }}
      />
    ),
  },
  {
    component: "PrecheckLine",
    name: "SlotConflict",
    render: () => (
      <PrecheckLine
        claim={{
          status: {
            conditions: [
              {
                type: "Ready",
                status: "False",
                reason: "SlotConflict",
                message:
                  'spec.cluster.externalUrl is filled by section "access", so the claim cannot set it',
              },
            ],
          },
        }}
      />
    ),
  },
];
