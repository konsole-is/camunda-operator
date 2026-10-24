import type { PluginRequest } from "@/generated/crds/pluginrequest";
import type { PluginSection } from "@/generated/crds/pluginsection";
import { SLOT_FIELDS, type SlotField, type SlotKind } from "@/lib/fields/slots";
import { readyOutputs } from "./inputs";
import { byCodeUnit, isObject, NOT_AN_OBJECT } from "./json";
import type { ClaimKind, PluginSelection, PolicyResult, Slot } from "./types";

/**
 * FieldValue is one field that a slot fills. section is the section whose
 * selection fills it. value is absent while the output is not ready: the
 * claim controller then keeps the live value of the field.
 */
export type FieldValue = {
  kind: SlotKind;
  path: string;
  section: string;
  value?: unknown;
};

/** slotOf returns the slot of section, or undefined for a section without one. */
export function slotOf(section: PluginSection): Slot | undefined {
  return section.spec.slot;
}

/**
 * slotFields returns the fields of the resources of a claim of kind that its
 * selections fill through the slots of their sections, sorted by kind and
 * then by path, by the rule of SlotFields of pkg/claimpolicy. A field has a
 * value only while readyOutputs gives its output. The externalAccess slot of
 * a plane fills five URLs from one host.
 *
 * It refuses with InvalidReference a selection in a section that does not
 * exist. An object output that is not a JSON object is an error.
 */
export function slotFields(
  kind: ClaimKind,
  sections: readonly PluginSection[],
  selections: readonly PluginSelection[],
  requests: readonly PluginRequest[],
): PolicyResult<FieldValue[]> {
  const fields: FieldValue[] = [];
  for (const s of selections) {
    const section = sectionOf(s.section, sections);
    if (!("value" in section)) return section;

    const outputs = readyOutputs(requests, s.section, s.registration) ?? {};
    for (const row of SLOT_FIELDS) {
      if (row.claimKind !== kind || row.slot !== section.value.spec.slot) {
        continue;
      }

      const field: FieldValue = {
        kind: row.kind,
        path: row.path,
        section: s.section,
      };
      if (row.output in outputs) {
        const value = valueOf(row, outputs[row.output]);
        if (value === undefined) {
          return {
            error: `output ${row.output} of section "${s.section}": ${NOT_AN_OBJECT}`,
          };
        }
        field.value = value;
      }
      fields.push(field);
    }
  }

  fields.sort(
    (a, b) => byCodeUnit(a.kind, b.kind) || byCodeUnit(a.path, b.path),
  );
  return { value: fields };
}

/** sectionOf returns the section named name, or refuses with InvalidReference. */
export function sectionOf(
  name: string,
  sections: readonly PluginSection[],
): PolicyResult<PluginSection> {
  const section = sections.find((s) => s.metadata.name === name);
  if (section) return { value: section };
  return {
    refusal: {
      reason: "InvalidReference",
      message: `PluginSection "${name}" does not exist`,
    },
  };
}

// valueOf returns undefined for an object output that is not a JSON object.
function valueOf(row: SlotField, output: string): unknown {
  if (!row.object) return (row.prefix ?? "") + output + (row.suffix ?? "");
  try {
    const object: unknown = JSON.parse(output);
    return isObject(object) ? object : undefined;
  } catch {
    return undefined;
  }
}
