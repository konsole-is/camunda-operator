import { describe, expect, it } from "vitest";
import type { CamundaPlatformConfig } from "@/generated/crds/camundaplatformconfig";
import { LAST_APPLIED_ANNOTATION } from "@/lib/labels";
import { foreignFields } from "@/lib/write/ownership";
import type { KubeObject } from "@/lib/write/types";
import {
  changeCount,
  createChange,
  deleteChange,
  draftErrors,
  draftOf,
  nameErrors,
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

  it("keeps a spec field that the form does not edit", () => {
    const draft = {
      ...draftOf(configOf("basic-lab")),
      tracing: { endpoint: "otel:4317" },
    } as PlatformDraft;

    expect(specOf(draft)).toEqual({
      auth: { method: "basic" },
      tracing: { endpoint: "otel:4317" },
    });
  });

  it("leaves out the license and the images when the form clears them", () => {
    const draft = draftOf(configOf("oidc-prod"));
    expect(draft).toHaveProperty("licenseSecretRef");
    draft.licenseSecretRef = undefined;
    draft.images = { camunda: "" };

    const spec = specOf(draft);

    expect(Object.keys(spec)).not.toContain("licenseSecretRef");
    expect(spec).not.toHaveProperty("images");
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
  it("keeps the labels and annotations without the last applied file, with the precondition and no force", () => {
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
    });
  });
});

describe("updateChange over the empty values of another tool", () => {
  // live holds an OIDC block that kubectl set, with an empty audience, an
  // empty map of management clients, and an empty map of images.
  const live: CamundaPlatformConfig = {
    ...configOf("basic-lab"),
    spec: {
      auth: {
        method: "oidc",
        oidc: {
          issuerUrl: "https://login.example.com/realms/camunda",
          clientId: "orchestration",
          clientSecretRef: SECRET,
          audience: "",
          management: { clients: {} },
        },
      },
      images: {},
    },
  };
  live.metadata = {
    ...live.metadata,
    managedFields: [
      {
        manager: "kubectl-client-side-apply",
        operation: "Update",
        fieldsV1: {
          "f:spec": {
            "f:auth": {
              "f:method": {},
              "f:oidc": {
                "f:issuerUrl": {},
                "f:clientId": {},
                "f:clientSecretRef": {
                  "f:namespace": {},
                  "f:name": {},
                  "f:key": {},
                },
                "f:audience": {},
                "f:management": { "f:clients": {} },
              },
            },
            "f:images": {},
          },
        },
      },
    ],
  };
  const rows = (draft: PlatformDraft) => {
    const change = updateChange(live, draft);
    const { changed, cleared } = foreignFields(
      live as unknown as KubeObject,
      change.apply[0],
      change.keep?.["CamundaPlatformConfig//basic-lab"],
    );
    return {
      changed: changed.map((f) => f.path),
      cleared: cleared.map((f) => f.path),
    };
  };

  it("shows only the changed field", () => {
    const draft = draftOf(live);
    draft.auth!.oidc!.issuerUrl = "https://login.example.com/realms/other";
    expect(rows(draft)).toEqual({
      changed: ["spec.auth.oidc.issuerUrl"],
      cleared: [],
    });
  });

  it("shows no row for an empty map that the edit typed into and emptied again", () => {
    const draft = draftOf(live);
    draft.images = { keycloak: "" };
    draft.auth!.oidc!.management = { clients: { console: { clientId: "" } } };
    draft.auth!.oidc!.issuerUrl = "https://login.example.com/realms/other";
    expect(rows(draft)).toEqual({
      changed: ["spec.auth.oidc.issuerUrl"],
      cleared: [],
    });
  });

  it("still shows the OIDC block on a switch to basic", () => {
    const draft = draftOf(live);
    draft.auth!.method = "basic";
    expect(rows(draft)).toEqual({
      changed: ["spec.auth.method"],
      cleared: ["spec.auth.oidc"],
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

const NAME_RULE =
  "The name has only lowercase letters, digits, and hyphens. It starts and ends with a letter or a digit, and has at most 253 characters.";

describe("nameErrors", () => {
  it.each([
    ["", "Give the platform config a name."],
    ["team.a", NAME_RULE],
    ["Basic", NAME_RULE],
    ["-basic", NAME_RULE],
    ["basic-", NAME_RULE],
    ["a".repeat(254), NAME_RULE],
    ["oidc-prod", "A platform config with this name exists."],
  ])("refuses the name %j", (name, message) => {
    expect(nameErrors(name, ["oidc-prod"])).toEqual([
      { path: "name", message },
    ]);
  });

  it("accepts a name that a cluster can name", () => {
    expect(nameErrors("basic-two", ["oidc-prod"])).toEqual([]);
    expect(nameErrors("a", [])).toEqual([]);
    expect(nameErrors("a".repeat(253), [])).toEqual([]);
  });
});

describe("createChange", () => {
  const license = {
    namespace: "camunda-system",
    name: "camunda-license",
    key: "license-key",
  };

  it("creates with the name and the spec only, without a precondition", () => {
    expect(createChange("basic-two", { licenseSecretRef: license })).toEqual({
      intent: "Create the platform config basic-two",
      apply: [
        {
          apiVersion: "core.camunda.io/v1",
          kind: "CamundaPlatformConfig",
          metadata: { name: "basic-two" },
          spec: { auth: { method: "basic" }, licenseSecretRef: license },
        },
      ],
      create: ["CamundaPlatformConfig//basic-two"],
    });
  });

  it("writes the spec as a save writes it", () => {
    const draft = {
      auth: {
        method: "oidc",
        oidc: {
          issuerUrl: "https://login.example.com",
          clientId: "orchestration",
          audience: "",
        },
      },
      images: { camunda: "" },
    } as PlatformDraft;

    expect(createChange("oidc-two", draft).apply[0].spec).toEqual({
      auth: {
        method: "oidc",
        oidc: {
          issuerUrl: "https://login.example.com",
          clientId: "orchestration",
        },
      },
    });
  });
});

describe("changeCount", () => {
  it("counts each top-level field of the spec that differs", () => {
    expect(changeCount(specOf({}), specOf({}))).toBe(0);
    expect(
      changeCount(
        specOf({}),
        specOf({ images: { camunda: "mirror.example.com/camunda/camunda" } }),
      ),
    ).toBe(1);
    expect(
      changeCount(
        specOf({}),
        specOf({
          licenseSecretRef: { namespace: "a", name: "b", key: "c" },
          images: { camunda: "mirror.example.com/camunda/camunda" },
        }),
      ),
    ).toBe(2);
  });
});
