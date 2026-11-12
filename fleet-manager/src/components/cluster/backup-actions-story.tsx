"use client";

import type { WriteResult } from "@/lib/write/types";
import type { BackupContext } from "./backup-actions";
import { BackupHeaderActions, BackupRowActions } from "./backup-actions-view";
import type { BackupRow, BackupStorage } from "./backups";

const STORAGE: Record<
  "elasticsearch" | "rdbms" | "archived" | "none",
  BackupStorage
> = {
  elasticsearch: {
    kind: "elasticsearch",
    bucket: { name: "backups", location: "s3://acme-backups/orders" },
    repository: "camunda-backups",
    schedules: [],
  },
  rdbms: {
    kind: "rdbms",
    bucket: { name: "backups", location: "s3://acme-backups/orders" },
    schedules: [
      { name: "nightly", cron: "0 2 * * *", completed: 7, failed: 3 },
    ],
  },
  archived: {
    kind: "rdbms",
    bucket: { name: "backups", location: "s3://acme-backups/orders" },
    database: {
      archive: {
        server: "orders-db",
        state: { tone: "ok", label: "Archiving" },
        window: [
          {
            from: "2026-11-04T02:00:00Z",
            to: "2026-11-07T18:20:00Z",
            failing: true,
          },
          { from: "2026-11-08T02:00:00Z" },
        ],
      },
    },
    schedules: [],
  },
  none: { kind: "none", schedules: [] },
};

const apply = async (): Promise<WriteResult> => ({ ok: true, mode: "direct" });
const show = () => {};

function context(
  storage: keyof typeof STORAGE,
  over: Partial<BackupContext> = {},
): BackupContext {
  return {
    namespace: "team-a",
    cluster: "orders",
    storage: STORAGE[storage],
    backupNames: [],
    restoreNames: [],
    readingRestores: [],
    clusterVersion: "8.9.4",
    now: "2026-11-10T14:30:05Z",
    ...over,
  };
}

/**
 * BackupHeaderStory shows the header actions of the Backups tab on storage.
 * restoring sets a running restore, which suspends the cluster.
 */
export function BackupHeaderStory({
  storage,
  restoring = false,
}: {
  storage: keyof typeof STORAGE;
  restoring?: boolean;
}) {
  const over: Partial<BackupContext> = restoring
    ? {
        suspension: "held",
        running: { what: "Restore", name: "orders-restore-1" },
        restoring: "orders-restore-1",
      }
    : {};
  return (
    <div className="max-w-4xl">
      <BackupHeaderActions
        context={context(storage, over)}
        apply={apply}
        show={show}
      />
    </div>
  );
}

const scheduled: BackupRow = {
  kind: "LogicalBackupRDBMS",
  name: "nightly-20261110",
  origin: "Scheduled",
  started: "2026-11-10 02:00 UTC",
  finished: "2026-11-10 02:09 UTC",
  duration: "9m 12s",
  phase: "Completed",
  tone: "ok",
  step: "-",
  version: "8.9.3",
  details: [],
};

/**
 * BackupRowStory shows the actions of a scheduled PostgreSQL backup in its
 * table cell. clusterVersion is the Camunda version of the cluster.
 */
export function BackupRowStory({
  clusterVersion = "8.9.4",
}: {
  clusterVersion?: string;
}) {
  return (
    <table className="text-sm">
      <tbody>
        <tr>
          <td className="pr-4 font-medium">{scheduled.name}</td>
          <td className="min-w-56 whitespace-normal">
            <BackupRowActions
              context={context("rdbms", { clusterVersion })}
              row={scheduled}
              apply={apply}
              show={show}
            />
          </td>
        </tr>
      </tbody>
    </table>
  );
}
