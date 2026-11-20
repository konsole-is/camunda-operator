// This module runs in the browser and on the server.
import type { Environment } from "@/generated/crds/environment";
import type { PluginRegistration } from "@/generated/crds/pluginregistration";
import type { PluginSection } from "@/generated/crds/pluginsection";
import type { Resource } from "@/lib/k8s/kinds";
import { ACCESS_SECTION } from "@/lib/labels";
import type { FieldPath } from "@/lib/merge";
import {
  deletionPolicyOf,
  objectOrEmpty,
  parameterPolicies,
  type ParameterPolicies,
  type PluginSelection,
  type ResolvedPolicy,
} from "@/lib/policy";
import { SLOT_FIELDS } from "./slots";
import type { FieldLock } from "./types";
import { pathFrom, type Step } from "./walk";

/** PlaneFieldGroup is where a field of a plane claim shows. */
export type PlaneFieldGroup =
  | "basics"
  | "identity"
  | "environments"
  | "components"
  | "versions"
  | "databases"
  | "access"
  | "deletion";

/**
 * PlaneField is one value of a plane claim. key is unique in one result.
 * value is undefined while the claim has no value. claimPath is where the
 * claim sets the value, and claimSteps are its steps from the root of the
 * claim. note is a fact that the page shows beside the value.
 */
export type PlaneField = {
  key: string;
  group: PlaneFieldGroup;
  label: string;
  value: unknown;
  claimPath: FieldPath;
  claimSteps: Step[];
  lock?: FieldLock;
  note?: string;
};

/**
 * IDENTITY_INITIAL_CLAIM_ANNOTATION is the annotation in which a plane in
 * OIDC mode records the administrator claim that Management Identity
 * started with. It is InitialClaimAnnotation of
 * pkg/components/camundamanagementcluster.
 */
export const IDENTITY_INITIAL_CLAIM_ANNOTATION =
  "camunda.io/identity-initial-claim";

type Mode = "keycloak" | "externalKeycloak" | "oidc";

const VERSIONS = [
  ["identity", "Management Identity version"],
  ["console", "Console version"],
  ["webModeler", "Web Modeler version"],
  ["keycloak", "Keycloak version"],
] as const;

const URLS = {
  keycloak: "Keycloak URL",
  identity: "Management Identity URL",
  console: "Console URL",
  webModeler: "Web Modeler URL",
  webModelerWebsockets: "Web Modeler websockets URL",
} as const;

/**
 * planeFields returns the fields of a plane claim, by group in the order of
 * PlaneFieldGroup. The admin fields follow the identity provider of the
 * claim: claimName and claimValue in OIDC mode, username and email in a
 * Keycloak mode. Versions and URLs show only for the apps that the claim
 * runs, Keycloak only in the keycloak mode.
 *
 * Locks: the name and the namespace are Set at creation. The option of
 * each selection is Chosen at creation, and a parameter is Set by your
 * admin unless its policy makes it claim-editable. The policy is the one
 * that the claim recorded in its status, else the one of the registration
 * and environment. In OIDC mode the administrator claim is Fixed after
 * Identity started once plane carries IDENTITY_INITIAL_CLAIM_ANNOTATION. The
 * typed URLs are Set by a section while the claim selects an option of the
 * access section.
 */
export function planeFields(input: {
  claim: Resource<"CamundaManagementClusterClaim">;
  plane?: Resource<"CamundaManagementCluster">;
  registrations: readonly PluginRegistration[];
  environment?: Environment;
  sections?: readonly PluginSection[];
}): PlaneField[] {
  const { claim, plane, registrations, environment, sections } = input;
  // The access section is the one of the externalAccess slot. Without the
  // sections, only its usual name is known.
  const isAccess = (s: PluginSelection) => {
    const section = sections?.find((x) => x.metadata?.name === s.section);
    return section
      ? section.spec.slot === "externalAccess"
      : s.section === ACCESS_SECTION;
  };
  const spec = claim.spec;
  // Another tool can write null where the CRD expects an object.
  const management: Partial<typeof spec.management> = spec.management ?? {};
  const mode = modeOf(management.identityProvider ?? {});
  const running = {
    identity: true,
    keycloak: mode === "keycloak",
    console: management.console != null,
    webModeler: management.webModeler != null,
  };
  const selections = (spec.plugins ?? []) as PluginSelection[];
  const recorded = claim.status?.resolvedPolicy as ResolvedPolicy | undefined;
  const fields: PlaneField[] = [];
  const add = (
    key: string,
    group: PlaneFieldGroup,
    label: string,
    value: unknown,
    claimSteps: Step[],
    extra: { lock?: FieldLock; note?: string } = {},
  ) =>
    fields.push({
      key,
      group,
      label,
      value,
      claimPath: pathFrom("", claimSteps),
      claimSteps,
      ...extra,
    });
  const atCreation = { lock: { reason: "atCreation" } as const };

  add(
    "name",
    "basics",
    "Name",
    claim.metadata.name,
    fieldSteps("metadata", "name"),
    atCreation,
  );
  add(
    "namespace",
    "basics",
    "Namespace",
    claim.metadata.namespace,
    fieldSteps("metadata", "namespace"),
    atCreation,
  );
  add(
    "platformConfig",
    "basics",
    "Platform config",
    spec.platformConfigRef || environment?.spec.platformConfigRef,
    fieldSteps("spec", "platformConfigRef"),
    spec.platformConfigRef ? {} : { note: "From the environment" },
  );

  add(
    "identityProvider",
    "identity",
    "Identity provider",
    mode,
    fieldSteps("spec", "management", "identityProvider"),
  );
  const admin: Partial<typeof spec.management.identity.admin> =
    management.identity?.admin ?? {};
  const adminSteps = (name: string) =>
    fieldSteps("spec", "management", "identity", "admin", name);
  if (mode === "oidc") {
    const started = Object.hasOwn(
      plane?.metadata.annotations ?? {},
      IDENTITY_INITIAL_CLAIM_ANNOTATION,
    );
    const lock = started
      ? {
          lock: { reason: "immutableAfterStart" } as const,
          note: "Management Identity keeps the first administrator in its database, so a change here has no effect.",
        }
      : {};
    add(
      "admin.claimName",
      "identity",
      "Administrator claim name",
      admin.claimName,
      adminSteps("claimName"),
      lock,
    );
    add(
      "admin.claimValue",
      "identity",
      "Administrator claim value",
      admin.claimValue,
      adminSteps("claimValue"),
      lock,
    );
  } else {
    add(
      "admin.username",
      "identity",
      "Administrator username",
      admin.username,
      adminSteps("username"),
      {
        note: "A new username creates a second user.",
      },
    );
    add(
      "admin.email",
      "identity",
      "Administrator email",
      admin.email,
      adminSteps("email"),
    );
  }

  add(
    "environments",
    "environments",
    "Environments served",
    spec.environments,
    fieldSteps("spec", "environments"),
  );
  add(
    "clusterSelector",
    "environments",
    "Cluster selector",
    spec.clusterSelector,
    fieldSteps("spec", "clusterSelector"),
  );

  add(
    "console",
    "components",
    "Console",
    running.console,
    fieldSteps("spec", "management", "console"),
  );
  add(
    "webModeler",
    "components",
    "Web Modeler",
    running.webModeler,
    fieldSteps("spec", "management", "webModeler"),
  );
  if (running.webModeler) {
    add(
      "webModeler.mail",
      "components",
      "Web Modeler mail",
      management.webModeler?.mail,
      fieldSteps("spec", "management", "webModeler", "mail"),
    );
  }

  const versions: Record<string, string | undefined> = spec.versions ?? {};
  for (const [app, label] of VERSIONS) {
    if (!running[app]) continue;
    add(
      `versions.${app}`,
      "versions",
      label,
      versions[app],
      fieldSteps("spec", "versions", app),
    );
  }

  for (const s of selections) {
    fields.push(
      ...selectionFields(s, isAccess(s), recorded, registrations, environment),
    );
  }

  const accessLock = selections.some(isAccess)
    ? { lock: { reason: "section" } as const }
    : {};
  for (const row of SLOT_FIELDS) {
    if (
      row.claimKind !== "CamundaManagementClusterClaim" ||
      row.slot !== "externalAccess"
    )
      continue;
    // The rows name the field of the CamundaManagementCluster. The claim
    // holds it under spec.management.
    const names = ["spec", "management", ...row.names.slice(1)];
    const id = urlIdOf(row.names.slice(1));
    const app = id === "webModelerWebsockets" ? "webModeler" : id;
    if (!running[app]) continue;
    add(
      `url.${id}`,
      "access",
      URLS[id],
      valueAt(spec, names.slice(1)),
      fieldSteps(...names),
      accessLock,
    );
  }

  add(
    "deletionPolicy",
    "deletion",
    "When the management plane is deleted",
    deletionPolicyOf(spec.deletionPolicy, recorded),
    fieldSteps("spec", "deletionPolicy"),
  );

  return fields.sort(
    (a, b) => GROUPS.indexOf(a.group) - GROUPS.indexOf(b.group),
  );
}

const GROUPS: readonly PlaneFieldGroup[] = [
  "basics",
  "identity",
  "environments",
  "components",
  "versions",
  "databases",
  "access",
  "deletion",
];

function selectionFields(
  s: PluginSelection,
  access: boolean,
  recorded: ResolvedPolicy | undefined,
  registrations: readonly PluginRegistration[],
  environment: Environment | undefined,
): PlaneField[] {
  const group: PlaneFieldGroup = access ? "access" : "databases";
  const item: Step[] = [
    { field: "spec" },
    { field: "plugins" },
    { item: { section: s.section, registration: s.registration } },
  ];
  const registration = registrations.find(
    (r) => r.metadata?.name === s.registration,
  );
  const policies = policiesOf(
    s.registration,
    recorded,
    registration,
    environment,
  );
  // The CRD does not give parameters or their schema a shape.
  const claimed = objectOrEmpty(s.parameters);
  const names = [
    ...new Set([
      ...Object.keys(
        objectOrEmpty(objectOrEmpty(registration?.spec.parameters).properties),
      ),
      ...Object.keys(policies),
      ...Object.keys(claimed),
    ]),
  ].sort();
  const key = `plugins.${s.section}.${s.registration}`;
  const option: Step[] = [...item, { field: "registration" }];
  return [
    {
      key,
      group,
      label: "Option",
      value: s.registration,
      claimPath: pathFrom("", option),
      claimSteps: option,
      lock: { reason: "chosenAtCreation" },
    },
    ...names.map((name): PlaneField => {
      const steps: Step[] = [...item, { field: "parameters" }, { field: name }];
      const policy = Object.hasOwn(policies, name) ? policies[name] : undefined;
      const editable = policy?.claimEditable === true;
      return {
        key: `${key}.${name}`,
        group,
        label: name,
        value:
          editable && Object.hasOwn(claimed, name)
            ? claimed[name]
            : policy?.value,
        claimPath: pathFrom("", steps),
        claimSteps: steps,
        ...(!editable && { lock: { reason: "admin" as const } }),
      };
    }),
  ];
}

function policiesOf(
  name: string,
  recorded: ResolvedPolicy | undefined,
  registration: PluginRegistration | undefined,
  environment: Environment | undefined,
): ParameterPolicies {
  const own = recorded?.parameters;
  if (own && Object.hasOwn(own, name)) return own[name];
  if (!registration) return {};
  const result = parameterPolicies(registration, environment);
  return "value" in result ? result.value : {};
}

function modeOf(provider: object): Mode {
  if ("oidc" in provider && provider.oidc != null) return "oidc";
  if ("externalKeycloak" in provider && provider.externalKeycloak != null)
    return "externalKeycloak";
  return "keycloak";
}

function urlIdOf(names: readonly string[]): keyof typeof URLS {
  if (names[0] === "identityProvider") return "keycloak";
  if (names.at(-1) === "websocketsExternalUrl") return "webModelerWebsockets";
  return names[0] as keyof typeof URLS;
}

function fieldSteps(...names: string[]): Step[] {
  return names.map((field) => ({ field }));
}

function valueAt(value: unknown, names: readonly string[]): unknown {
  return names.reduce<unknown>((at, name) => objectOrEmpty(at)[name], value);
}
