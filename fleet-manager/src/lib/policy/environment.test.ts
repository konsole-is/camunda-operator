import { describe, expect, it } from "vitest";
import type { Environment } from "@/generated/crds/environment";
import { ENVIRONMENT_LABEL } from "@/lib/labels";
import { environmentNameOf, environmentOf } from "./environment";

const env = (name: string) => ({ metadata: { name }, spec: {} }) as Environment;
const ns = (labels?: Record<string, string>) => ({
  metadata: { name: "team-a", labels },
});

describe("environmentOf", () => {
  it("refuses a namespace without the label while an environment exists with NoEnvironment", () => {
    expect(environmentOf(ns(), [env("dev")])).toMatchObject({
      reason: "NoEnvironment",
    });
  });

  it("refuses a label that names no environment with UnknownEnvironment", () => {
    expect(
      environmentOf(ns({ [ENVIRONMENT_LABEL]: "staging" }), [env("dev")]),
    ).toMatchObject({ reason: "UnknownEnvironment" });
  });

  it("returns the environment that the label names", () => {
    expect(
      environmentOf(ns({ [ENVIRONMENT_LABEL]: "dev" }), [
        env("prod"),
        env("dev"),
      ]).environment?.metadata.name,
    ).toBe("dev");
  });

  it("returns no environment and no reason when no environment exists and the label is absent or empty", () => {
    expect(environmentOf(ns(), [])).toEqual({});
    expect(environmentOf(ns({ [ENVIRONMENT_LABEL]: "" }), [])).toEqual({});
  });

  it("refuses an empty label while an environment exists with NoEnvironment", () => {
    expect(
      environmentOf(ns({ [ENVIRONMENT_LABEL]: "" }), [env("dev")]),
    ).toMatchObject({ reason: "NoEnvironment" });
  });
});

describe("environmentNameOf", () => {
  it("gives the name of the Environment that the label names", () => {
    expect(
      environmentNameOf(ns({ [ENVIRONMENT_LABEL]: "dev" }), [env("dev")]),
    ).toBe("dev");
  });

  it("gives no name for no label, an empty label, or a label that names no Environment", () => {
    for (const labels of [
      undefined,
      { [ENVIRONMENT_LABEL]: "" },
      { [ENVIRONMENT_LABEL]: "ghost" },
    ]) {
      expect(environmentNameOf(ns(labels), [env("dev")])).toBeUndefined();
    }
  });
});
