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
  TemplateChanged: "No longer matches its template in the registration.",
};

/**
 * GrantView is one Grant. state is ready while Ready is True, refused while
 * Ready is False, and waiting without Ready. error says why its conditions
 * cannot be read, and its state is then waiting.
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
  state: "ready" | "refused" | "waiting";
  error?: string;
};

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
  grants: GrantView[];
};

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
        : ready?.status === "False"
          ? "refused"
          : "waiting",
    ...("error" in read && { error: read.error }),
  };
}

/**
 * grantTemplates gives each grant template of registration with its Grants
 * (by spec.template), in template order, and the Grants whose template the
 * registration no longer has. A template that is not valid is an Invalid.
 */
export function grantTemplates(
  registration: PluginRegistration,
  grants: readonly Grant[],
): {
  templates: (GrantTemplateView | Invalid)[];
  withoutTemplate: GrantView[];
} {
  const templates = itemsOf(
    registration.spec.grantTemplates,
    isTemplate,
    "Grant template",
    "grant templates",
  );
  const names = new Set(
    templates.flatMap((t) => ("error" in t ? [] : [t.name])),
  );
  const ofTemplate = (name: string) =>
    grants.filter((g) => g.spec.template === name).map(grantView);
  return {
    templates: templates.map((t) => {
      if ("error" in t) return t;
      const views = ofTemplate(t.name);
      return {
        name: t.name,
        ...(t.description && { description: t.description }),
        format: t.format,
        template: JSON.stringify(t.template, null, 2),
        requiresInputs: t.requiresInputs ?? [],
        ready: views.filter((g) => g.state === "ready").length,
        refused: views.filter((g) => g.state === "refused").length,
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
