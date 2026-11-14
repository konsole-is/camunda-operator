import { type ComponentProps, useMemo } from "react";
import { objectDiff } from "@/lib/diff";
import type { Step } from "@/lib/fields";
import { CLAIM_LABEL } from "@/lib/labels";
import {
  openSession,
  resetValue,
  setValue,
  type ClaimObject,
  type Session,
} from "@/lib/session";
import { ClaimReview } from "./claim-review";
import { Toast, useWriteToast } from "./toast";

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

/**
 * ClaimReviewHarness shows ClaimReview over a session of claim with changes,
 * where an undefined value is a reset, and shows the toast of the write
 * under it, as a page does.
 */
export function ClaimReviewHarness({
  claim,
  changes,
  show,
  ...props
}: Omit<ComponentProps<typeof ClaimReview>, "session" | "intent"> & {
  claim: ClaimObject;
  changes: { steps: Step[]; value?: unknown }[];
}) {
  // ClaimReview previews again for each new session.
  const session = useMemo(() => {
    let s: Session = openSession({
      claim,
      resource: {
        metadata: { labels: { [CLAIM_LABEL]: claim.metadata.name! } },
      },
      locks: () => undefined,
    })!;
    for (const { steps, value } of changes) {
      const next =
        value === undefined ? resetValue(s, steps) : setValue(s, steps, value);
      if ("session" in next) s = next.session;
    }
    return s;
  }, [claim, changes]);
  const toast = useWriteToast();
  return (
    <div className="flex flex-col gap-4">
      <ClaimReview
        {...props}
        session={session}
        intent={`Change the claim ${claim.metadata.name}`}
        show={(result, message) => {
          show(result, message);
          toast.show(result, message);
        }}
      />
      {toast.toast && <Toast result={toast.toast} onDismiss={toast.dismiss} />}
    </div>
  );
}
