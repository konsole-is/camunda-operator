import type { ReactNode } from "react";

/**
 * A story is one component with fixed props in one state. render passes no
 * event handler, because the story page is a server component.
 */
export type Story = {
  component: string;
  name: string;
  render: () => ReactNode;
};

/** slug lowers the text and joins its runs of letters and digits with "-". */
export function slug(text: string): string {
  return text
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "");
}

/** storyId is the path of a story under /dev/gallery and of its screenshots. */
export function storyId(story: Story): string {
  return `${slug(story.component)}/${slug(story.name)}`;
}

export function findStory(
  stories: readonly Story[],
  id: string,
): Story | undefined {
  return stories.find((story) => storyId(story) === id);
}

/** groupByComponent keeps the order in which each component first appears. */
export function groupByComponent(
  stories: readonly Story[],
): [string, Story[]][] {
  const groups = new Map<string, Story[]>();
  for (const story of stories) {
    groups.set(story.component, [
      ...(groups.get(story.component) ?? []),
      story,
    ]);
  }
  return [...groups.entries()];
}
