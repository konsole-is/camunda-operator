import { backupsStories } from "@/components/cluster/backups.stories";
import { clusterStories } from "@/components/cluster/cluster.stories";
import { settingsStories } from "@/components/cluster/settings.stories";
import { confirmStories } from "@/components/fleet/confirm.stories";
import { createFlowStories } from "@/components/fleet/create-flow.stories";
import { dataTableStories } from "@/components/fleet/data-table.stories";
import { workloadEditorStories } from "@/components/fleet/workload-editors.stories";
import { editorStories } from "@/components/fleet/editors.stories";
import { helpStories } from "@/components/fleet/help.stories";
import { lockedFieldStories } from "@/components/fleet/locked-field.stories";
import { observeStories } from "@/components/fleet/observe.stories";
import { planesStories } from "@/components/planes/planes.stories";
import { reviewStories } from "@/components/fleet/review.stories";
import { sectionCardsStories } from "@/components/fleet/section-cards.stories";
import { shellStories } from "@/components/fleet/shell.stories";
import { statusStories } from "@/components/fleet/status.stories";
import { settingsPageStories } from "@/components/setup/settings-page.stories";
import { uiStories } from "@/components/ui/ui.stories";
import type { Story } from "./registry";

/** Every story of the gallery. A new component adds its stories file here. */
export const stories: readonly Story[] = [
  ...shellStories,
  ...statusStories,
  ...lockedFieldStories,
  ...reviewStories,
  ...confirmStories,
  ...helpStories,
  ...dataTableStories,
  ...planesStories,
  ...editorStories,
  ...workloadEditorStories,
  ...sectionCardsStories,
  ...createFlowStories,
  ...observeStories,
  ...clusterStories,
  ...backupsStories,
  ...settingsStories,
  ...settingsPageStories,
  ...uiStories,
];
