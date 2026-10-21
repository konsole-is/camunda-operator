import { describe, expect, it } from "vitest";
import type { Environment } from "@/generated/crds/environment";
import { ENVIRONMENT_LABEL } from "@/lib/labels";
import { environmentOf } from "./environment";

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
