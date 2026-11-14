import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import type { ComponentProps } from "react";
import { fn } from "storybook/test";
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

// RowCell shows the actions of a row in its table cell.
function RowCell(props: ComponentProps<typeof BackupRowActions>) {
  return (
    <table className="text-sm">
      <tbody>
        <tr>
          <td className="pr-4 font-medium">{props.row.name}</td>
          <td className="min-w-56 whitespace-normal">
            <BackupRowActions {...props} />
          </td>
        </tr>
      </tbody>
    </table>
  );
}

const meta = {
  title: "cluster/BackupActions",
  component: BackupHeaderActions,
  args: {
    context: context("elasticsearch"),
    apply: fn(async (): Promise<WriteResult> => ({ ok: true, mode: "direct" })),
    show: fn(),
  },
  decorators: [
    (Story) => (
      <div className="max-w-4xl">
        <Story />
      </div>
    ),
  ],
} satisfies Meta<typeof BackupHeaderActions>;
export default meta;
type Story = StoryObj<typeof meta>;

export const BackUpNowOnElasticsearch: Story = {
  name: "Back up now on Elasticsearch",
};

export const BackUpNowWhileARestoreRuns: Story = {
  name: "Back up now while a restore runs",
  args: {
    context: context("rdbms", {
      suspension: "held",
      running: { what: "Restore", name: "orders-restore-1" },
      restoring: "orders-restore-1",
    }),
  },
};

export const BackUpNowWithoutBackupStorage: Story = {
  name: "Back up now without backup storage",
  args: { context: context("none") },
};

export const RestoreACompletedPostgreSQLBackup: Story = {
  name: "Restore a completed PostgreSQL backup",
  args: { context: context("rdbms") },
  render: (args) => <RowCell {...args} row={scheduled} />,
};

export const RestoreOfABackupThatRecordedNoVersion: Story = {
  name: "Restore of a backup that recorded no version",
  args: { context: context("rdbms") },
  render: (args) => <RowCell {...args} row={{ ...scheduled, version: "-" }} />,
};

export const PointInTimeOnPostgreSQLWithRecoveryByTheOperator: Story = {
  name: "Point in time on PostgreSQL with recovery by the operator",
  args: { context: context("archived") },
};
