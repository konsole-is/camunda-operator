import type { ConditionRow } from "@/components/fleet/conditions-table";
import type { Grant } from "@/generated/crds/grant";
import type { PluginRegistration } from "@/generated/crds/pluginregistration";
import { REQUEST_LABEL } from "@/lib/labels";
import {
  type Invalid,
  conditionsOf,
  isRecord,
  isText,
  itemsOf,
  listOf,
} from "./values";

/**
 * GRANT_REASON_TEXT says what each reason of a refused Grant means. The keys
 * are ReasonOverCeiling, ReasonOverSizeLimit, and ReasonRejected of Applied,
 * and ReasonTemplateChanged of MatchesTemplate, in
 * api/plugins/v1alpha1/conditions.go.
 */
export const GRANT_REASON_TEXT: Readonly<
  Record<
    "OverCeiling" | "OverSizeLimit" | "Rejected" | "TemplateChanged",
    string
  >
> = {
  OverCeiling: "Asks for more than the ceiling of the identity allows.",
  OverSizeLimit:
    "Makes the policy of the identity larger than the cloud allows.",
  Rejected: "The identity or the cloud refused it. The message says why.",
  TemplateChanged:
    "The role still has the statement that this Grant holds. The controller of this registration writes the Grant from its own template at its next reconcile. If this state stays, the registration was changed outside its controller, and the old statement stays.",
};

/** WITHOUT_TEMPLATE is the title of the Grants of a template that their registration no longer has. */
export const WITHOUT_TEMPLATE =
  "Grants of a template the registration no longer has";

/** TEMPLATE_GONE says what happens to the Grants of WITHOUT_TEMPLATE. */
export const TEMPLATE_GONE =
  "The role keeps the statement of each of these Grants until the controller of this registration deletes that Grant.";

/**
 * GrantView is one Grant. state is ready while Ready is True, oldTemplate
 * while Ready is False with Applied True and MatchesTemplate False, refused
 * while Ready is otherwise False, and waiting while Ready is neither True
 * nor False. error says why its conditions cannot be read, and its state is
 * then waiting.
 */
export type GrantView = {
  namespace: string;
  name: string;
  cluster?: string;
  /** request is the value of REQUEST_LABEL. */
  request?: string;
  applied?: ConditionRow;
  matchesTemplate?: ConditionRow;
  ready?: ConditionRow;
  state: "ready" | "oldTemplate" | "refused" | "waiting";
  error?: string;
};

/** GRANT_CONDITIONS are the condition types of a Grant, in the order its status lists them. */
export const GRANT_CONDITIONS = ["Applied", "MatchesTemplate", "Ready"];

/** grantConditions returns the conditions of grant of GRANT_CONDITIONS that it has. */
export function grantConditions(grant: GrantView): ConditionRow[] {
  return [grant.applied, grant.matchesTemplate, grant.ready].filter(
    (c): c is ConditionRow => c !== undefined,
  );
}

/**
 * refusalText gives the meaning of the reason of the first False condition
 * of a refused Grant, when GRANT_REASON_TEXT has one.
 */
export function refusalText(grant: GrantView): string | undefined {
  if (grant.state !== "refused") return undefined;
  const reason = grantConditions(grant).find(
    (c) => c.status === "False",
  )?.reason;
  return reason && Object.hasOwn(GRANT_REASON_TEXT, reason)
    ? GRANT_REASON_TEXT[reason as keyof typeof GRANT_REASON_TEXT]
    : undefined;
}

/** GrantTemplateView is one grant template of a registration with its Grants. */
export type GrantTemplateView = {
  name: string;
  description?: string;
  format: string;
  /** template is the template as indented JSON, with its Go placeholders as written. */
  template: string;
  requiresInputs: readonly string[];
  ready: number;
  refused: number;
  oldTemplate: number;
  grants: GrantView[];
};

/** InvalidTemplate is an error of the grant templates, with the Grants of its name when it has one. */
export type InvalidTemplate = Invalid & { name?: string; grants: GrantView[] };

/** grantView returns the view of grant. */
export function grantView(grant: Grant): GrantView {
  const read = conditionsOf(grant.status?.conditions);
  const conditions = "conditions" in read ? read.conditions : [];
  const find = (type: string) => conditions.find((c) => c.type === type);
  const ready = find("Ready");
  const cluster = isRecord(grant.spec.cluster)
    ? grant.spec.cluster.name
    : undefined;
  const request = grant.metadata.labels?.[REQUEST_LABEL];
  const applied = find("Applied");
  const matchesTemplate = find("MatchesTemplate");
  return {
    namespace: grant.metadata.namespace ?? "",
    name: grant.metadata.name ?? "",
    ...(typeof cluster === "string" && { cluster }),
    ...(request && { request }),
    ...(applied && { applied }),
    ...(matchesTemplate && { matchesTemplate }),
    ...(ready && { ready }),
    state:
      ready?.status === "True"
        ? "ready"
        : ready?.status !== "False"
          ? "waiting"
          : applied?.status === "True" && matchesTemplate?.status === "False"
            ? "oldTemplate"
            : "refused",
    ...("error" in read && { error: read.error }),
  };
}

/**
 * grantTemplates gives each grant template of registration with its Grants
 * (by spec.template), in template order, and the Grants whose template the
 * registration no longer has. A template that is not valid is an
 * InvalidTemplate.
 */
export function grantTemplates(
  registration: PluginRegistration,
  grants: readonly Grant[],
): {
  templates: (GrantTemplateView | InvalidTemplate)[];
  withoutTemplate: GrantView[];
} {
  const raw = listOf(registration.spec.grantTemplates) ?? [];
  const templates = itemsOf(
    registration.spec.grantTemplates,
    isTemplate,
    "Grant template",
    "grant templates",
  );
  const nameOf = (i: number) => {
    const item = raw[i];
    return isRecord(item) && typeof item.name === "string"
      ? item.name
      : undefined;
  };
  const names = new Set(
    templates.flatMap((t, i) => {
      const name = "error" in t ? nameOf(i) : t.name;
      return name === undefined ? [] : [name];
    }),
  );
  const ofTemplate = (name: string) =>
    grants.filter((g) => g.spec.template === name).map(grantView);
  return {
    templates: templates.map((t, i) => {
      if ("error" in t) {
        const name = nameOf(i);
        return name === undefined
          ? { ...t, grants: [] }
          : { ...t, name, grants: ofTemplate(name) };
      }
      const views = ofTemplate(t.name);
      return {
        name: t.name,
        ...(t.description && { description: t.description }),
        format: t.format,
        template: JSON.stringify(t.template, null, 2),
        requiresInputs: t.requiresInputs ?? [],
        ready: views.filter((g) => g.state === "ready").length,
        refused: views.filter((g) => g.state === "refused").length,
        oldTemplate: views.filter((g) => g.state === "oldTemplate").length,
        grants: views,
      };
    }),
    withoutTemplate: grants
      .filter((g) => !names.has(g.spec.template))
      .map(grantView),
  };
}

type Template = {
  name: string;
  description?: string;
  format: string;
  template: Record<string, unknown>;
  requiresInputs?: string[];
};

function isTemplate(item: unknown): item is Template {
  return (
    isRecord(item) &&
    isText(item.name) &&
    isText(item.description, true) &&
    isText(item.format) &&
    isRecord(item.template) &&
    (item.requiresInputs === undefined ||
      (Array.isArray(item.requiresInputs) &&
        item.requiresInputs.every((i) => typeof i === "string")))
  );
}
