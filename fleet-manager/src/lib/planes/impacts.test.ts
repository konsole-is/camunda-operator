import { describe, expect, it } from "vitest";
import type { Step } from "@/lib/fields";
import type { Resource } from "@/lib/k8s/kinds";
import { CLAIM_LABEL, ENVIRONMENT_LABEL } from "@/lib/labels";
import {
  openSession,
  resetValue,
  setValue,
  type PlaneClaim,
  type Session,
} from "@/lib/session";
import { EMPTY_MAIL } from "./components";
import { mailOf } from "./draft";
import { componentEdits, identityMoveEdits, type PlaneEdit } from "./edits";
import {
  CALLBACKS_MOVE,
  CONSOLE_OFF,
  CONSOLE_ON,
  FIRST_ADMIN_STAYS,
  KEYCLOAK_DELETED,
  MOVE_AT_ONCE,
  MOVE_WAITS,
  OIDC_CALLBACKS,
  PREVIEW_NEEDS_ISSUER,
  WEB_MODELER_OFF,
  planeImpacts,
  planeModeMove,
  servedChanges,
  type PlaneChangeInput,
} from "./impacts";
import { KEYCLOAK_REALM_WARNING, KEYCLOAK_RESTART_WARNING } from "./versions";

const at = (...names: string[]): Step[] => names.map((field) => ({ field }));
const asEdit = (e: PlaneEdit): [Step[], unknown] => [
  e.steps,
  "reset" in e ? undefined : e.value,
];

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

const externalClaim = (spec: Partial<PlaneClaim["spec"]> = {}) =>
  planeClaim({
    management: {
      identityProvider: {
        externalKeycloak: {
          url: "https://kc.example.com",
          adminCredentialsSecretRef: { name: "kc" },
        },
      },
      identity: { admin: { username: "admin" } },
    },
    versions: { identity: "8.9.14" },
    ...spec,
  });

function changed(
  claim: PlaneClaim,
  ...edits: [Step[], unknown][]
): Session<PlaneClaim> {
  let s = openSession({
    claim,
    resource: { metadata: { labels: { [CLAIM_LABEL]: "mgmt" } } },
    locks: () => undefined,
  })!;
  for (const [steps, value] of edits) {
    const result =
      value === undefined ? resetValue(s, steps) : setValue(s, steps, value);
    expect("session" in result).toBe(true);
    if ("session" in result) s = result.session;
  }
  return s;
}

const namespace = (name: string, environment: string) =>
  ({
    metadata: { name, labels: { [ENVIRONMENT_LABEL]: environment } },
  }) as Resource<"Namespace">;
const cluster = (namespace: string, name: string) =>
  ({
    metadata: { namespace, name },
    spec: { platformConfigRef: "basic" },
    status: { gateway: { grpc: "orders:26500" } },
  }) as unknown as Resource<"CamundaCluster">;
const platformConfigs = [
  { metadata: { name: "basic" }, spec: { auth: { method: "basic" } } },
  {
    metadata: { name: "sso" },
    spec: {
      auth: { method: "oidc", oidc: { issuerUrl: "https://sso.example.com" } },
    },
  },
] as Resource<"CamundaPlatformConfig">[];

function input(
  session: Session<PlaneClaim>,
  over: Partial<PlaneChangeInput> | Record<string, unknown> = {},
): PlaneChangeInput {
  return {
    session,
    plane: { metadata: { name: "mgmt", namespace: "mgmt-ns" } },
    environment: {
      metadata: { name: "production" },
      spec: { platformConfigRef: "basic" },
    },
    platformConfigs,
    clusters: [cluster("team-orders", "orders"), cluster("team-qa", "qa")],
    namespaces: [
      namespace("team-orders", "production"),
      namespace("team-qa", "staging"),
    ],
    ...over,
  } as PlaneChangeInput;
}

const consoleClaim = (spec: Partial<PlaneClaim["spec"]> = {}) =>
  planeClaim({
    management: {
      identityProvider: {
        keycloak: { externalUrl: "https://mgmt.example.com/auth" },
      },
      identity: { admin: { username: "admin" } },
      console: {},
    },
    versions: { identity: "8.9.14", keycloak: "26.6.4", console: "8.9.133" },
    ...spec,
  });

// keycloakPlane runs the Keycloak of the keycloak mode, as the claim
// controller writes it from consoleClaim.
const keycloakPlane = {
  metadata: { name: "mgmt", namespace: "mgmt-ns" },
  spec: {
    identityProvider: {
      keycloak: { externalUrl: "https://mgmt.example.com/auth" },
    },
  },
  status: {
    endpoints: [
      {
        name: "keycloak",
        service: "mgmt-keycloak-service",
        port: 8080,
        pathPrefix: "/auth",
        protocol: "HTTP",
      },
    ],
  },
};
const kcTrust = {
  metadata: { name: "kc-trust" },
  spec: {
    auth: {
      method: "oidc",
      oidc: {
        issuerUrl: "https://mgmt.example.com/auth/realms/camunda-platform",
      },
    },
  },
} as Resource<"CamundaPlatformConfig">;
const kcServiceTrust = {
  metadata: { name: "kc-service-trust" },
  spec: {
    auth: {
      method: "oidc",
      oidc: {
        issuerUrl:
          "http://mgmt-keycloak-service.mgmt-ns.svc:8080/auth/realms/camunda-platform",
      },
    },
  },
} as Resource<"CamundaPlatformConfig">;
const trusting = (namespace: string, name: string, config: string) =>
  ({
    ...cluster(namespace, name),
    spec: { platformConfigRef: config },
  }) as unknown as Resource<"CamundaCluster">;

describe("planeImpacts", () => {
  it("lists nothing for a change that moves nothing", () => {
    const s = changed(planeClaim(), [at("spec", "deletionPolicy"), "Orphan"]);
    expect(planeImpacts(input(s))).toEqual([]);
  });

  it("lists a cluster that attaches when an environment is added", () => {
    const s = changed(consoleClaim(), [
      at("spec", "environments"),
      ["production", "staging"],
    ]);
    expect(planeImpacts(input(s))).toEqual([
      "team-qa/qa attaches and restarts its pods once, because it gets the Console settings.",
    ]);
  });

  it("lists a cluster that detaches when an environment is removed", () => {
    const live = consoleClaim({ environments: ["production", "staging"] });
    const s = changed(live, [at("spec", "environments"), ["production"]]);
    expect(planeImpacts(input(s))).toEqual([
      "team-qa/qa detaches. It loses its Console settings and restarts once.",
    ]);
  });

  it("lists a cluster that detaches when the selector stops matching", () => {
    const s = changed(consoleClaim(), [
      at("spec", "clusterSelector"),
      { matchLabels: { tier: "gold" } },
    ]);
    expect(planeImpacts(input(s))).toEqual([
      "team-orders/orders detaches. It loses its Console settings and restarts once.",
    ]);
  });

  it("names no restart for a cluster that attaches or detaches while the plane runs no Console", () => {
    const s = changed(planeClaim(), [at("spec", "environments"), ["staging"]]);
    expect(planeImpacts(input(s))).toEqual([
      "team-qa/qa attaches.",
      "team-orders/orders detaches.",
    ]);
  });

  it("says that each served cluster restarts when Console turns on", () => {
    const live = planeClaim();
    const s = changed(
      live,
      ...componentEdits(live.spec, "console", true).map(asEdit),
      [at("spec", "versions", "console"), "8.9.133"],
    );
    expect(planeImpacts(input(s))).toEqual([CONSOLE_ON]);
  });

  it("shows the callback move of a change of the External Keycloak realm", () => {
    const s = changed(externalClaim(), [
      at("spec", "management", "identityProvider", "externalKeycloak", "realm"),
      "other",
    ]);
    const plane = {
      metadata: { name: "mgmt", namespace: "mgmt-ns" },
      status: { optimize: [{ namespace: "team-orders", name: "optimize" }] },
    };
    expect(planeImpacts(input(s, { plane }))).toEqual([
      CALLBACKS_MOVE,
      MOVE_WAITS,
    ]);
  });

  it("moves at once on a change of the External Keycloak URL while the plane serves no Optimize", () => {
    const s = changed(externalClaim(), [
      at("spec", "management", "identityProvider", "externalKeycloak", "url"),
      "https://other.example.com",
    ]);
    expect(planeImpacts(input(s))).toEqual([CALLBACKS_MOVE, MOVE_AT_ONCE]);
  });

  it("names no callback move for a new administrator Secret of the External Keycloak", () => {
    const s = changed(externalClaim(), [
      at(
        "spec",
        "management",
        "identityProvider",
        "externalKeycloak",
        "adminCredentialsSecretRef",
        "name",
      ),
      "kc-2",
    ]);
    expect(planeImpacts(input(s))).toEqual([]);
  });

  it("detaches an OIDC cluster that trusts the Keycloak of the plane on a move to OIDC", () => {
    const live = consoleClaim({ platformConfigRef: "sso" });
    const s = changed(
      live,
      ...identityMoveEdits(live.spec, "oidc").map(asEdit),
    );
    const lines = planeImpacts(
      input(s, {
        plane: keycloakPlane,
        clusters: [trusting("team-orders", "orders", "kc-trust")],
        platformConfigs: [...platformConfigs, kcTrust],
      }),
    );
    expect(lines).toContain(
      "team-orders/orders detaches. It loses its Console settings and restarts once.",
    );
  });

  it("attaches an OIDC cluster that trusts the service address of the Keycloak of the plane", () => {
    const s = changed(consoleClaim(), [
      at("spec", "environments"),
      ["production", "staging"],
    ]);
    const lines = planeImpacts(
      input(s, {
        plane: keycloakPlane,
        clusters: [
          cluster("team-orders", "orders"),
          trusting("team-qa", "qa", "kc-service-trust"),
        ],
        platformConfigs: [...platformConfigs, kcServiceTrust],
      }),
    );
    expect(lines).toEqual([
      "team-qa/qa attaches and restarts its pods once, because it gets the Console settings.",
    ]);
  });

  it("shows the callback move of a move from External Keycloak to OIDC", () => {
    const live = externalClaim({ platformConfigRef: "sso" });
    const s = changed(
      live,
      ...identityMoveEdits(live.spec, "oidc").map(asEdit),
    );
    expect(planeImpacts(input(s))).toEqual([
      "Sign-in moves from External Keycloak to OIDC.",
      FIRST_ADMIN_STAYS,
      CALLBACKS_MOVE,
      MOVE_AT_ONCE,
      OIDC_CALLBACKS,
    ]);
  });

  it("waits while the plane serves an Optimize on a move to another Keycloak", () => {
    const live = externalClaim();
    const s = changed(
      live,
      ...identityMoveEdits(live.spec, "keycloak").map(asEdit),
    );
    const plane = {
      metadata: { name: "mgmt", namespace: "mgmt-ns" },
      status: { optimize: [{ namespace: "team-orders", name: "optimize" }] },
    };
    const lines = planeImpacts(input(s, { plane }));
    expect(lines).toContain(MOVE_WAITS);
    expect(lines).not.toContain(MOVE_AT_ONCE);
  });

  it("moves at once on a move to another Keycloak while the plane serves no Optimize", () => {
    const live = externalClaim();
    const s = changed(
      live,
      ...identityMoveEdits(live.spec, "keycloak").map(asEdit),
    );
    expect(planeImpacts(input(s))).toContain(MOVE_AT_ONCE);
  });

  it("a claim-only plane moves at once", () => {
    const live = externalClaim();
    const s = changed(
      live,
      ...identityMoveEdits(live.spec, "keycloak").map(asEdit),
    );
    const lines = planeImpacts(input(s, { plane: undefined }));
    expect(lines).toContain(MOVE_AT_ONCE);
    expect(lines).not.toContain(MOVE_WAITS);
  });

  it("says that a move away from the keycloak mode deletes the Keycloak", () => {
    const live = planeClaim({ platformConfigRef: "sso" });
    const s = changed(
      live,
      ...identityMoveEdits(live.spec, "oidc").map(asEdit),
    );
    const lines = planeImpacts(input(s));
    expect(lines.slice(0, 4)).toEqual([
      "Sign-in moves from Keycloak to OIDC.",
      FIRST_ADMIN_STAYS,
      KEYCLOAK_DELETED,
      OIDC_CALLBACKS,
    ]);
    expect(lines).not.toContain(CALLBACKS_MOVE);
  });

  it("names the move inside the Keycloak family without the OIDC line", () => {
    const live = planeClaim();
    const external = {
      url: "https://kc.example.com",
      realm: "",
      adminSecret: "kc",
    };
    const s = changed(
      live,
      ...identityMoveEdits(live.spec, "externalKeycloak", external).map(asEdit),
    );
    expect(planeImpacts(input(s))).toEqual([
      "Sign-in moves from Keycloak to External Keycloak.",
      FIRST_ADMIN_STAYS,
      KEYCLOAK_DELETED,
    ]);
  });

  it("planeImpacts names no mode move while the live mode is unknown", () => {
    const live = planeClaim({
      management: {
        identityProvider: {},
        identity: { admin: { username: "admin" } },
      },
    });
    const s = changed(live, [
      at("spec", "management", "identityProvider"),
      { keycloak: {} },
    ]);
    expect(planeModeMove(s)).toBeUndefined();
    expect(
      planeImpacts(input(s)).some((l) => l.startsWith("Sign-in moves")),
    ).toBe(false);
  });

  it("an unknown issuer lists no attach or detach and says the preview needs the issuer", () => {
    const live = planeClaim({ platformConfigRef: "sso-without-issuer" });
    const s = changed(
      live,
      ...identityMoveEdits(live.spec, "oidc").map(asEdit),
    );
    const lines = planeImpacts(
      input(s, {
        platformConfigs: [
          {
            metadata: { name: "sso-without-issuer" },
            spec: { auth: { method: "oidc", oidc: {} } },
          },
        ],
      }),
    );
    expect(lines).toContain(PREVIEW_NEEDS_ISSUER);
    expect(
      lines.some((l) => l.includes("attaches") || l.includes("detaches")),
    ).toBe(false);
  });

  it("says that the preview needs the issuer only when the served clusters can move", () => {
    const live = planeClaim({
      platformConfigRef: "sso-without-issuer",
      management: {
        identityProvider: { oidc: {} },
        identity: { admin: { claimName: "sub", claimValue: "42" } },
      },
      versions: { identity: "8.9.14" },
    });
    const configs = {
      platformConfigs: [
        {
          metadata: { name: "sso-without-issuer" },
          spec: { auth: { method: "oidc", oidc: {} } },
        },
      ],
    };
    const policy = changed(live, [at("spec", "deletionPolicy"), "Orphan"]);
    const served = changed(live, [
      at("spec", "environments"),
      ["production", "staging"],
    ]);
    expect(planeImpacts(input(policy, configs))).toEqual([]);
    expect(planeImpacts(input(served, configs))).toEqual([
      PREVIEW_NEEDS_ISSUER,
    ]);
  });

  it("says what Console off and Web Modeler off do", () => {
    const live = planeClaim({
      management: {
        identityProvider: { keycloak: {} },
        identity: { admin: { username: "admin", email: "a@example.com" } },
        console: {},
        webModeler: { mail: mailOf(EMPTY_MAIL) },
      },
      versions: {
        identity: "8.9.14",
        keycloak: "26.6.4",
        console: "8.9.133",
        webModeler: "8.9.15",
      },
    });
    const s = changed(
      live,
      ...componentEdits(live.spec, "console", false).map(asEdit),
      ...componentEdits(live.spec, "webModeler", false).map(asEdit),
    );
    expect(planeImpacts(input(s))).toEqual([CONSOLE_OFF, WEB_MODELER_OFF]);
  });

  it("lists a version move with its warnings", () => {
    const s = changed(
      planeClaim(),
      [at("spec", "versions", "identity"), "8.9.10"],
      [at("spec", "versions", "keycloak"), "26.6.5"],
    );
    expect(planeImpacts(input(s))).toEqual([
      "Management Identity moves from 8.9.14 to 8.9.10.",
      "Keycloak moves from 26.6.4 to 26.6.5.",
      "Management Identity moves down from 8.9.14 to 8.9.10. Camunda documents no downgrade.",
      KEYCLOAK_RESTART_WARNING,
    ]);
  });

  it("adds the minor warning and the realm warning after a version move", () => {
    const live = planeClaim({
      management: {
        identityProvider: { keycloak: {} },
        identity: { admin: { username: "admin" } },
        console: {},
      },
      versions: { identity: "8.9.14", keycloak: "26.6.4", console: "8.9.133" },
    });
    const s = changed(
      live,
      [at("spec", "versions", "console"), "8.10.1"],
      [at("spec", "versions", "keycloak"), "26.7.0"],
    );
    expect(planeImpacts(input(s))).toEqual([
      "Console moves from 8.9.133 to 8.10.1.",
      "Keycloak moves from 26.6.4 to 26.7.0.",
      KEYCLOAK_RESTART_WARNING,
      "Management Identity and Console run on different minors: 8.9, 8.10. Camunda supports them together on one minor only.",
      KEYCLOAK_REALM_WARNING,
    ]);
  });

  it("names no callback move for another spelling of the same External Keycloak", () => {
    const s = changed(externalClaim(), [
      at("spec", "management", "identityProvider", "externalKeycloak", "url"),
      "https://admin@KC.example.com.:443/",
    ]);
    expect(planeImpacts(input(s))).toEqual([]);
  });

  it("shows the callback move when an escaped slash of the External Keycloak URL becomes a path segment", () => {
    const live = externalClaim({
      management: {
        identityProvider: {
          externalKeycloak: {
            url: "https://kc.example.com/auth%2Fblue",
            adminCredentialsSecretRef: { name: "kc" },
          },
        },
        identity: { admin: { username: "admin" } },
      },
    });
    const s = changed(live, [
      at("spec", "management", "identityProvider", "externalKeycloak", "url"),
      "https://kc.example.com/auth/blue",
    ]);
    expect(planeImpacts(input(s))).toEqual([CALLBACKS_MOVE, MOVE_AT_ONCE]);
  });

  it("names no callback move for an escape of an unreserved character", () => {
    const live = externalClaim({
      management: {
        identityProvider: {
          externalKeycloak: {
            url: "https://kc.example.com/auth",
            adminCredentialsSecretRef: { name: "kc" },
          },
        },
        identity: { admin: { username: "admin" } },
      },
    });
    const s = changed(live, [
      at("spec", "management", "identityProvider", "externalKeycloak", "url"),
      "https://kc.example.com/%61uth",
    ]);
    expect(planeImpacts(input(s))).toEqual([]);
  });

  it("shows the callback move when a dot segment enters the External Keycloak URL", () => {
    const live = externalClaim({
      management: {
        identityProvider: {
          externalKeycloak: {
            url: "https://kc.example.com/x",
            adminCredentialsSecretRef: { name: "kc" },
          },
        },
        identity: { admin: { username: "admin" } },
      },
    });
    for (const url of [
      "https://kc.example.com/auth/../x",
      "https://kc.example.com/auth/%2e%2e/x",
    ]) {
      const s = changed(live, [
        at("spec", "management", "identityProvider", "externalKeycloak", "url"),
        url,
      ]);
      expect(planeImpacts(input(s))).toEqual([CALLBACKS_MOVE, MOVE_AT_ONCE]);
    }
  });

  it("names no callback move when a dot segment of the External Keycloak URL is escaped", () => {
    const live = externalClaim({
      management: {
        identityProvider: {
          externalKeycloak: {
            url: "https://kc.example.com/auth/../x",
            adminCredentialsSecretRef: { name: "kc" },
          },
        },
        identity: { admin: { username: "admin" } },
      },
    });
    const s = changed(live, [
      at("spec", "management", "identityProvider", "externalKeycloak", "url"),
      "https://kc.example.com/auth/%2E%2e/x",
    ]);
    expect(planeImpacts(input(s))).toEqual([]);
  });

  it("adds no minor warning for a move into the keycloak mode", () => {
    const live = externalClaim({
      management: {
        identityProvider: {
          externalKeycloak: {
            url: "https://kc.example.com",
            adminCredentialsSecretRef: { name: "kc" },
          },
        },
        identity: { admin: { username: "admin" } },
        console: {},
      },
      versions: { identity: "8.10.1", console: "8.9.133" },
    });
    const s = changed(
      live,
      ...identityMoveEdits(live.spec, "keycloak").map(asEdit),
    );
    expect(planeImpacts(input(s))).not.toContain(
      "Management Identity and Console run on different minors: 8.10, 8.9. Camunda supports them together on one minor only.",
    );
  });

  it("warns on two minors when the change turns on an app with a version from before", () => {
    const live = planeClaim({
      versions: { identity: "8.10.1", keycloak: "26.6.4", console: "8.9.133" },
    });
    const s = changed(
      live,
      ...componentEdits(live.spec, "console", true).map(asEdit),
    );
    expect(planeImpacts(input(s))).toContain(
      "Management Identity and Console run on different minors: 8.10, 8.9. Camunda supports them together on one minor only.",
    );
  });
});

describe("servedChanges", () => {
  it("gives the rows after the change beside the attach and detach rows", () => {
    const s = changed(planeClaim(), [at("spec", "environments"), ["staging"]]);
    const result = servedChanges(input(s));
    expect(result?.attach.map((r) => r.name)).toEqual(["qa"]);
    expect(result?.detach.map((r) => r.name)).toEqual(["orders"]);
    expect(result?.after.map((r) => r.name)).toEqual(["qa"]);
  });

  it("is undefined while the issuer after the change is unknown", () => {
    const live = planeClaim({ platformConfigRef: "missing" });
    const s = changed(
      live,
      ...identityMoveEdits(live.spec, "oidc").map(asEdit),
    );
    expect(servedChanges(input(s))).toBeUndefined();
  });
});
