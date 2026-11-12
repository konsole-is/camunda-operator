"use client";

import type { ReactNode } from "react";
import type { ReviewConfirm } from "@/components/fleet/claim-review";
import type { Session } from "@/lib/session";
import type { WriteMode } from "@/lib/write/types";
import { type BackupsLink, backupReview } from "../backup-caution";
import type { ClusterSlotContext } from "../slots";
import { changeInputOf } from "./change-input";
import type { ClusterClaim } from "./cluster-edit";
import { downgradeOf } from "./downgrade";

const DOWNGRADE_CONFIRM: ReviewConfirm = {
  danger: true,
  label: "Downgrade",
  gitLabel: "Push downgrade",
};

/**
 * reviewGuards returns the props of ClaimReview that guard the review of
 * session: the impacts, the caution, and the confirm of backupReview, with
 * the downgrade caution before the backup caution. The confirm of a backup
 * caution wins over the one of a downgrade. Only a client component can
 * call it.
 */
export function reviewGuards(
  context: ClusterSlotContext,
  session: Session<ClusterClaim>,
  mode: WriteMode,
  link: BackupsLink,
): { impacts: string[]; caution?: ReactNode; confirm?: ReviewConfirm } {
  const backup = backupReview(
    changeInputOf(context, session),
    context.backups,
    mode,
    link,
  );
  const downgrade = downgradeOf(context, session);
  if (!downgrade) return backup;
  return {
    impacts: backup.impacts,
    caution: (
      <div className="flex flex-col gap-3">
        <p>
          Camunda does not support a downgrade of a running cluster. This change
          moves the cluster from {downgrade.from} to {downgrade.to}.
        </p>
        {backup.caution}
      </div>
    ),
    confirm: backup.confirm ?? DOWNGRADE_CONFIRM,
  };
}
