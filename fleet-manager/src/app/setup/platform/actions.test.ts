import { beforeEach, describe, expect, it, vi } from "vitest";
import { fakeWriter } from "../../../../test/fakes/writer";
import {
  applyPlatformConfigChange,
  previewPlatformConfigChange,
} from "./actions";

vi.mock("@/lib/write/writer", () => import("../../../../test/fakes/writer"));

const configChange = (kind = "CamundaPlatformConfig") => ({
  intent: "Change the platform config oidc-prod",
  apply: [
    {
      apiVersion: "core.camunda.io/v1",
      kind,
      metadata: { name: "oidc-prod" },
      spec: { auth: { method: "basic" } },
    },
  ],
  preconditions: { [`${kind}//oidc-prod` as const]: "21" },
  force: true,
});

const deleteWithout = {
  intent: "Delete the platform config basic-lab",
  apply: [],
  delete: [
    {
      apiVersion: "core.camunda.io/v1",
      kind: "CamundaPlatformConfig",
      name: "basic-lab",
    },
  ],
};

beforeEach(() => fakeWriter.reset());

describe("applyPlatformConfigChange", () => {
  it("writes a CamundaPlatformConfig change through the writer", async () => {
    expect(await applyPlatformConfigChange(configChange())).toEqual({
      ok: true,
      mode: "direct",
    });
    expect(fakeWriter.applied).toEqual([configChange()]);
  });

  it("refuses a change of another kind and writes nothing", async () => {
    const result = await applyPlatformConfigChange(configChange("Environment"));
    expect(!result.ok && result.errors[0].code).toBe("Invalid");
    expect(fakeWriter.applied).toEqual([]);
  });

  it("refuses a delete without a precondition", async () => {
    const result = await applyPlatformConfigChange(deleteWithout);
    expect(!result.ok && result.errors[0].code).toBe("Invalid");
    expect(fakeWriter.applied).toEqual([]);
  });
});

describe("previewPlatformConfigChange", () => {
  it("returns the dry run of the writer", async () => {
    await previewPlatformConfigChange(configChange());
    expect(fakeWriter.previewed).toEqual([configChange()]);
  });

  it("refuses a change of another kind", async () => {
    const result = await previewPlatformConfigChange(
      configChange("Environment"),
    );
    expect(result.errors[0].code).toBe("Invalid");
    expect(fakeWriter.previewed).toEqual([]);
  });
});
