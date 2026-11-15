import { beforeEach, describe, expect, it, vi } from "vitest";
import { fakeWriter } from "../../../../test/fakes/writer";
import { applyReleaseChange, previewReleaseChange } from "./actions";

vi.mock("@/lib/write/writer", () => import("../../../../test/fakes/writer"));

const releaseChange = (kind = "CamundaRelease") => ({
  intent: "Change the release camunda-8-9-0",
  apply: [
    {
      apiVersion: "core.camunda.io/v1",
      kind,
      metadata: { name: "camunda-8-9-0" },
      spec: { version: "8.9.1" },
    },
  ],
  preconditions: { [`${kind}//camunda-8-9-0` as const]: "3" },
  force: true,
});

const deleteWithout = {
  intent: "Delete the release camunda-8-9-1",
  apply: [],
  delete: [
    {
      apiVersion: "core.camunda.io/v1",
      kind: "CamundaRelease",
      name: "camunda-8-9-1",
    },
  ],
};

beforeEach(() => fakeWriter.reset());

describe("applyReleaseChange", () => {
  it("writes a CamundaRelease update", async () => {
    expect(await applyReleaseChange(releaseChange())).toEqual({
      ok: true,
      mode: "direct",
    });
    expect(fakeWriter.applied).toEqual([releaseChange()]);
  });

  it("refuses a change that writes another kind", async () => {
    const result = await applyReleaseChange(releaseChange("Environment"));
    expect(!result.ok && result.errors[0].code).toBe("Invalid");
    expect(fakeWriter.applied).toEqual([]);
  });

  it("refuses a delete without a precondition", async () => {
    const result = await applyReleaseChange(deleteWithout);
    expect(!result.ok && result.errors[0].code).toBe("Invalid");
    expect(fakeWriter.applied).toEqual([]);
  });
});

describe("previewReleaseChange", () => {
  it("previews a CamundaRelease update", async () => {
    await previewReleaseChange(releaseChange());
    expect(fakeWriter.previewed).toEqual([releaseChange()]);
  });

  it("refuses a change that writes another kind", async () => {
    const result = await previewReleaseChange(releaseChange("Environment"));
    expect(result.errors[0].code).toBe("Invalid");
    expect(fakeWriter.previewed).toEqual([]);
  });
});
