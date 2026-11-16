// This module runs in the browser and on the server.
import type { Environment } from "@/generated/crds/environment";
import type { PluginRegistration } from "@/generated/crds/pluginregistration";
import type { PluginRequest } from "@/generated/crds/pluginrequest";
import type { PluginSection } from "@/generated/crds/pluginsection";
import {
  parseTemplate,
  renderJSON,
  type TemplateData,
  type TemplateError,
} from "@/lib/gotemplate";
import {
  type ClaimKind,
  claimEditable,
  parameterPolicies,
  type PluginSelection,
  resolve,
  targetServiceAccount,
} from "@/lib/policy";
import { requestName } from "./requests";

/** TemplateClaim is the claim that a template renders for. */
export type TemplateClaim = {
  kind: ClaimKind;
  namespace: string;
  name: string;
  plugins: PluginSelection[];
};

/**
 * templateData returns the placeholder values of claim for the request of
 * registration in section. A live request of the claim gives its name and
 * its target account; else requestName and targetServiceAccount give them.
 * parameters are the request parameters that resolve gives under
 * environment, with the selection of the claim, or none. outputs is empty,
 * because a template parameter cannot read them. A policy that resolve
 * refuses gives no parameters.
 */
export function templateData(input: {
  claim: TemplateClaim;
  section: PluginSection;
  registration: PluginRegistration;
  environment: Environment | undefined;
  registrations: readonly PluginRegistration[];
  requests: readonly PluginRequest[];
}): TemplateData {
  const { claim, section, registration } = input;
  const sectionName = section.metadata.name ?? "";
  const registrationName = registration.metadata.name ?? "";
  const live = input.requests.find(
    (r) =>
      r.metadata.namespace === claim.namespace &&
      r.spec.claimRef.kind === claim.kind &&
      r.spec.claimRef.name === claim.name &&
      r.spec.section === sectionName &&
      r.spec.registration === registrationName,
  );
  const selection = claim.plugins.find(
    (p) => p.section === sectionName && p.registration === registrationName,
  ) ?? { section: sectionName, registration: registrationName };
  const resolved = resolve({
    environment: input.environment,
    registrations: [...input.registrations],
    selections: [selection],
  });
  const parameters =
    ("value" in resolved && resolved.value.requests?.[0]?.parameters) || {};
  return {
    namespace: claim.namespace,
    claim: claim.name,
    request:
      live?.metadata.name ??
      requestName(claim.name, selection, section.spec.selection),
    serviceAccount:
      live?.spec.target.serviceAccountName ??
      targetServiceAccount(claim.kind, claim.name, section),
    parameters,
    outputs: {},
  };
}

/**
 * GrantPreview is one grant template of one selection, rendered before the
 * write. result is the rendered template, or the first error of the
 * template or of a parameter that the plugin renders first, with the path
 * of the string that failed. onlyWhen names the inputs that must have a
 * value before the plugin writes the grant.
 */
export type GrantPreview = {
  section: string;
  sectionLabel: string;
  registration: string;
  optionLabel: string;
  template: string;
  description?: string;
  onlyWhen?: string[];
  result: { value: unknown } | (TemplateError & { path: string });
};

/**
 * grantPreviews renders each grant template of each selection of claim, in
 * claim order and then in template order. A selection whose section or
 * registration is missing gives nothing, and so does a template that
 * requires an input of a section that claim does not select. No ARN of a
 * parameter shows: a string, number, or boolean value that a choice gives
 * shows as <label: choice label>, any other ARN as <label: An ARN that you
 * entered> when the selection sets it, else as <label: Set by your admin>. An output, which exists only after the
 * write, shows as <description from section label>.
 */
export function grantPreviews(input: {
  claim: TemplateClaim;
  sections: readonly PluginSection[];
  registrations: readonly PluginRegistration[];
  environment: Environment | undefined;
}): GrantPreview[] {
  const { claim, environment } = input;
  return claim.plugins.flatMap((selection) => {
    const section = input.sections.find(
      (s) => s.metadata.name === selection.section,
    );
    const registration = input.registrations.find(
      (r) => r.metadata.name === selection.registration,
    );
    if (!section || !registration) return [];
    const sectionLabel = section.spec.label || selection.section;
    const data = templateData({
      claim,
      section,
      registration,
      environment,
      registrations: input.registrations,
      // Before the write, no request of the claim exists.
      requests: [],
    });
    const shown = shownParameters(
      data.parameters,
      selection,
      registration,
      environment,
    );
    // The plugin renders the real values, so its errors come from them. The
    // preview renders the shown values, so that a template parameter cannot
    // copy an ARN into the preview.
    const real = withTemplateParameters(data, registration);
    const display = withTemplateParameters(
      { ...data, parameters: shown },
      registration,
    );
    const failed =
      "error" in real ? real : "error" in display ? display : undefined;
    const parameters = "error" in display ? {} : display.value;
    return (registration.spec.grantTemplates ?? []).flatMap(
      (template): GrantPreview[] => {
        const onlyWhen = template.requiresInputs ?? [];
        const selected = onlyWhen.every((name) => {
          const from = registration.spec.inputs?.find(
            (i) => i.name === name,
          )?.from;
          const inputSection = from?.split("/")[0];
          return claim.plugins.some((p) => p.section === inputSection);
        });
        // The plugin writes no Grant for a template whose required input
        // has no value, and an input of a section without a selection has
        // none.
        if (!selected) return [];
        const outputs = Object.fromEntries(
          outputKeys(template.template).map((key) => {
            const description =
              registration.spec.outputs?.find((o) => o.key === key)
                ?.description || key;
            return [key, `<${description} from ${sectionLabel}>`];
          }),
        );
        return [
          {
            section: selection.section,
            sectionLabel,
            registration: selection.registration,
            optionLabel: registration.spec.label || selection.registration,
            template: template.name,
            ...(template.description && { description: template.description }),
            ...(onlyWhen.length > 0 && { onlyWhen }),
            result:
              failed ??
              renderJSON(
                template.template,
                { ...data, parameters, outputs },
                "grant",
              ),
          },
        ];
      },
    );
  });
}

// withTemplateParameters returns the parameters of data with each one that
// the schema of registration marks with x-fleet-template rendered, as the
// plugin renders them before a grant template. It returns the first error,
// in name order.
function withTemplateParameters(
  data: TemplateData,
  registration: PluginRegistration,
): { value: Record<string, unknown> } | (TemplateError & { path: string }) {
  const schema = registration.spec.parameters as
    | { properties?: Record<string, { "x-fleet-template"?: unknown }> }
    | undefined;
  const parameters = { ...data.parameters };
  for (const name of Object.keys(schema?.properties ?? {}).sort()) {
    if (schema?.properties?.[name]?.["x-fleet-template"] !== true) continue;
    if (!Object.hasOwn(parameters, name)) continue;
    const rendered = renderJSON(
      parameters[name],
      { ...data, outputs: {} },
      "parameter",
    );
    if ("error" in rendered) {
      const at = rendered.path.startsWith("[") ? "" : ".";
      return {
        ...rendered,
        path: `parameters.${name}${rendered.path && at}${rendered.path}`,
      };
    }
    parameters[name] = rendered.value;
  }
  return { value: parameters };
}

function shownParameters(
  values: Record<string, unknown>,
  selection: PluginSelection,
  registration: PluginRegistration,
  environment: Environment | undefined,
): Record<string, unknown> {
  const rules = claimEditable(registration, environment);
  const policies = parameterPolicies(registration, environment);
  const set = selection.parameters ?? {};
  return Object.fromEntries(
    Object.entries(values).map(([name, value]) => {
      // A grant template cannot print a list or an object, so such a value
      // stays for render to refuse.
      if (typeof value === "object") return [name, value];
      const rule = rules.find((r) => r.name === name);
      const label = rule?.label ?? name;
      const named = (text: string) => `<${label}: ${text}>`;
      const allowed = rule?.claimEditable ? (rule.allowed ?? []) : [];
      const choiceLabel = (n: unknown) =>
        allowed.find((c) => c.name === n)?.label;
      // The selection holds the names of the choices, before resolve
      // turns them into their values.
      if (Object.hasOwn(set, name) && allowed.length > 0) {
        const picked = set[name];
        const labels = (Array.isArray(picked) ? picked : [picked]).map(
          (n) => choiceLabel(n) ?? String(n),
        );
        return [name, named(labels.join(", "))];
      }
      const choices =
        ("value" in policies && policies.value[name]?.allowed) || [];
      const choice = choices.find(
        (c) => JSON.stringify(c.value) === JSON.stringify(value),
      );
      if (choice) return [name, named(choice.label || choice.name)];
      const arn = Object.hasOwn(set, name)
        ? named("An ARN that you entered")
        : named("Set by your admin");
      return [
        name,
        typeof value === "string" && value.startsWith("arn:") ? arn : value,
      ];
    }),
  );
}

// outputKeys returns each output key that a string of template references.
function outputKeys(template: unknown): string[] {
  const keys = new Set<string>();
  const walk = (value: unknown) => {
    if (typeof value === "string") {
      const parsed = parseTemplate(value, "grant");
      if (!("error" in parsed)) for (const key of parsed.outputs) keys.add(key);
    } else if (Array.isArray(value)) value.forEach(walk);
    else if (typeof value === "object" && value !== null)
      Object.values(value).forEach(walk);
  };
  walk(template);
  return [...keys];
}
