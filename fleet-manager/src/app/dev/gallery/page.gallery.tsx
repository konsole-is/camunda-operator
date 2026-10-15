import Link from "next/link";
import { groupByComponent, slug, storyId } from "@/gallery/registry";
import { stories } from "@/gallery/stories";

export default function GalleryIndex() {
  return (
    <main className="mx-auto max-w-[1280px] px-4 py-6 md:px-8">
      <h1 className="mb-6 font-heading text-3xl font-bold">
        Component gallery
      </h1>
      {groupByComponent(stories).map(([component, entries]) => (
        <section
          key={component}
          aria-labelledby={`gallery-${slug(component)}`}
          className="mb-6"
        >
          <h2
            id={`gallery-${slug(component)}`}
            className="mb-2 font-heading text-lg font-semibold"
          >
            {component}
          </h2>
          <ul className="flex flex-col gap-1">
            {entries.map((story) => (
              <li key={storyId(story)}>
                <Link
                  href={`/dev/gallery/${storyId(story)}`}
                  data-story-id={storyId(story)}
                  data-component={story.component}
                  data-story-name={story.name}
                  className="text-info"
                >
                  {story.name}
                </Link>
              </li>
            ))}
          </ul>
        </section>
      ))}
    </main>
  );
}
