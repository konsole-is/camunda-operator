import { spawnSync } from "node:child_process";
import path from "node:path";
import { describe, expect, it } from "vitest";

const MODULE = path.resolve("e2e/scenarios/shifted-clock.mts");

function run(script: string, start?: string) {
  const env = { ...process.env, FLEET_CLOCK_START: start };
  if (start === undefined) delete env.FLEET_CLOCK_START;
  return spawnSync(process.execPath, ["--import", MODULE, "-e", script], {
    env,
    encoding: "utf8",
  });
}

describe("shifted-clock", () => {
  it("starts the clock of the process at FLEET_CLOCK_START and lets it run", () => {
    const out = run(
      `const a = Date.now();
       const t = new Date().toISOString();
       const until = performance.now() + 50;
       while (performance.now() < until) {}
       console.log(JSON.stringify({ a, t, b: Date.now() }));`,
      "2026-01-03T10:00:00Z",
    );
    expect(out.stderr).toBe("");
    const { a, t, b } = JSON.parse(out.stdout);
    const start = Date.parse("2026-01-03T10:00:00Z");
    expect(a - start).toBeGreaterThanOrEqual(0);
    expect(a - start).toBeLessThan(5_000);
    expect(Date.parse(t) - start).toBeLessThan(5_000);
    expect(b - a).toBeGreaterThanOrEqual(50);
  });

  it("leaves a Date of a given time, Date.parse, and instanceof as they are", () => {
    const out = run(
      `console.log(JSON.stringify([
         new Date(0).toISOString(),
         new Date("2025-05-05T05:05:05Z").getTime(),
         Date.parse("2025-05-05T05:05:05Z"),
         Date.UTC(2025, 0, 1),
         new Date() instanceof Date,
         typeof Date(),
       ]));`,
      "2026-01-03T10:00:00Z",
    );
    expect(JSON.parse(out.stdout)).toEqual([
      "1970-01-01T00:00:00.000Z",
      Date.parse("2025-05-05T05:05:05Z"),
      Date.parse("2025-05-05T05:05:05Z"),
      Date.UTC(2025, 0, 1),
      true,
      "string",
    ]);
  });

  it("refuses to start without FLEET_CLOCK_START", () => {
    const out = run("console.log('started')");
    expect(out.status).not.toBe(0);
    expect(out.stdout).toBe("");
    expect(out.stderr).toContain("Set FLEET_CLOCK_START");
  });
});
