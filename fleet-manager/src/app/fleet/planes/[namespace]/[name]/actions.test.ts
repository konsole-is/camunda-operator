import { beforeEach, describe, expect, it, vi } from "vitest";
import { fakeWriter } from "../../../../../../test/fakes/writer";
import {
  applyPlaneClaimChange,
  applyPlaneClaimDelete,
  previewPlaneClaimChange,
  previewPlaneClaimDelete,
} from "./actions";

vi.mock(
  "@/lib/write/writer",
  () => import("../../../../../../test/fakes/writer"),
);

const change = (kind = "CamundaManagementClusterClaim") => ({
  intent: "Suspend the management plane plane-a",
  apply: [
    {
      apiVersion: "fleet.camunda.io/v1alpha1",
      kind,
      metadata: { name: "plane-a", namespace: "platform" },
      spec: { suspend: true },
    },
  ],
  preconditions: { [`${kind}/platform/plane-a` as const]: "7" },
});

const remove = (kind = "CamundaManagementClusterClaim") => ({
  intent: "Delete the management plane plane-a",
  apply: [],
  delete: [
    {
      apiVersion: "fleet.camunda.io/v1alpha1",
      kind,
      namespace: "platform",
      name: "plane-a",
    },
  ],
  uids: { [`${kind}/platform/plane-a`]: "uid-1" },
  preconditions: { [`${kind}/platform/plane-a` as const]: "7" },
});

beforeEach(() => fakeWriter.reset());

describe("applyPlaneClaimChange", () => {
  it("applyPlaneClaimChange writes a change of a plane claim through the writer", async () => {
    expect(await applyPlaneClaimChange(change())).toEqual({
      ok: true,
      mode: "direct",
    });
    expect(fakeWriter.applied).toEqual([change()]);
  });

  it("applyPlaneClaimChange refuses a change of another kind and writes nothing", async () => {
    const result = await applyPlaneClaimChange(change("Environment"));
    expect(result.ok).toBe(false);
    expect(!result.ok && result.errors[0].code).toBe("Invalid");
    expect(fakeWriter.applied).toEqual([]);
  });
});

describe("previewPlaneClaimChange", () => {
  it("previewPlaneClaimChange previews through the writer", async () => {
    const refused = {
      objects: [],
      errors: [{ code: "Invalid" as const, message: "CEL", path: "spec" }],
    };
    fakeWriter.previews = [refused];
    expect(await previewPlaneClaimChange(change())).toEqual(refused);
    expect(fakeWriter.previewed).toEqual([change()]);
  });

  it("previewPlaneClaimChange refuses a change of another kind and previews nothing", async () => {
    const result = await previewPlaneClaimChange(change("Environment"));
    expect(result.errors[0].code).toBe("Invalid");
    expect(fakeWriter.previewed).toEqual([]);
  });
});

describe("applyPlaneClaimDelete", () => {
  it("applyPlaneClaimDelete deletes one plane claim through the writer", async () => {
    expect(await applyPlaneClaimDelete(remove())).toEqual({
      ok: true,
      mode: "direct",
    });
    expect(fakeWriter.applied).toEqual([remove()]);
  });

  it("applyPlaneClaimDelete refuses a delete of another kind and writes nothing", async () => {
    const result = await applyPlaneClaimDelete(remove("Environment"));
    expect(!result.ok && result.errors[0].code).toBe("Invalid");
    expect(fakeWriter.applied).toEqual([]);
  });

  it("applyPlaneClaimDelete refuses a change that applies an object and writes nothing", async () => {
    const result = await applyPlaneClaimDelete({
      ...remove(),
      apply: change().apply,
    });
    expect(!result.ok && result.errors[0].code).toBe("Invalid");
    expect(fakeWriter.applied).toEqual([]);
  });
});

describe("previewPlaneClaimDelete", () => {
  it("previewPlaneClaimDelete previews the delete through the writer", async () => {
    expect(await previewPlaneClaimDelete(remove())).toEqual({
      objects: [],
      errors: [],
    });
    expect(fakeWriter.previewed).toEqual([remove()]);
  });

  it("previewPlaneClaimDelete refuses a delete of another kind and previews nothing", async () => {
    const result = await previewPlaneClaimDelete(remove("Environment"));
    expect(result.errors[0].code).toBe("Invalid");
    expect(fakeWriter.previewed).toEqual([]);
  });
});
