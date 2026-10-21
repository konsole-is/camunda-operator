"use client";

import type { BackupRef, RestoreRef } from "@/lib/policy/backups";

const BANNER =
  "rounded-md border border-l-4 border-border bg-panel px-4 py-3 text-sm text-text-2";

/** UnmanagedBanner says that the fleet manager does not change this cluster. */
export function UnmanagedBanner() {
  return (
    <p role="status" className={`${BANNER} border-l-info`}>
      Not managed by the fleet manager. You can read this cluster but not change
      it here.
    </p>
  );
}

/**
 * ActivityBanner names the running backup and restore of a cluster, with a
 * link to the Backups tab. It renders nothing when neither runs. With
 * onOpenBackups, a click on the link calls it in place of the navigation.
 */
export function ActivityBanner({
  backup,
  restore,
  backupsHref,
  onOpenBackups,
}: {
  backup?: BackupRef;
  restore?: RestoreRef;
  backupsHref: string;
  onOpenBackups?: () => void;
}) {
  if (!backup && !restore) return null;
  return (
    <div
      role="status"
      className={`${BANNER} flex flex-wrap items-center gap-x-4 gap-y-1 border-l-warn`}
    >
      {backup && <span>Backup {backup.name} is running.</span>}
      {restore && (
        <span>
          {restore.kind === "PointInTimeRestore"
            ? "Point-in-time restore"
            : "Restore"}{" "}
          {restore.name} is running.
        </span>
      )}
      <a
        href={backupsHref}
        className="font-medium text-text underline underline-offset-4 hover:text-accent"
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
