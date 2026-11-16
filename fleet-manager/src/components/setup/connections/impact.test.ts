import { describe, expect, it } from "vitest";
import type { ContractKind, ContractUser } from "@/lib/usage";
import { saveEffects } from "./impact";

const user = (kind: string, name = "orders"): ContractUser => ({
  kind,
  namespace: "team-a",
  name,
  fields: [],
});

describe("saveEffects", () => {
  it("names each user and what it does on the change", () => {
    expect(
      saveEffects(
        "ObjectStorageConfig",
        [user("CamundaCluster"), user("DatabaseServer", "orders-pg")],
      ),
    ).toEqual([
      "CamundaCluster team-a/orders restarts its pods, all but Connectors.",
      "DatabaseServer team-a/orders-pg applies its archive again. A new bucket, base path, region, or endpoint moves the archive: a backup from before the move no longer counts for a rollback.",
    ]);
  });

  it.each<[ContractKind, string]>([
    ["ObjectStorageConfig", "CamundaCluster"],
    ["ObjectStorageConfig", "CamundaClusterClaim"],
    ["ObjectStorageConfig", "ElasticsearchCluster"],
    ["ObjectStorageConfig", "PluginRequest"],
    ["SecondaryStorageConfig", "CamundaCluster"],
    ["SecondaryStorageConfig", "CamundaClusterClaim"],
    ["DatabaseServerConfig", "DatabaseConfig"],
    ["DatabaseConfig", "SecondaryStorageConfig"],
    ["DatabaseConfig", "CamundaManagementCluster"],
    ["DatabaseConfig", "CamundaManagementClusterClaim"],
    ["ManagementAuthConfig", "CamundaOptimize"],
    ["ManagementAuthConfig", "CamundaClusterClaim"],
  ])(
    "says that a %s change restarts the pods that a %s runs or leads to",
    (kind, userKind) => {
      const [line] = saveEffects(kind, [user(userKind)]);
      expect(line).toMatch(/restart/);
    },
  );

  it("says that no resource uses an unused contract", () => {
    expect(saveEffects("DatabaseConfig", [])).toEqual([
      "No resource uses this contract.",
    ]);
  });

  it("names a user of a Management Identity contract with its namespace", () => {
    expect(
      saveEffects(
        "ManagementAuthConfig",
        [user("CamundaOptimize", "optimize")],
      ),
    ).toEqual(["CamundaOptimize team-a/optimize restarts its pods."]);
  });
});
