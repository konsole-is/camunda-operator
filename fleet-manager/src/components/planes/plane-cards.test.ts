import { describe, expect, it } from "vitest";
import type { PlaneField } from "@/lib/fields";
import type { Resource } from "@/lib/k8s/kinds";
import { COMPONENT_LABEL, MANAGEMENT_CLUSTER_LABEL } from "@/lib/labels";
import {
  planeCardFields,
  planeCardHref,
  planeCardOf,
  planeCardSelector,
  planeCards,
} from "./plane-cards";

const condition = (type: string, status = "True", reason = "Healthy") => ({
  type,
  status,
  reason,
  message: "",
  lastTransitionTime: "2026-11-15T12:00:00Z",
});

function planeOf(
  spec: Record<string, unknown>,
  conditions = [
    condition("IdentityReady"),
    condition("ConsoleReady"),
    condition("WebModelerReady"),
    condition("KeycloakReady"),
    condition("SecretsReady"),
  ],
): Resource<"CamundaManagementCluster"> {
  return {
    apiVersion: "core.camunda.io/v1",
    kind: "CamundaManagementCluster",
    metadata: { namespace: "platform", name: "plane-a" },
    spec: { platformConfigRef: "oidc-platform", identity: {}, ...spec },
    status: { conditions },
  } as unknown as Resource<"CamundaManagementCluster">;
}

const oidc = planeOf({
  identityProvider: { oidc: {} },
  console: {},
  webModeler: {},
});
const keycloak = planeOf({ identityProvider: { keycloak: {} } });
const external = (realm?: string) =>
  planeOf({
    identityProvider: {
      externalKeycloak: {
        url: "https://keycloak.example.com/",
        ...(realm && { realm }),
        adminCredentialsSecretRef: { name: "admin" },
      },
    },
  });

const config = {
  metadata: { name: "oidc-platform" },
  spec: {
    auth: {
      method: "oidc",
      oidc: { issuerUrl: "https://login.example.com/realms/camunda" },
    },
  },
} as unknown as Resource<"CamundaPlatformConfig">;

function pod(name: string, component: string, ready = true): Resource<"Pod"> {
  return {
    metadata: {
      name,
      namespace: "platform",
      labels: {
        [COMPONENT_LABEL]: component,
        [MANAGEMENT_CLUSTER_LABEL]: "plane-a",
      },
    },
    status: {
      phase: "Running",
      conditions: [{ type: "Ready", status: ready ? "True" : "False" }],
      containerStatuses: [{ name: "app", ready, restartCount: 0 }],
    },
  } as unknown as Resource<"Pod">;
}

const ids = (cards: { id: string }[]) => cards.map((c) => c.id);
const card = (cards: ReturnType<typeof planeCards>, id: string) =>
  cards.find((c) => c.id === id)!;

describe("planeCards", () => {
  it("an OIDC plane with Console and Web Modeler has the identity provider, Identity, Console, and both Web Modeler cards, and no Keycloak card", () => {
    expect(ids(planeCards({ plane: oidc, pods: [], config }))).toEqual([
      "identityProvider",
      "identity",
      "console",
      "webModelerRestapi",
      "webModelerWebsockets",
    ]);
  });

  it("a Keycloak plane has the Keycloak card and no identity provider card", () => {
    expect(ids(planeCards({ plane: keycloak, pods: [] }))).toEqual([
      "keycloak",
      "identity",
    ]);
  });

  it("the identity provider card shows the mode and the issuer, counts no pods, and has no link", () => {
    const provider = card(
      planeCards({ plane: oidc, pods: [], config }),
      "identityProvider",
    );
    expect(provider).toMatchObject({
      label: "Identity provider",
      used: true,
      pods: [],
      numbers: [
        { label: "Mode", value: "OIDC" },
        { label: "Issuer", value: "https://login.example.com/realms/camunda" },
      ],
    });
    expect(planeCardHref("identityProvider")).toBeUndefined();
  });

  it("an external Keycloak issuer is the realm URL, with camunda-platform when the realm is unset", () => {
    const issuer = (plane: Resource<"CamundaManagementCluster">) =>
      card(planeCards({ plane, pods: [] }), "identityProvider").numbers[1];
    expect(issuer(external())).toEqual({
      label: "Issuer",
      value: "https://keycloak.example.com/realms/camunda-platform",
    });
    expect(issuer(external("team")).value).toBe(
      "https://keycloak.example.com/realms/team",
    );
  });

  it("an OIDC issuer that the platform config does not hold reads Unknown", () => {
    const provider = card(
      planeCards({ plane: oidc, pods: [] }),
      "identityProvider",
    );
    expect(provider.numbers[1]).toEqual({ label: "Issuer", value: "Unknown" });
  });

  it("an OIDC issuer names the platform config kind while the page cannot read it", () => {
    const provider = card(
      planeCards({
        plane: oidc,
        pods: [],
        config,
        unread: ["CamundaPlatformConfig"],
      }),
      "identityProvider",
    );
    expect(provider.numbers[1]).toEqual({
      label: "Issuer",
      value: "Could not read CamundaPlatformConfig",
      severity: "Warning",
    });
  });

  it("IdentityReady False makes the Identity card condition False", () => {
    const plane = planeOf({ identityProvider: { oidc: {} } }, [
      condition("IdentityReady", "False", "Down"),
    ]);
    expect(
      card(planeCards({ plane, pods: [] }), "identity").condition,
    ).toMatchObject({ type: "IdentityReady", status: "False", reason: "Down" });
  });

  it("both Web Modeler cards carry WebModelerReady", () => {
    const cards = planeCards({ plane: oidc, pods: [], config });
    for (const id of ["webModelerRestapi", "webModelerWebsockets"]) {
      expect(card(cards, id).condition?.type).toBe("WebModelerReady");
    }
  });

  it("a card counts the pods of its component value", () => {
    const pods = [
      pod("plane-a-identity-1", "management-identity"),
      pod("plane-a-identity-0", "management-identity", false),
      pod("plane-a-console-0", "console"),
    ];
    const cards = planeCards({ plane: oidc, pods, config, usage: [] });
    expect(card(cards, "identity")).toMatchObject({
      pods: ["plane-a-identity-0", "plane-a-identity-1"],
      numbers: [
        { label: "Pods ready", value: "1/2" },
        { label: "Highest use", value: "Not available" },
      ],
    });
    expect(card(cards, "console").pods).toEqual(["plane-a-console-0"]);
    expect(card(cards, "webModelerRestapi").pods).toEqual([]);
  });

  it("a card states nothing about pods while the page cannot read them", () => {
    const cards = planeCards({
      plane: oidc,
      pods: [pod("plane-a-identity-0", "management-identity")],
      config,
      usage: [],
      unread: ["Pod"],
    });
    expect(card(cards, "identity").numbers).toEqual([
      { label: "Pods ready", value: "Could not read Pod", severity: "Warning" },
      { label: "Highest use", value: "Not known" },
    ]);
  });

  it("marks every card condition as not known while the page cannot read the plane", () => {
    const cards = planeCards({
      plane: oidc,
      pods: [],
      config,
      unread: ["CamundaManagementCluster"],
    });
    for (const c of cards.filter((c) => c.id !== "identityProvider")) {
      expect(c.unknown).toBe(true);
    }
  });
});

describe("planeCardOf", () => {
  const cards = planeCards({ plane: oidc, pods: [], config });

  it("returns the card that the component parameter names", () => {
    expect(planeCardOf("console", cards)).toBe("console");
  });

  it("refuses the identity provider and an id that no card has", () => {
    expect(planeCardOf("identityProvider", cards)).toBeUndefined();
    expect(planeCardOf("keycloak", cards)).toBeUndefined();
    expect(planeCardOf(null, cards)).toBeUndefined();
  });
});

describe("planeCardHref", () => {
  it("opens the Overview tab with the detail of a card, or with the cards for null", () => {
    expect(planeCardHref("webModelerRestapi")).toBe(
      "?tab=overview&component=webModelerRestapi",
    );
    expect(planeCardHref(null)).toBe("?tab=overview");
  });
});

describe("planeCardSelector", () => {
  it("adds the component value of the card to the selector of the plane", () => {
    expect(
      planeCardSelector("camunda.io/management-cluster=plane-a", "identity"),
    ).toBe(
      "camunda.io/management-cluster=plane-a,camunda.io/component=management-identity",
    );
  });
});

describe("planeCardFields", () => {
  const field = (key: string): PlaneField => ({
    key,
    group: "identity",
    label: key,
    value: undefined,
    claimPath: key,
    claimSteps: [],
  });
  const fields = [
    "name",
    "identityProvider",
    "admin.claimName",
    "admin.claimValue",
    "console",
    "webModeler",
    "webModeler.mail",
    "versions.identity",
    "versions.console",
    "versions.webModeler",
    "versions.keycloak",
    "url.keycloak",
    "url.identity",
    "url.console",
    "url.webModeler",
    "url.webModelerWebsockets",
    "deletionPolicy",
  ].map(field);
  const keys = (id: Parameters<typeof planeCardFields>[0]) =>
    planeCardFields(id, fields).map((f) => f.key);

  it("gives the admin fields to Identity and the mail to both Web Modeler cards", () => {
    expect(keys("identity")).toEqual([
      "identityProvider",
      "admin.claimName",
      "admin.claimValue",
      "versions.identity",
      "url.identity",
    ]);
    for (const id of ["webModelerRestapi", "webModelerWebsockets"] as const) {
      expect(keys(id)).toEqual([
        "webModeler",
        "webModeler.mail",
        "versions.webModeler",
        "url.webModeler",
        "url.webModelerWebsockets",
      ]);
    }
  });

  it("gives Keycloak and Console their version and URL, and the identity provider none", () => {
    expect(keys("keycloak")).toEqual(["versions.keycloak", "url.keycloak"]);
    expect(keys("console")).toEqual([
      "console",
      "versions.console",
      "url.console",
    ]);
    expect(keys("identityProvider")).toEqual([]);
  });
});
