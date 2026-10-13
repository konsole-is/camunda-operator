/** The two parts of the app in the top navigation (contract CL36). */
export type Section = "fleet" | "setup";

export type SetupItem = { slug: string; label: string; href: string };

const item = (slug: string, label: string): SetupItem => ({
  slug,
  label,
  href: `/setup/${slug}`,
});

/** The Setup pages, in the order of the Setup side navigation. */
export const SETUP_ITEMS: readonly SetupItem[] = [
  item("environments", "Environments"),
  item("presets", "Presets"),
  item("releases", "Releases"),
  item("platform", "Platform"),
  item("connections", "Connections"),
  item("plugins", "Plugins"),
  item("settings", "Settings"),
];

/** sectionOf returns the part of the app that a path is in, by its first segment only. */
export function sectionOf(pathname: string): Section | undefined {
  const first = pathname.split("/")[1];
  return first === "fleet" || first === "setup" ? first : undefined;
}

/** setupItemOf returns the Setup page of a list or detail path under /setup. */
export function setupItemOf(pathname: string): SetupItem | undefined {
  const [, section, slug] = pathname.split("/");
  if (section !== "setup") return undefined;
  return SETUP_ITEMS.find((candidate) => candidate.slug === slug);
}

/**
 * setupBreadcrumb returns the list and the object name of a Setup detail
 * path, /setup/<list>/<name>. A name with bad percent-encoding stays as it is
 * in the path.
 */
export function setupBreadcrumb(
  pathname: string,
): { item: SetupItem; name: string } | undefined {
  const found = setupItemOf(pathname);
  const raw = pathname.split("/")[3];
  if (!found || !raw) return undefined;
  let name = raw;
  try {
    name = decodeURIComponent(raw);
  } catch {
    // URIError: keep the raw segment.
  }
  return { item: found, name };
}
