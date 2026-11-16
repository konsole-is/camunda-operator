import { describe, expect, it } from "vitest";
import { releaseEnvironments } from "./environments";
import { releaseFixtures } from "./fixtures";

describe("releaseEnvironments", () => {
  it("gives Default, Allowed, or Not allowed for each environment", () => {
    const { environments } = releaseFixtures();
    expect(releaseEnvironments("camunda-8-9-0", environments)).toEqual([
      { name: "production", label: "Production", state: "Default" },
      { name: "staging", label: "Staging", state: "Allowed" },
    ]);
    expect(releaseEnvironments("camunda-8-9-1", environments)).toEqual([
      { name: "production", label: "Production", state: "Not allowed" },
      { name: "staging", label: "Staging", state: "Not allowed" },
    ]);
  });

  it("gives no row without environments", () => {
    expect(releaseEnvironments("camunda-8-9-0", [])).toEqual([]);
  });
});
