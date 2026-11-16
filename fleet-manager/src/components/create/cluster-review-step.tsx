"use client";

import { CircleAlert } from "lucide-react";
import { useId } from "react";
import { Term, Terms } from "@/components/cluster/claim-tab";
import { DiffView } from "@/components/fleet/diff-view";
import { ReviewPanel } from "@/components/fleet/review-panel";
import { useStepperErrors } from "@/components/fleet/wizard-stepper";
import type { GrantPreview } from "@/lib/claims/templates";
import { objectDiff } from "@/lib/diff";
import type {
  Change,
  Preview,
  WriteMode,
  WriteResult,
} from "@/lib/write/types";
import { GrantPreviewList } from "./grant-preview";
import { useCreateReview } from "./use-create-review";

/**
 * ClusterReviewSummary is the summary of the steps before the review, one
 * group for each step in step order, each with its labelled values.
 */
export type ClusterReviewSummary = {
  label: string;
  terms: { label: string; value: string; note?: string }[];
}[];

// The kinds that the grant preview reads.
const GRANT_KINDS = ["Environment", "PluginSection", "PluginRegistration"];

/**
 * ClusterReviewStep is step 5 of the cluster create flow: the YAML of the
 * claim of change, the summary of each step, the YAML of the new namespace
 * when change creates one, the grant previews, effects, the refusals of the
 * dry run, and the confirm. Create stays off until the dry run of change
 * is clean, and while a write runs, while blocked, or while unread names a
 * kind that could not be read. Create gives the result of create to
 * onWritten. Back, beside Create, calls onCancel.
 */
export function ClusterReviewStep({
  change,
  summary,
  effects,
  grants,
  mode,
  preview,
  create,
  onWritten,
  onCancel,
  blocked = false,
  unread = [],
}: {
  change: Change;
  summary: ClusterReviewSummary;
  effects: string[];
  grants: readonly GrantPreview[];
  mode: WriteMode;
  preview: (change: Change) => Promise<Preview>;
  create: (change: Change) => Promise<WriteResult>;
  onWritten: (result: WriteResult) => void;
  onCancel: () => void;
  blocked?: boolean;
  unread?: readonly string[];
}) {
  const { refusals, pending, confirm } = useCreateReview({
    change,
    preview,
    create,
    onWritten,
    blocked: blocked || unread.length > 0,
  });
  const describedBy = useStepperErrors();
  const claim = change.apply.at(-1);
  const namespace =
    change.apply[0]?.kind === "Namespace" ? change.apply[0] : undefined;
  return (
    <ReviewPanel
      diffLabel="The claim"
      lines={objectDiff(undefined, claim)}
      details={
        <>
          {summary.map((group) => (
            <SummaryGroup key={group.label} {...group} />
          ))}
          {namespace && (
            <div className="flex min-w-0 flex-col gap-2">
              <h3 className="font-heading text-sm font-semibold">
                The new namespace
              </h3>
              <DiffView
                label="The namespace"
                lines={objectDiff(undefined, namespace)}
              />
            </div>
          )}
          <GrantPreviewList
            previews={grants}
            unread={unread.filter((kind) => GRANT_KINDS.includes(kind))}
          />
        </>
      }
      effects={effects}
      notices={
        <>
          {unread.length > 0 && (
            <p role="alert" className="text-sm text-bad">
              {`The fleet manager could not read the ${unread.join(", ")} objects. The review can miss what the create makes, so Create stays off.`}
            </p>
          )}
          {refusals.length > 0 && (
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
          )}
        </>
      }
      mode={mode}
      confirmLabel="Create"
      pending={pending}
      describedBy={describedBy}
      onConfirm={confirm}
      onCancel={onCancel}
    />
  );
}

function SummaryGroup({ label, terms }: ClusterReviewSummary[number]) {
  const id = useId();
  return (
    <section aria-labelledby={id} className="flex min-w-0 flex-col gap-2">
      <h3 id={id} className="font-heading text-sm font-semibold">
        {label}
      </h3>
      <Terms>
        {terms.map((term) => (
          <Term key={term.label} label={term.label} note={term.note}>
            {term.value}
          </Term>
        ))}
      </Terms>
    </section>
  );
}
