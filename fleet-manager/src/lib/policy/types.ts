import type { OpenAPISchema } from "@/lib/schema";

/**
 * SectionView is one section of a create flow as a claim author sees it in
 * one environment. options holds only the options that the environment
 * allows. required hides the empty option. emptyOptionLabel is empty when
 * the section has no empty option.
 */
export type SectionView = {
  name: string;
  label: string;
  description?: string;
  selection: "One" | "Many";
  emptyOptionLabel?: string;
  required: boolean;
  options: OptionView[];
};

/**
 * OptionView is one registration of a section. isDefault marks the option
 * that the environment preselects. disabledReason, when set, says why the
 * option cannot be chosen, for example "Needs an encryption key". choices
 * are the presets or resources of the registration, and parameter names the
 * parameter that a pick sets.
 */
export type OptionView = {
  registration: string;
  label: string;
  description?: string;
  docsUrl?: string;
  isDefault: boolean;
  disabledReason?: string;
  parameters: ParameterRule[];
  choices?: {
    parameter: string;
    items: { name: string; specs?: { label: string; value: string }[] }[];
  };
};

/**
 * ParameterRule is one parameter of a registration under the policy.
 * claimEditable says whether a claim can change it. An editable parameter
 * can have value, the value that the environment or the registration gives
 * it, and allowed, which lists the choices by name and label. allowed holds
 * no value: the values of the choices stay with the claim controller
 * (contract CL40). A locked parameter has no value: summary says what it
 * holds, for example "3 policies". note is a line under a locked parameter,
 * for example the pattern of a hostname.
 */
export type ParameterRule = EditableParameter | LockedParameter;

type ParameterBase = {
  name: string;
  label: string;
  schema?: OpenAPISchema;
  required?: boolean;
};

export type EditableParameter = ParameterBase & {
  claimEditable: true;
  value?: unknown;
  allowed?: { name: string; label: string }[];
};

export type LockedParameter = ParameterBase & {
  claimEditable: false;
  summary: string;
  note?: string;
};

/**
 * SectionSelection is the selection of one section that a claim stores.
 * parameters holds only the claim-editable parameters whose value differs
 * from the policy.
 */
export type SectionSelection = {
  section: string;
  registration: string;
  parameters?: Record<string, unknown>;
};
