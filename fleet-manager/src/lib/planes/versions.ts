// This module runs in the browser and on the server.
import type { CamundaRelease } from "@/generated/crds/camundarelease";
import type { Environment } from "@/generated/crds/environment";
import { parseVersion, versionBelow } from "@/lib/policy";
import { listOf } from "@/lib/text";
import type { ImagesTable } from "@/startup/bundled-data";

/** PlaneVersionField is a field of spec.versions of a plane claim. */
export type PlaneVersionField =
  "identity" | "console" | "webModeler" | "keycloak";

/** PlaneVersions holds a version for each field that has one. */
export type PlaneVersions = Partial<Record<PlaneVersionField, string>>;

/** PLANE_IMAGES is the image of the images table that gives each version. */
export const PLANE_IMAGES: Record<PlaneVersionField, string> = {
  identity: "identity",
  console: "console",
  webModeler: "web-modeler-restapi",
  keycloak: "keycloak",
};

/** VERSION_LABEL is the name of each app in the copy. */
export const VERSION_LABEL: Record<PlaneVersionField, string> = {
  identity: "Management Identity",
  console: "Console",
  webModeler: "Web Modeler",
  keycloak: "Keycloak",
};

/** Running says which of the optional apps run. Identity always runs. */
export type Running = {
  keycloak: boolean;
  console: boolean;
  webModeler: boolean;
};

/** VersionPrefill is the release whose minor gave the versions, and the versions of the table for that minor. */
export type VersionPrefill = {
  release: string;
  minor: string;
  versions: PlaneVersions;
};

/** KEYCLOAK_REALM_WARNING is the warning of Keycloak 26.7.0 or later with Management Identity 8.9. */
export const KEYCLOAK_REALM_WARNING =
  "Management Identity 8.9 cannot create its realm in Keycloak 26.7.0 or later. Use a Keycloak version below 26.7.0.";

/** KEYCLOAK_RESTART_WARNING is the warning of a change of the version of a Keycloak that the plane runs. */
export const KEYCLOAK_RESTART_WARNING =
  "Sign-in stops while Keycloak restarts.";

// The limits of pkg/components/camundamanagementcluster/validate.go.
const CAMUNDA_FLOOR = [8, 9, 0];
const KEYCLOAK_FLOOR = [26, 0, 0];
const KEYCLOAK_CEILING = [27, 0, 0];
const KEYCLOAK_REALM_BREAK = [26, 7, 0];

const FIELDS: readonly PlaneVersionField[] = [
  "identity",
  "console",
  "webModeler",
  "keycloak",
];

/** runningFields returns the version fields of the apps that run, in screen order. */
export function runningFields(running: Running): PlaneVersionField[] {
  return FIELDS.filter((field) => field === "identity" || running[field]);
}

/**
 * defaultRelease returns the default release of the first environment of
 * served. It is undefined when that environment or its default is unset,
 * or when no release has that name.
 */
export function defaultRelease(
  served: readonly string[],
  environments: readonly Environment[],
  releases: readonly CamundaRelease[],
): CamundaRelease | undefined {
  const name = environments.find((e) => e.metadata.name === served[0])?.spec
    .releases?.default;
  return name === undefined
    ? undefined
    : releases.find((r) => r.metadata.name === name);
}

/**
 * versionPrefill returns the version of each plane image in the row of the
 * images table for the minor of release. It is undefined without a release
 * and for a minor that the table does not hold.
 */
export function versionPrefill(
  table: ImagesTable,
  release: CamundaRelease | undefined,
): VersionPrefill | undefined {
  const minor = wellFormedMinor(release?.spec.version);
  if (minor === undefined || !Object.hasOwn(table.minors, minor)) {
    return undefined;
  }
  const row = table.minors[minor];
  const versions: PlaneVersions = {};
  for (const field of FIELDS) {
    const image = PLANE_IMAGES[field];
    if (Object.hasOwn(row, image)) versions[field] = row[image].version;
  }
  return { release: release?.metadata.name ?? "", minor, versions };
}

/**
 * versionsOf returns the version of each running app: the typed one, also
 * when it is empty, else the one of prefill. An app without either gets no
 * entry.
 */
export function versionsOf(
  typed: PlaneVersions,
  prefill: VersionPrefill | undefined,
  running: Running,
): PlaneVersions {
  const versions: PlaneVersions = {};
  for (const field of runningFields(running)) {
    const value = Object.hasOwn(typed, field)
      ? typed[field]
      : prefill?.versions[field];
    if (value !== undefined) versions[field] = value;
  }
  return versions;
}

/** versionErrors returns one error for each running app whose version is missing, malformed, or outside the operator limits. */
export function versionErrors(
  versions: PlaneVersions,
  running: Running,
): string[] {
  return runningFields(running).flatMap(
    (field) => versionError(field, versions[field]) ?? [],
  );
}

/** versionError returns the error of versionErrors for value as the version of field, or undefined when value passes. */
export function versionError(
  field: PlaneVersionField,
  value: string | undefined,
): string | undefined {
  const label = VERSION_LABEL[field];
  if (!value) return `Enter the ${label} version.`;
  const parsed = parseVersion(value);
  const keycloak = field === "keycloak";
  if (!parsed) {
    const example = keycloak ? "26.6.4" : "8.9.12";
    return `Enter the ${label} version as major.minor.patch, for example ${example}.`;
  }
  if (keycloak) {
    return versionBelow(parsed, KEYCLOAK_FLOOR) ||
      !versionBelow(parsed, KEYCLOAK_CEILING)
      ? "Keycloak needs a version from 26.0.0 up to, but not including, 27.0.0."
      : undefined;
  }
  return versionBelow(parsed, CAMUNDA_FLOOR)
    ? `${label} needs version 8.9.0 or later.`
    : undefined;
}

/**
 * versionChangeWarnings returns the warnings of a move of the versions of
 * the running apps from before to after, in field order: a Camunda app
 * that moves down or skips a minor, and KEYCLOAK_RESTART_WARNING for any
 * change of the Keycloak version. An app without a version before or after
 * gives no warning, and a malformed version gives no downgrade or skip
 * warning. A warning never blocks a save.
 */
export function versionChangeWarnings(
  before: PlaneVersions,
  after: PlaneVersions,
  running: Running,
): string[] {
  return runningFields(running).flatMap((field) => {
    const from = before[field];
    const to = after[field];
    if (!from || !to || from === to) return [];
    if (field === "keycloak") return [KEYCLOAK_RESTART_WARNING];
    const old = parseVersion(from);
    const next = parseVersion(to);
    if (!old || !next) return [];
    const label = VERSION_LABEL[field];
    if (versionBelow(next, old)) {
      return [
        `${label} moves down from ${from} to ${to}. Camunda documents no downgrade.`,
      ];
    }
    const skips =
      next[0] > old[0] || (next[0] === old[0] && next[1] > old[1] + 1);
    return skips
      ? [
          `${label} skips ${old[0]}.${old[1] + 1}: ${from} to ${to}. Move one minor at a time.`,
        ]
      : [];
  });
}

/**
 * versionMatrixWarning returns a warning when the running Camunda apps with
 * a well-formed version are on more than one minor. It compares the minor
 * only: the apps of one Camunda minor have their own patch lines, as the
 * images table shows. Keycloak is not a Camunda app. A warning never blocks
 * a save.
 */
export function versionMatrixWarning(
  versions: PlaneVersions,
  running: Running,
): string | undefined {
  const apps = runningFields(running).flatMap((field) => {
    const minor =
      field === "keycloak" ? undefined : wellFormedMinor(versions[field]);
    return minor ? [{ label: VERSION_LABEL[field], minor }] : [];
  });
  const minors = [...new Set(apps.map((a) => a.minor))];
  if (minors.length < 2) return undefined;
  return `${listOf(apps.map((a) => a.label))} run on different minors: ${minors.join(", ")}. Camunda supports them together on one minor only.`;
}

/**
 * versionWarnings returns KEYCLOAK_REALM_WARNING when Keycloak runs at
 * 26.7.0 or later with Management Identity 8.9. A warning never blocks the
 * step.
 */
export function versionWarnings(
  versions: PlaneVersions,
  running: Running,
): string[] {
  if (!running.keycloak) return [];
  const identity = parseVersion(versions.identity);
  const keycloak = parseVersion(versions.keycloak);
  return identity?.[0] === 8 &&
    identity[1] === 9 &&
    keycloak &&
    !versionBelow(keycloak, KEYCLOAK_REALM_BREAK)
    ? [KEYCLOAK_REALM_WARNING]
    : [];
}

/** prefillNote is the line under the versions: where they come from, or why none came. */
export function prefillNote(
  prefill: VersionPrefill | undefined,
  release: CamundaRelease | undefined,
): string {
  if (prefill) return `Tested with release ${prefill.minor}`;
  const minor = wellFormedMinor(release?.spec.version);
  if (minor === undefined) {
    return "The first served environment has no default release. Enter each version.";
  }
  return `The images table has no versions for Camunda ${minor}. Enter each version.`;
}

// wellFormedMinor gives the minor only of a version of the form
// major.minor.patch, the form that the operator reads, as numbers: 8.09.1
// gives 8.9. minorOf of lib/merge keeps the digits as written.
function wellFormedMinor(version: string | undefined): string | undefined {
  const parsed = parseVersion(version);
  return parsed && `${parsed[0]}.${parsed[1]}`;
}
