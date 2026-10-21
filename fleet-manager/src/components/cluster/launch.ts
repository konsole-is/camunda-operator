/** LaunchLink opens one web app of a cluster. */
export type LaunchLink = {
  label: "Operate" | "Tasklist" | "Admin";
  href: string;
};

const APPS = [
  ["Operate", "operate"],
  ["Tasklist", "tasklist"],
  ["Admin", "admin"],
] as const;

/**
 * launchLinks returns the links to Operate, Tasklist, and Admin, in that
 * order. With a non-empty externalUrl, each link is that URL with the path of
 * the app appended. Else each comes from the access link of the same name
 * (operate, tasklist, admin), and an app without one has no link. Without
 * both, the result is empty.
 */
export function launchLinks(
  externalUrl: string | undefined,
  accessLinks: readonly { name: string; url: string }[] | undefined,
): LaunchLink[] {
  if (externalUrl) {
    const base = externalUrl.replace(/\/+$/, "");
    return APPS.map(([label, path]) => ({ label, href: `${base}/${path}` }));
  }
  return APPS.flatMap(([label, name]) => {
    const link = accessLinks?.find((l) => l.name === name);
    return link ? [{ label, href: link.url }] : [];
  });
}
