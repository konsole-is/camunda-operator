import { lockedFieldStories } from "@/components/fleet/locked-field.stories";
import { shellStories } from "@/components/fleet/shell.stories";
import { statusStories } from "@/components/fleet/status.stories";
import { uiStories } from "@/components/ui/ui.stories";
import type { Story } from "./registry";

/** Every story of the gallery. A new component adds its stories file here. */
export const stories: readonly Story[] = [
  ...shellStories,
  ...statusStories,
  ...lockedFieldStories,
  ...uiStories,
];
