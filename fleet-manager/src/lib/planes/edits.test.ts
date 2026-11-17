import { describe, expect, it } from "vitest";
import type { Step } from "@/lib/fields";
import { EMPTY_MAIL } from "./components";
import { mailOf } from "./draft";
import {
  componentEdits,
  identityMoveEdits,
  type PlaneClaimSpec,
} from "./edits";

const at = (...names: string[]): Step[] => names.map((field) => ({ field }));

const keycloakSpec = {
  environments: ["production"],
  management: {
    identityProvider: { keycloak: {} },
    identity: {
      admin: {
        username: "admin",
        email: "admin@example.com",
        passwordSecretRef: { name: "pw", key: "password" },
      },
    },
  },
  versions: { identity: "8.9.14", keycloak: "26.6.4" },
} as PlaneClaimSpec;

const oidcSpec = {
  environments: ["production"],
  management: {
    identityProvider: { oidc: {} },
    identity: { admin: { claimName: "sub", claimValue: "42" } },
  },
  versions: { identity: "8.9.14" },
} as PlaneClaimSpec;

describe("identityMoveEdits", () => {
  it("sets the new provider and removes the Keycloak admin fields on a move to OIDC", () => {
    expect(identityMoveEdits(keycloakSpec, "oidc")).toEqual([
      {
        steps: at("spec", "management", "identityProvider"),
        value: { oidc: {} },
      },
      {
        steps: at("spec", "management", "identity", "admin", "username"),
        reset: true,
      },
      {
        steps: at(
          "spec",
          "management",
          "identity",
          "admin",
          "passwordSecretRef",
        ),
        reset: true,
      },
      { steps: at("spec", "versions", "keycloak"), reset: true },
    ]);
  });

  it("removes the claim pair on a move from OIDC to a Keycloak mode", () => {
    expect(identityMoveEdits(oidcSpec, "keycloak")).toEqual([
      {
        steps: at("spec", "management", "identityProvider"),
        value: { keycloak: {} },
      },
      {
        steps: at("spec", "management", "identity", "admin", "claimName"),
        reset: true,
      },
      {
        steps: at("spec", "management", "identity", "admin", "claimValue"),
        reset: true,
      },
    ]);
  });

  it("keeps the admin inside the Keycloak family and writes the external Keycloak", () => {
    const external = {
      url: "https://kc.example.com",
      realm: "",
      adminSecret: "kc",
    };
    expect(
      identityMoveEdits(keycloakSpec, "externalKeycloak", external),
    ).toEqual([
      {
        steps: at("spec", "management", "identityProvider"),
        value: {
          externalKeycloak: {
            url: "https://kc.example.com",
            adminCredentialsSecretRef: { name: "kc" },
          },
        },
      },
      { steps: at("spec", "versions", "keycloak"), reset: true },
    ]);
  });

  it("writes an empty External Keycloak when no fields are given", () => {
    expect(identityMoveEdits(oidcSpec, "externalKeycloak")[0]).toEqual({
      steps: at("spec", "management", "identityProvider"),
      value: {
        externalKeycloak: { url: "", adminCredentialsSecretRef: { name: "" } },
      },
    });
  });

  it("resets only the admin fields that the claim holds", () => {
    const noPassword = {
      ...keycloakSpec,
      management: {
        ...keycloakSpec.management,
        identity: { admin: { username: "admin" } },
      },
      versions: { identity: "8.9.14" },
    } as PlaneClaimSpec;
    expect(identityMoveEdits(noPassword, "oidc")).toEqual([
      {
        steps: at("spec", "management", "identityProvider"),
        value: { oidc: {} },
      },
      {
        steps: at("spec", "management", "identity", "admin", "username"),
        reset: true,
      },
    ]);
  });

  it("gives no edit for a move to the live mode", () => {
    expect(identityMoveEdits(keycloakSpec, "keycloak")).toEqual([]);
    expect(identityMoveEdits(oidcSpec, "oidc")).toEqual([]);
  });
});

describe("componentEdits", () => {
  it("turns on an app whose block is null, as the panel shows it off", () => {
    const spec = {
      ...keycloakSpec,
      management: { ...keycloakSpec.management, console: null },
    } as unknown as PlaneClaimSpec;
    expect(componentEdits(spec, "console", true)).toEqual([
      { steps: at("spec", "management", "console"), value: {} },
    ]);
  });

  it("removes Console and its version when Console goes off", () => {
    const spec = {
      ...keycloakSpec,
      management: { ...keycloakSpec.management, console: {} },
      versions: { ...keycloakSpec.versions, console: "8.9.133" },
    } as PlaneClaimSpec;
    expect(componentEdits(spec, "console", false)).toEqual([
      { steps: at("spec", "management", "console"), reset: true },
      { steps: at("spec", "versions", "console"), reset: true },
    ]);
  });

  it("removes Web Modeler without a version that the claim does not hold", () => {
    const spec = {
      ...keycloakSpec,
      management: {
        ...keycloakSpec.management,
        webModeler: { mail: mailOf(EMPTY_MAIL) },
      },
    } as PlaneClaimSpec;
    expect(componentEdits(spec, "webModeler", false)).toEqual([
      { steps: at("spec", "management", "webModeler"), reset: true },
    ]);
  });

  it("turns Console on with an empty object", () => {
    expect(componentEdits(keycloakSpec, "console", true)).toEqual([
      { steps: at("spec", "management", "console"), value: {} },
    ]);
  });

  it("turns Web Modeler on with the default mail", () => {
    expect(componentEdits(keycloakSpec, "webModeler", true)).toEqual([
      {
        steps: at("spec", "management", "webModeler"),
        value: { mail: mailOf(EMPTY_MAIL) },
      },
    ]);
  });

  it("gives no edit when the app is already in that state", () => {
    expect(componentEdits(keycloakSpec, "console", false)).toEqual([]);
    const on = {
      ...keycloakSpec,
      management: { ...keycloakSpec.management, console: {} },
    } as PlaneClaimSpec;
    expect(componentEdits(on, "console", true)).toEqual([]);
  });
});
