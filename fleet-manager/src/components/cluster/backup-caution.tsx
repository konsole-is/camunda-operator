"use client";

import type { ReactNode } from "react";
import type { ReviewConfirm } from "@/components/fleet/claim-review";
import type { RollBackupLike } from "@/lib/policy/backups";
import type { WriteMode } from "@/lib/write/types";
import { backupWarning } from "./backup-warning";
import { clusterImpacts, type ClusterChangeInput } from "./impacts";

/**
 * BackupsLink is the link to the Backups tab of the cluster. With
 * onOpenBackups, a click calls it in place of the navigation.
 */
export type BackupsLink = { backupsHref: string; onOpenBackups?: () => void };

const CAUTION = "CAUTION:";

/**
 * BackupCaution is the body of the caution of a change that fails a running
 * backup: the lines of backupWarning, then the link to the Backups tab. The
 * caution box of ReviewPanel draws its frame.
 */
export function BackupCaution({
  lines,
  backupsHref,
  onOpenBackups,
}: { lines: string[] } & BackupsLink) {
  return (
    <div className="flex flex-col gap-1">
      {lines.map((line, i) => (
        <p key={i}>
          {line.startsWith(CAUTION) ? (
            <>
              <span className="font-semibold">{CAUTION}</span>
              {line.slice(CAUTION.length)}
            </>
          ) : (
            line
          )}
        </p>
      ))}
      <a
        href={backupsHref}
        className="self-start font-medium text-text underline underline-offset-4 hover:text-accent"
        onClick={
          onOpenBackups
            ? (event) => {
                event.preventDefault();
                onOpenBackups();
              }
            : undefined
        }
      >
        Open the Backups tab
      </a>
    </div>
  );
}

/**
 * backupReview gives the props of ClaimReview that the running backup of the
 * cluster decides: the impacts of the change, with the muted line of the
 * backup at the end, and for a caution the caution and the danger confirm.
 * A host spreads the result into ClaimReview. Only a client component can
 * call it.
 */
export function backupReview(
  input: ClusterChangeInput,
  backups: readonly RollBackupLike[],
  mode: WriteMode,
  link: BackupsLink,
): { impacts: string[]; caution?: ReactNode; confirm?: ReviewConfirm } {
  const impacts = clusterImpacts(input);
  const warning = backupWarning(input, backups, mode);
  switch (warning.kind) {
    case "none":
      return { impacts };
    case "note":
      return { impacts: [...impacts, warning.note] };
    case "caution":
      return {
        impacts,
        caution: <BackupCaution lines={warning.lines} {...link} />,
        confirm: warning.confirm,
      };
  }
}
