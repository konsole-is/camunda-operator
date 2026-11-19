import {
  DESCRIPTION_ANNOTATION,
  LAST_APPLIED_ANNOTATION,
  presetDescription,
} from "@/lib/labels";
import { equalJSON } from "@/lib/policy";
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
      [live.kind === "DatabaseServerPreset" ? "server" : "cluster"]: draft.body,
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
