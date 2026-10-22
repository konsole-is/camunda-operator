import type { CamundaClusterClaim } from "@/generated/crds/camundaclusterclaim";
import type { PluginSectionSpec } from "@/generated/crds/pluginsection";
import type { OpenAPISchema } from "@/lib/schema";
import type { BackupRef } from "./backups";

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
 * parameter that a pick sets. schema is the parameter schema of the
 * registration; SectionCards shows the rules of its root, which span more
 * than one parameter.
 */
export type OptionView = {
  registration: string;
  label: string;
  description?: string;
  docsUrl?: string;
  isDefault: boolean;
  disabledReason?: string;
  parameters: ParameterRule[];
  schema?: OpenAPISchema;
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
 * holds, for example "3 items". note is a line under a locked parameter,
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
 * PluginSelection is the selection of one section that a claim stores in
 * spec.plugins. parameters holds only the claim-editable parameters whose
 * value differs from the policy, by parameter name. The CRD schema does not
 * give parameters a shape, so this type adds it.
 */
export type PluginSelection = Omit<GeneratedPluginSelection, "parameters"> & {
  parameters?: Record<string, unknown>;
};

type GeneratedPluginSelection = NonNullable<
  CamundaClusterClaim["spec"]["plugins"]
>[number];

/** RefusalReason is a reason of the Ready condition of a claim that the policy refuses. */
export type RefusalReason =
  | "NoEnvironment"
  | "UnknownEnvironment"
  | "NotAllowed"
  | "MissingInput"
  | "InvalidReference"
  | "SlotConflict"
  | "WiringCycle";

/** Refusal is a value of a claim that the policy refuses, as the claim controller refuses it in its Ready condition. */
export type Refusal = { reason: RefusalReason; message: string };

/** PolicyResult is the outcome of a rule: a value, a refusal, or an error for a malformed registration or plugin value. */
export type PolicyResult<T> =
  { value: T } | { refusal: Refusal } | { error: string };

export type Slot = NonNullable<PluginSectionSpec["slot"]>;
export type ClaimKind = PluginSectionSpec["appliesTo"][number];

/** InputRef is one input of a registration: its name and the section and output it reads (from = "<section>/<output>"). */
export type InputRef = {
  name: string;
  from: string;
  section: string;
  output: string;
};

/**
 * MoveCheck says what a move of a cluster to a release meets. allowed is
 * false when the Environment does not allow the release. current is
 * status.management.version, absent while the cluster publishes none, for
 * example while it is suspended. downgrade is true when target is below
 * current. skippedMinor is the minor to move to first, as major.minor, when
 * target skips one. backup is the backup that runs now.
 */
export type MoveCheck = {
  allowed: boolean;
  target: string;
  current?: string;
  downgrade: boolean;
  skippedMinor?: string;
  backup?: BackupRef;
};
