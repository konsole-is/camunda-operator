/** The two parts of the app in the top navigation. */
export type Section = "fleet" | "setup";

/**
 * SetupItem is one Setup page. href is its list. create is the create page
 * of a page that creates objects: /setup/new-<kind>, beside the lists, so
 * the name of an object never takes its path. Its label is New and the
 * kind, with a space for each hyphen.
 */
export type SetupItem = {
  slug: string;
  label: string;
  href: string;
  create?: { href: string; label: string };
};

const item = (slug: string, label: string, kind?: string): SetupItem => ({
  slug,
  label,
  href: `/setup/${slug}`,
  ...(kind && {
    create: {
      href: `/setup/new-${kind}`,
      label: `New ${kind.replaceAll("-", " ")}`,
    },
  }),
});

/** The Setup pages, in the order of the Setup side navigation. */
export const SETUP_ITEMS: readonly SetupItem[] = [
  item("environments", "Environments", "environment"),
  item("presets", "Presets", "preset"),
  item("releases", "Releases", "release"),
  item("platform", "Platform", "platform-config"),
  item("connections", "Connections", "connection-contract"),
  item("plugins", "Plugins"),
  item("settings", "Settings"),
];

/** sectionOf returns the part of the app that a path is in, by its first segment only. */
export function sectionOf(pathname: string): Section | undefined {
  const first = pathname.split("/")[1];
  return first === "fleet" || first === "setup" ? first : undefined;
}

/** setupItemOf returns the Setup page of a list, detail, or create path under /setup. */
export function setupItemOf(pathname: string): SetupItem | undefined {
  const [, section, slug] = pathname.split("/");
  if (section !== "setup") return undefined;
  return SETUP_ITEMS.find(
    (candidate) =>
      candidate.slug === slug || candidate.create?.href === `/setup/${slug}`,
  );
}

/**
 * setupBreadcrumb returns the list and the object name of a Setup detail
 * path, and the list and the label of a create page. The name is the last
 * segment of /setup/<list>/<name> and /setup/<list>/<kind>/<name>, and
 * <namespace>/<name> of /setup/<list>/<kind>/<namespace>/<name>. Each
 * segment is decoded on its own, and a segment with bad percent-encoding
 * stays as it is in the path.
 */
export function setupBreadcrumb(
  pathname: string,
): { item: SetupItem; name: string } | undefined {
  const found = setupItemOf(pathname);
  if (found?.create && pathname === found.create.href)
    return { item: found, name: found.create.label };
  const rest = pathname.split("/").slice(3);
  if (!found || rest.length === 0 || rest.length > 3 || rest.includes(""))
    return undefined;
  const named = rest.length === 3 ? rest.slice(1) : rest.slice(-1);
  return { item: found, name: named.map(decoded).join("/") };
}

function decoded(segment: string): string {
  try {
    return decodeURIComponent(segment);
  } catch {
    // URIError: keep the raw segment.
    return segment;
  }
}
