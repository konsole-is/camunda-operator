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
    name: "Row actions of a scheduled PostgreSQL backup",
    render: () => <BackupRowStory />,
  },
];
