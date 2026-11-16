import type {
  CamundaRelease,
  CamundaReleaseSpec,
} from "@/generated/crds/camundarelease";
import { isDNSSubdomain, LAST_APPLIED_ANNOTATION } from "@/lib/labels";
import { equalJSON } from "@/lib/policy";
import type { ValidationError } from "@/lib/schema";
import type { ReleaseUsage } from "@/lib/usage";
import { refKey } from "@/lib/write/refs";
import type { Change, KubeObject } from "@/lib/write/types";
import { usedByText } from "./rows";

const API_VERSION = "core.camunda.io/v1";
const KIND = "CamundaRelease";

/** RELEASE_CHANGE_NOTICE is the notice above the save of a release that exists. */
export const RELEASE_CHANGE_NOTICE =
  "A change reaches every resource that uses this release. Their pods roll. A version change moves each user that does not set its own version. The Rollout tab moves clusters one by one.";

/** ReleaseDraft is the form of one release: its name and the spec as the admin edits it. */
export type ReleaseDraft = { name: string; spec: CamundaReleaseSpec };

/** draftOf returns the draft of release, or of a new release without a name and a version. */
export function draftOf(release: CamundaRelease | undefined): ReleaseDraft {
  return copyDraft(release, release?.metadata.name ?? "");
}

/** copyDraft returns a draft named name with a copy of the spec of source, or with an empty version without source. */
export function copyDraft(
  source: CamundaRelease | undefined,
  name: string,
): ReleaseDraft {
  return {
    name,
    spec: source ? structuredClone(source.spec) : { version: "" },
  };
}

// The blocks of the spec whose fields specOf compacts, and that it drops
// when they end up empty.
const BLOCKS = new Set([
  "connectors",
  "elasticsearch",
  "databaseServer",
  "images",
  "zeebe",
  "gateway",
  "operate",
  "tasklist",
  "admin",
]);

/**
 * specOf drops each empty string and empty list of the draft spec and of
 * its blocks, and each block that ends up empty. It keeps every other field
 * as it is.
 */
export function specOf(draft: ReleaseDraft): CamundaReleaseSpec {
  const spec: Record<string, unknown> = {};
  for (const [key, value] of Object.entries(structuredClone(draft.spec))) {
    const kept = BLOCKS.has(key) && isObject(value) ? compact(value) : value;
    if (!isEmpty(kept)) spec[key] = kept;
  }
  return spec as unknown as CamundaReleaseSpec;
}

const VERSION = /^\d+\.\d+\.\d+$/;
const MAJOR = /^\d+$/;

/**
 * draftErrors returns one error for each rule of the CRD that the draft
 * breaks, by path. The name is checked only for a new release, against
 * taken, the names of the releases that exist.
 */
export function draftErrors(
  draft: ReleaseDraft,
  opts: { isNew: boolean; taken: readonly string[] },
): ValidationError[] {
  const errors: ValidationError[] = [];
  const add = (path: string, message: string) => errors.push({ path, message });
  if (opts.isNew) {
    if (draft.name === "") add("name", "Give the release a name.");
    else if (draft.name.length > 253 || !isDNSSubdomain(draft.name))
      add("name", "The name must be a DNS name of at most 253 characters.");
    else if (opts.taken.includes(draft.name))
      add("name", "A release with this name exists.");
  }
  const { spec } = draft;
  if (!VERSION.test(spec.version))
    add("spec.version", "The Camunda version is a version such as 8.9.0.");
  const optional = (
    path: string,
    value: string | undefined,
    rule: RegExp,
    message: string,
  ) => {
    if (value && !rule.test(value)) add(path, message);
  };
  optional(
    "spec.connectors.version",
    spec.connectors?.version,
    VERSION,
    "The Connectors version is a version such as 8.9.7.",
  );
  optional(
    "spec.elasticsearch.version",
    spec.elasticsearch?.version,
    VERSION,
    "The Elasticsearch version is a version such as 9.2.4.",
  );
  optional(
    "spec.databaseServer.version",
    spec.databaseServer?.version,
    MAJOR,
    "The PostgreSQL version is a major version such as 17.",
  );
  return errors;
}

/**
 * changeCount counts the top-level fields of the spec that the draft
 * changes against base, and the name of a new release.
 */
export function changeCount(
  base: CamundaRelease | undefined,
  draft: ReleaseDraft,
  isNew: boolean,
): number {
  const before: Record<string, unknown> = { ...specOf(draftOf(base)) };
  const after: Record<string, unknown> = { ...specOf(draft) };
  const keys = new Set([...Object.keys(before), ...Object.keys(after)]);
  const changed = [...keys].filter((k) => !equalJSON(before[k], after[k]));
  return changed.length + (isNew && draft.name !== "" ? 1 : 0);
}

/**
 * createChange applies a new release with the spec of specOf. The write
 * fails with Stale when a release of that name exists.
 */
export function createChange(draft: ReleaseDraft): Change {
  return {
    intent: `Create the release ${draft.name}`,
    apply: [
      {
        apiVersion: API_VERSION,
        kind: KIND,
        metadata: { name: draft.name },
        spec: specOf(draft),
      },
    ],
    create: [refKey({ kind: KIND, name: draft.name })],
  };
}

/**
 * updateChange applies live with the spec of specOf, with the
 * resourceVersion of live as its precondition, and with force, so a field
 * that kubectl apply set changes too. It keeps the labels and annotations
 * of live, without the last applied file of kubectl.
 */
export function updateChange(
  live: CamundaRelease,
  draft: ReleaseDraft,
): Change {
  const name = live.metadata.name ?? "";
  const { labels, annotations } = live.metadata;
  const kept = Object.fromEntries(
    Object.entries(annotations ?? {}).filter(
      ([key]) => key !== LAST_APPLIED_ANNOTATION,
    ),
  );
  const object: KubeObject = {
    apiVersion: API_VERSION,
    kind: KIND,
    metadata: {
      name,
      ...(labels && { labels }),
      ...(Object.keys(kept).length > 0 && { annotations: kept }),
    },
    spec: specOf(draft),
  };
  return {
    intent: `Change the release ${name}`,
    apply: [object],
    ...preconditionsOf([live]),
    force: true,
  };
}

/** deleteChange deletes each of releases while it still has its resourceVersion and its UID. */
export function deleteChange(releases: readonly CamundaRelease[]): Change {
  const names = releases.map((r) => r.metadata.name ?? "");
  const uids = Object.fromEntries(
    releases.flatMap((r) =>
      r.metadata.uid
        ? [
            [
              refKey({ kind: KIND, name: r.metadata.name ?? "" }),
              r.metadata.uid,
            ],
          ]
        : [],
    ),
  );
  return {
    intent:
      names.length === 1
        ? `Delete the release ${names[0]}`
        : `Delete the releases ${names.join(", ")}`,
    apply: [],
    delete: names.map((name) => ({
      apiVersion: API_VERSION,
      kind: KIND,
      name,
    })),
    ...preconditionsOf(releases),
    ...(Object.keys(uids).length > 0 && { uids }),
  };
}

/**
 * saveEffects returns the effect lines of the save of draft for the users
 * of the release. Each version that the save changes gets its own line,
 * when a resource of the kind that runs that version uses the release.
 */
export function saveEffects(
  base: CamundaRelease | undefined,
  draft: ReleaseDraft,
  usage: ReleaseUsage,
): string[] {
  if (!base) return ["The release is new. No resource uses it yet."];
  const resources =
    usage.clusters.length +
    usage.elasticsearchClusters.length +
    usage.databaseServers.length;
  if (resources > 0) {
    const lines = [
      `${usedByText(usage)} ${resources === 1 ? "uses" : "use"} this release. Their pods roll.`,
    ];
    for (const v of VERSIONS) {
      const before = v.of(base.spec) ?? "";
      const after = v.of(draft.spec) ?? "";
      if (before !== after && usage[v.users].length > 0)
        lines.push(
          `The ${v.label} version changes from ${before || "none"} to ${after || "none"} for each ${v.user} that does not set its own.`,
        );
    }
    return lines;
  }
  const claims = usage.claims.length;
  if (claims > 0)
    return [
      `${usedByText(usage)} ${claims === 1 ? "names" : "name"} this release. Their clusters take the change when they are made.`,
    ];
  return ["No resource uses this release."];
}

const VERSIONS: readonly {
  label: string;
  of: (spec: CamundaReleaseSpec) => string | undefined;
  users: keyof ReleaseUsage;
  user: string;
}[] = [
  {
    label: "Camunda",
    of: (spec) => spec.version,
    users: "clusters",
    user: "cluster",
  },
  {
    label: "Connectors",
    of: (spec) => spec.connectors?.version,
    users: "clusters",
    user: "cluster",
  },
  {
    label: "Elasticsearch",
    of: (spec) => spec.elasticsearch?.version,
    users: "elasticsearchClusters",
    user: "Elasticsearch cluster",
  },
  {
    label: "PostgreSQL",
    of: (spec) => spec.databaseServer?.version,
    users: "databaseServers",
    user: "database server",
  },
];

function preconditionsOf(
  releases: readonly CamundaRelease[],
): Pick<Change, "preconditions"> {
  const preconditions = Object.fromEntries(
    releases.flatMap((r) =>
      r.metadata.resourceVersion === undefined
        ? []
        : [
            [
              refKey({ kind: KIND, name: r.metadata.name ?? "" }),
              r.metadata.resourceVersion,
            ],
          ],
    ),
  );
  return Object.keys(preconditions).length > 0 ? { preconditions } : {};
}

function compact(block: Record<string, unknown>): Record<string, unknown> {
  return Object.fromEntries(
    Object.entries(block).filter(([, value]) => !isEmpty(value)),
  );
}

function isObject(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function isEmpty(value: unknown): boolean {
  if (value === undefined || value === "") return true;
  if (Array.isArray(value)) return value.length === 0;
  if (isObject(value)) return Object.keys(value).length === 0;
  return false;
}
