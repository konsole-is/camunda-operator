import { beforeEach, describe, expect, it, vi } from "vitest";
import { fakeWriter } from "../../../../test/fakes/writer";
import {
  applyEnvironmentChange,
  applyNamespaceLabel,
  previewEnvironmentChange,
  previewNamespaceLabel,
} from "./actions";

vi.mock("@/lib/write/writer", () => import("../../../../test/fakes/writer"));

const environmentChange = (kind = "Environment") => ({
  intent: "Change the environment staging",
  apply: [
    {
      apiVersion: "fleet.camunda.io/v1alpha1",
      kind,
      metadata: { name: "staging" },
      spec: { displayName: "Staging" },
    },
  ],
  preconditions: { [`${kind}//staging` as const]: "3" },
  force: true,
});

const labelChange = (directOnly = true) => ({
  intent: "Add the namespace team-a to the environment staging",
  apply: [
    {
      apiVersion: "v1",
      kind: "Namespace",
      metadata: {
        name: "team-a",
        labels: { "fleet.camunda.io/environment": "staging" },
      },
    },
  ],
  preconditions: { "Namespace//team-a": "7" as const },
  ...(directOnly && { directOnly }),
});

const refused = {
  objects: [],
  errors: [{ code: "Invalid" as const, message: "CEL", path: "spec" }],
};

beforeEach(() => fakeWriter.reset());

describe("applyEnvironmentChange", () => {
  it("applyEnvironmentChange writes an Environment change through the writer", async () => {
    expect(await applyEnvironmentChange(environmentChange())).toEqual({
      ok: true,
      mode: "direct",
    });
    expect(fakeWriter.applied).toEqual([environmentChange()]);
  });

  it("applyEnvironmentChange refuses a change of another kind and writes nothing", async () => {
    const result = await applyEnvironmentChange(
      environmentChange("CamundaRelease"),
    );
    expect(!result.ok && result.errors[0].code).toBe("Invalid");
    expect(fakeWriter.applied).toEqual([]);
  });
});

describe("previewEnvironmentChange", () => {
  it("previewEnvironmentChange previews an Environment change through the writer", async () => {
    fakeWriter.previews = [refused];
    expect(await previewEnvironmentChange(environmentChange())).toEqual(
      refused,
    );
    expect(fakeWriter.previewed).toEqual([environmentChange()]);
  });

  it("previewEnvironmentChange refuses a change of another kind and previews nothing", async () => {
    const result = await previewEnvironmentChange(
      environmentChange("CamundaRelease"),
    );
    expect(result.errors[0].code).toBe("Invalid");
    expect(fakeWriter.previewed).toEqual([]);
  });
});

describe("applyNamespaceLabel", () => {
  it("applyNamespaceLabel writes the label change through the writer", async () => {
    fakeWriter.reset("git");
    expect(await applyNamespaceLabel(labelChange())).toEqual({
      ok: true,
      mode: "direct",
    });
    expect(fakeWriter.applied).toEqual([labelChange()]);
  });

  it("applyNamespaceLabel refuses a change without directOnly", async () => {
    const result = await applyNamespaceLabel(labelChange(false));
    expect(!result.ok && result.errors[0].code).toBe("Invalid");
    expect(fakeWriter.applied).toEqual([]);
  });
});

describe("previewNamespaceLabel", () => {
  it("previewNamespaceLabel previews the label change through the writer", async () => {
    fakeWriter.previews = [refused];
    expect(await previewNamespaceLabel(labelChange())).toEqual(refused);
    expect(fakeWriter.previewed).toEqual([labelChange()]);
  });

  it("previewNamespaceLabel refuses a change without directOnly", async () => {
    const result = await previewNamespaceLabel(labelChange(false));
    expect(result.errors[0].code).toBe("Invalid");
    expect(fakeWriter.previewed).toEqual([]);
  });
});
