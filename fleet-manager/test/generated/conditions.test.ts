import { describe, expect, it } from "vitest";
import { coreCamundaClusterConditionTypes } from "@/generated/conditions";
import { typeCheck } from "./typecheck";

describe("conditions", { timeout: 60_000 }, () => {
  it("gives CamundaCluster its own and the shared core condition types", () => {
    expect(coreCamundaClusterConditionTypes).toEqual(
      expect.arrayContaining([
        "Ready",
        "MirroredSecretsReady",
        "ZeebeReady",
        "AdminSecretReady",
      ]),
    );
  });

  it("fails the build of a placement map that leaves out a condition type", () => {
    const name = "test/generated/__virtual__/placement.ts";
    const result = typeCheck({
      [name]: `import type { CoreCamundaManagementClusterConditionType } from "../../../src/generated/conditions";\nexport const placement: Record<CoreCamundaManagementClusterConditionType, string> = { Ready: "resource" };\n`,
    });

    expect(result[name].join("\n")).toContain("KeycloakReady");
  });
});
