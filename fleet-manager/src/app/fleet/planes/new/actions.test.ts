import { beforeEach, describe, expect, it, vi } from "vitest";
import { createChangeOf } from "@/lib/write/create";
import { fakeWriter } from "../../../../../test/fakes/writer";
import { createPlane, previewPlaneCreate } from "./actions";

vi.mock("@/lib/write/writer", () => import("../../../../../test/fakes/writer"));

const claim = {
  apiVersion: "fleet.camunda.io/v1alpha1",
  kind: "CamundaManagementClusterClaim",
  metadata: { name: "plane-a", namespace: "team-new" },
  spec: { environments: ["production"] },
};
const change = createChangeOf("Create the management plane plane-a", claim, {
  name: "team-new",
  labels: { "fleet.camunda.io/environment": "production" },
});

beforeEach(() => fakeWriter.reset());

describe("createPlane", () => {
  it("createPlane writes a new namespace and the claim as one create change", async () => {
    expect(await createPlane(change)).toEqual({ ok: true, mode: "direct" });
    expect(fakeWriter.applied).toEqual([change]);
    expect(fakeWriter.applied[0].apply.map((o) => o.kind)).toEqual([
      "Namespace",
      "CamundaManagementClusterClaim",
    ]);
  });

  it("createPlane refuses a change that is not a create and writes nothing", async () => {
    const result = await createPlane({ intent: "x", apply: [claim] });
    expect(result.ok).toBe(false);
    expect(!result.ok && result.errors[0].code).toBe("Invalid");
    expect(fakeWriter.applied).toEqual([]);
  });
});

describe("previewPlaneCreate", () => {
  it("previewPlaneCreate previews through the writer", async () => {
    const refused = {
      objects: [],
      errors: [{ code: "Invalid" as const, message: "CEL", path: "spec" }],
    };
    fakeWriter.previews = [refused];
    expect(await previewPlaneCreate(change)).toEqual(refused);
    expect(fakeWriter.previewed).toEqual([change]);
  });

  it("previewPlaneCreate refuses a change that is not a create and previews nothing", async () => {
    const result = await previewPlaneCreate({ intent: "x", apply: [claim] });
    expect(result.errors[0].code).toBe("Invalid");
    expect(fakeWriter.previewed).toEqual([]);
  });
});
