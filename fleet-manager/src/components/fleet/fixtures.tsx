import { objectDiff } from "@/lib/diff";

export const now = "2026-10-21T12:00:00Z";

export const claimPush = {
  path: "clusters/prod/team-a/camundaclusterclaims/orders.yaml",
  revision: "b",
  pushedAt: "2026-10-21T11:58:00Z",
};

export const namespacePush = {
  path: "clusters/prod/_cluster/namespaces/team-a.yaml",
  revision: "a",
  pushedAt: "2026-10-21T11:58:00Z",
};

export const heading = (text: string) => (
  <h1 className="font-heading text-3xl font-bold tracking-[-0.02em]">{text}</h1>
);

export const ordersClaim = {
  apiVersion: "fleet.camunda.io/v1alpha1",
  kind: "CamundaClusterClaim",
  metadata: { name: "orders", namespace: "team-payments" },
  spec: { cluster: { zeebe: { replicas: 3, partitions: 3 } } },
};

export const scaleDiff = objectDiff(ordersClaim, {
  ...ordersClaim,
  spec: { cluster: { zeebe: { replicas: 5, partitions: 3 } } },
});
