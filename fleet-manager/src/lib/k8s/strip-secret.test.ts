import { describe, expect, it } from "vitest";
import { stripSecret } from "./strip-secret";

describe("stripSecret", () => {
  it("keeps the name, the labels, and the key names, and no value", () => {
    const summary = stripSecret({
      apiVersion: "v1",
      kind: "Secret",
      metadata: {
        name: "git",
        namespace: "team",
        labels: { app: "fleet" },
        annotations: {
          "kubectl.kubernetes.io/last-applied-configuration":
            '{"stringData":{"token":"hunter2"}}',
        },
        managedFields: [{ manager: "kubectl" }],
      },
      type: "Opaque",
      data: { username: "YWRtaW4=" },
      stringData: { token: "hunter2" },
    });

    expect(summary).toEqual({
      apiVersion: "v1",
      kind: "Secret",
      metadata: { name: "git", namespace: "team", labels: { app: "fleet" } },
      type: "Opaque",
      keys: ["token", "username"],
    });
    expect(JSON.stringify(summary)).not.toMatch(/hunter2|YWRtaW4=/);
  });

  it("gives an empty key list to a Secret without data", () => {
    expect(
      stripSecret({ apiVersion: "v1", kind: "Secret", metadata: { name: "e" } })
        .keys,
    ).toEqual([]);
  });
});
