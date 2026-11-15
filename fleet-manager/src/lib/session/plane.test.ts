import { describe, expect, it } from "vitest";
import { pathFrom, type Step } from "@/lib/fields";
import { CLAIM_LABEL } from "@/lib/labels";
import { EMPTY_MAIL } from "@/lib/planes/components";
import { mailOf } from "@/lib/planes/draft";
import {
  componentEdits,
  identityMoveEdits,
  type PlaneEdit,
} from "@/lib/planes/edits";
import {
  validatePlaneClaim,
  type PlaneClaim,
  type PlaneClaimContext,
} from "./plane";
import { openSession, resetValue, setValue, type Session } from "./session";

const managed = { metadata: { labels: { [CLAIM_LABEL]: "mgmt" } } };
const at = (...names: string[]): Step[] => names.map((field) => ({ field }));
const ADMIN = ["spec", "management", "identity", "admin"];

function planeClaim(spec: Partial<PlaneClaim["spec"]> = {}): PlaneClaim {
  return {
    apiVersion: "fleet.camunda.io/v1alpha1",
    kind: "CamundaManagementClusterClaim",
    metadata: { name: "mgmt", namespace: "mgmt-ns", resourceVersion: "1" },
    spec: {
      environments: ["production"],
      clusterSelector: {},
      management: {
        identityProvider: { keycloak: {} },
        identity: { admin: { username: "admin" } },
      },
      versions: { identity: "8.9.14", keycloak: "26.6.4" },
      ...spec,
    },
  } as PlaneClaim;
}

const oidcClaim = () =>
  planeClaim({
    platformConfigRef: "sso",
    management: {
      identityProvider: { oidc: {} },
      identity: { admin: { claimName: "sub", claimValue: "42" } },
    },
    versions: { identity: "8.9.14" },
  });

const asEdit = (e: PlaneEdit): [Step[], unknown] => [
  e.steps,
  "reset" in e ? undefined : e.value,
];

function changed(
  claim: PlaneClaim,
  ...edits: [Step[], unknown][]
): Session<PlaneClaim> {
  let s = openSession({ claim, resource: managed, locks: () => undefined })!;
  for (const [steps, value] of edits) {
    const result =
      value === undefined ? resetValue(s, steps) : setValue(s, steps, value);
    expect("session" in result).toBe(true);
    if ("session" in result) s = result.session;
  }
  return s;
}

const basic = {
  metadata: { name: "basic" },
  spec: { auth: { method: "basic" } },
};
const sso = {
  metadata: { name: "sso" },
  spec: {
    auth: { method: "oidc", oidc: { issuerUrl: "https://sso.example.com" } },
  },
};
const ctx = (over: Partial<PlaneClaimContext> = {}): PlaneClaimContext =>
  ({
    environment: {
      metadata: { name: "platform" },
      spec: { platformConfigRef: "basic" },
    },
    environments: [
      { metadata: { name: "production" }, spec: {} },
      { metadata: { name: "staging" }, spec: {} },
    ],
    platformConfigs: [basic, sso],
    sections: [],
    registrations: [],
    requests: [],
    ...over,
  }) as PlaneClaimContext;

describe("validatePlaneClaim", () => {
  it("accepts a change that breaks no rule", () => {
    const s = changed(planeClaim(), [
      at("spec", "environments"),
      ["production", "staging"],
    ]);
    expect(validatePlaneClaim(s, ctx())).toEqual([]);
  });

  it("asks for one environment or more", () => {
    const s = changed(planeClaim(), [at("spec", "environments"), []]);
    expect(validatePlaneClaim(s, ctx())).toEqual([
      { path: "spec.environments", message: "Pick at least one environment." },
    ]);
  });

  it("refuses an added name that no Environment has", () => {
    const s = changed(planeClaim(), [
      at("spec", "environments"),
      ["production", "qa"],
    ]);
    expect(validatePlaneClaim(s, ctx())).toEqual([
      { path: "spec.environments", message: "Environment qa does not exist." },
    ]);
  });

  it("a deletion policy change passes while the live claim has an unknown environment", () => {
    const live = planeClaim({ environments: ["gone"] });
    const s = changed(live, [at("spec", "deletionPolicy"), "Delete"]);
    expect(validatePlaneClaim(s, ctx())).toEqual([]);
  });

  it("a deletion policy change passes while the live claim has a version that is now refused", () => {
    const live = planeClaim({
      versions: { identity: "8.8.0", keycloak: "26.6.4" },
    });
    const s = changed(live, [at("spec", "deletionPolicy"), "Delete"]);
    expect(validatePlaneClaim(s, ctx())).toEqual([]);
  });

  it("refuses a move to OIDC while the platform config uses basic authentication", () => {
    const live = planeClaim();
    const s = changed(
      live,
      ...identityMoveEdits(live.spec, "oidc").map(asEdit),
    );
    expect(validatePlaneClaim(s, ctx())).toContainEqual({
      path: "spec.management.identityProvider",
      message:
        "OIDC needs a platform config with OIDC authentication. basic uses basic authentication.",
    });
  });

  it("takes the platform config of the claim over the one of the Environment", () => {
    const live = planeClaim({ platformConfigRef: "basic" });
    const s = changed(
      live,
      ...identityMoveEdits(live.spec, "oidc").map(asEdit),
    );
    const environment = {
      metadata: { name: "platform" },
      spec: { platformConfigRef: "sso" },
    };
    expect(
      validatePlaneClaim(s, ctx({ environment } as Partial<PlaneClaimContext>)),
    ).toContainEqual({
      path: "spec.management.identityProvider",
      message:
        "OIDC needs a platform config with OIDC authentication. basic uses basic authentication.",
    });
  });

  it("does not refuse a plane that stays in OIDC mode with a basic platform config", () => {
    const live = oidcClaim();
    const s = changed(live, [at("spec", "deletionPolicy"), "Delete"]);
    const environment = {
      metadata: { name: "platform" },
      spec: { platformConfigRef: "basic" },
    };
    const claimOnBasic = changed(
      { ...live, spec: { ...live.spec, platformConfigRef: undefined } },
      [at("spec", "deletionPolicy"), "Delete"],
    );
    expect(validatePlaneClaim(s, ctx())).toEqual([]);
    expect(
      validatePlaneClaim(
        claimOnBasic,
        ctx({ environment } as Partial<PlaneClaimContext>),
      ),
    ).toEqual([]);
  });

  it("asks where users sign in when the provider names no mode or two", () => {
    const none = changed(planeClaim(), [
      at("spec", "management", "identityProvider"),
      {},
    ]);
    const two = changed(planeClaim(), [
      at("spec", "management", "identityProvider"),
      { keycloak: {}, oidc: {} },
    ]);
    const error = {
      path: "spec.management.identityProvider",
      message: "Pick where users sign in.",
    };
    expect(validatePlaneClaim(none, ctx())).toContainEqual(error);
    expect(validatePlaneClaim(two, ctx())).toContainEqual(error);
  });

  it("asks for the claim pair after a move to OIDC", () => {
    const live = planeClaim({ platformConfigRef: "sso" });
    const s = changed(
      live,
      ...identityMoveEdits(live.spec, "oidc").map(asEdit),
    );
    expect(validatePlaneClaim(s, ctx())).toEqual([
      {
        path: "spec.management.identity.admin.claimName",
        message: "Enter the claim name.",
      },
      {
        path: "spec.management.identity.admin.claimValue",
        message: "Enter the claim value.",
      },
    ]);
  });

  it("refuses a claim name with an equals sign", () => {
    const s = changed(oidcClaim(), [at(...ADMIN, "claimName"), "a=b"]);
    expect(validatePlaneClaim(s, ctx())).toEqual([
      {
        path: "spec.management.identity.admin.claimName",
        message: "The claim name cannot hold an equals sign.",
      },
    ]);
  });

  it("asks to remove the Keycloak admin fields in OIDC mode", () => {
    const s = changed(
      oidcClaim(),
      [at(...ADMIN, "username"), "admin"],
      [at(...ADMIN, "passwordSecretRef"), { name: "pw", key: "password" }],
    );
    expect(validatePlaneClaim(s, ctx())).toEqual([
      {
        path: "spec.management.identity.admin.username",
        message:
          "Remove the username. OIDC mode names the administrator by a claim.",
      },
      {
        path: "spec.management.identity.admin.passwordSecretRef",
        message: "Remove the password Secret. OIDC mode takes none.",
      },
    ]);
  });

  it("asks for the username after a move from OIDC to a Keycloak mode", () => {
    const live = oidcClaim();
    const s = changed(
      live,
      ...identityMoveEdits(live.spec, "keycloak").map(asEdit),
    );
    expect(validatePlaneClaim(s, ctx())).toEqual([
      {
        path: "spec.management.identity.admin.username",
        message: "Enter the username.",
      },
      {
        path: "spec.versions.keycloak",
        message: "Enter the Keycloak version.",
      },
    ]);
  });

  it("asks to remove the claim pair in a Keycloak mode", () => {
    const s = changed(planeClaim(), [at(...ADMIN, "claimName"), "sub"]);
    expect(validatePlaneClaim(s, ctx())).toEqual([
      {
        path: "spec.management.identity.admin.claimName",
        message:
          "Remove the claim name and value. A Keycloak mode names the administrator by a username.",
      },
    ]);
  });

  it("asks for the admin email when Web Modeler runs in a Keycloak mode", () => {
    const live = planeClaim();
    const s = changed(
      live,
      ...componentEdits(live.spec, "webModeler", true).map(asEdit),
    );
    expect(validatePlaneClaim(s, ctx())).toContainEqual({
      path: "spec.management.identity.admin.email",
      message:
        "Web Modeler needs the email of the administrator in a Keycloak mode.",
    });
  });

  it("asks for the version and the mail of an app that the change turns on", () => {
    const live = planeClaim({
      management: {
        identityProvider: { keycloak: {} },
        identity: { admin: { username: "admin", email: "a@example.com" } },
      },
    });
    const s = changed(
      live,
      ...componentEdits(live.spec, "webModeler", true).map(asEdit),
    );
    const mail = "spec.management.webModeler.mail";
    expect(validatePlaneClaim(s, ctx())).toEqual([
      {
        path: "spec.versions.webModeler",
        message: "Enter the Web Modeler version.",
      },
      { path: mail, message: "Enter the SMTP host." },
      {
        path: mail,
        message: "Enter the address that Web Modeler sends from.",
      },
    ]);
  });

  it("gives each version error its field path, with Keycloak checked on its own range", () => {
    const s = changed(planeClaim(), [
      at("spec", "versions", "keycloak"),
      "27.0.0",
    ]);
    expect(validatePlaneClaim(s, ctx())).toEqual([
      {
        path: "spec.versions.keycloak",
        message:
          "Keycloak needs a version from 26.0.0 up to, but not including, 27.0.0.",
      },
    ]);
  });

  it("gives no error for a version warning", () => {
    const s = changed(planeClaim(), [
      at("spec", "versions", "identity"),
      "8.9.10",
    ]);
    expect(validatePlaneClaim(s, ctx())).toEqual([]);
  });

  it("checks the External Keycloak fields at their claim paths", () => {
    const live = planeClaim();
    const external = { url: "keycloak", realm: "", adminSecret: "" };
    const s = changed(
      live,
      ...identityMoveEdits(live.spec, "externalKeycloak", external).map(asEdit),
    );
    expect(validatePlaneClaim(s, ctx()).map((e) => e.path)).toEqual([
      "spec.management.identityProvider.externalKeycloak.url",
      "spec.management.identityProvider.externalKeycloak.adminCredentialsSecretRef.name",
    ]);
  });

  it("refuses Orphan when a selected registration cannot keep its resources", () => {
    const live = planeClaim({
      plugins: [{ section: "access", registration: "gateway" }],
    });
    const registrations = [
      { metadata: { name: "gateway" }, spec: { deletionPolicies: ["Delete"] } },
    ];
    const s = changed(live, [at("spec", "deletionPolicy"), "Orphan"]);
    expect(
      validatePlaneClaim(
        s,
        ctx({ registrations } as unknown as Partial<PlaneClaimContext>),
      ),
    ).toEqual([
      {
        message:
          'the deletion policy is Orphan, and these registrations do not support it: PluginRegistration "gateway" of section "access"',
      },
    ]);
  });

  it("refuses a parameter change that the registration schema refuses", () => {
    const live = planeClaim({
      plugins: [
        {
          section: "access",
          registration: "gateway",
          parameters: { replicas: 1 },
        },
      ],
    });
    const registrations = [
      {
        metadata: { name: "gateway" },
        spec: {
          deletionPolicies: ["Delete"],
          parameters: {
            type: "object",
            properties: { replicas: { type: "integer", minimum: 1 } },
          },
        },
      },
    ];
    const steps: Step[] = [
      ...at("spec", "plugins"),
      { item: { section: "access", registration: "gateway" } },
      ...at("parameters", "replicas"),
    ];
    const s = changed(live, [steps, 0]);
    expect(
      validatePlaneClaim(
        s,
        ctx({ registrations } as unknown as Partial<PlaneClaimContext>),
      ).map((e) => e.path),
    ).toEqual([pathFrom("", steps)]);
  });

  it("refuses a registration that the resolved policy does not allow", () => {
    const live = {
      ...planeClaim({
        plugins: [{ section: "access", registration: "gateway" }],
      }),
      status: {
        resolvedPolicy: { sections: { access: { allowed: ["other"] } } },
      },
    } as PlaneClaim;
    const s = changed(live, [at("spec", "deletionPolicy"), "Delete"]);
    const errors = validatePlaneClaim(s, ctx());
    expect(errors).toHaveLength(1);
    expect(errors[0].path).toBeUndefined();
    expect(errors[0].message).toMatch(/gateway/);
  });

  it("a deletion policy change passes while the live Web Modeler mail breaks a rule", () => {
    const live = planeClaim({
      management: {
        identityProvider: { keycloak: {} },
        identity: { admin: { username: "admin", email: "a@example.com" } },
        webModeler: { mail: mailOf(EMPTY_MAIL) },
      },
      versions: {
        identity: "8.9.14",
        keycloak: "26.6.4",
        webModeler: "8.9.15",
      },
    });
    const s = changed(live, [at("spec", "deletionPolicy"), "Delete"]);
    expect(validatePlaneClaim(s, ctx())).toEqual([]);
  });
});
