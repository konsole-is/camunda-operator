"use client";

import { CircleAlert } from "lucide-react";
import { useCreateReview } from "@/components/create/use-create-review";
import { ReviewPanel } from "@/components/fleet/review-panel";
import { useStepperErrors } from "@/components/fleet/wizard-stepper";
import { objectDiff } from "@/lib/diff";
import {
  type PlaneContext,
  type PlaneCreateClaim,
  type PlaneDraft,
} from "@/lib/planes/draft";
import {
  accessSelection,
  contractNameOf,
  databaseSection,
  planeSelections,
} from "@/lib/planes/storage";
import type {
  Change,
  Preview,
  WriteMode,
  WriteResult,
} from "@/lib/write/types";

/** ATTACH_CAUTION is the caution of the review: what a cluster that attaches to the plane goes through. */
export const ATTACH_CAUTION =
  "Each cluster that attaches restarts its pods once, because it gets the Console settings.";

/**
 * PlaneReviewStep is step 5 of the plane create flow: the YAML of claim,
 * what change creates, and the confirm. It previews change once for each
 * change, and Create stays disabled until that preview is clean and while
 * a write runs or while blocked. Create sends change to create once and
 * gives the result to onWritten, also when the write is refused. With
 * onBack, Back sits beside Create.
 */
export function PlaneReviewStep({
  change,
  claim,
  draft,
  ctx,
  mode,
  preview,
  create,
  onWritten,
  blocked = false,
  onBack,
}: {
  change: Change;
  claim: PlaneCreateClaim;
  draft: PlaneDraft;
  ctx: PlaneContext;
  mode: WriteMode;
  preview: (change: Change) => Promise<Preview>;
  create: (change: Change) => Promise<WriteResult>;
  onWritten: (result: WriteResult) => void;
  blocked?: boolean;
  onBack?: () => void;
}) {
  const { refusals, pending, confirm } = useCreateReview({
    change,
    preview,
    create,
    onWritten,
    blocked,
  });
  const describedBy = useStepperErrors();

  return (
    <ReviewPanel
      diffLabel="The claim"
      lines={objectDiff(undefined, claim)}
      effects={effectsOf(draft, ctx, claim)}
      caution={draft.console ? ATTACH_CAUTION : undefined}
      notices={
        refusals.length > 0 && (
          <div
            role="alert"
            className="flex gap-3 rounded-md border border-bad/40 bg-bad/10 p-3 text-sm text-text"
          >
            <CircleAlert
              aria-hidden
              className="mt-0.5 size-4 shrink-0 text-bad"
            />
            <ul className="flex flex-col gap-1">
              {refusals.map((text, i) => (
                <li key={i}>{text}</li>
              ))}
            </ul>
          </div>
        )
      }
      mode={mode}
      confirmLabel="Create"
      pending={pending}
      describedBy={describedBy}
      onConfirm={confirm}
      onCancel={onBack}
    />
  );
}

function effectsOf(
  draft: PlaneDraft,
  ctx: PlaneContext,
  claim: PlaneCreateClaim,
): string[] {
  const { name, namespace } = claim.metadata;
  const label = (registration: string | undefined) =>
    ctx.registrations.find((r) => r.metadata.name === registration)?.spec
      .label ?? registration;
  const databases = databaseSection(ctx.sections)?.metadata.name;
  const database = planeSelections(draft).find((p) => p.section === databases);
  const access = accessSelection(draft);
  return [
    ...(draft.namespace?.kind === "new"
      ? [`Creates the namespace ${namespace}.`]
      : []),
    `Creates the claim ${name}.`,
    ...(database
      ? [`Requests the databases: ${label(database.registration)}.`]
      : []),
    ...(access ? [`Requests access: ${label(access.registration)}.`] : []),
    `Creates the CamundaManagementCluster ${name}.`,
    `The plane writes the contract ${contractNameOf(draft)}.`,
  ];
}
