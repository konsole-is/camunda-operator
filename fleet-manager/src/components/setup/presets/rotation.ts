import type { ReadError } from "@/components/planes/planes-table";
import type { Resource } from "@/lib/k8s/kinds";
import { type PresetUser, presetResources } from "@/lib/usage";
import type { Change } from "@/lib/write/types";
import { presetDraftOf, presetUpdateChange } from "./draft";
import type { PresetSources } from "./sources";

/** ROTATION_KINDS are the lists that rotationTargets reads. */
export const ROTATION_KINDS = ["CamundaCluster", "CamundaPlatformConfig"];

/**
 * rotationToken returns now as the UTC date and time to the minute, such as
 * "2026-11-15T10:42Z". The operator never rotates the same value twice, so
 * a second rotation in the same minute rotates nothing.
 */
export function rotationToken(now: Date): string {
  return `${now.toISOString().slice(0, 16)}Z`;
}

/**
 * rotationTargets returns the clusters whose admin password a rotation on
 * preset changes: each CamundaCluster that inherits it, whose platform
 * config sets basic auth or no auth method, and that sets no auth.basic of
 * its own. A cluster whose platform config is not in sources does not
 * count.
 */
export function rotationTargets(
  preset: Resource<"CamundaClusterPreset">,
  sources: PresetSources,
): PresetUser[] {
  return presetResources(
    "CamundaClusterPreset",
    preset.metadata.name ?? "",
    sources,
  ).filter((user) => {
    const cluster = sources.clusters.find(
      (c) =>
        c.metadata.namespace === user.namespace &&
        c.metadata.name === user.name,
    );
    if (!cluster || cluster.spec.auth?.basic) return false;
    const config = sources.platformConfigs.find(
      (p) => p.metadata.name === cluster.spec.platformConfigRef,
    );
    const method = config?.spec.auth?.method;
    return config !== undefined && (method === undefined || method === "basic");
  });
}

/** rotationUnread returns each list of ROTATION_KINDS that is in errors (not read) or pending (not current). */
export function rotationUnread(
  errors: readonly ReadError[],
  pending: readonly string[],
): string[] {
  return ROTATION_KINDS.filter(
    (k) => pending.includes(k) || errors.some((e) => e.kind === k),
  );
}

/**
 * rotationChange applies live with auth.basic.passwordRotation set to the
 * rotationToken of now, and every other field of live as it is. It carries
 * the resourceVersion of live as its precondition and force, as
 * presetUpdateChange does.
 */
export function rotationChange(
  live: Resource<"CamundaClusterPreset">,
  now: Date,
): Change {
  const draft = presetDraftOf(live);
  const body =
    draft.body as Resource<"CamundaClusterPreset">["spec"]["cluster"];
  const rotated = {
    ...body,
    auth: {
      ...body.auth,
      basic: { ...body.auth?.basic, passwordRotation: rotationToken(now) },
    },
  };
  return {
    ...presetUpdateChange(live, { ...draft, body: rotated }),
    intent: `Rotate the admin password of the clusters of the preset ${live.metadata.name ?? ""}`,
  };
}
