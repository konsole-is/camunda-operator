import { notFound } from "next/navigation";
import { findStory } from "@/gallery/registry";
import { stories } from "@/gallery/stories";

export default async function StoryPage({
  params,
}: {
  params: Promise<{ component: string; story: string }>;
}) {
  const { component, story } = await params;
  const entry = findStory(stories, `${component}/${story}`);
  if (!entry) notFound();
  return <div className="p-4">{entry.render()}</div>;
}
