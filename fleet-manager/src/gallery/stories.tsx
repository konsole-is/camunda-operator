import { fleetViewStories } from "@/components/fleet-view/fleet-view.stories";
import { planeCreateStories } from "@/components/planes/create/create.stories";
import { planesStories } from "@/components/planes/planes.stories";
import { environmentStories } from "@/components/setup/environments/environments.stories";
import type { Story } from "./registry";

/** Every story of the gallery. A new component adds its stories file here. */
export const stories: readonly Story[] = [
  ...planesStories,
  ...fleetViewStories,
  ...planeCreateStories,
  ...environmentStories,
];
