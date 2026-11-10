// This module runs in the browser and on the server.
import type { ConditionLike } from "@/components/fleet/status";
import type { Resource } from "@/lib/k8s/kinds";
import { byCodeUnit, type PluginSelection } from "@/lib/policy";

/** SectionComponentId is the component id of a selection of a section, as the cluster page cards use it. */
export type SectionComponentId = `section:${string}`;

/**
 * SectionComponent is one section of a claim that has a selection. label is
 * the PluginSection label, or the section name when the section is not
 * found. order is spec.order of the section, or 0 when it has none, and a
 * missing section sorts last. request is the PluginRequest that
 * status.requests of the claim names for the section and the registration
 * of the selection, and condition its Ready condition.
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
  status?: {
    requests?: { section: string; registration: string; name: string }[];
  };
};

/**
 * sectionComponentId is the component id of a selection. It is
 * "section:<section>" in a section that holds one selection, and
 * "section:<section>/<registration>" in a section that holds many.
 */
export function sectionComponentId(
  selection: PluginSelection,
  many: boolean,
): SectionComponentId {
  return many
    ? `section:${selection.section}/${selection.registration}`
    : `section:${selection.section}`;
}

/**
 * sectionComponents returns one component for each selection of claim,
 * sorted by order, then by section name, then by registration. A request
 * counts only when it is in the namespace of the claim. A section that is
 * not found holds many selections when the claim selects it more than once.
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
  const selections = claim.spec.plugins ?? [];
  return selections
    .map((selection): SectionComponent => {
      const found = sections.find(
        (s) => s.metadata?.name === selection.section,
      );
      const many = found
        ? found.spec.selection === "Many"
        : selections.filter((s) => s.section === selection.section).length > 1;
      const requestName = claim.status?.requests?.find(
        (r) =>
          r.section === selection.section &&
          r.registration === selection.registration,
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
        id: sectionComponentId(selection, many),
        section: selection.section,
        label: found?.spec.label ?? selection.section,
        order: found ? (found.spec.order ?? 0) : Number.MAX_SAFE_INTEGER,
        selection,
        ...(request ? { request } : {}),
        ...(condition ? { condition } : {}),
      };
    })
    .sort(
      (a, b) =>
        a.order - b.order ||
        byCodeUnit(a.section, b.section) ||
        byCodeUnit(a.selection.registration, b.selection.registration),
    );
}
