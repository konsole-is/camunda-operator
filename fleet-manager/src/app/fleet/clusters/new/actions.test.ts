import { beforeEach, describe, expect, it, vi } from "vitest";
import { createChangeOf } from "@/lib/write/create";
import { fakeWriter } from "../../../../../test/fakes/writer";
import { createCluster, previewClusterCreate } from "./actions";

vi.mock("@/lib/write/writer", () => import("../../../../../test/fakes/writer"));

const claim = {
  apiVersion: "fleet.camunda.io/v1alpha1",
  kind: "CamundaClusterClaim",
  metadata: { name: "orders", namespace: "team-new" },
  spec: { presetRef: "small" },
};
const change = createChangeOf("Create the cluster orders in team-new", claim, {
  name: "team-new",
  labels: { "fleet.camunda.io/environment": "prod" },
});

beforeEach(() => fakeWriter.reset());

describe("createCluster", () => {
  it("writes a new namespace and the claim as one create change", async () => {
    expect(await createCluster(change)).toEqual({ ok: true, mode: "direct" });
    expect(fakeWriter.applied).toEqual([change]);
  });

  it("refuses a change that is not a create and writes nothing", async () => {
    const result = await createCluster({ intent: "x", apply: [claim] });
    expect(result.ok).toBe(false);
    expect(!result.ok && result.errors[0].code).toBe("Invalid");
    expect(fakeWriter.applied).toEqual([]);
  });
});

describe("previewClusterCreate", () => {
  it("previews through the writer", async () => {
    const refused = {
      objects: [],
      errors: [{ code: "Invalid" as const, message: "CEL", path: "spec" }],
    };
    fakeWriter.previews = [refused];
    expect(await previewClusterCreate(change)).toEqual(refused);
    expect(fakeWriter.previewed).toEqual([change]);
  });

  it("refuses a change that is not a create and previews nothing", async () => {
    const result = await previewClusterCreate({ intent: "x", apply: [claim] });
    expect(result.errors[0].code).toBe("Invalid");
    expect(fakeWriter.previewed).toEqual([]);
  });
});
