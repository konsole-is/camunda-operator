import { describe, expect, it } from "vitest";
import { ENV_SCOPES, envOf, namedSecretRefs, withEnv } from "./env";

describe("ENV_SCOPES", () => {
  it("holds the top level and each component block, and no Optimize scope", () => {
    expect(ENV_SCOPES.map((s) => s.scope)).toEqual([
      "release",
      "connectors",
      "zeebe",
      "gateway",
      "operate",
      "tasklist",
      "admin",
    ]);
  });
});

describe("envOf and withEnv", () => {
  it("reads and writes the top level and each component scope", () => {
    const spec = { version: "8.9.0", connectors: { version: "8.9.7" } };
    const top = withEnv(spec, "release", {
      extraEnv: [{ name: "A", value: "1" }],
    });
    expect(top.extraEnv).toEqual([{ name: "A", value: "1" }]);
    expect(envOf(top, "release")).toEqual({
      extraEnv: [{ name: "A", value: "1" }],
    });

    const zeebe = withEnv(top, "zeebe", {
      extraEnvFrom: [{ configMapRef: { name: "tuning" } }],
    });
    expect(zeebe.zeebe).toEqual({
      extraEnvFrom: [{ configMapRef: { name: "tuning" } }],
    });
    expect(envOf(zeebe, "zeebe").extraEnvFrom).toEqual([
      { configMapRef: { name: "tuning" } },
    ]);
    expect(zeebe.extraEnv).toEqual([{ name: "A", value: "1" }]);
  });

  it("keeps the other fields of a scope block", () => {
    const spec = { version: "8.9.0", connectors: { version: "8.9.7" } };
    expect(
      withEnv(spec, "connectors", { extraEnv: [{ name: "B", value: "2" }] })
        .connectors,
    ).toEqual({ version: "8.9.7", extraEnv: [{ name: "B", value: "2" }] });
  });

  it("removes a scope block that ends up empty", () => {
    const spec = {
      version: "8.9.0",
      zeebe: { extraEnv: [{ name: "A", value: "1" }] },
    };
    expect(withEnv(spec, "zeebe", {})).toEqual({ version: "8.9.0" });
    expect(withEnv(spec, "zeebe", { extraEnv: [] })).toEqual({
      version: "8.9.0",
    });
  });
});

describe("namedSecretRefs", () => {
  it("lists every Secret key that the release names, in every scope", () => {
    expect(
      namedSecretRefs({
        version: "8.9.0",
        extraEnv: [
          {
            name: "A",
            valueFrom: { secretKeyRef: { name: "db", key: "user" } },
          },
          { name: "B", value: "plain" },
        ],
        operate: {
          extraEnv: [
            {
              name: "C",
              valueFrom: { secretKeyRef: { name: "db", key: "password" } },
            },
          ],
        },
        tasklist: {
          extraEnv: [
            {
              name: "D",
              valueFrom: { secretKeyRef: { name: "license", key: "key" } },
            },
          ],
        },
      }),
    ).toEqual([
      { name: "db", keys: ["password", "user"] },
      { name: "license", keys: ["key"] },
    ]);
  });
});
