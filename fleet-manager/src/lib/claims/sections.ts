// This module runs in the browser and on the server.
import type { ConditionLike } from "@/components/fleet/status";
import type { Resource } from "@/lib/k8s/kinds";
import { byCodeUnit, type PluginSelection } from "@/lib/policy";

/** SectionComponentId is the component id of a section, as the cluster page cards use it. */
export type SectionComponentId = `section:${string}`;

/**
 * SectionComponent is one section of a claim that has a selection. label is
 * the PluginSection label, or the section name when the section is not
 * found. order is spec.order of the section, and a missing section sorts
 * last. request is the PluginRequest that status.requests of the claim
 * names for the section, and condition its Ready condition.
 */
export type SectionComponent = {
  id: SectionComponentId;
  section: string;
  label: string;
  order: number;
  selection: PluginSelection;
  request?: Resource<"PluginRequest">;
  condition?: ConditionLike;
};

/** SectionClaim is the part of a claim of either kind that sectionComponents reads. */
export type SectionClaim = {
  metadata?: { namespace?: string };
  spec: { plugins?: PluginSelection[] };
  status?: { requests?: { section: string; name: string }[] };
};

/** sectionComponentId is the component id of the section of that name. */
export function sectionComponentId(section: string): SectionComponentId {
  return `section:${section}`;
}

/**
 * sectionComponents returns one component for each selection of claim,
 * sorted by order, then by section name. A request counts only when it is
 * in the namespace of the claim.
 */
export function sectionComponents({
  claim,
  sections,
  requests,
}: {
  claim: SectionClaim;
  sections: readonly Resource<"PluginSection">[];
  requests: readonly Resource<"PluginRequest">[];
}): SectionComponent[] {
  const namespace = claim.metadata?.namespace;
  return (claim.spec.plugins ?? [])
    .map((selection): SectionComponent => {
      const found = sections.find(
        (s) => s.metadata?.name === selection.section,
      );
      const requestName = claim.status?.requests?.find(
        (r) => r.section === selection.section,
      )?.name;
      const request =
        requestName === undefined
          ? undefined
          : requests.find(
              (r) =>
                r.metadata?.namespace === namespace &&
                r.metadata?.name === requestName,
            );
      const condition = request?.status?.conditions?.find(
        (c) => c.type === "Ready",
      );
      return {
        id: sectionComponentId(selection.section),
        section: selection.section,
        label: found?.spec.label ?? selection.section,
        order: found?.spec.order ?? Number.MAX_SAFE_INTEGER,
        selection,
        ...(request ? { request } : {}),
        ...(condition ? { condition } : {}),
      };
    })
    .sort((a, b) => a.order - b.order || byCodeUnit(a.section, b.section));
}
