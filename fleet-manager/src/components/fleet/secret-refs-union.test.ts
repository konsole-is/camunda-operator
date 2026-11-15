import { describe, expect, it } from "vitest";
import { secretRefsUnion, withSecretRefs } from "./secret-refs-union";

const byNamespace = {
  "team-b": [{ name: "db", keys: ["user", "password"] }],
  "team-a": [
    { name: "license", keys: ["key"] },
    { name: "db", keys: ["password"] },
  ],
};

describe("secretRefsUnion", () => {
  it("lists each Secret of any namespace with the union of its keys, sorted", () => {
    expect(secretRefsUnion(byNamespace).refs).toEqual([
      { name: "db", keys: ["password", "user"] },
      { name: "license", keys: ["key"] },
    ]);
  });

  it("names each namespace that lacks the Secret or the key", () => {
    const { missingIn } = secretRefsUnion(byNamespace);
    expect(missingIn("db", "user")).toEqual(["team-a"]);
    expect(missingIn("license", "key")).toEqual(["team-b"]);
    expect(missingIn("db", "password")).toEqual([]);
    expect(missingIn("gone", "x")).toEqual(["team-a", "team-b"]);
  });

  it("has no refs and no missing namespace without namespaces", () => {
    const union = secretRefsUnion({});
    expect(union.refs).toEqual([]);
    expect(union.missingIn("db", "password")).toEqual([]);
  });
});

describe("withSecretRefs", () => {
  it("adds the names and keys of more to refs", () => {
    expect(
      withSecretRefs(
        [{ name: "db", keys: ["password"] }],
        [
          { name: "gone", keys: ["x"] },
          { name: "db", keys: ["user", "password"] },
        ],
      ),
    ).toEqual([
      { name: "db", keys: ["password", "user"] },
      { name: "gone", keys: ["x"] },
    ]);
  });
});
