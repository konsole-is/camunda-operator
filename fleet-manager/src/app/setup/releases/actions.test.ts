import { beforeEach, describe, expect, it, vi } from "vitest";
import { fakeWriter } from "../../../../test/fakes/writer";
import {
  applyReleaseChange,
  applyReleaseMove,
  previewReleaseChange,
  previewReleaseMove,
} from "./actions";

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

const clusterClaim = (namespace: string, name: string) => ({
  apiVersion: "fleet.camunda.io/v1alpha1",
  kind: "CamundaClusterClaim",
  metadata: { namespace, name },
  spec: { releaseRef: "camunda-8-9-1" },
});

const twoClaimMove = {
  intent: "Move 2 clusters to the release camunda-8-9-1",
  apply: [clusterClaim("team-a", "orders"), clusterClaim("team-b", "payments")],
  preconditions: {
    "CamundaClusterClaim/team-a/orders": "4",
    "CamundaClusterClaim/team-b/payments": "9",
  } as const,
};

const clusterMove = {
  intent: "Move the cluster team-a/orders to the release camunda-8-9-1",
  apply: [
    {
      apiVersion: "core.camunda.io/v1",
      kind: "CamundaCluster",
      metadata: { namespace: "team-a", name: "orders" },
      spec: { releaseRef: "camunda-8-9-1" },
    },
  ],
  preconditions: { "CamundaCluster/team-a/orders": "4" } as const,
};

describe("applyReleaseMove", () => {
  it("sends a move of two claims to the writer as one change", async () => {
    expect(await applyReleaseMove(twoClaimMove)).toEqual({
      ok: true,
      mode: "direct",
    });
    expect(fakeWriter.applied).toEqual([twoClaimMove]);
  });

  it("refuses a move that writes the CamundaCluster", async () => {
    const result = await applyReleaseMove(clusterMove);
    expect(!result.ok && result.errors[0].code).toBe("Invalid");
    expect(fakeWriter.applied).toEqual([]);
  });
});

describe("previewReleaseMove", () => {
  it("previews a move of two claims as one change", async () => {
    await previewReleaseMove(twoClaimMove);
    expect(fakeWriter.previewed).toEqual([twoClaimMove]);
  });

  it("refuses a move that writes the CamundaCluster", async () => {
    const result = await previewReleaseMove(clusterMove);
    expect(result.errors).toEqual([
      expect.objectContaining({ code: "Invalid" }),
    ]);
    expect(fakeWriter.previewed).toEqual([]);
  });
});
