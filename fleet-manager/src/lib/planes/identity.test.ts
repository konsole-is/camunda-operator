import { describe, expect, it } from "vitest";
import type { CamundaPlatformConfig } from "@/generated/crds/camundaplatformconfig";
import {
  IDENTITY_MODE_LABEL,
  authMethod,
  externalKeycloakDraftOf,
  externalKeycloakErrors,
  externalKeycloakSpecOf,
  identityModeOf,
  oidcDisabledReason,
  planeIssuer,
  sameFamily,
} from "./identity";

const config = (auth?: unknown): CamundaPlatformConfig =>
  ({
    metadata: { name: "pc" },
    spec: auth === undefined ? {} : { auth },
  }) as CamundaPlatformConfig;

describe("authMethod", () => {
  it.each([
    ["no auth block", config(), "basic"],
    ["no method", config({}), "basic"],
    ["basic", config({ method: "basic" }), "basic"],
    [
      "oidc",
      config({ method: "oidc", oidc: { issuerUrl: "https://idp" } }),
      "oidc",
    ],
  ])("%s is %s", (_, c, want) => {
    expect(authMethod(c)).toBe(want);
  });

  it("is basic without a platform config", () => {
    expect(authMethod(undefined)).toBe("basic");
  });
});

describe("oidcDisabledReason", () => {
  it("names the basic-auth platform config", () => {
    expect(oidcDisabledReason(config())).toBe(
      "OIDC needs a platform config with OIDC authentication. pc uses basic authentication.",
    );
  });

  it("is undefined for an oidc platform config", () => {
    expect(
      oidcDisabledReason(
        config({ method: "oidc", oidc: { issuerUrl: "https://idp" } }),
      ),
    ).toBeUndefined();
  });

  it("asks for a platform config when none is picked", () => {
    expect(oidcDisabledReason(undefined)).toBe(
      "OIDC needs a platform config with OIDC authentication.",
    );
  });
});

describe("externalKeycloakErrors", () => {
  const ok = {
    url: "https://kc.example.com/auth",
    realm: "",
    adminSecret: "kc-admin",
  };

  it.each([
    "http://kc.internal:8080",
    "https://kc.example.com",
    "https://kc.example.com/any/path/",
    "https://kc.example.com/a%20b",
  ])("accepts %s", (url) => {
    expect(externalKeycloakErrors({ ...ok, url })).toEqual([]);
  });

  it.each([
    ["ftp://kc", "url"],
    ["kc.example.com", "url"],
    ["", "url"],
    ["https://kc.example.com/auth?x=1", "url"],
    ["https://kc.example.com/auth#x", "url"],
    ["https://user:pw@kc.example.com", "url"],
    ["https://kc.example.com/" + "a".repeat(2048), "url"],
    ["https:kc.example.com", "url"],
    ["https:/kc.example.com", "url"],
    ["https://kc.example.com/a%zzb", "url"],
    ["https://kc.example.com/a%2", "url"],
  ])("refuses %j", (url, path) => {
    expect(externalKeycloakErrors({ ...ok, url }).map((e) => e.path)).toContain(
      path,
    );
  });

  it("needs https for a CA bundle", () => {
    expect(
      externalKeycloakErrors({
        ...ok,
        url: "http://kc.internal",
        caBundle: { name: "ca", key: "ca.crt" },
      }),
    ).toEqual([
      { path: "caBundle", message: "A CA bundle needs an https URL." },
    ]);
  });

  it("accepts a CA bundle with an https URL", () => {
    expect(
      externalKeycloakErrors({
        ...ok,
        caBundle: { name: "ca", key: "ca.crt" },
      }),
    ).toEqual([]);
  });

  it.each(["-x", "x-", "a b", "r".repeat(256)])(
    "refuses the realm %j",
    (realm) => {
      expect(
        externalKeycloakErrors({ ...ok, realm }).map((e) => e.path),
      ).toEqual(["realm"]);
    },
  );

  it.each(["camunda-platform", "a", "a.b_c-d"])(
    "accepts the realm %j",
    (realm) => {
      expect(externalKeycloakErrors({ ...ok, realm })).toEqual([]);
    },
  );

  it("needs the admin credentials Secret", () => {
    expect(
      externalKeycloakErrors({ ...ok, adminSecret: "" }).map((e) => e.path),
    ).toEqual(["adminSecret"]);
  });
});

describe("planeIssuer", () => {
  const ext = {
    url: "https://kc.example.com/auth/",
    realm: "",
    adminSecret: "a",
  };

  it("is the realm URL of an external Keycloak, default realm camunda-platform", () => {
    expect(planeIssuer("externalKeycloak", ext, undefined)).toEqual({
      kind: "known",
      url: "https://kc.example.com/auth/realms/camunda-platform",
    });
  });

  it("uses the realm of an external Keycloak", () => {
    expect(
      planeIssuer("externalKeycloak", { ...ext, realm: "ops" }, undefined),
    ).toEqual({ kind: "known", url: "https://kc.example.com/auth/realms/ops" });
  });

  it("is the oidc issuer of the platform config in OIDC mode", () => {
    expect(
      planeIssuer(
        "oidc",
        ext,
        config({ method: "oidc", oidc: { issuerUrl: "https://idp/x" } }),
      ),
    ).toEqual({ kind: "known", url: "https://idp/x" });
  });

  it("is unknown in OIDC mode with a basic-auth platform config", () => {
    expect(planeIssuer("oidc", ext, config())).toEqual({ kind: "unknown" });
  });

  it("is new in Keycloak mode", () => {
    expect(planeIssuer("keycloak", ext, undefined)).toEqual({ kind: "new" });
  });

  it("is unknown without a mode", () => {
    expect(planeIssuer(undefined, ext, undefined)).toEqual({ kind: "unknown" });
  });
});

describe("identityModeOf", () => {
  it("returns the one mode that the provider sets", () => {
    expect(identityModeOf({ oidc: {} })).toBe("oidc");
    expect(identityModeOf({ keycloak: {} })).toBe("keycloak");
    expect(
      identityModeOf({
        externalKeycloak: {
          url: "https://kc.example.com",
          adminCredentialsSecretRef: { name: "kc" },
        },
      }),
    ).toBe("externalKeycloak");
  });

  it("returns undefined for an empty provider and for two providers", () => {
    expect(identityModeOf({})).toBeUndefined();
    expect(identityModeOf(undefined)).toBeUndefined();
    expect(identityModeOf({ keycloak: {}, oidc: {} })).toBeUndefined();
  });
});

describe("IDENTITY_MODE_LABEL", () => {
  it("names each mode", () => {
    expect(IDENTITY_MODE_LABEL).toEqual({
      keycloak: "Keycloak",
      externalKeycloak: "External Keycloak",
      oidc: "OIDC",
    });
  });
});

describe("sameFamily", () => {
  it("puts the two Keycloak modes together and OIDC alone", () => {
    expect(sameFamily("keycloak", "externalKeycloak")).toBe(true);
    expect(sameFamily("externalKeycloak", "keycloak")).toBe(true);
    expect(sameFamily("oidc", "oidc")).toBe(true);
    expect(sameFamily("oidc", "keycloak")).toBe(false);
    expect(sameFamily("externalKeycloak", "oidc")).toBe(false);
  });
});

describe("externalKeycloakDraftOf and externalKeycloakSpecOf", () => {
  it("read back what they write", () => {
    const draft = {
      url: "https://kc.example.com/auth",
      realm: "teams",
      adminSecret: "kc-admin",
      caBundle: { name: "kc-ca", key: "ca.crt" },
    };
    expect(externalKeycloakDraftOf(externalKeycloakSpecOf(draft))).toEqual(
      draft,
    );
  });

  it("leaves an empty realm out of the spec", () => {
    expect(
      externalKeycloakSpecOf({
        url: "https://kc.example.com",
        realm: "",
        adminSecret: "kc",
      }),
    ).toEqual({
      url: "https://kc.example.com",
      adminCredentialsSecretRef: { name: "kc" },
    });
  });

  it("reads a missing spec and a missing realm as empty fields", () => {
    expect(externalKeycloakDraftOf(undefined)).toEqual({
      url: "",
      realm: "",
      adminSecret: "",
    });
    expect(
      externalKeycloakDraftOf({
        url: "https://kc.example.com",
        adminCredentialsSecretRef: { name: "kc" },
      }),
    ).toEqual({ url: "https://kc.example.com", realm: "", adminSecret: "kc" });
  });
});
