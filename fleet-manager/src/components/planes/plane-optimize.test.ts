import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";
import type { Resource } from "@/lib/k8s/kinds";
import {
  contractReaders,
  isOidc,
  OPTIMIZE_CALLBACK_PATH,
  optimizeRows,
} from "./plane-optimize";

// goConstant reads the value of a Go string constant from a file of the
// repository.
function goConstant(file: string, name: string): string | undefined {
  const source = readFileSync(
    new URL(`../../../../${file}`, import.meta.url),
    "utf8",
  );
  return new RegExp(`\\b${name}\\s*=\\s*"([^"]*)"`).exec(source)?.[1];
}

function planeOf(
  identityProvider: Record<string, unknown>,
  status: Record<string, unknown>,
): Resource<"CamundaManagementCluster"> {
  return {
    apiVersion: "core.camunda.io/v1",
    kind: "CamundaManagementCluster",
    metadata: { namespace: "platform", name: "production" },
    spec: { identityProvider },
    status,
  } as unknown as Resource<"CamundaManagementCluster">;
}

function optimizeOf(
  namespace: string,
  name: string,
  managementAuthRef: string,
  externalUrl?: unknown,
): Resource<"CamundaOptimize"> {
  return {
    apiVersion: "core.camunda.io/v1",
    kind: "CamundaOptimize",
    metadata: { namespace, name },
    spec: {
      clusterRef: { name: "orders" },
      managementAuthRef,
      ...(externalUrl !== undefined && { externalUrl }),
    },
  } as unknown as Resource<"CamundaOptimize">;
}

const keycloakPlane = planeOf(
  { keycloak: {} },
  {
    managementAuthConfig: "production",
    optimize: [
      {
        namespace: "team-orders",
        name: "orders-optimize",
        externalUrl: "https://orders.example.com/optimize",
      },
    ],
  },
);
const oidcPlane = planeOf({ oidc: {} }, { managementAuthConfig: "production" });

const optimizes = [
  optimizeOf(
    "team-orders",
    "orders-optimize",
    "production",
    "https://orders.example.com/optimize",
  ),
  optimizeOf("team-lab", "no-url-optimize", "production"),
  optimizeOf(
    "team-billing",
    "other-optimize",
    "sandbox",
    "https://billing.example.com/optimize",
  ),
];

describe("OPTIMIZE_CALLBACK_PATH", () => {
  it("matches OptimizeCallbackPath of the operator", () => {
    expect(OPTIMIZE_CALLBACK_PATH).toBe(
      goConstant(
        "pkg/components/camundamanagementcluster/optimize.go",
        "OptimizeCallbackPath",
      ),
    );
  });
});

describe("optimizeRows", () => {
  it("gives each row of status.optimize with its callback in a Keycloak mode", () => {
    expect(optimizeRows({ plane: keycloakPlane, optimizes: [] })).toEqual([
      {
        namespace: "team-orders",
        name: "orders-optimize",
        externalUrl: "https://orders.example.com/optimize",
        callbackUrl:
          "https://orders.example.com/optimize/api/authentication/callback",
      },
    ]);
  });

  it("gives the readers with an external URL in OIDC mode", () => {
    expect(
      optimizeRows({ plane: oidcPlane, optimizes }).map((r) => r.name),
    ).toEqual(["orders-optimize"]);
  });

  it("gives no rows without a plane, or in OIDC mode without a contract name", () => {
    expect(optimizeRows({ plane: undefined, optimizes })).toEqual([]);
    expect(
      optimizeRows({ plane: planeOf({ oidc: {} }, {}), optimizes }),
    ).toEqual([]);
  });

  it("gives a row with no URL for an external URL that is not a string, and does not throw", () => {
    const odd = optimizeOf("team-odd", "odd-optimize", "production", 42);
    expect(optimizeRows({ plane: oidcPlane, optimizes: [odd] })).toEqual([
      { namespace: "team-odd", name: "odd-optimize" },
    ]);
    const plane = planeOf(
      { keycloak: {} },
      {
        optimize: [null, { namespace: "team-a", name: "a", externalUrl: null }],
      },
    );
    expect(optimizeRows({ plane, optimizes: [] })).toEqual([
      { namespace: "team-a", name: "a" },
    ]);
    expect(
      optimizeRows({
        plane: planeOf({ keycloak: {} }, { optimize: 3 }),
        optimizes: [],
      }),
    ).toEqual([]);
  });
});

describe("contractReaders", () => {
  it("lists every reader of a contract, with or without an external URL", () => {
    expect(
      contractReaders(optimizes, "production").map((o) => o.metadata?.name),
    ).toEqual(["no-url-optimize", "orders-optimize"]);
  });

  it("lists no reader without a contract name", () => {
    expect(contractReaders(optimizes, undefined)).toEqual([]);
  });
});

describe("isOidc", () => {
  it("is true only for a plane in OIDC mode", () => {
    expect(isOidc(oidcPlane)).toBe(true);
    expect(isOidc(keycloakPlane)).toBe(false);
    expect(isOidc(undefined)).toBe(false);
  });
});
