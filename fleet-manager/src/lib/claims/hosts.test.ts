import { describe, expect, it } from "vitest";
import type { Environment } from "@/generated/crds/environment";
import type {
  PluginRegistration,
  PluginRegistrationSpec,
} from "@/generated/crds/pluginregistration";
import type { PluginRequest } from "@/generated/crds/pluginrequest";
import type { PluginSection } from "@/generated/crds/pluginsection";
import {
  defaultAccessPatterns,
  hostConflicts,
  renderHosts,
  takenHosts,
} from "./hosts";

const PATTERNS = {
  hostnamePattern: "{name}.{namespace}.example.com",
  grpcHostnamePattern: "grpc-{name}.{namespace}.example.com",
};

function section(
  appliesTo: PluginSection["spec"]["appliesTo"] = [
    "CamundaClusterClaim",
    "CamundaManagementClusterClaim",
  ],
): PluginSection {
  return {
    apiVersion: "plugins.camunda.io/v1alpha1",
    kind: "PluginSection",
    metadata: { name: "access" },
    spec: { appliesTo, label: "Access", selection: "One" },
  };
}

function registration(
  name: string,
  spec: Partial<PluginRegistrationSpec> = {},
): PluginRegistration {
  return {
    apiVersion: "plugins.camunda.io/v1alpha1",
    kind: "PluginRegistration",
    metadata: { name },
    spec: {
      sections: ["access"],
      mode: "Optional",
      label: name,
      deletionPolicies: ["Delete"],
      serviceAccount: { name, namespace: "plugins" },
      parameterValues: PATTERNS,
      ...spec,
    },
    status: {
      conditions: ["Accepted", "Ready"].map((type) => ({
        type,
        status: "True" as const,
        reason: "Test",
        message: "",
        lastTransitionTime: "2026-10-30T00:00:00Z",
      })),
    },
  };
}

function environment(
  access: { allowed?: string[]; default?: string },
): Environment {
  return {
    apiVersion: "fleet.camunda.io/v1alpha1",
    kind: "Environment",
    metadata: { name: "prod" },
    spec: {
      sections: { access },
      parameters: {
        "gateway-b": { hostnamePattern: { value: "{name}.prod.example.com" } },
      },
    },
  };
}

function request(
  namespace: string,
  claim: string,
  over: { outputs?: Record<string, string>; parameters?: unknown } = {},
): PluginRequest {
  return {
    apiVersion: "plugins.camunda.io/v1alpha1",
    kind: "PluginRequest",
    metadata: { namespace, name: `${claim}-access` },
    spec: {
      claimRef: { kind: "CamundaClusterClaim", name: claim },
      registration: "gateway-a",
      section: "access",
      ...(over.parameters !== undefined && { parameters: over.parameters }),
    },
    ...(over.outputs && { status: { outputs: over.outputs } }),
  } as PluginRequest;
}

const gatewayA = registration("gateway-a");
const gatewayB = registration("gateway-b");
const prod = environment({
  allowed: ["gateway-a", "gateway-b"],
  default: "gateway-b",
});

describe("renderHosts", () => {
  it("renders each pattern with the name and the namespace", () => {
    expect(
      renderHosts(
        {
          hostnamePattern: "{name}.{namespace}.Example.com",
          grpcHostnamePattern: "grpc-{name}.x",
        },
        "orders",
        "team-a",
      ),
    ).toEqual([
      { parameter: "hostnamePattern", host: "orders.team-a.example.com" },
      { parameter: "grpcHostnamePattern", host: "grpc-orders.x" },
    ]);
  });

  it("replaces every placeholder of a pattern", () => {
    expect(
      renderHosts({ hostnamePattern: "{name}-{name}.{namespace}" }, "a", "b"),
    ).toEqual([{ parameter: "hostnamePattern", host: "a-a.b" }]);
  });

  it("skips a pattern that is unset, empty, or not a string, and other parameters", () => {
    expect(
      renderHosts(
        { hostnamePattern: "", grpcHostnamePattern: 3, other: "{name}.x" },
        "orders",
        "team-a",
      ),
    ).toEqual([]);
  });

  it("renders no host while the name or the namespace is empty", () => {
    expect(renderHosts({ hostnamePattern: "{name}.x" }, "orders", "")).toEqual(
      [],
    );
    expect(renderHosts({ hostnamePattern: "{name}.x" }, "", "team-a")).toEqual(
      [],
    );
  });
});

describe("defaultAccessPatterns", () => {
  it("takes the patterns of the default access option of the environment", () => {
    expect(
      defaultAccessPatterns(
        [section()],
        [gatewayA, gatewayB],
        prod,
        "CamundaClusterClaim",
      ),
    ).toEqual({
      hostnamePattern: "{name}.prod.example.com",
      grpcHostnamePattern: "grpc-{name}.{namespace}.example.com",
    });
  });

  it("takes the registration with mode Default without an Environment", () => {
    const fallback = registration("gateway-a", {
      mode: "Default",
      parameterValues: { hostnamePattern: "{name}.fallback.example.com" },
    });
    expect(
      defaultAccessPatterns(
        [section()],
        [fallback, gatewayB],
        undefined,
        "CamundaClusterClaim",
      ),
    ).toEqual({ hostnamePattern: "{name}.fallback.example.com" });
  });

  it("gives no patterns when the environment has no default access option", () => {
    expect(
      defaultAccessPatterns(
        [section()],
        [gatewayA, gatewayB],
        environment({ allowed: ["gateway-a", "gateway-b"] }),
        "CamundaClusterClaim",
      ),
    ).toBeUndefined();
  });

  it("gives no patterns when no access section exists", () => {
    expect(
      defaultAccessPatterns([], [gatewayA], prod, "CamundaClusterClaim"),
    ).toBeUndefined();
  });

  it("gives no patterns when the access section does not apply to the claim kind", () => {
    expect(
      defaultAccessPatterns(
        [section(["CamundaManagementClusterClaim"])],
        [gatewayA, gatewayB],
        prod,
        "CamundaClusterClaim",
      ),
    ).toBeUndefined();
  });

  it("gives no patterns when the default registration has malformed values", () => {
    const broken = registration("gateway-b", { parameterValues: "nope" });
    expect(
      defaultAccessPatterns(
        [section()],
        [gatewayA, broken],
        prod,
        "CamundaClusterClaim",
      ),
    ).toBeUndefined();
  });
});

describe("takenHosts", () => {
  it("takes the hosts of the outputs of each access request", () => {
    const billing = request("team-b", "billing", {
      outputs: {
        externalUrl: "https://billing.team-b.example.com",
        grpcHost: "grpc-billing.team-b.example.com",
        optimizeUrl: "https://opt-billing.example.com",
      },
    });
    expect(takenHosts([billing])).toEqual(
      new Map([
        ["billing.team-b.example.com", "team-b/billing"],
        ["grpc-billing.team-b.example.com", "team-b/billing"],
        ["opt-billing.example.com", "team-b/billing"],
      ]),
    );
  });

  it("skips an output that is neither a web URL nor a host", () => {
    const billing = request("team-b", "billing", {
      outputs: { secret: "billing-tls/ca.crt", note: "two words" },
    });
    expect(takenHosts([billing])).toEqual(new Map());
  });

  it("takes the hosts of a request that has no outputs yet", () => {
    const pending = request("team-c", "search", {
      parameters: { hostnamePattern: "{name}.{namespace}.example.com" },
    });
    expect([...takenHosts([pending]).keys()]).toEqual([
      "search.team-c.example.com",
    ]);
  });

  it("keeps the first claim of a host that two requests give", () => {
    const a = request("team-a", "one", { outputs: { h: "x.example.com" } });
    const b = request("team-b", "two", { outputs: { h: "X.example.com" } });
    expect(takenHosts([a, b])).toEqual(
      new Map([["x.example.com", "team-a/one"]]),
    );
  });
});

describe("hostConflicts", () => {
  it("blocks each rendered host that another claim holds", () => {
    expect(
      hostConflicts(
        [
          { parameter: "hostnamePattern", host: "billing.team-b.example.com" },
          { parameter: "grpcHostnamePattern", host: "grpc-new.example.com" },
        ],
        new Map([["billing.team-b.example.com", "team-b/billing"]]),
      ),
    ).toEqual([
      "The host billing.team-b.example.com is taken by team-b/billing. Choose another name.",
    ]);
  });

  it("names a host once when two patterns render it", () => {
    expect(
      hostConflicts(
        [
          { parameter: "hostnamePattern", host: "a.example.com" },
          { parameter: "grpcHostnamePattern", host: "a.example.com" },
        ],
        new Map([["a.example.com", "team-b/billing"]]),
      ),
    ).toHaveLength(1);
  });
});
