"use client";

import Link from "next/link";
import type { ReactNode } from "react";
import { EditorSection } from "@/components/setup/form-parts";
import { platformUserCount, type PlatformUsers } from "@/lib/usage";
import { unknownUsersText } from "./model";

type Item = { key: string; content: ReactNode };

/**
 * UsersSection lists each user of a platform config, grouped by kind. A
 * group without users does not show. While unread names a kind, the users
 * are not known and none shows.
 */
export function UsersSection({
  users,
  unread = [],
}: {
  users: PlatformUsers;
  unread?: readonly string[];
}): ReactNode {
  const groups: { title: string; items: Item[] }[] = [
    {
      title: "Clusters",
      items: users.clusters.map((c) => {
        const ref = refOf(c);
        return {
          key: ref,
          content: (
            <Link
              href={`/fleet/clusters/${ref}`}
              className="relative text-info underline hit-area hover:text-text"
            >
              {ref}
            </Link>
          ),
        };
      }),
    },
    {
      title: "Management planes",
      items: users.planes.map((p) => ({ key: refOf(p), content: refOf(p) })),
    },
    {
      title: "Database servers",
      items: users.databaseServers.map(({ server, preset }) => ({
        key: refOf(server),
        content: preset
          ? `${refOf(server)} from the preset ${preset}`
          : refOf(server),
      })),
    },
    {
      title: "Claims without a cluster or management plane yet",
      items: [...users.clusterClaims, ...users.planeClaims].map((c) => ({
        key: `${c.kind}/${refOf(c)}`,
        content: refOf(c),
      })),
    },
    {
      title: "Environments that default to it",
      items: users.environments.map((name) => ({
        key: name,
        content: (
          <Link
            href={`/setup/environments/${encodeURIComponent(name)}`}
            className="relative text-info underline hit-area hover:text-text"
          >
            {name}
          </Link>
        ),
      })),
    },
  ];
  return (
    <EditorSection title="Used by">
      {unread.length > 0 ? (
        <p className="text-sm text-muted">{unknownUsersText(unread)}</p>
      ) : platformUserCount(users) === 0 ? (
        <p className="text-sm text-muted">Nothing uses this config.</p>
      ) : (
        groups
          .filter((g) => g.items.length > 0)
          .map((g) => (
            <div key={g.title} className="flex flex-col gap-1.5">
              <h3 className="text-sm font-medium text-text">{g.title}</h3>
              <ul
                aria-label={g.title}
                className="flex flex-col gap-1 font-mono text-sm break-all"
              >
                {g.items.map((item) => (
                  <li key={item.key}>{item.content}</li>
                ))}
              </ul>
            </div>
          ))
      )}
    </EditorSection>
  );
}

function refOf(o: { metadata: { namespace?: string; name?: string } }): string {
  return `${o.metadata.namespace ?? ""}/${o.metadata.name ?? ""}`;
}
