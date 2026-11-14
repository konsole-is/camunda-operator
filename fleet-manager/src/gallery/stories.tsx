import { backupActionsStories } from "@/components/cluster/backup-actions.stories";
import { backupReviewStories } from "@/components/cluster/backup-review.stories";
import { backupsStories } from "@/components/cluster/backups.stories";
import { cardsStories } from "@/components/cluster/cards.stories";
import { clusterStories } from "@/components/cluster/cluster.stories";
import { detailStories } from "@/components/cluster/detail.stories";
import { claimEditorStories } from "@/components/cluster/edit/claim-editor.stories";
import { settingsStories } from "@/components/cluster/settings.stories";
import { settingsEditorStories } from "@/components/cluster/settings-editor.stories";
import { topologyEditorStories } from "@/components/cluster/topology-editor.stories";
import type { Story } from "./registry";

/** Every story of the gallery. A new component adds its stories file here. */
export const stories: readonly Story[] = [
  ...backupReviewStories,
  ...clusterStories,
  ...topologyEditorStories,
  ...claimEditorStories,
  ...cardsStories,
  ...detailStories,
  ...backupsStories,
  ...backupActionsStories,
  ...settingsStories,
  ...settingsEditorStories,
];
