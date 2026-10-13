import { readFileSync } from "node:fs";
import path from "node:path";
import { describe, expect, it } from "vitest";

const app = path.resolve(import.meta.dirname, "../..");
const dockerfile = readFileSync(path.join(app, "Dockerfile"), "utf8");
const nodeVersion = readFileSync(
  path.join(app, ".node-version"),
  "utf8",
).trim();
const nodeMajor = nodeVersion.split(".")[0];

describe("fleet-manager/Dockerfile", () => {
  it("builds with the Node.js release of .node-version", () => {
    expect(dockerfile).toMatch(
      new RegExp(
        `^FROM node:${nodeVersion.replaceAll(".", "\\.")}-\\S+ AS build$`,
        "m",
      ),
    );
  });

  it("runs on the distroless Node.js image of the same major, as non-root", () => {
    expect(dockerfile).toMatch(
      new RegExp(
        `^FROM gcr\\.io/distroless/nodejs${nodeMajor}-debian13:nonroot$`,
        "m",
      ),
    );
    expect(dockerfile).toMatch(/^USER 65532:65532$/m);
  });

  it("installs the pnpm of packageManager with npm, not corepack", () => {
    expect(dockerfile).toMatch(/npm install --global .*packageManager/);
    expect(dockerfile).toContain("pnpm install --frozen-lockfile");
    expect(dockerfile.toLowerCase()).not.toContain("corepack");
  });

  it("starts the standalone server", () => {
    expect(dockerfile).toContain(".next/standalone");
    expect(dockerfile).toMatch(/^CMD \["server\.js"\]$/m);
  });

  it("ships the images table and the catalog", () => {
    expect(dockerfile).toContain(
      "COPY pkg/images/table.json /app/data/images-table.json",
    );
    expect(dockerfile).toContain(
      "COPY fleet-manager/catalog/official-plugins.json /app/data/official-plugins.json",
    );
  });
});
