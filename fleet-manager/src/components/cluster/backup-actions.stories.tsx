import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import type { ComponentProps } from "react";
import {
  expect,
  fireEvent,
  fn,
  userEvent,
  waitFor,
  within,
} from "storybook/test";
import type { WriteResult } from "@/lib/write/types";
import type { BackupContext } from "./backup-actions";
import { BackupHeaderActions, BackupRowActions } from "./backup-actions-view";
import type { BackupRow, BackupStorage } from "./backups";

const STORAGE: Record<
  | "elasticsearch"
  | "rdbms"
  | "archived"
  | "manual"
  | "manualWithoutPitr"
  | "none",
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
  manual: {
    kind: "rdbms",
    bucket: { name: "backups", location: "s3://acme-backups/orders" },
    database: {
      serverConfig: "orders-pg",
      endpoint: "orders-pg.abc123.eu-west-1.rds.amazonaws.com:5432",
      pitr: { enabled: true, recovery: "external", retentionDays: 3 },
    },
    schedules: [],
  },
  manualWithoutPitr: {
    kind: "rdbms",
    bucket: { name: "backups", location: "s3://acme-backups/orders" },
    database: {
      serverConfig: "orders-pg",
      endpoint: "orders-pg.abc123.eu-west-1.rds.amazonaws.com:5432",
      pitr: { enabled: false, recovery: "external" },
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

export const PointInTimeAfterARollbackOutsideTheOperator: Story = {
  name: "Point in time after a rollback outside the operator",
  args: { context: context("manual") },
  play: async ({ canvas, canvasElement }) => {
    await userEvent.click(
      canvas.getByRole("button", { name: "Restore to a point in time" }),
    );
    // The dialog renders in a portal outside the canvas.
    const dialog = within(canvasElement.ownerDocument.body).getByRole(
      "dialog",
      { name: "Restore orders to a point in time" },
    );
    fireEvent.change(within(dialog).getByLabelText("Time (UTC)"), {
      target: { value: "2026-11-09T09:30:15" },
    });
    // The dialog fades in, so the checkbox is not visible at once.
    await waitFor(() =>
      expect(
        within(dialog).getByRole("checkbox", {
          name: "I rolled the database server back to this time",
        }),
      ).toBeVisible(),
    );
    await expect(
      within(dialog).queryByLabelText("Type orders to confirm"),
    ).toBeNull();
  },
};

export const PointInTimeOnAServerWithoutPointInTimeRecovery: Story = {
  name: "Point in time on a server without point-in-time recovery",
  args: { context: context("manualWithoutPitr") },
};
