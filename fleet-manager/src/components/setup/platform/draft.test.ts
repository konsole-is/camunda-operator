import { describe, expect, it } from "vitest";
import type { CamundaPlatformConfig } from "@/generated/crds/camundaplatformconfig";
import { LAST_APPLIED_ANNOTATION } from "@/lib/labels";
import {
  deleteChange,
  draftErrors,
  draftOf,
  type PlatformDraft,
  specOf,
  updateChange,
} from "./draft";
import { platformFixtures } from "./fixtures";

const configOf = (name: string): CamundaPlatformConfig =>
  platformFixtures().platformConfigs.find((c) => c.metadata.name === name)!;

const SECRET = { namespace: "camunda-system", name: "oidc", key: "secret" };

// oidcDraft is a valid OIDC draft with the given fields of oidc replaced.
function oidcDraft(oidc: Record<string, unknown>): PlatformDraft {
  return {
    auth: {
      method: "oidc",
      oidc: {
        issuerUrl: "https://login.example.com/realms/camunda",
        clientId: "orchestration",
        clientSecretRef: SECRET,
        ...oidc,
      } as NonNullable<NonNullable<PlatformDraft["auth"]>["oidc"]>,
    },
  };
}

describe("draftOf and specOf", () => {
  it("keeps the spec of a config unchanged through the draft", () => {
    const config = configOf("oidc-prod");

    expect(specOf(draftOf(config))).toEqual(config.spec);
  });

  it("gives an empty draft without a config", () => {
    expect(draftOf(undefined)).toEqual({});
  });

  it("does not share memory with the config", () => {
    const config = configOf("oidc-prod");
    const draft = draftOf(config);
    draft.images!.camunda = "changed";

    expect(config.spec.images?.camunda).toBe(
      "registry.example.com/camunda/camunda",
    );
  });

  it("turns OIDC off with an explicit basic method and no oidc block", () => {
    const draft = draftOf(configOf("oidc-prod"));
    draft.auth = { ...draft.auth, method: "basic" };

    const spec = specOf(draft);

    expect(spec.auth).toEqual({ method: "basic" });
    expect(spec.licenseSecretRef).toEqual(
      configOf("oidc-prod").spec.licenseSecretRef,
    );
  });

  it("sends the basic method for a config without auth", () => {
    expect(specOf(draftOf(configOf("basic-lab")))).toEqual({
      auth: { method: "basic" },
    });
  });

  it("drops empty fields, empty management clients, and empty images", () => {
    const draft = oidcDraft({
      audience: "",
      providerType: "" as never,
      jwksUrl: "",
      management: {
        clients: {
          console: { clientId: "", audience: "" },
          optimize: { clientId: "optimize", audience: "" },
        },
      },
    });
    draft.images = { camunda: "", keycloak: "" };

    expect(specOf(draft)).toEqual({
      auth: {
        method: "oidc",
        oidc: {
          issuerUrl: "https://login.example.com/realms/camunda",
          clientId: "orchestration",
          clientSecretRef: SECRET,
          management: { clients: { optimize: { clientId: "optimize" } } },
        },
      },
    });
  });

  it("drops the management block when no client is left", () => {
    const draft = oidcDraft({
      management: { clients: { console: { clientId: "" } } },
    });

    expect(specOf(draft).auth?.oidc).not.toHaveProperty("management");
  });
});

describe("draftErrors", () => {
  it("gives no error for a valid config", () => {
    expect(draftErrors(draftOf(configOf("oidc-prod")))).toEqual([]);
    expect(draftErrors(draftOf(configOf("basic-lab")))).toEqual([]);
  });

  it("checks no OIDC field in basic mode", () => {
    const draft = oidcDraft({ issuerUrl: "", clientId: "" });
    draft.auth!.method = "basic";

    expect(draftErrors(draft)).toEqual([]);
  });

  it.each([
    ["", "Enter the issuer URL."],
    ["login.example.com", "The issuer URL is an http or https URL."],
  ])("asks for an issuer URL of %j", (issuerUrl, message) => {
    expect(draftErrors(oidcDraft({ issuerUrl }))).toEqual([
      { path: "spec.auth.oidc.issuerUrl", message },
    ]);
  });

  it.each([
    ["jwksUrl", "The JWKS URL is an http or https URL."],
    ["tokenUrl", "The token URL is an http or https URL."],
    ["authUrl", "The authorization URL is an http or https URL."],
  ])("refuses a %s that is not an http or https URL", (field, message) => {
    expect(
      draftErrors(oidcDraft({ [field]: "ftp://keys.example.com" })),
    ).toEqual([{ path: `spec.auth.oidc.${field}`, message }]);
    expect(draftErrors(oidcDraft({ [field]: "" }))).toEqual([]);
  });

  it("asks for the client ID", () => {
    expect(draftErrors(oidcDraft({ clientId: "" }))).toEqual([
      { path: "spec.auth.oidc.clientId", message: "Enter the client ID." },
    ]);
  });

  it("asks for the Secret key of the client secret", () => {
    expect(draftErrors(oidcDraft({ clientSecretRef: undefined }))).toEqual([
      {
        path: "spec.auth.oidc.clientSecretRef",
        message: "Pick the Secret key of the client secret.",
      },
    ]);
  });

  it("asks for the client ID of a management client with a field set", () => {
    const draft = oidcDraft({
      management: { clients: { webModeler: { clientId: "", audience: "wm" } } },
    });

    expect(draftErrors(draft)).toEqual([
      {
        path: "spec.auth.oidc.management.clients.webModeler.clientId",
        message: "Enter the client ID of Web Modeler.",
      },
    ]);
  });

  it("asks for the client secret of a management client with a client ID", () => {
    const draft = oidcDraft({
      management: { clients: { webModelerApi: { clientId: "wm-api" } } },
    });

    expect(draftErrors(draft)).toEqual([
      {
        path: "spec.auth.oidc.management.clients.webModelerApi.clientSecretRef",
        message: "Pick the Secret key of the client secret of Web Modeler API.",
      },
    ]);
  });

  it("asks for no secret of a management client that has none", () => {
    const draft = oidcDraft({
      management: { clients: { console: { clientId: "console" } } },
    });

    expect(draftErrors(draft)).toEqual([]);
  });

  it.each([
    "Registry.example.com/camunda/optimize",
    "mirror.example.com/camunda/optimize:8.9.1",
    "registry:5000",
  ])("refuses the image repository %s", (repository) => {
    expect(draftErrors({ images: { optimize: repository } })).toEqual([
      {
        path: "spec.images.optimize",
        message:
          "A repository is lowercase, with a registry and no tag, as in mirror.example.com/camunda/optimize.",
      },
    ]);
  });

  it.each([
    "mirror.example.com/camunda/optimize",
    "registry:5000/camunda/optimize",
    "camunda/optimize",
  ])("accepts the image repository %s", (repository) => {
    expect(draftErrors({ images: { optimize: repository } })).toEqual([]);
  });
});

describe("updateChange", () => {
  it("keeps the labels and annotations without the last applied file, with the precondition and force", () => {
    const live = configOf("oidc-prod");
    live.metadata.labels = { team: "platform" };
    live.metadata.annotations = {
      note: "kept",
      [LAST_APPLIED_ANNOTATION]: "{}",
    };
    const draft = draftOf(live);
    draft.images = { ...draft.images, keycloak: "mirror.example.com/keycloak" };

    expect(updateChange(live, draft)).toEqual({
      intent: "Change the platform config oidc-prod",
      apply: [
        {
          apiVersion: "core.camunda.io/v1",
          kind: "CamundaPlatformConfig",
          metadata: {
            name: "oidc-prod",
            labels: { team: "platform" },
            annotations: { note: "kept" },
          },
          spec: specOf(draft),
        },
      ],
      preconditions: { "CamundaPlatformConfig//oidc-prod": "21" },
      force: true,
    });
  });
});

describe("deleteChange", () => {
  it("deletes the config with its precondition and its UID", () => {
    expect(deleteChange(configOf("basic-lab"))).toEqual({
      intent: "Delete the platform config basic-lab",
      apply: [],
      delete: [
        {
          apiVersion: "core.camunda.io/v1",
          kind: "CamundaPlatformConfig",
          name: "basic-lab",
        },
      ],
      preconditions: { "CamundaPlatformConfig//basic-lab": "21" },
      uids: { "CamundaPlatformConfig//basic-lab": "uid-basic-lab" },
    });
  });
});
