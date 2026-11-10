import type { V1Namespace } from "@kubernetes/client-node";
import type { Environment } from "@/generated/crds/environment";
import {
  ENVIRONMENT_LABEL,
  isDNSSubdomain,
  isLabelKey,
  isLabelValue,
  LAST_APPLIED_ANNOTATION,
} from "@/lib/labels";
import { APP_FIELD_MANAGER, child, fieldManagers } from "@/lib/merge";
import type { ValidationError } from "@/lib/schema";
import { refKey } from "@/lib/write/refs";
import type { Change, KubeObject } from "@/lib/write/types";

const API_VERSION = "fleet.camunda.io/v1alpha1";
const KIND = "Environment";
const MAX_REQUIRED_LABEL_KEYS = 16;
const MAX_FIXED_LABELS = 32;
const MAX_ALLOWED_RELEASES = 64;

/** isHexColor says whether color is a hex color of six digits, as spec.color of the CRD must be. */
export function isHexColor(color: string): boolean {
  return /^#[0-9a-fA-F]{6}$/.test(color);
}

/** EnvironmentDraft is the form of one environment: its name and the spec as the admin edits it. */
export type EnvironmentDraft = { name: string; spec: Environment["spec"] };

/** draftOf returns the draft of environment, or of a new environment with the deletion policy Delete. */
export function draftOf(
  environment: Environment | undefined,
): EnvironmentDraft {
  if (!environment) return { name: "", spec: { deletionPolicy: "Delete" } };
  return {
    name: environment.metadata.name ?? "",
    spec: structuredClone(environment.spec),
  };
}

/**
 * specOf drops each empty string, list, and map of the draft spec, and
 * spec.releases when it has neither allowed nor default. It drops
 * namespaceCreation.allowed when it is false, because unset means the same.
 * It keeps sections and parameters as they are.
 */
export function specOf(draft: EnvironmentDraft): Environment["spec"] {
  const { presets, releases, namespaceCreation, ...rest } = structuredClone(
    draft.spec,
  );
  const spec: Environment["spec"] = {};
  for (const [key, value] of Object.entries(rest)) {
    if (value !== "" && value !== undefined)
      Object.assign(spec, { [key]: value });
  }
  const compactPresets = compact({
    camundaCluster: presets?.camundaCluster,
    elasticsearchCluster: presets?.elasticsearchCluster,
    databaseServer: presets?.databaseServer,
  });
  if (compactPresets) spec.presets = compactPresets;
  const compactReleases = compact({
    allowed: releases?.allowed,
    default: releases?.default,
  });
  if (compactReleases) spec.releases = compactReleases;
  const compactCreation = compact({
    allowed: namespaceCreation?.allowed || undefined,
    labels: namespaceCreation?.labels,
    requiredLabelKeys: namespaceCreation?.requiredLabelKeys,
  });
  if (compactCreation) spec.namespaceCreation = compactCreation;
  return spec;
}

/**
 * draftErrors returns one error for each rule of the CRD and of the page
 * that the draft breaks, by path. The name is checked only for a new
 * environment, against taken, the names of the environments that exist.
 */
export function draftErrors(
  draft: EnvironmentDraft,
  opts: { isNew: boolean; taken: readonly string[] },
): ValidationError[] {
  const errors: ValidationError[] = [];
  const add = (path: string, message: string) => errors.push({ path, message });
  if (opts.isNew) {
    const message = nameError(draft.name, opts.taken);
    if (message) add("name", message);
  }
  const { spec } = draft;
  if ((spec.displayName ?? "").length > 64)
    add("spec.displayName", "The display name has at most 64 characters.");
  if (spec.color && !isHexColor(spec.color))
    add("spec.color", "The color is a hex color such as #2E7D32.");
  if ((spec.releases?.allowed ?? []).length > MAX_ALLOWED_RELEASES)
    add(
      "spec.releases.allowed",
      `At most ${MAX_ALLOWED_RELEASES} allowed releases.`,
    );
  const defaultRelease = spec.releases?.default;
  if (
    defaultRelease &&
    !(spec.releases?.allowed ?? []).includes(defaultRelease)
  )
    add(
      "spec.releases.default",
      "The default release must be one of the allowed releases.",
    );

  const labels = spec.namespaceCreation?.labels ?? {};
  const labelsPath = "spec.namespaceCreation.labels";
  if (Object.hasOwn(labels, ENVIRONMENT_LABEL))
    add(labelsPath, ENVIRONMENT_LABEL_MESSAGE);
  else if (
    !Object.entries(labels).every(([k, v]) => isLabelKey(k) && isLabelValue(v))
  )
    add(labelsPath, "Each fixed label is a valid label.");
  if (Object.keys(labels).length > MAX_FIXED_LABELS)
    add(labelsPath, `At most ${MAX_FIXED_LABELS} fixed labels.`);

  const keys = spec.namespaceCreation?.requiredLabelKeys ?? [];
  const keysPath = "spec.namespaceCreation.requiredLabelKeys";
  if (keys.includes(ENVIRONMENT_LABEL))
    add(keysPath, ENVIRONMENT_LABEL_MESSAGE);
  else if (!keys.every(isLabelKey))
    add(keysPath, "A required label key is a valid label key.");
  if (new Set(keys).size < keys.length)
    add(keysPath, "Each required label key is used once.");
  if (keys.length > MAX_REQUIRED_LABEL_KEYS)
    add(keysPath, `At most ${MAX_REQUIRED_LABEL_KEYS} required label keys.`);
  return errors;
}

const ENVIRONMENT_LABEL_MESSAGE = `The fleet manager sets ${ENVIRONMENT_LABEL} itself.`;

function nameError(name: string, taken: readonly string[]): string | undefined {
  if (name === "") return "Give the environment a name.";
  if (name.length > 63 || !isDNSSubdomain(name))
    return "The name must be a DNS name of at most 63 characters.";
  // The route /setup/environments/new is the create form.
  if (name === "new")
    return "The name new is kept for the page of a new environment.";
  if (taken.includes(name)) return "An environment with this name exists.";
  return undefined;
}

/**
 * createChange applies a new environment with the spec of specOf. The
 * write fails with Stale when an environment of that name exists.
 */
export function createChange(draft: EnvironmentDraft): Change {
  return {
    intent: `Create the environment ${draft.name}`,
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
 * that kubectl apply set changes too. When the edit turns namespace
 * creation off, the spec holds namespaceCreation.allowed false. It leaves
 * out the last applied file of kubectl.
 */
export function updateChange(
  live: Environment,
  draft: EnvironmentDraft,
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
    spec: updatedSpec(live, draft),
  };
  return {
    intent: `Change the environment ${name}`,
    apply: [object],
    ...preconditionOf(KIND, name, live.metadata.resourceVersion),
    force: true,
  };
}

// Another field manager can own namespaceCreation.allowed, and a save that
// leaves the field out keeps it true.
function updatedSpec(
  live: Environment,
  draft: EnvironmentDraft,
): Environment["spec"] {
  const spec = specOf(draft);
  if (live.spec.namespaceCreation?.allowed && !spec.namespaceCreation?.allowed)
    spec.namespaceCreation = { ...spec.namespaceCreation, allowed: false };
  return spec;
}

/** deleteChange deletes live when it still has its resourceVersion. */
export function deleteChange(live: Environment): Change {
  const name = live.metadata.name ?? "";
  return {
    intent: `Delete the environment ${name}`,
    apply: [],
    delete: [{ apiVersion: API_VERSION, kind: KIND, name }],
    ...preconditionOf(KIND, name, live.metadata.resourceVersion),
  };
}

/**
 * namespaceLabelChange sets the label of environment on namespace while
 * namespace still has its resourceVersion. namespace must carry its
 * metadata.managedFields. Without them the apply drops the labels and
 * annotations the app applied before. It goes to the cluster in Git mode
 * too.
 */
export function namespaceLabelChange(
  namespace: V1Namespace,
  environment: string,
): Change {
  const name = namespace.metadata?.name ?? "";
  const annotations = ownedEntries(namespace, "annotations");
  return {
    intent: `Add the namespace ${name} to the environment ${environment}`,
    apply: [
      {
        apiVersion: "v1",
        kind: "Namespace",
        metadata: {
          name,
          labels: {
            ...ownedEntries(namespace, "labels"),
            [ENVIRONMENT_LABEL]: environment,
          },
          ...(Object.keys(annotations).length > 0 && { annotations }),
        },
      },
    ],
    ...preconditionOf("Namespace", name, namespace.metadata?.resourceVersion),
    directOnly: true,
  };
}

// Server-side apply removes a label or annotation that the app applied
// before and now leaves out, unless another manager also owns it. The
// change must hold these entries to keep them. This is why a label that
// the create flow set stays.
function ownedEntries(
  namespace: V1Namespace,
  field: "labels" | "annotations",
): Record<string, string> {
  const applied = (namespace.metadata?.managedFields ?? []).filter(
    (e) => e.manager === APP_FIELD_MANAGER && e.operation === "Apply",
  );
  const owned = fieldManagers({ metadata: { managedFields: applied } });
  const parent = child("metadata", field);
  return Object.fromEntries(
    Object.entries(namespace.metadata?.[field] ?? {}).filter(
      ([key]) => owned[child(parent, key)] !== undefined,
    ),
  );
}

function preconditionOf(
  kind: string,
  name: string,
  resourceVersion: string | undefined,
): Pick<Change, "preconditions"> {
  return resourceVersion === undefined
    ? {}
    : { preconditions: { [refKey({ kind, name })]: resourceVersion } };
}

// compact returns the entries of object that hold a value that is not
// empty, or undefined when none does.
function compact<T extends Record<string, unknown>>(object: T): T | undefined {
  const entries = Object.entries(object).filter(([, value]) => !isEmpty(value));
  return entries.length > 0 ? (Object.fromEntries(entries) as T) : undefined;
}

function isEmpty(value: unknown): boolean {
  if (value === undefined || value === "") return true;
  if (Array.isArray(value)) return value.length === 0;
  if (typeof value === "object" && value !== null)
    return Object.keys(value).length === 0;
  return false;
}
