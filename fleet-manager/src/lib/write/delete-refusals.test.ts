import { beforeEach, describe, expect, it, vi } from "vitest";
import { type DeleteRule, deleteRefusals } from "./delete-refusals";
import { adminChangeSchema } from "./run-change";
import type { Change } from "./types";

const cache = vi.hoisted(() => ({
  read: [] as string[],
  failing: [] as string[],
}));
vi.mock("@/lib/k8s/read", () => ({
  list: async (kind: string) => {
    cache.read.push(kind);
    if (cache.failing.includes(kind)) throw new Error(`${kind} is forbidden`);
    return [{ kind, metadata: { name: "a" } }];
  },
}));

beforeEach(() => {
  cache.read = [];
  cache.failing = [];
});

const schema = adminChangeSchema(["CamundaRelease"]);

const KINDS = {
  clusters: "CamundaCluster",
  environments: "Environment",
  namespaces: "Namespace",
} as const;

// remove deletes the releases named, with a precondition for each.
function remove(...names: string[]): Change {
  return {
    intent: "Delete releases",
    apply: [],
    delete: names.map((name) => ({
      apiVersion: "core.camunda.io/v1",
      kind: "CamundaRelease",
      name,
    })),
    preconditions: Object.fromEntries(
      names.map((name) => [`CamundaRelease//${name}`, "1"]),
    ),
  };
}

function blocking(...names: string[]): DeleteRule<typeof KINDS> {
  return {
    kinds: KINDS,
    needs: () => ["CamundaCluster", "Environment"],
    blockOf: () => (ref) =>
      names.includes(ref.name)
        ? { reason: "In use.", users: ["CamundaCluster a/b"] }
        : undefined,
  };
}

describe("deleteRefusals", () => {
  it("refuses each delete that the rule blocks, with the reason and the users", async () => {
    expect(
      await deleteRefusals(remove("old", "used"), schema, blocking("used")),
    ).toEqual([
      {
        code: "Forbidden",
        message: "In use. Used by CamundaCluster a/b.",
        ref: {
          apiVersion: "core.camunda.io/v1",
          kind: "CamundaRelease",
          name: "used",
        },
      },
    ]);
  });

  it("reads only the kinds that the rule needs and gives an empty list for each other kind", async () => {
    let seen: unknown;
    await deleteRefusals(remove("old"), schema, {
      ...blocking(),
      blockOf: (lists) => {
        seen = lists;
        return () => undefined;
      },
    });
    expect(cache.read.sort()).toEqual(["CamundaCluster", "Environment"]);
    expect(seen).toEqual({
      clusters: [{ kind: "CamundaCluster", metadata: { name: "a" } }],
      environments: [{ kind: "Environment", metadata: { name: "a" } }],
      namespaces: [],
    });
  });

  it("gives the rule each needed kind that could not be read", async () => {
    cache.failing = ["Environment", "Namespace"];
    let unread: readonly string[] = [];
    await deleteRefusals(remove("old"), schema, {
      ...blocking(),
      blockOf: (_, failed) => {
        unread = failed;
        return () => undefined;
      },
    });
    expect(unread).toEqual(["Environment"]);
  });

  it("refuses each delete when the rule throws", async () => {
    const errors = await deleteRefusals(remove("old", "new"), schema, {
      ...blocking(),
      blockOf: () => () => {
        throw new TypeError("spec is undefined");
      },
    });
    expect(errors.map((e) => [e.code, e.message])).toEqual([
      [
        "Forbidden",
        "Could not check the users of CamundaRelease old: spec is undefined",
      ],
      [
        "Forbidden",
        "Could not check the users of CamundaRelease new: spec is undefined",
      ],
    ]);
  });

  it("reads nothing and refuses nothing for a change without a delete", async () => {
    const change: Change = { intent: "Nothing", apply: [] };
    expect(await deleteRefusals(change, schema, blocking("old"))).toEqual([]);
    expect(cache.read).toEqual([]);
  });

  it("refuses nothing for a change that the schema refuses", async () => {
    const change = { ...remove("old"), preconditions: undefined };
    expect(await deleteRefusals(change, schema, blocking("old"))).toEqual([]);
    expect(cache.read).toEqual([]);
  });
});
