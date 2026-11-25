import { spawnSync } from "node:child_process";
import path from "node:path";
import { describe, expect, it } from "vitest";

const MODULE = path.resolve("e2e/scenarios/fixed-clock.mts");

function run(script: string, fixed?: string) {
  const env = { ...process.env, FLEET_FIXED_NOW: fixed };
  if (fixed === undefined) delete env.FLEET_FIXED_NOW;
  return spawnSync(process.execPath, ["--import", MODULE, "-e", script], {
    env,
    encoding: "utf8",
  });
}

describe("fixed-clock", () => {
  it("reads FLEET_FIXED_NOW as the time, also after time passed", () => {
    const out = run(
      `const a = Date.now();
       const until = performance.now() + 50;
       while (performance.now() < until) {}
       console.log(JSON.stringify([a, Date.now(), new Date().toISOString(), Date()]));`,
      "2026-01-03T10:00:00Z",
    );
    expect(out.stderr).toBe("");
    const start = Date.parse("2026-01-03T10:00:00Z");
    expect(JSON.parse(out.stdout)).toEqual([
      start,
      start,
      "2026-01-03T10:00:00.000Z",
      new Date(start).toString(),
    ]);
  });

  it("leaves timers on their real durations", () => {
    const out = run(
      `const t = performance.now();
       setTimeout(() => console.log(performance.now() - t >= 40), 50);`,
      "2026-01-03T10:00:00Z",
    );
    expect(out.stdout.trim()).toBe("true");
  });

  it("leaves a Date of a given time, Date.parse, and instanceof as they are", () => {
    const out = run(
      `console.log(JSON.stringify([
         new Date(0).toISOString(),
         new Date("2025-05-05T05:05:05Z").getTime(),
         Date.parse("2025-05-05T05:05:05Z"),
         Date.UTC(2025, 0, 1),
         new Date() instanceof Date,
       ]));`,
      "2026-01-03T10:00:00Z",
    );
    expect(JSON.parse(out.stdout)).toEqual([
      "1970-01-01T00:00:00.000Z",
      Date.parse("2025-05-05T05:05:05Z"),
      Date.parse("2025-05-05T05:05:05Z"),
      Date.UTC(2025, 0, 1),
      true,
    ]);
  });

  it("refuses to start without FLEET_FIXED_NOW", () => {
    const out = run("console.log('started')");
    expect(out.status).not.toBe(0);
    expect(out.stdout).toBe("");
    expect(out.stderr).toContain("Set FLEET_FIXED_NOW");
  });
});
