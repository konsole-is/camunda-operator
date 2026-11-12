import type { Story } from "@/gallery/registry";
import { BackupHeaderStory, BackupRowStory } from "./backup-actions-story";

export const backupActionsStories: Story[] = [
  {
    component: "BackupActions",
    name: "Back up now on Elasticsearch",
    render: () => <BackupHeaderStory storage="elasticsearch" />,
  },
  {
    component: "BackupActions",
    name: "Back up now while a restore runs",
    render: () => <BackupHeaderStory storage="rdbms" restoring />,
  },
  {
    component: "BackupActions",
    name: "Back up now without backup storage",
    render: () => <BackupHeaderStory storage="none" />,
  },
  {
    component: "BackupActions",
    name: "Restore a completed PostgreSQL backup",
    render: () => <BackupRowStory />,
  },
  {
    component: "BackupActions",
    name: "Restore blocked by the version rule",
    render: () => <BackupRowStory clusterVersion="8.11.0" />,
  },
  {
    component: "BackupActions",
    name: "Point in time on PostgreSQL with recovery by the operator",
    render: () => <BackupHeaderStory storage="archived" />,
  },
];
