import {
  DESCRIPTION_ANNOTATION,
  LAST_APPLIED_ANNOTATION,
  presetDescription,
  referenceNameError,
} from "@/lib/labels";
import { equalJSON } from "@/lib/policy";
import type { ValidationError } from "@/lib/schema";
import type { PresetKind } from "@/lib/usage";
import { keepEmpty } from "@/lib/write/ownership";
import { refKey } from "@/lib/write/refs";
import type { Change, KubeObject } from "@/lib/write/types";
import { type PresetObject, presetBody } from "./kinds";

/** PresetDraft is the form of one preset: its body (spec.cluster or spec.server) and its description. */
export type PresetDraft = {
  body: Record<string, unknown>;
  description: string;
};

/** presetDraftOf returns the draft of preset, or an empty draft without one. */
export function presetDraftOf(preset: PresetObject | undefined): PresetDraft {
  return {
    body: structuredClone(preset ? (presetBody(preset) ?? {}) : {}),
    description: preset ? (presetDescription(preset) ?? "") : "",
  };
}

/**
 * presetUpdateChange applies live with the body and the description of
 * draft, with the resourceVersion of live as its precondition, and without
 * force. A blank description leaves out the annotation. In direct mode,
 * an annotation that another field manager set stays, unless the take-over
 * of the review removes it. It keeps the labels and the other annotations
 * of live, without the last applied file of kubectl.
 */
export function presetUpdateChange(
  live: PresetObject,
  draft: PresetDraft,
): Change {
  const name = live.metadata.name ?? "";
  const { labels, annotations } = live.metadata;
  const kept: Record<string, string> = Object.fromEntries(
    Object.entries(annotations ?? {}).filter(
      ([key]) =>
        key !== LAST_APPLIED_ANNOTATION && key !== DESCRIPTION_ANNOTATION,
    ),
  );
  const description = draft.description.trim();
  const edited = { ...kept, [DESCRIPTION_ANNOTATION]: description };
  if (description !== "") kept[DESCRIPTION_ANNOTATION] = description;
  const object: KubeObject = {
    apiVersion: live.apiVersion,
    kind: live.kind,
    metadata: {
      name,
      ...(labels && { labels }),
      ...(Object.keys(kept).length > 0 && { annotations: kept }),
    },
    spec: {
      [bodyField(live.kind)]: draft.body,
    },
  };
  return keepEmpty(
    {
      intent: `Change the preset ${name}`,
      apply: [object],
      ...(live.metadata.resourceVersion !== undefined && {
        preconditions: {
          [refKey({ kind: live.kind, name })]: live.metadata.resourceVersion,
        },
      }),
    },
    live,
    { ...object, metadata: { ...object.metadata, annotations: edited } },
  );
}

/**
 * presetChangeCount counts the top-level fields of the body that draft
 * changes against live, plus one when the trimmed description changes.
 * Without live it counts against an empty preset.
 */
export function presetChangeCount(
  live: PresetObject | undefined,
  draft: PresetDraft,
): number {
  const base = presetDraftOf(live);
  const keys = new Set([...Object.keys(base.body), ...Object.keys(draft.body)]);
  const changed = [...keys].filter(
    (k) => !equalJSON(base.body[k], draft.body[k]),
  ).length;
  return changed + (base.description !== draft.description.trim() ? 1 : 0);
}

/** PresetCreateDraft is the form of a new preset: its name, and its body and description. */
export type PresetCreateDraft = { name: string; preset: PresetDraft };

/**
 * nameErrors returns the error of name for a new preset, with the path
 * "name", or none. taken are the names of the presets of the same kind.
 */
export function nameErrors(
  name: string,
  taken: readonly string[],
): ValidationError[] {
  const error = (message: string) => [{ path: "name", message }];
  if (name === "") return error("Give the preset a name.");
  const invalid = referenceNameError(name);
  if (invalid) return error(invalid);
  if (taken.includes(name))
    return error("A preset of this kind with this name exists.");
  return [];
}

/**
 * createChange creates the preset name of kind with the body and the
 * description of draft. A blank description leaves out the annotation.
 * The write fails with Stale when the preset exists.
 */
export function createChange(
  kind: PresetKind,
  name: string,
  draft: PresetDraft,
): Change {
  const description = draft.description.trim();
  return {
    intent: `Create the preset ${name}`,
    apply: [
      {
        apiVersion: "core.camunda.io/v1",
        kind,
        metadata: {
          name,
          ...(description !== "" && {
            annotations: { [DESCRIPTION_ANNOTATION]: description },
          }),
        },
        spec: { [bodyField(kind)]: draft.body },
      },
    ],
    create: [refKey({ kind, name })],
  };
}

/**
 * createEffects says what the create of a preset changes. allowed are the
 * environments that list its name already, or undefined when they cannot
 * be read.
 */
export function createEffects(
  allowed: readonly string[] | undefined,
): string[] {
  const uses = "A resource that names this preset uses it from now on.";
  if (allowed === undefined)
    return [
      uses,
      "The environments that allow it are not known while the fleet manager cannot read them.",
    ];
  if (allowed.length === 0) return [uses, "No environment allows it yet."];
  return [
    uses,
    allowed.length === 1
      ? `The environment ${allowed[0]} lists its name and allows it at once.`
      : `The environments ${allowed.join(", ")} list its name and allow it at once.`,
  ];
}

function bodyField(kind: PresetKind): "cluster" | "server" {
  return kind === "DatabaseServerPreset" ? "server" : "cluster";
}
