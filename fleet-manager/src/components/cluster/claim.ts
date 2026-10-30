import { stringify } from "yaml";
import type { ConditionRow } from "@/components/fleet/conditions-table";
import type { CamundaClusterSpec } from "@/generated/crds/camundacluster";
import { fleetCamundaClusterClaimConditionTypes } from "@/generated/conditions";
import { CLAIM_PLACEMENT, placementOf } from "@/lib/conditions";
import { type ClusterField, parameterFields } from "@/lib/fields";
import type { Resource } from "@/lib/k8s/kinds";
import type { Merged } from "@/lib/merge";

/**
 * ClaimSection is one selection of the claim: a section with the option
 * picked for it. A label falls back to the object name when the
 * PluginSection or PluginRegistration is not found.
 */
export type ClaimSection = {
  section: string;
  sectionLabel: string;
  registration: string;
  optionLabel: string;
  parameters: ClusterField[];
};

/** ClaimRef is a value of the claim. fromEnvironment is true when the claim does not set it. */
export type ClaimRef = { value?: string; fromEnvironment: boolean };

/** ClaimRequestRow is one entry of status.requests of the claim. */
export type ClaimRequestRow = {
  name: string;
  sectionLabel: string;
  optionLabel: string;
  outputsReady?: string;
  ready?: string;
};

/**
 * ClaimView is the claim as the Claim tab reads it. environment is
 * undefined when the status names none.
 */
export type ClaimView = {
  name: string;
  namespace: string;
  environment?: string;
  preset?: string;
  release: ClaimRef;
  platformConfig: ClaimRef;
  deletionPolicy: ClaimRef;
  sections: ClaimSection[];
  optimize?: { externalUrl?: string; managementAuthConfigRef: string };
  backupSchedule?: { schedule: string; completed?: number; failed?: number };
  conditions: ConditionRow[];
  requests: ClaimRequestRow[];
};

/**
 * claimView returns the read view of claim. cluster is the live cluster of
 * the claim, and merged its merged spec. A ref that the claim leaves empty
 * shows the value that the cluster or the resolved policy got. The sections
 * come by the order of their PluginSection, then by name, and the requests
 * in the order of the status.
 */
export function claimView(input: {
  claim: Resource<"CamundaClusterClaim">;
  cluster: Resource<"CamundaCluster">;
  merged: Merged<CamundaClusterSpec>;
  environments: readonly Resource<"Environment">[];
  sections: readonly Resource<"PluginSection">[];
  registrations: readonly Resource<"PluginRegistration">[];
}): ClaimView {
  const { claim, cluster, sections, registrations } = input;
  const { spec, status } = claim;
  const sectionOf = (name: string) =>
    sections.find((s) => s.metadata?.name === name);
  const sectionLabel = (name: string) => sectionOf(name)?.spec.label ?? name;
  const optionLabel = (name: string) =>
    registrations.find((r) => r.metadata?.name === name)?.spec.label ?? name;

  const fields = parameterFields({
    cluster,
    merged: input.merged,
    claim,
    policy: status?.resolvedPolicy,
    sections,
    registrations,
  });
  const selections = (spec.plugins ?? []).map((s): ClaimSection => ({
    section: s.section,
    sectionLabel: sectionLabel(s.section),
    registration: s.registration,
    optionLabel: optionLabel(s.registration),
    parameters: fields.filter(
      (f) =>
        f.target.kind === "PluginRequest" &&
        f.target.section === s.section &&
        f.target.registration === s.registration,
    ),
  }));
  const order = (name: string) => {
    const known = sectionOf(name);
    return known ? (known.spec.order ?? 0) : Number.MAX_SAFE_INTEGER;
  };
  selections.sort(
    (a, b) =>
      order(a.section) - order(b.section) || a.section.localeCompare(b.section),
  );

  const environmentName = status?.environment || undefined;
  const optimize = spec.attachments?.optimize;
  const schedule = spec.attachments?.backupSchedule;
  return {
    name: claim.metadata?.name ?? "",
    namespace: claim.metadata?.namespace ?? "",
    environment:
      environmentName &&
      (input.environments.find((e) => e.metadata?.name === environmentName)
        ?.spec?.displayName ||
        environmentName),
    preset: spec.presetRef,
    release: ref(spec.releaseRef, cluster.spec.releaseRef),
    platformConfig: ref(spec.platformConfigRef, cluster.spec.platformConfigRef),
    // The claim controller falls back to Delete (claimpolicy.DeletionPolicy).
    deletionPolicy: ref(
      spec.deletionPolicy,
      status?.resolvedPolicy?.deletionPolicy || "Delete",
    ),
    sections: selections,
    ...(optimize && {
      optimize: {
        ...(optimize.externalUrl && { externalUrl: optimize.externalUrl }),
        managementAuthConfigRef: optimize.managementAuthConfigRef,
      },
    }),
    ...(schedule && {
      backupSchedule: {
        schedule: schedule.schedule,
        ...(schedule.retained?.completed !== undefined && {
          completed: schedule.retained.completed,
        }),
        ...(schedule.retained?.failed !== undefined && {
          failed: schedule.retained.failed,
        }),
      },
    }),
    conditions: claimConditions(claim),
    requests: (status?.requests ?? []).map((r) => ({
      name: r.name,
      sectionLabel: sectionLabel(r.section),
      optionLabel: optionLabel(r.registration),
      outputsReady: r.outputsReady,
      ready: r.ready,
    })),
  };
}

function ref(own: string | undefined, resolved: string | undefined): ClaimRef {
  return own
    ? { value: own, fromEnvironment: false }
    : { value: resolved || undefined, fromEnvironment: true };
}

const GENERATED_ORDER: readonly string[] =
  fleetCamundaClusterClaimConditionTypes;

/**
 * claimConditions returns every condition of the claim: the header type
 * (Ready) first, then the other types of the placement map in the order of
 * the generated types, then any type that the map does not know.
 */
export function claimConditions(
  claim: Resource<"CamundaClusterClaim">,
): ConditionRow[] {
  const rank = (type: string) => {
    if (!Object.hasOwn(CLAIM_PLACEMENT, type)) return Number.MAX_SAFE_INTEGER;
    if (placementOf(CLAIM_PLACEMENT, type) === "header") return -1;
    return GENERATED_ORDER.indexOf(type);
  };
  return [...((claim.status?.conditions ?? []) as ConditionRow[])].sort(
    (a, b) => rank(a.type) - rank(b.type),
  );
}

/** storedYaml returns the claim as YAML, as stored, without metadata.managedFields. */
export function storedYaml(claim: Resource<"CamundaClusterClaim">): string {
  const metadata = { ...claim.metadata };
  delete metadata.managedFields;
  return stringify({ ...claim, metadata });
}
