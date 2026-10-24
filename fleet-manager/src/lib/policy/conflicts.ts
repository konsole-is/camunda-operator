import type { CamundaClusterClaimSpec } from "@/generated/crds/camundaclusterclaim";
import type { CamundaManagementClusterClaimSpec } from "@/generated/crds/camundamanagementclusterclaim";
import type { PluginSection } from "@/generated/crds/pluginsection";
import { sectionOf } from "./slots";
import type { PluginSelection, PolicyResult, Slot } from "./types";

/**
 * Overrides are the fields of a claim that a selected slot can fill. A
 * cluster claim sets cluster and optimize, and a plane claim sets management.
 */
export type Overrides = {
  cluster?: CamundaClusterClaimSpec["cluster"];
  optimize?: NonNullable<CamundaClusterClaimSpec["attachments"]>["optimize"];
  management?: CamundaManagementClusterClaimSpec["management"];
};

type Conflict = {
  slot: Slot;
  path: string;
  isSet: (o: Overrides) => boolean;
};

// A string is set when it is not empty, and annotations with a key.
// storageClassName is set also when it is empty. storageRef and
// backupStorageRef have no row, because the API server refuses them in
// spec.cluster.
const CONFLICTS: readonly Conflict[] = [
  {
    slot: "externalAccess",
    path: "spec.cluster.externalUrl",
    isSet: (o) => Boolean(o.cluster?.externalUrl),
  },
  {
    slot: "externalAccess",
    path: "spec.attachments.optimize.externalUrl",
    isSet: (o) => Boolean(o.optimize?.externalUrl),
  },
  {
    slot: "externalAccess",
    path: "spec.management.identityProvider.keycloak.externalUrl",
    isSet: (o) =>
      Boolean(o.management?.identityProvider?.keycloak?.externalUrl),
  },
  {
    slot: "externalAccess",
    path: "spec.management.identity.externalUrl",
    isSet: (o) => Boolean(o.management?.identity?.externalUrl),
  },
  {
    slot: "externalAccess",
    path: "spec.management.console.externalUrl",
    isSet: (o) => Boolean(o.management?.console?.externalUrl),
  },
  {
    slot: "externalAccess",
    path: "spec.management.webModeler.externalUrl",
    isSet: (o) => Boolean(o.management?.webModeler?.externalUrl),
  },
  {
    slot: "externalAccess",
    path: "spec.management.webModeler.websocketsExternalUrl",
    isSet: (o) => Boolean(o.management?.webModeler?.websocketsExternalUrl),
  },
  {
    slot: "workloadIdentity",
    path: "spec.cluster.serviceAccount.annotations",
    isSet: (o) =>
      Object.keys(o.cluster?.serviceAccount?.annotations ?? {}).length > 0,
  },
  {
    slot: "connectorsIdentity",
    path: "spec.cluster.connectors.serviceAccount.annotations",
    isSet: (o) =>
      Object.keys(o.cluster?.connectors?.serviceAccount?.annotations ?? {})
        .length > 0,
  },
  {
    slot: "volumeEncryption",
    path: "spec.cluster.zeebe.storageClassName",
    isSet: (o) =>
      o.cluster?.zeebe?.storageClassName !== undefined &&
      o.cluster.zeebe.storageClassName !== null,
  },
];

/**
 * checkSlotConflicts refuses with SlotConflict the first field of overrides
 * that the slot of a selected section fills, in claim order, by the rule of
 * CheckSlotConflicts of pkg/claimpolicy. It refuses with InvalidReference a
 * selection in a section that does not exist. The value is an empty object
 * when nothing conflicts.
 */
export function checkSlotConflicts(
  sections: readonly PluginSection[],
  selections: readonly PluginSelection[],
  overrides: Overrides,
): PolicyResult<Record<string, never>> {
  for (const s of selections) {
    const section = sectionOf(s.section, sections);
    if (!("value" in section)) return section;

    const conflict = CONFLICTS.find(
      (c) => c.slot === section.value.spec.slot && c.isSet(overrides),
    );
    if (conflict) {
      return {
        refusal: {
          reason: "SlotConflict",
          message: `${conflict.path} is filled by section "${s.section}", so the claim cannot set it`,
        },
      };
    }
  }

  return { value: {} };
}
