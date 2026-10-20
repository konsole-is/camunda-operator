import type { Story } from "@/gallery/registry";
import { objectDiff } from "@/lib/diff";
import { DiffView } from "./diff-view";
import { GitWaitingLine } from "./git-waiting-line";
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
];
