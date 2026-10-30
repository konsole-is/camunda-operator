import { beforeEach, describe, expect, it, vi } from "vitest";
import { fakeWriter } from "../../../../../../test/fakes/writer";
import { applyClaimChange, previewClaimChange } from "./actions";

vi.mock(
  "@/lib/write/writer",
  () => import("../../../../../../test/fakes/writer"),
);

const change = (kind = "CamundaClusterClaim") => ({
  intent: "Scale the brokers of orders",
  apply: [
    {
      apiVersion: "fleet.camunda.io/v1alpha1",
      kind,
      metadata: { name: "orders", namespace: "team-a" },
      spec: { cluster: { zeebe: { replicas: 5 } } },
    },
  ],
  preconditions: { [`${kind}/team-a/orders` as const]: "41" },
});

beforeEach(() => fakeWriter.reset());

describe("applyClaimChange", () => {
  it("applyClaimChange writes the change through the writer", async () => {
    expect(await applyClaimChange(change())).toEqual({
      ok: true,
      mode: "direct",
    });
    expect(fakeWriter.applied).toEqual([change()]);
  });

  it("applyClaimChange refuses a change of another kind and writes nothing", async () => {
    const result = await applyClaimChange(change("Environment"));
    expect(result.ok).toBe(false);
    expect(!result.ok && result.errors[0].code).toBe("Invalid");
    expect(fakeWriter.applied).toEqual([]);
  });
});

describe("previewClaimChange", () => {
  it("previewClaimChange previews through the writer", async () => {
    const refused = {
      objects: [],
      errors: [{ code: "Invalid" as const, message: "CEL", path: "spec" }],
    };
    fakeWriter.previews = [refused];
    expect(await previewClaimChange(change())).toEqual(refused);
    expect(fakeWriter.previewed).toEqual([change()]);
  });

  it("previewClaimChange refuses a change of another kind and previews nothing", async () => {
    const result = await previewClaimChange(change("Environment"));
    expect(result.errors[0].code).toBe("Invalid");
    expect(fakeWriter.previewed).toEqual([]);
  });
});
