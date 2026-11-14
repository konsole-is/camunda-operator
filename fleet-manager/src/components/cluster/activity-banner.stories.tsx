import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { fn } from "storybook/test";
import { ActivityBanner } from "./banners";

const meta = {
  title: "cluster/ActivityBanner",
  component: ActivityBanner,
  args: { backupsHref: "?tab=backups", onOpenBackups: fn() },
} satisfies Meta<typeof ActivityBanner>;
export default meta;
type Story = StoryObj<typeof meta>;

export const BackupRunning: Story = {
  name: "Backup running",
  args: {
    backup: {
      kind: "LogicalBackupRDBMS",
      namespace: "team-a",
      name: "nightly-1",
    },
  },
};

export const PointInTimeRestoreRunning: Story = {
  name: "Point-in-time restore running",
  args: {
    restore: {
      kind: "PointInTimeRestore",
      namespace: "team-a",
      name: "before-incident",
    },
  },
};
