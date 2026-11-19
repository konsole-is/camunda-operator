import { describe, expect, it } from "vitest";
import type { Resource } from "@/lib/k8s/kinds";
import { ENVIRONMENT_LABEL } from "@/lib/labels";
import {
  type DeletePlaneData,
  deletePlaneModel,
  KEPT_PLANE,
} from "./delete-plane-model";

// cast gives a fixture the type that the model reads.
const cast = (value: object) => value as never;

const controlledBy = (uid: string) => ({
  ownerReferences: [{ controller: true, uid }],
});

const claim = (spec: object = {}, status: object = {}) =>
  cast({
    apiVersion: "fleet.camunda.io/v1alpha1",
    kind: "CamundaManagementClusterClaim",
    metadata: { namespace: "platform", name: "plane-a", uid: "claim-uid" },
    spec: {
      environments: ["platform"],
      plugins: [
        {
          section: "management-database",
          registration: "existing-database-configs",
        },
      ],
      ...spec,
    },
    status: {
      managementCluster: { name: "plane-a" },
      requests: [
        {
          section: "management-database",
          registration: "existing-database-configs",
          name: "plane-a-management-database",
        },
      ],
      ...status,
    },
  }) as Resource<"CamundaManagementClusterClaim">;

const plane = (meta: object = controlledBy("claim-uid"), status: object = {}) =>
  cast({
    kind: "CamundaManagementCluster",
    metadata: { namespace: "platform", name: "plane-a", ...meta },
    spec: {},
    status: {
      managementAuthConfig: "plane-a",
      clusters: [
        { namespace: "team-orders", name: "orders", attached: true },
        {
          namespace: "team-pay",
          name: "pay",
          attached: false,
          reason: "NotReady",
        },
      ],
      ...status,
    },
  }) as Resource<"CamundaManagementCluster">;

const request = (name: string, uid = "claim-uid") =>
  cast({
    kind: "PluginRequest",
    metadata: { namespace: "platform", name, ...controlledBy(uid) },
    spec: { section: "management-database" },
  }) as Resource<"PluginRequest">;

function data(over: Partial<DeletePlaneData> = {}): DeletePlaneData {
  return {
    claim: claim(),
    plane: plane(),
    namespace: cast({
      metadata: {
        name: "platform",
        labels: { [ENVIRONMENT_LABEL]: "platform" },
      },
    }),
    environments: [],
    sections: [],
    registrations: [],
    requests: [request("plane-a-management-database"), request("other", "x")],
    optimizes: [
      cast({
        kind: "CamundaOptimize",
        metadata: { namespace: "team-orders", name: "orders" },
        spec: { managementAuthRef: "plane-a" },
      }),
      cast({
        kind: "CamundaOptimize",
        metadata: { namespace: "team-pay", name: "pay" },
        spec: { managementAuthRef: "other" },
      }),
    ],
    errors: [],
    ...over,
  };
}

describe("deletePlaneModel", () => {
  it("deletes the plane that the claim controls, then the requests it controls, then the claim", () => {
    expect(
      deletePlaneModel(data(), "Delete").targets.map(
        (t) => `${t.kind} ${t.name}`,
      ),
    ).toEqual([
      "CamundaManagementCluster plane-a",
      "PluginRequest plane-a-management-database",
      "CamundaManagementClusterClaim plane-a",
    ]);
  });

  it("keeps a plane of the name that the claim does not control", () => {
    const model = deletePlaneModel(data({ plane: plane({}) }), "Delete");
    expect(model.targets.map((t) => t.kind)).not.toContain(
      "CamundaManagementCluster",
    );
    expect(model.kept).toEqual([
      expect.objectContaining({ name: "plane-a", note: KEPT_PLANE }),
    ]);
    expect(model.served).toEqual([]);
    expect(model.contract).toBeUndefined();
  });

  it("names the served clusters that stay, and only the attached ones", () => {
    expect(deletePlaneModel(data(), "Delete").served).toEqual([
      { namespace: "team-orders", name: "orders" },
    ]);
  });

  it("says whether the plane that goes runs its own Keycloak", () => {
    const withProvider = (identityProvider: object) => {
      const p = plane() as { spec: object };
      p.spec = { identityProvider };
      return p as Resource<"CamundaManagementCluster">;
    };
    expect(
      deletePlaneModel(
        data({ plane: withProvider({ keycloak: {} }) }),
        "Delete",
      ).ownKeycloak,
    ).toBe(true);
    expect(
      deletePlaneModel(data({ plane: withProvider({ oidc: {} }) }), "Delete")
        .ownKeycloak,
    ).toBe(false);
    expect(
      deletePlaneModel(
        data({ plane: withProvider({ externalKeycloak: {} }) }),
        "Delete",
      ).ownKeycloak,
    ).toBe(false);
  });

  it("names the contract that goes and each CamundaOptimize that reads it", () => {
    const model = deletePlaneModel(data(), "Delete");
    expect(model.contract).toBe("plane-a");
    expect(model.readers).toEqual([
      { namespace: "team-orders", name: "orders" },
    ]);
  });

  it("states no served cluster and no contract while the page cannot read the plane", () => {
    const model = deletePlaneModel(data(), "Delete", [
      "CamundaManagementCluster",
    ]);
    expect(model.served).toBeUndefined();
    expect(model.contract).toBeUndefined();
  });

  it("states no reader of the contract while the page cannot read the CamundaOptimize objects", () => {
    const model = deletePlaneModel(data(), "Delete", ["CamundaOptimize"]);
    expect(model.contract).toBe("plane-a");
    expect(model.readers).toBeUndefined();
  });

  it("takes the plane of status as a target while the plane is gone", () => {
    expect(
      deletePlaneModel(data({ plane: undefined }), "Delete").targets[0].kind,
    ).toBe("CamundaManagementCluster");
    expect(
      deletePlaneModel(
        data({
          plane: undefined,
          claim: claim({}, { managementCluster: undefined }),
        }),
        "Delete",
      ).targets[0].kind,
    ).toBe("PluginRequest");
  });

  it("does not know the fates while a kind that decides them is unread", () => {
    expect(deletePlaneModel(data(), "Delete").fatesKnown).toBe(true);
    expect(
      deletePlaneModel(data(), "Delete", ["PluginRegistration"]).fatesKnown,
    ).toBe(false);
  });

  it("preselects the policy of the claim, else Delete, and sets the pick", () => {
    expect(deletePlaneModel(data(), "Delete").choice.defaultPolicy).toBe(
      "Delete",
    );
    expect(
      deletePlaneModel(
        data({ claim: claim({ deletionPolicy: "Orphan" }) }),
        "Orphan",
      ).choice.policy,
    ).toBe("Orphan");
  });
});

describe("the deletion choice of a plane claim whose registration does not exist", () => {
  // gone holds Orphan, recorded its policy, and selects
  // existing-database-configs, which registrations do not hold. Its request
  // holds no deletion policy yet.
  const gone = () =>
    data({
      claim: claim(
        { deletionPolicy: "Orphan" },
        { resolvedPolicy: { deletionPolicy: "Delete" } },
      ),
    });

  it("refuses Keep while a request does not hold Orphan, and names the selection", () => {
    const { choice } = deletePlaneModel(gone(), "Orphan");
    expect(choice.keepAllowed).toBe(false);
    expect(choice.missing).toEqual([
      {
        section: "management-database",
        label: "existing-database-configs (management-database)",
      },
    ]);
  });

  it("does not count the registration as missing while PluginRegistration is not read", () => {
    const { choice } = deletePlaneModel(gone(), "Orphan", [
      "PluginRegistration",
    ]);
    expect(choice.keepAllowed).toBe(true);
    expect(choice.missing).toBeUndefined();
  });
});
