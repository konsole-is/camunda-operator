import { describe, expect, it } from "vitest";
import { deleteChangeOf } from "./delete";

describe("deleteChangeOf", () => {
  it("deletes each object with the resourceVersion and the UID that the page read", () => {
    expect(
      deleteChangeOf(
        [
          {
            apiVersion: "core.camunda.io/v1",
            kind: "CamundaRelease",
            metadata: { name: "a", resourceVersion: "7", uid: "uid-a" },
          },
          {
            apiVersion: "core.camunda.io/v1",
            kind: "DatabaseConfig",
            metadata: {
              name: "b",
              namespace: "team-a",
              resourceVersion: "8",
              uid: "uid-b",
            },
          },
        ],
        "Delete two objects",
      ),
    ).toEqual({
      intent: "Delete two objects",
      apply: [],
      delete: [
        { apiVersion: "core.camunda.io/v1", kind: "CamundaRelease", name: "a" },
        {
          apiVersion: "core.camunda.io/v1",
          kind: "DatabaseConfig",
          namespace: "team-a",
          name: "b",
        },
      ],
      preconditions: {
        "CamundaRelease//a": "7",
        "DatabaseConfig/team-a/b": "8",
      },
      uids: {
        "CamundaRelease//a": "uid-a",
        "DatabaseConfig/team-a/b": "uid-b",
      },
    });
  });

  it("leaves out preconditions and uids when no object has them", () => {
    const change = deleteChangeOf(
      [{ apiVersion: "v1", kind: "Environment", metadata: { name: "a" } }],
      "Delete the environment a",
    );
    expect(change).not.toHaveProperty("preconditions");
    expect(change).not.toHaveProperty("uids");
  });
});
