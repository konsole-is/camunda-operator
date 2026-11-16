import type { PluginRegistration } from "@/generated/crds/pluginregistration";
import type { PluginSection } from "@/generated/crds/pluginsection";
import type { SlotField } from "@/lib/fields/slots";
import { sectionFields } from "./groups";
import { type Invalid, isRecord, isText, itemsOf, listOf } from "./values";

/** ContractSection is one section that a registration serves. label is unset for a section that does not exist. */
export type ContractSection = {
  name: string;
  label?: string;
  fields: SlotField[];
};

/** ContractInput is one input of a registration. fromLabel is "<section label> / <output>" when the section of from exists. */
export type ContractInput = {
  name: string;
  from: string;
  fromLabel?: string;
  formats: string[];
  required: boolean;
};

/** ContractOutput is one output of a registration. */
export type ContractOutput = {
  key: string;
  description?: string;
  format?: string;
};

/**
 * ContractView is what a registration takes and gives. An item that is not
 * valid is an Invalid with its reason. sectionsUnknown is true while
 * PluginSection is not read, so a section without a label can exist.
 */
export type ContractView = {
  sections: (ContractSection | Invalid)[];
  sectionsUnknown?: true;
  inputs: (ContractInput | Invalid)[];
  outputs: (ContractOutput | Invalid)[];
  deletionPolicies: readonly ("Delete" | "Orphan")[] | Invalid;
};

/**
 * contractOf gives the sections, inputs, outputs, and deletion policies that
 * registration declares, with the labels of sections. unread holds the
 * kinds that are not read. Only Delete and Orphan count as deletion
 * policies.
 */
export function contractOf(
  registration: PluginRegistration,
  sections: readonly PluginSection[],
  unread: readonly string[],
): ContractView {
  const { spec } = registration;
  const sectionOf = (name: string) =>
    sections.find((s) => s.metadata.name === name);
  const policies = listOf(spec.deletionPolicies);
  return {
    sections: itemsOf(spec.sections, isName, "Section", "sections").map(
      (item) => {
        if (typeof item !== "string") return item;
        const section = sectionOf(item);
        return section
          ? {
              name: item,
              label: isText(section.spec.label) ? section.spec.label : item,
              fields: sectionFields(section),
            }
          : { name: item, fields: [] };
      },
    ),
    ...(unread.includes("PluginSection") && { sectionsUnknown: true }),
    inputs: itemsOf(spec.inputs, isInput, "Input", "inputs").map((item) => {
      if ("error" in item) return item;
      const [section, output] = splitFrom(item.from);
      const label = section && sectionOf(section)?.spec.label;
      return {
        name: item.name,
        from: item.from,
        ...(label && { fromLabel: `${label} / ${output}` }),
        formats: item.formats ?? [],
        required: item.required === true,
      };
    }),
    outputs: itemsOf(spec.outputs, isOutput, "Output", "outputs").map((item) =>
      "error" in item
        ? item
        : {
            key: item.key,
            ...(item.description && { description: item.description }),
            ...(item.format && { format: item.format }),
          },
    ),
    deletionPolicies: policies
      ? (["Delete", "Orphan"] as const).filter((p) => policies.includes(p))
      : { error: "The deletion policies are not a list." },
  };
}

type Input = {
  name: string;
  from: string;
  formats?: string[];
  required?: boolean;
};

function isName(item: unknown): item is string {
  return typeof item === "string";
}

function isInput(item: unknown): item is Input {
  return (
    isRecord(item) &&
    isText(item.name) &&
    isText(item.from) &&
    (item.formats === undefined ||
      (Array.isArray(item.formats) && item.formats.every(isName))) &&
    (item.required === undefined || typeof item.required === "boolean")
  );
}

function isOutput(item: unknown): item is ContractOutput {
  return (
    isRecord(item) &&
    isText(item.key) &&
    isText(item.description, true) &&
    isText(item.format, true)
  );
}

// splitFrom gives the section and the output of "<section>/<output>".
function splitFrom(from: string): [string, string] | [] {
  const slash = from.indexOf("/");
  return slash < 0 ? [] : [from.slice(0, slash), from.slice(slash + 1)];
}
