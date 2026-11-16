import { describe, expect, expectTypeOf, it } from "vitest";
import {
  type CoreCamundaClusterConditionType,
  coreCamundaClusterConditionTypes,
  coreCamundaManagementClusterConditionTypes,
  coreCamundaOptimizeConditionTypes,
  fleetCamundaClusterClaimConditionTypes,
  fleetCamundaManagementClusterClaimConditionTypes,
} from "@/generated/conditions";
import {
  CAMUNDA_CLUSTER_PLACEMENT,
  CAMUNDA_OPTIMIZE_PLACEMENT,
  CLAIM_PLACEMENT,
  MANAGEMENT_CLUSTER_PLACEMENT,
  PLANE_CLAIM_PLACEMENT,
  type PlacementMap,
  conditionsAt,
  placementOf,
} from ".";

describe("placement maps", () => {
  it.each([
    [
      "CamundaCluster",
      CAMUNDA_CLUSTER_PLACEMENT,
      coreCamundaClusterConditionTypes,
    ],
    [
      "CamundaOptimize",
      CAMUNDA_OPTIMIZE_PLACEMENT,
      coreCamundaOptimizeConditionTypes,
    ],
    [
      "CamundaClusterClaim",
      CLAIM_PLACEMENT,
      fleetCamundaClusterClaimConditionTypes,
    ],
    [
      "CamundaManagementCluster",
      MANAGEMENT_CLUSTER_PLACEMENT,
      coreCamundaManagementClusterConditionTypes,
    ],
    [
      "CamundaManagementClusterClaim",
      PLANE_CLAIM_PLACEMENT,
      fleetCamundaManagementClusterClaimConditionTypes,
    ],
  ] as const)(
    "the %s map places exactly the generated types",
    (_, map, types) => {
      expect(Object.keys(map).sort()).toEqual([...types].sort());
    },
  );

  it("a map without a generated type does not compile", () => {
    const withoutZeebe: Omit<
      PlacementMap<CoreCamundaClusterConditionType>,
      "ZeebeReady"
    > = CAMUNDA_CLUSTER_PLACEMENT;
    // @ts-expect-error ZeebeReady has no placement.
    const missing: PlacementMap<CoreCamundaClusterConditionType> = withoutZeebe;
    expect(missing).toBeDefined();
    expectTypeOf(CAMUNDA_CLUSTER_PLACEMENT).toEqualTypeOf<
      PlacementMap<CoreCamundaClusterConditionType>
    >();
  });

  it("puts each condition where the issue says", () => {
    expect(CAMUNDA_CLUSTER_PLACEMENT).toEqual({
      Ready: "header",
      ZeebeReady: "zeebe",
      GatewayReady: "gateway",
      OperateReady: "operate",
      TasklistReady: "tasklist",
      AdminReady: "admin",
      ConnectorsReady: "connectors",
      AdminSecretReady: "resource",
      MirroredSecretsReady: "resource",
    });
    expect(CAMUNDA_OPTIMIZE_PLACEMENT).toEqual({
      Ready: "optimize",
      WebappReady: "optimize",
      ImporterReady: "optimize",
      MirroredSecretsReady: "resource",
    });
    expect(CLAIM_PLACEMENT).toEqual({
      Ready: "header",
      RequestsReady: "claim",
      ClusterReady: "claim",
      AttachmentsReady: "claim",
    });
    expect(MANAGEMENT_CLUSTER_PLACEMENT).toEqual({
      Ready: "header",
      KeycloakReady: "keycloak",
      IdentityReady: "identity",
      ConsoleReady: "console",
      WebModelerReady: "webModeler",
      SecretsReady: "resource",
      MirroredSecretsReady: "resource",
      ServiceAccountsReady: "resource",
      ManagementAuthReady: "resource",
      OptimizeCallbacksReady: "resource",
    });
    expect(PLANE_CLAIM_PLACEMENT).toEqual({
      Ready: "header",
      RequestsReady: "claim",
      ManagementClusterReady: "claim",
    });
  });

  it("the Conditions section of a plane never holds the condition of a component", () => {
    const rows = coreCamundaManagementClusterConditionTypes.map((type) => ({
      type,
    }));
    expect(
      conditionsAt(MANAGEMENT_CLUSTER_PLACEMENT, rows, "resource")
        .map((r) => r.type)
        .sort(),
    ).toEqual([
      "ManagementAuthReady",
      "MirroredSecretsReady",
      "OptimizeCallbacksReady",
      "SecretsReady",
      "ServiceAccountsReady",
    ]);
    expect(
      conditionsAt(MANAGEMENT_CLUSTER_PLACEMENT, rows, "identity"),
    ).toEqual([{ type: "IdentityReady" }]);
  });
});

describe("conditionsAt", () => {
  const conditions = [
    { type: "Ready" },
    { type: "AdminSecretReady" },
    { type: "ZeebeReady" },
    { type: "MirroredSecretsReady" },
    { type: "FutureReady" },
  ];

  it("keeps the conditions of one place in their order", () => {
    expect(
      conditionsAt(CAMUNDA_CLUSTER_PLACEMENT, conditions, "resource").map(
        (c) => c.type,
      ),
    ).toEqual(["AdminSecretReady", "MirroredSecretsReady", "FutureReady"]);
    expect(
      conditionsAt(CAMUNDA_CLUSTER_PLACEMENT, conditions, "zeebe"),
    ).toEqual([{ type: "ZeebeReady" }]);
  });

  it("puts a type that the map does not know in resource", () => {
    expect(placementOf(CAMUNDA_CLUSTER_PLACEMENT, "FutureReady")).toBe(
      "resource",
    );
    expect(placementOf(CAMUNDA_CLUSTER_PLACEMENT, "constructor")).toBe(
      "resource",
    );
  });

  it("gives no conditions for an object without status", () => {
    expect(
      conditionsAt(CAMUNDA_CLUSTER_PLACEMENT, undefined, "resource"),
    ).toEqual([]);
  });
});
